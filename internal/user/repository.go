package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type repository struct {
	query *dbgen.Queries
}

func NewRepository(q *dbgen.Queries) *repository {
	return &repository{query: q}
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

func (r *repository) UpdateAvatar(ctx context.Context, userID, uploadID uuid.UUID) error {
	if err := r.query.UpdateUserAvatar(ctx, dbgen.UpdateUserAvatarParams{ID: userID, AvatarUploadID: pgtype.UUID{Bytes: uploadID, Valid: true}}); err != nil {
		return fmt.Errorf("update_avatar: %w", err)
	}
	return nil
}

func (r *repository) UpdateBanner(ctx context.Context, userID, uploadID uuid.UUID) error {
	if err := r.query.UpdateUserBanner(ctx, dbgen.UpdateUserBannerParams{ID: userID, BannerUploadID: pgtype.UUID{Bytes: uploadID, Valid: true}}); err != nil {
		return fmt.Errorf("update_banner: %w", err)
	}
	return nil
}

func (r *repository) FindCompletedUploadByOwner(ctx context.Context, uploadID, userID uuid.UUID) error {
	_, err := r.query.FindCompletedUploadByOwner(ctx, dbgen.FindCompletedUploadByOwnerParams{ID: uploadID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUploadNotFound
		}
		return fmt.Errorf("find_completed_upload: %w", err)
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
