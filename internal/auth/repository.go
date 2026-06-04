package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type repository struct {
	query  *dbgen.Queries
	logger *slog.Logger
}

func NewRepository(q *dbgen.Queries, logger *slog.Logger) *repository {
	return &repository{query: q, logger: logger}
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
