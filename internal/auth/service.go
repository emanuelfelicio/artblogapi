package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Repository interface {
	CreateUser(ctx context.Context, user User) (User, error)
	CheckEmailAndUsername(ctx context.Context, username, email string) error
	FindUserByCredential(ctx context.Context, credential string) (User, error)
	WithTransaction(ctx context.Context, fn func(repo Repository) error) error
	SaveSession(ctx context.Context, s Session) error
	FindSessionByID(ctx context.Context, id string) (Session, error)
	FindSessionByIDForUpdate(ctx context.Context, id string) (Session, error)
	RevokeSessionByID(ctx context.Context, id string) error
	RevokeAllSessionsByUserID(ctx context.Context, userID uuid.UUID) error
}

type TokenGenerator interface {
	GenerateAccessToken(user User) (string, error)
	GenerateRefreshToken() (string, error)
	HashToken(raw string) string
	AccessTokenTTL() time.Duration
	RefreshTokenTTL() time.Duration
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

func (s *service) Register(ctx context.Context, username, email, rawPassword, userAgent, ip, deviceID string) (Auth, error) {
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

	refreshToken, err := s.tokenProvider.GenerateRefreshToken()
	if err != nil {
		return Auth{}, fmt.Errorf("generate_refresh_token: %w", err)
	}

	session := Session{
		ID:        s.tokenProvider.HashToken(refreshToken),
		UserID:    createdUser.ID,
		ExpiresAt: time.Now().Add(s.tokenProvider.RefreshTokenTTL()),
		CreatedAt: time.Now(),
		Revoked:   false,
		UserAgent: userAgent,
		IP:        ip,
		DeviceID:  deviceID,
	}

	if err := s.repo.SaveSession(ctx, session); err != nil {
		return Auth{}, fmt.Errorf("save_session: %w", err)
	}

	s.logger.Info("auth_register_success", slog.String("user_id", createdUser.ID.String()))
	return Auth{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.tokenProvider.AccessTokenTTL().Seconds()),
		RefreshTTL:   int(s.tokenProvider.RefreshTokenTTL().Seconds()),
	}, nil
}
func (s *service) Login(ctx context.Context, credential, password, userAgent, ip, deviceID string) (Auth, error) {
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

	refreshToken, err := s.tokenProvider.GenerateRefreshToken()
	if err != nil {
		return Auth{}, fmt.Errorf("generate_refresh_token: %w", err)
	}

	session := Session{
		ID:        s.tokenProvider.HashToken(refreshToken),
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(s.tokenProvider.RefreshTokenTTL()),
		CreatedAt: time.Now(),
		Revoked:   false,
		UserAgent: userAgent,
		IP:        ip,
		DeviceID:  deviceID,
	}

	if err := s.repo.SaveSession(ctx, session); err != nil {
		return Auth{}, fmt.Errorf("save_session: %w", err)
	}

	s.logger.Info("auth_login_success", slog.String("user_id", user.ID.String()))
	return Auth{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.tokenProvider.AccessTokenTTL().Seconds()),
		RefreshTTL:   int(s.tokenProvider.RefreshTokenTTL().Seconds()),
	}, nil
}

func (s *service) Refresh(ctx context.Context, refreshToken, userAgent, ip, deviceID string) (Auth, error) {
	hashedToken := s.tokenProvider.HashToken(refreshToken)

	var result Auth
	var sessionID string
	var userID uuid.UUID

	err := s.repo.WithTransaction(ctx, func(txRepo Repository) error {
		session, err := txRepo.FindSessionByIDForUpdate(ctx, hashedToken)
		if err != nil {
			return err
		}

		sessionID = session.ID
		userID = session.UserID

		if session.Revoked {
			return ErrTokenReuse
		}

		if time.Now().After(session.ExpiresAt) {
			return ErrTokenExpired
		}

		// Device mismatch check (if deviceID is provided)
		if session.DeviceID != "" && deviceID != "" && session.DeviceID != deviceID {
			return ErrDeviceMismatch
		}

		// Revoke current session
		if err := txRepo.RevokeSessionByID(ctx, session.ID); err != nil {
			return err
		}

		// Issue new pair
		user := User{ID: session.UserID}
		accessToken, err := s.tokenProvider.GenerateAccessToken(user)
		if err != nil {
			return err
		}

		newRefreshToken, err := s.tokenProvider.GenerateRefreshToken()
		if err != nil {
			return err
		}

		newSession := Session{
			ID:        s.tokenProvider.HashToken(newRefreshToken),
			UserID:    session.UserID,
			ExpiresAt: time.Now().Add(s.tokenProvider.RefreshTokenTTL()),
			CreatedAt: time.Now(),
			Revoked:   false,
			UserAgent: userAgent,
			IP:        ip,
			DeviceID:  deviceID,
		}

		if err := txRepo.SaveSession(ctx, newSession); err != nil {
			return err
		}

		result = Auth{
			AccessToken:  accessToken,
			RefreshToken: newRefreshToken,
			TokenType:    "Bearer",
			ExpiresIn:    int64(s.tokenProvider.AccessTokenTTL().Seconds()),
			RefreshTTL:   int(s.tokenProvider.RefreshTokenTTL().Seconds()),
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, ErrTokenReuse) {
			if revErr := s.repo.RevokeAllSessionsByUserID(ctx, userID); revErr != nil {
				return Auth{}, WrapDomainErr(revErr)
			}
			return Auth{}, WrapDomainErr(ErrTokenReuse)
		}

		if errors.Is(err, ErrDeviceMismatch) {
			if revErr := s.repo.RevokeSessionByID(ctx, sessionID); revErr != nil {
				return Auth{}, WrapDomainErr(revErr)
			}
			return Auth{}, WrapDomainErr(ErrDeviceMismatch)
		}

		if errors.Is(err, ErrTokenExpired) || errors.Is(err, ErrSessionNotFound) {
			return Auth{}, WrapDomainErr(err)
		}

		return Auth{}, err
	}

	return result, nil
}

func (s *service) Logout(ctx context.Context, refreshToken string, currentUserID uuid.UUID) error {
	hashedToken := s.tokenProvider.HashToken(refreshToken)

	session, err := s.repo.FindSessionByID(ctx, hashedToken)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return err
	}

	if session.UserID != currentUserID {
		s.logger.Warn("auth_logout_ownership_violation",
			slog.String("session_user_id", session.UserID.String()),
			slog.String("current_user_id", currentUserID.String()))
		return nil // Ownership enforcement
	}

	return s.repo.RevokeSessionByID(ctx, session.ID)
}

func (s *service) validateCredentials(ctx context.Context, credential, password string) (User, error) {

	user, err := s.repo.FindUserByCredential(ctx, credential)
	if err != nil {
		return User{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return User{}, ErrInvalidCredentials
	}

	return user, nil
}
