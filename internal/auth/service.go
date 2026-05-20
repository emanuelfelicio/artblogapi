package auth

import (
	"context"
	"fmt"
	"log/slog"
)

type Repository interface {
	CreateUser(ctx context.Context, user User) (User, error)
	CheckEmailAndUsername(ctx context.Context, username, email string) error
}

type TokenProvider interface {
	GenerateAccessToken(user User) (string, error)
}

type service struct {
	repo          Repository
	logger        *slog.Logger
	tokenProvider TokenProvider
}

func NewService(repo Repository, logger *slog.Logger, tokenProvider TokenProvider) *service {
	return &service{
		repo:          repo,
		logger:        logger,
		tokenProvider: tokenProvider,
	}
}

func (s *service) Register(ctx context.Context, username, email, password string) (Auth, error) {
	if err := s.repo.CheckEmailAndUsername(ctx, username, email); err != nil {
		s.logger.Warn("auth_register_conflict", slog.String("error", err.Error()))
		return Auth{}, err
	}

	user, err := NewUser(username, email, password)
	if err != nil {
		return Auth{}, err
	}

	createdUser, err := s.repo.CreateUser(ctx, user)
	if err != nil {
		return Auth{}, err
	}

	accessToken, err := s.tokenProvider.GenerateAccessToken(createdUser)
	if err != nil {
		return Auth{}, fmt.Errorf("generate_access_token: %w", err)
	}

	s.logger.Info("auth_register_success", slog.String("user_id", createdUser.ID.String()))
	return Auth{AccessToken: accessToken, User: createdUser}, nil
}
