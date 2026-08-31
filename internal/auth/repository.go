package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	query  *dbgen.Queries
	logger *slog.Logger
	pool   *pgxpool.Pool
}

func NewRepository(q *dbgen.Queries, logger *slog.Logger, pool *pgxpool.Pool) *repository {
	return &repository{query: q, logger: logger, pool: pool}
}

func (r *repository) WithTransaction(ctx context.Context, fn func(repo Repository) error) error {
	if r.pool == nil {
		return errors.New("with_transaction: database connection pool is nil")
	}

	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		txRepo := &repository{
			query:  r.query.WithTx(tx),
			logger: r.logger,
			pool:   r.pool,
		}
		return fn(txRepo)
	})
}

func (r *repository) CreateUser(ctx context.Context, user User) (User, error) {
	userParam := dbgen.CreateUserParams{
		ID:           user.ID,
		Username:     user.Username,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
	}
	dbUser, err := r.query.CreateUser(ctx, userParam)
	if err != nil {
		// Map UNIQUE constraint violation to domain sentinel errors
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
			detail := strings.ToLower(pgErr.Detail)
			if strings.Contains(detail, "email") {
				return User{}, ErrEmailAlreadyExists
			}
			if strings.Contains(detail, "username") {
				return User{}, ErrUsernameAlreadyExists
			}
		}

		return User{}, fmt.Errorf("create_user %w", err)
	}

	return User{
		ID:       dbUser.ID,
		Username: dbUser.Username,
		Email:    dbUser.Email,
		IsActive: dbUser.IsActive,
	}, nil
}

func (r *repository) CheckEmailAndUsername(ctx context.Context, username, email string) error {
	row, err := r.query.CheckEmailUsername(ctx, dbgen.CheckEmailUsernameParams{Email: email, Username: username})

	if err != nil {
		return fmt.Errorf("check_email_username %w", err)
	}

	var errs []error
	if row.EmailExists {
		errs = append(errs, ErrEmailAlreadyExists)
	}
	if row.UsernameExists {
		errs = append(errs, ErrUsernameAlreadyExists)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (r *repository) FindUserByCredential(ctx context.Context, credential string) (User, error) {
	dbUser, err := r.query.FindUserByCredential(ctx, credential)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrInvalidCredentials
		}

		return User{}, fmt.Errorf("find_user_by_credential %w", err)
	}

	return User{
		ID:           dbUser.ID,
		Username:     dbUser.Username,
		Email:        dbUser.Email,
		PasswordHash: dbUser.PasswordHash,
		IsActive:     dbUser.IsActive,
	}, nil
}

func (r *repository) SaveSession(ctx context.Context, s Session) error {
	arg := dbgen.CreateSessionParams{
		ID:        s.ID,
		UserID:    s.UserID,
		ExpiresAt: pgtype.Timestamptz{Time: s.ExpiresAt, Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: s.CreatedAt, Valid: true},
		Revoked:   s.Revoked,
		UserAgent: pgtype.Text{String: s.UserAgent, Valid: s.UserAgent != ""},
		Ip:        pgtype.Text{String: s.IP, Valid: s.IP != ""},
		DeviceID:  pgtype.Text{String: s.DeviceID, Valid: s.DeviceID != ""},
	}

	err := r.query.CreateSession(ctx, arg)
	if err != nil {
		return fmt.Errorf("save_session: %w", err)
	}

	return nil
}

func (r *repository) FindSessionByID(ctx context.Context, id string) (Session, error) {
	dbSess, err := r.query.FindSessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, fmt.Errorf("find_session_by_id: %w", err)
	}

	return Session{
		ID:        dbSess.ID,
		UserID:    dbSess.UserID,
		ExpiresAt: dbSess.ExpiresAt.Time,
		CreatedAt: dbSess.CreatedAt.Time,
		Revoked:   dbSess.Revoked,
		UserAgent: dbSess.UserAgent.String,
		IP:        dbSess.Ip.String,
		DeviceID:  dbSess.DeviceID.String,
	}, nil
}

func (r *repository) FindSessionByIDForUpdate(ctx context.Context, id string) (Session, error) {
	dbSess, err := r.query.FindSessionByIDForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, fmt.Errorf("find_session_by_id_for_update: %w", err)
	}

	return Session{
		ID:        dbSess.ID,
		UserID:    dbSess.UserID,
		ExpiresAt: dbSess.ExpiresAt.Time,
		CreatedAt: dbSess.CreatedAt.Time,
		Revoked:   dbSess.Revoked,
		UserAgent: dbSess.UserAgent.String,
		IP:        dbSess.Ip.String,
		DeviceID:  dbSess.DeviceID.String,
	}, nil
}

func (r *repository) RevokeSessionByID(ctx context.Context, id string) error {
	if err := r.query.RevokeSessionByID(ctx, id); err != nil {
		return fmt.Errorf("revoke_session_by_id: %w", err)
	}
	return nil
}

func (r *repository) RevokeAllSessionsByUserID(ctx context.Context, userID uuid.UUID) error {
	if err := r.query.RevokeAllSessionsByUserID(ctx, userID); err != nil {
		return fmt.Errorf("revoke_all_sessions_by_user_id: %w", err)
	}
	return nil
}
