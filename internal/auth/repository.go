package auth

import (
	"context"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
)

type repository struct {
	query *dbgen.Queries
}

func NewRepository(q *dbgen.Queries) *repository {
	return &repository{query: q}
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
		return User{}, err
	}

	return User{
		ID:       dbUser.ID,
		Username: dbUser.Username,
		Email:    dbUser.Email,
		IsActive: dbUser.IsActive,
	}, nil
}

func (r *repository) CheckEmailExists(ctx context.Context, email string) (bool, error) {
	return r.query.CheckEmailExists(ctx, email)
}

func (r *repository) CheckUsernameExists(ctx context.Context, username string) (bool, error) {
	return r.query.CheckUsernameExists(ctx, username)
}
