package auth

import (
	"context"
	"log/slog"
)

type Repository interface {
	CreateUser(ctx context.Context, user User) (User, error)
	CheckEmailExists(ctx context.Context, email string) (bool, error)
	CheckUsernameExists(ctx context.Context, username string) (bool, error)
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
	exists, err := s.repo.CheckEmailExists(ctx, email)
	if err != nil {
		s.logger.Error("auth_register_check_email_failed", slog.String("error", err.Error()))
		return Auth{}, err
	}
	if exists {
		s.logger.Warn("auth_register_conflict", slog.String("reason", "email_already_exists"))
		return Auth{}, ErrEmailAlreadyExists
	}

	exists, err = s.repo.CheckUsernameExists(ctx, username)
	if err != nil {
		s.logger.Error("auth_register_check_username_failed", slog.String("error", err.Error()))
		return Auth{}, err
	}
	if exists {
		s.logger.Warn("auth_register_conflict", slog.String("reason", "username_already_exists"))
		return Auth{}, ErrUsernameAlreadyExists
	}

	user, err := NewUser(username, email, password)
	if err != nil {
		s.logger.Error("auth_register_build_user_failed", slog.String("error", err.Error()))
		return Auth{}, err
	}

	createdUser, err := s.repo.CreateUser(ctx, user)
	if err != nil {
		s.logger.Error("auth_register_create_user_failed", slog.String("error", err.Error()))
		return Auth{}, err
	}

	accessToken, err := s.tokenProvider.GenerateAccessToken(createdUser)
	if err != nil {
		s.logger.Error("auth_register_token_generation_failed", slog.String("error", err.Error()))
		return Auth{}, err
	}

	s.logger.Info("auth_register_success", slog.String("user_id", createdUser.ID.String()))
	return Auth{AccessToken: accessToken, User: createdUser}, nil
}
