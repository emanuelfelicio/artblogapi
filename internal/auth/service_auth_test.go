package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type stubRepository struct {
	checkEmailAndUsername func(context.Context, string, string) error
	createUser            func(context.Context, User) (User, error)
	findUserByCredential  func(context.Context, string) (User, error)

	lastCreatedUser User
}

func (s *stubRepository) CheckEmailAndUsername(ctx context.Context, username, email string) error {
	if s.checkEmailAndUsername != nil {
		return s.checkEmailAndUsername(ctx, username, email)
	}

	return nil
}

func (s *stubRepository) CreateUser(ctx context.Context, user User) (User, error) {
	s.lastCreatedUser = user
	if s.createUser != nil {
		return s.createUser(ctx, user)
	}

	return user, nil
}

func (s *stubRepository) FindUserByCredential(ctx context.Context, credential string) (User, error) {
	if s.findUserByCredential != nil {
		return s.findUserByCredential(ctx, credential)
	}

	return User{}, errors.New("not implemented")
}

type stubTokenGenerator struct {
	token string
	ttl   time.Duration
}

func (s stubTokenGenerator) GenerateAccessToken(user User) (string, error) {
	return s.token, nil
}

func (s stubTokenGenerator) AccessTokenTTL() time.Duration {
	return s.ttl
}

func hashPasswordForTest(t *testing.T, password string) string {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword() error = %v", err)
	}

	return string(hash)
}

func generateTestUser(t *testing.T, password string, isActive bool) User {
	t.Helper()

	return User{
		PasswordHash: hashPasswordForTest(t, password),
		IsActive:     isActive,
	}
}

func TestService_Register(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func() (*stubRepository, stubTokenGenerator)
		username  string
		email     string
		password  string
		wantErr   error
		wantToken string
		wantTTL   time.Duration
	}{
		{
			name: "success",
			setup: func() (*stubRepository, stubTokenGenerator) {

				return &stubRepository{}, stubTokenGenerator{token: "test-access-token", ttl: time.Hour}

			},
			username:  "john.doe",
			email:     "john.doe@example.com",
			password:  "StrongPass!123",
			wantToken: "test-access-token",
			wantTTL:   time.Hour,
		},
		{
			name: "email conflict",
			setup: func() (*stubRepository, stubTokenGenerator) {
				repo := &stubRepository{}
				repo.checkEmailAndUsername = func(ctx context.Context, username, email string) error {
					return ErrEmailAlreadyExists
				}
				return repo, stubTokenGenerator{token: "unused", ttl: time.Hour}
			},
			username: "john.doe",
			email:    "john.doe@example.com",
			password: "StrongPass!123",
			wantErr:  ErrEmailAlreadyExists,
		},
		{
			name: "username conflict",
			setup: func() (*stubRepository, stubTokenGenerator) {
				repo := &stubRepository{}
				repo.checkEmailAndUsername = func(ctx context.Context, username, email string) error {
					return ErrUsernameAlreadyExists
				}
				return repo, stubTokenGenerator{token: "unused", ttl: time.Hour}
			},
			username: "john.doe",
			email:    "john.doe@example.com",
			password: "StrongPass!123",
			wantErr:  ErrUsernameAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, tokenProvider := tt.setup()
			service := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)), tokenProvider)

			auth, err := service.Register(context.Background(), tt.username, tt.email, tt.password)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("service.Register() error = %v", err)
			}
			if auth.AccessToken != tt.wantToken {
				t.Fatalf("access token = %q, want %q", auth.AccessToken, tt.wantToken)
			}
			if auth.TokenType != "Bearer" {
				t.Fatalf("token type = %q, want %q", auth.TokenType, "Bearer")
			}
			if auth.ExpiresIn != int64(tt.wantTTL.Seconds()) {
				t.Fatalf("expires in = %d, want %d", auth.ExpiresIn, int64(tt.wantTTL.Seconds()))
			}
			if repo.lastCreatedUser.PasswordHash == tt.password {
				t.Fatalf("password hash should not equal raw password")
			}

			if err := bcrypt.CompareHashAndPassword([]byte(repo.lastCreatedUser.PasswordHash), []byte(tt.password)); err != nil {
				t.Fatalf("stored password hash does not match raw password: %v", err)
			}

		})
	}
}

func TestService_Login_Success(t *testing.T) {
	t.Parallel()

	const (
		credential = "john.doe"
		password   = "StrongPass!123"
	)

	repo := &stubRepository{}
	repo.findUserByCredential = func(ctx context.Context, gotCredential string) (User, error) {
		if gotCredential != credential {
			t.Fatalf("credential = %q, want %q", gotCredential, credential)
		}

		return generateTestUser(t, password, true), nil
	}

	tokenProvider := stubTokenGenerator{token: "test-access-token", ttl: time.Hour}
	service := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)), tokenProvider)

	auth, err := service.Login(context.Background(), credential, password)
	if err != nil {
		t.Fatalf("service.Login() error = %v", err)
	}
	if auth.AccessToken != tokenProvider.token {
		t.Fatalf("access token = %q, want %q", auth.AccessToken, tokenProvider.token)
	}
	if auth.TokenType != "Bearer" {
		t.Fatalf("token type = %q, want %q", auth.TokenType, "Bearer")
	}
	if auth.ExpiresIn != int64(tokenProvider.ttl.Seconds()) {
		t.Fatalf("expires in = %d, want %d", auth.ExpiresIn, int64(tokenProvider.ttl.Seconds()))
	}
}

func TestService_Login_InvalidCredentials_WrongPassword(t *testing.T) {
	t.Parallel()

	const (
		credential  = "john.doe"
		correctPass = "CorrectPass!123"
		wrongPass   = "WrongPass!123"
	)

	repo := &stubRepository{}
	repo.findUserByCredential = func(ctx context.Context, gotCredential string) (User, error) {
		if gotCredential != credential {
			t.Fatalf("credential = %q, want %q", gotCredential, credential)
		}

		return generateTestUser(t, correctPass, true), nil
	}

	service := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)), stubTokenGenerator{token: "unused", ttl: time.Hour})

	_, err := service.Login(context.Background(), credential, wrongPass)
	if err == nil {
		t.Fatalf("service.Login() error = nil, want %v", ErrInvalidCredentials)
	}
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("service.Login() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestService_Login_UserInactive(t *testing.T) {
	t.Parallel()

	const (
		credential = "john.doe"
		password   = "StrongPass!123"
	)

	repo := &stubRepository{}
	repo.findUserByCredential = func(ctx context.Context, gotCredential string) (User, error) {
		if gotCredential != credential {
			t.Fatalf("credential = %q, want %q", gotCredential, credential)
		}

		return generateTestUser(t, password, false), nil
	}

	service := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)), stubTokenGenerator{token: "unused", ttl: time.Hour})

	_, err := service.Login(context.Background(), credential, password)
	if err == nil {
		t.Fatalf("service.Login() error = nil, want %v", ErrUserInactive)
	}
	if !errors.Is(err, ErrUserInactive) {
		t.Fatalf("service.Login() error = %v, want %v", err, ErrUserInactive)
	}
}
