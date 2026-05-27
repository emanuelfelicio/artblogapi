package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Repository interface {
	CreateUser(ctx context.Context, user User) (User, error)
	CheckEmailAndUsername(ctx context.Context, username, email string) error
	FindUserByCredential(ctx context.Context, credential string) (User, error)
}

type TokenGenerator interface {
	GenerateAccessToken(user User) (string, error)
	AccessTokenTTL() time.Duration
}

type service struct {
	repo          Repository
	logger        *slog.Logger
	tokenProvider TokenGenerator
}

func NewService(repo Repository, logger *slog.Logger, tokenProvider TokenGenerator) *service {
	return &service{
		repo:          repo,
		logger:        logger,
		tokenProvider: tokenProvider,
	}
}

func (s *service) Register(ctx context.Context, username, email, rawPassword string) (Auth, error) {
	if err := s.repo.CheckEmailAndUsername(ctx, username, email); err != nil {
		s.logger.Warn("auth_register_conflict", slog.String("error", err.Error()))
		return Auth{}, err
	}

	hashPassword, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return Auth{}, fmt.Errorf("hash_password: %w", err)
	}

	user, err := NewUser(username, email, string(hashPassword))
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
	return Auth{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.tokenProvider.AccessTokenTTL().Seconds()),
	}, nil
}

func (s *service) Login(ctx context.Context, credential, password string) (Auth, error) {
	user, err := s.validateCredentials(ctx, credential, password)
	if err != nil {
		return Auth{}, err
	}

	if !user.IsActive {
		return Auth{}, ErrUserInactive
	}

	accessToken, err := s.tokenProvider.GenerateAccessToken(user)
	if err != nil {
		return Auth{}, fmt.Errorf("generate_access_token: %w", err)
	}
	// Ensure the password hash is not returned in the response.
	user.PasswordHash = ""

	s.logger.Info("auth_login_success", slog.String("user_id", user.ID.String()))
	return Auth{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.tokenProvider.AccessTokenTTL().Seconds()),
	}, nil
}

func (s *service) validateCredentials(ctx context.Context, credential, password string) (User, error) {

	user, err := s.repo.FindUserByCredential(ctx, credential)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return User{}, ErrInvalidCredentials
	}

	return user, nil
}
