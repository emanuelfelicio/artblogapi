package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	query *dbgen.Queries
	pool  *pgxpool.Pool
}

func NewRepository(q *dbgen.Queries, pool *pgxpool.Pool) *repository {
	return &repository{query: q, pool: pool}
}

func (r *repository) FindByUsername(ctx context.Context, username string) (User, error) {
	row, err := r.query.GetPublicUserProfileByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("find_by_username: %w", err)
	}

	return mapPublicProfile(row), nil
}

func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.query.GetMyUserProfileByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("find_by_id: %w", err)
	}

	return mapMyProfile(row), nil
}

func (r *repository) UpdateProfile(ctx context.Context, id uuid.UUID, displayName, bio *string) (User, error) {
	if _, err := r.query.UpdateUserProfile(ctx, dbgen.UpdateUserProfileParams{
		ID:          id,
		DisplayName: textParam(displayName),
		Bio:         textParam(bio),
	}); err != nil {
		return User{}, fmt.Errorf("update_profile: %w", err)
	}

	// re-fetches to return the current DB state (including DB-computed fields)
	return r.FindByID(ctx, id)
}

func (r *repository) WithTransaction(ctx context.Context, fn func(repo Repository) error) error {
	if r.pool == nil {
		return errors.New("with_transaction: database connection pool is nil")
	}

	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		txRepo := &repository{
			query: r.query.WithTx(tx),
			pool:  r.pool,
		}
		return fn(txRepo)
	})
}

func (r *repository) FindUploadByIDForUpdate(ctx context.Context, uploadID uuid.UUID) (UserUpload, error) {
	row, err := r.query.GetUploadForUpdate(ctx, uploadID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserUpload{}, ErrUploadNotFound
		}
		return UserUpload{}, fmt.Errorf("find_upload_by_id_for_update: %w", err)
	}
	return UserUpload{
		ID:        row.ID,
		UserID:    row.UserID,
		Status:    string(row.Status),
		Purpose:   string(row.Purpose),
		ObjectKey: row.ObjectKey,
	}, nil
}

func (r *repository) FindUserByIDForUpdate(ctx context.Context, userID uuid.UUID) (User, error) {
	row, err := r.query.GetUserForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("find_user_by_id_for_update: %w", err)
	}
	u := User{ID: row.ID}
	if row.AvatarUploadID.Valid {
		id := uuid.UUID(row.AvatarUploadID.Bytes)
		u.AvatarUploadID = &id
	}
	if row.BannerUploadID.Valid {
		id := uuid.UUID(row.BannerUploadID.Bytes)
		u.BannerUploadID = &id
	}
	return u, nil
}

func (r *repository) UpdateUploadStatus(ctx context.Context, uploadID uuid.UUID, status string) error {
	if err := r.query.UpdateUploadStatus(ctx, dbgen.UpdateUploadStatusParams{
		ID:     uploadID,
		Status: dbgen.UploadStatus(status),
	}); err != nil {
		return fmt.Errorf("update_upload_status: %w", err)
	}
	return nil
}

func (r *repository) UpdateAvatar(ctx context.Context, userID, uploadID uuid.UUID) error {
	if err := r.query.UpdateUserAvatar(ctx, dbgen.UpdateUserAvatarParams{
		ID:             userID,
		AvatarUploadID: pgtype.UUID{Bytes: uploadID, Valid: true},
	}); err != nil {
		return fmt.Errorf("update_avatar: %w", err)
	}
	return nil
}

func (r *repository) UpdateBanner(ctx context.Context, userID, uploadID uuid.UUID) error {
	if err := r.query.UpdateUserBanner(ctx, dbgen.UpdateUserBannerParams{
		ID:             userID,
		BannerUploadID: pgtype.UUID{Bytes: uploadID, Valid: true},
	}); err != nil {
		return fmt.Errorf("update_banner: %w", err)
	}
	return nil
}

func mapPublicProfile(row dbgen.GetPublicUserProfileByUsernameRow) User {
	u := User{
		ID:        row.ID,
		Username:  row.Username,
		Email:     row.Email,
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
	if row.DisplayName.Valid {
		u.DisplayName = row.DisplayName.String
	}
	if row.Bio.Valid {
		u.Bio = row.Bio.String
	}
	if row.AvatarKey.Valid {
		value := row.AvatarKey.String
		u.AvatarKey = &value
	}
	if row.BannerKey.Valid {
		value := row.BannerKey.String
		u.BannerKey = &value
	}
	return u
}

func mapMyProfile(row dbgen.GetMyUserProfileByIDRow) User {
	u := User{
		ID:        row.ID,
		Username:  row.Username,
		Email:     row.Email,
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
	if row.DisplayName.Valid {
		u.DisplayName = row.DisplayName.String
	}
	if row.Bio.Valid {
		u.Bio = row.Bio.String
	}
	if row.AvatarKey.Valid {
		value := row.AvatarKey.String
		u.AvatarKey = &value
	}
	if row.BannerKey.Valid {
		value := row.BannerKey.String
		u.BannerKey = &value
	}
	return u
}

// textParam converts an optional string to pgtype.Text.
// A nil pointer produces Valid=false, which maps to NULL in Postgres.
func textParam(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}
