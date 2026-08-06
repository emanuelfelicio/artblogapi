package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// --- STUBS ---

type stubRepository struct {
	checkEmailAndUsername     func(context.Context, string, string) error
	createUser                func(context.Context, User) (User, error)
	findUserByCredential      func(context.Context, string) (User, error)
	saveSession               func(context.Context, Session) error
	findSessionByID           func(context.Context, string) (Session, error)
	findSessionByIDForUpdate  func(context.Context, string) (Session, error)
	revokeSessionByID         func(context.Context, string) error
	revokeAllSessionsByUserID func(context.Context, uuid.UUID) error

	lastCreatedUser  User
	lastSavedSession Session
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

func (s *stubRepository) WithTransaction(ctx context.Context, fn func(repo Repository) error) error {
	return fn(s)
}

func (s *stubRepository) SaveSession(ctx context.Context, sess Session) error {
	s.lastSavedSession = sess
	if s.saveSession != nil {
		return s.saveSession(ctx, sess)
	}
	return nil
}

func (s *stubRepository) FindSessionByID(ctx context.Context, id string) (Session, error) {
	if s.findSessionByID != nil {
		return s.findSessionByID(ctx, id)
	}
	return Session{}, nil
}

func (s *stubRepository) FindSessionByIDForUpdate(ctx context.Context, id string) (Session, error) {
	if s.findSessionByIDForUpdate != nil {
		return s.findSessionByIDForUpdate(ctx, id)
	}
	return Session{}, nil
}

func (s *stubRepository) RevokeSessionByID(ctx context.Context, id string) error {
	if s.revokeSessionByID != nil {
		return s.revokeSessionByID(ctx, id)
	}
	return nil
}

func (s *stubRepository) RevokeAllSessionsByUserID(ctx context.Context, userID uuid.UUID) error {
	if s.revokeAllSessionsByUserID != nil {
		return s.revokeAllSessionsByUserID(ctx, userID)
	}
	return nil
}

type stubTokenGenerator struct {
	AccessToken     string
	RefreshToken    string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func (s stubTokenGenerator) GenerateAccessToken(user User) (string, error) {
	return s.AccessToken, nil
}

func (s stubTokenGenerator) GenerateRefreshToken() (string, error) {
	return s.RefreshToken, nil
}

func (s stubTokenGenerator) HashToken(raw string) string {
	return "hashed-" + raw
}

func (s stubTokenGenerator) AccessTokenTTL() time.Duration {
	return s.accessTokenTTL
}

func (s stubTokenGenerator) RefreshTokenTTL() time.Duration {
	return s.refreshTokenTTL
}

// --- HELPERS ---

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
		ID:           uuid.New(),
		PasswordHash: hashPasswordForTest(t, password),
		IsActive:     isActive,
	}
}

func assertRefreshTokenHashed(t *testing.T, repo *stubRepository, tokenProvider TokenGenerator, refreshToken string) {
	t.Helper()
	if repo.lastSavedSession.ID == refreshToken {
		t.Fatalf("refresh token should be hashed in repository, got raw token")
	}
	if repo.lastSavedSession.ID != tokenProvider.HashToken(refreshToken) {
		t.Fatalf("stored session ID does not match hashed refresh token")
	}
}

// --- TESTS ---

func TestService_Register(t *testing.T) {
	t.Parallel()
	const (
		ua       = "test-agent"
		ip       = "127.0.0.1"
		deviceID = "dev-123"
	)

	tests := []struct {
		name             string
		setup            func() (*stubRepository, stubTokenGenerator)
		username         string
		email            string
		password         string
		wantErr          error
		wantAccessToken  string
		wantRefreshToken string
		wantTTL          time.Duration
	}{
		{
			name: "success",
			setup: func() (*stubRepository, stubTokenGenerator) {
				return &stubRepository{}, stubTokenGenerator{
					AccessToken:     "test-access-token",
					RefreshToken:    "test-refresh-token",
					accessTokenTTL:  time.Hour,
					refreshTokenTTL: 24 * time.Hour,
				}
			},
			username:         "john.doe",
			email:            "john.doe@example.com",
			password:         "StrongPass!123",
			wantAccessToken:  "test-access-token",
			wantRefreshToken: "test-refresh-token",
			wantTTL:          time.Hour,
		},
		{
			name: "email conflict",
			setup: func() (*stubRepository, stubTokenGenerator) {
				repo := &stubRepository{}
				repo.checkEmailAndUsername = func(ctx context.Context, username, email string) error {
					return ErrEmailAlreadyExists
				}
				return repo, stubTokenGenerator{}
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
				return repo, stubTokenGenerator{}
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

			auth, err := service.Register(context.Background(), tt.username, tt.email, tt.password, ua, ip, deviceID)

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
			if auth.AccessToken != tt.wantAccessToken {
				t.Fatalf("access token = %q, want %q", auth.AccessToken, tt.wantAccessToken)
			}
			if auth.RefreshToken != tt.wantRefreshToken {
				t.Fatalf("refresh token = %q, want %q", auth.RefreshToken, tt.wantRefreshToken)
			}
			if auth.TokenType != "Bearer" {
				t.Fatalf("token type = %q, want %q", auth.TokenType, "Bearer")
			}
			if auth.ExpiresIn != int64(tt.wantTTL.Seconds()) {
				t.Fatalf("expires in = %d, want %d", auth.ExpiresIn, int64(tt.wantTTL.Seconds()))
			}

			assertRefreshTokenHashed(t, repo, tokenProvider, auth.RefreshToken)

			if repo.lastCreatedUser.PasswordHash == tt.password {
				t.Fatalf("password hash should not equal raw password")
			}

			if err := bcrypt.CompareHashAndPassword([]byte(repo.lastCreatedUser.PasswordHash), []byte(tt.password)); err != nil {
				t.Fatalf("stored password hash does not match raw password: %v", err)
			}
		})
	}
}

func TestService_Login(t *testing.T) {
	t.Parallel()
	const (
		credential  = "john.doe"
		correctPass = "CorrectPass!123"
		wrongPass   = "WrongPass!123"
		ua          = "test-agent"
		ip          = "127.0.0.1"
		deviceID    = "dev-123"
	)

	tests := []struct {
		name             string
		setup            func() (*stubRepository, stubTokenGenerator)
		password         string
		wantErr          error
		wantAccessToken  string
		wantRefreshToken string
		wantTTL          time.Duration
	}{
		{
			name: "success",
			setup: func() (*stubRepository, stubTokenGenerator) {
				repo := &stubRepository{}
				repo.findUserByCredential = func(ctx context.Context, gotCredential string) (User, error) {
					return generateTestUser(t, correctPass, true), nil
				}
				return repo, stubTokenGenerator{
					AccessToken:    "test-access-token",
					RefreshToken:   "test-refresh-token",
					accessTokenTTL: time.Hour,
				}
			},
			password:         correctPass,
			wantAccessToken:  "test-access-token",
			wantRefreshToken: "test-refresh-token",
			wantTTL:          time.Hour,
		},
		{
			name: "invalid credentials - wrong password",
			setup: func() (*stubRepository, stubTokenGenerator) {
				repo := &stubRepository{}
				repo.findUserByCredential = func(ctx context.Context, gotCredential string) (User, error) {
					return generateTestUser(t, correctPass, true), nil
				}
				return repo, stubTokenGenerator{}
			},
			password: wrongPass,
			wantErr:  ErrInvalidCredentials,
		},
		{
			name: "user inactive",
			setup: func() (*stubRepository, stubTokenGenerator) {
				repo := &stubRepository{}
				repo.findUserByCredential = func(ctx context.Context, gotCredential string) (User, error) {
					return generateTestUser(t, correctPass, false), nil
				}
				return repo, stubTokenGenerator{}
			},
			password: correctPass,
			wantErr:  ErrUserInactive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, tokenProvider := tt.setup()
			service := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)), tokenProvider)

			auth, err := service.Login(context.Background(), credential, tt.password, ua, ip, deviceID)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
			if auth.AccessToken != tt.wantAccessToken {
				t.Fatalf("access token = %q, want %q", auth.AccessToken, tt.wantAccessToken)
			}
			if auth.RefreshToken != tt.wantRefreshToken {
				t.Fatalf("refresh token = %q, want %q", auth.RefreshToken, tt.wantRefreshToken)
			}
			if auth.TokenType != "Bearer" {
				t.Fatalf("token type = %q, want %q", auth.TokenType, "Bearer")
			}
			if auth.ExpiresIn != int64(tt.wantTTL.Seconds()) {
				t.Fatalf("expires in = %d, want %d", auth.ExpiresIn, int64(tt.wantTTL.Seconds()))
			}

			assertRefreshTokenHashed(t, repo, tokenProvider, auth.RefreshToken)
		})
	}
}

func TestService_Refresh(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	const (
		oldToken = "old-refresh-token"
		deviceID = "dev-123"
	)

	tests := []struct {
		name              string
		setup             func() (*stubRepository, stubTokenGenerator)
		wantErr           error
		wantAccessToken   string
		wantRefreshToken  string
		wantTTL           time.Duration
		wantRevokeCalled  bool
		wantRevokeAllCall bool
	}{
		{
			name: "success",
			setup: func() (*stubRepository, stubTokenGenerator) {
				repo := &stubRepository{}
				repo.findSessionByIDForUpdate = func(ctx context.Context, id string) (Session, error) {
					return Session{ID: id, UserID: userID, DeviceID: deviceID, ExpiresAt: time.Now().Add(time.Hour)}, nil
				}
				return repo, stubTokenGenerator{
					AccessToken:    "new-access-token",
					RefreshToken:   "new-refresh-token",
					accessTokenTTL: time.Hour,
				}
			},
			wantAccessToken:  "new-access-token",
			wantRefreshToken: "new-refresh-token",
			wantTTL:          time.Hour,
			wantRevokeCalled: true,
		},
		{
			name: "token reuse detection",
			setup: func() (*stubRepository, stubTokenGenerator) {
				repo := &stubRepository{}
				repo.findSessionByIDForUpdate = func(ctx context.Context, id string) (Session, error) {
					return Session{ID: id, UserID: userID, Revoked: true}, nil
				}
				return repo, stubTokenGenerator{}
			},
			wantErr:           ErrTokenReuse,
			wantRevokeAllCall: true,
		},
		{
			name: "token expired",
			setup: func() (*stubRepository, stubTokenGenerator) {
				repo := &stubRepository{}
				repo.findSessionByIDForUpdate = func(ctx context.Context, id string) (Session, error) {
					return Session{ID: id, UserID: userID, ExpiresAt: time.Now().Add(-time.Hour)}, nil
				}
				return repo, stubTokenGenerator{}
			},
			wantErr: ErrTokenExpired,
		},
		{
			name: "device mismatch",
			setup: func() (*stubRepository, stubTokenGenerator) {
				repo := &stubRepository{}
				repo.findSessionByIDForUpdate = func(ctx context.Context, id string) (Session, error) {
					return Session{ID: id, UserID: userID, DeviceID: "other-device", ExpiresAt: time.Now().Add(time.Hour)}, nil
				}
				return repo, stubTokenGenerator{}
			},
			wantErr:          ErrDeviceMismatch,
			wantRevokeCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, tokenProvider := tt.setup()

			var revokeCalled bool
			repo.revokeSessionByID = func(ctx context.Context, id string) error {
				revokeCalled = true
				return nil
			}

			var revokeAllCalled bool
			repo.revokeAllSessionsByUserID = func(ctx context.Context, id uuid.UUID) error {
				revokeAllCalled = true
				return nil
			}

			service := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)), tokenProvider)

			auth, err := service.Refresh(context.Background(), oldToken, "ua", "ip", deviceID)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("error = %v, want nil", err)
			}

			if tt.wantErr == nil {
				if auth.AccessToken != tt.wantAccessToken {
					t.Fatalf("access token = %q, want %q", auth.AccessToken, tt.wantAccessToken)
				}
				if auth.RefreshToken != tt.wantRefreshToken {
					t.Fatalf("refresh token = %q, want %q", auth.RefreshToken, tt.wantRefreshToken)
				}
				if auth.TokenType != "Bearer" {
					t.Fatalf("token type = %q, want %q", auth.TokenType, "Bearer")
				}
				if auth.ExpiresIn != int64(tt.wantTTL.Seconds()) {
					t.Fatalf("expires in = %d, want %d", auth.ExpiresIn, int64(tt.wantTTL.Seconds()))
				}

				assertRefreshTokenHashed(t, repo, tokenProvider, auth.RefreshToken)
			}

			if revokeCalled != tt.wantRevokeCalled {
				t.Fatalf("revokeCalled = %v, want %v", revokeCalled, tt.wantRevokeCalled)
			}

			if revokeAllCalled != tt.wantRevokeAllCall {
				t.Fatalf("revokeAllCalled = %v, want %v", revokeAllCalled, tt.wantRevokeAllCall)
			}
		})
	}
}

func TestService_Logout(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	const refreshToken = "test-refresh-token"

	tests := []struct {
		name          string
		setup         func() *stubRepository
		currentUserID uuid.UUID
		wantErr       error
		wantRevoked   bool
	}{
		{
			name: "success",
			setup: func() *stubRepository {
				repo := &stubRepository{}
				repo.findSessionByID = func(ctx context.Context, id string) (Session, error) {
					return Session{ID: id, UserID: userID}, nil
				}
				return repo
			},
			currentUserID: userID,
			wantRevoked:   true,
		},
		{
			name: "ownership violation",
			setup: func() *stubRepository {
				repo := &stubRepository{}
				repo.findSessionByID = func(ctx context.Context, id string) (Session, error) {
					return Session{ID: id, UserID: uuid.New()}, nil
				}
				return repo
			},
			currentUserID: userID,
			wantRevoked:   false,
		},
		{
			name: "session not found (idempotent)",
			setup: func() *stubRepository {
				repo := &stubRepository{}
				repo.findSessionByID = func(ctx context.Context, id string) (Session, error) {
					return Session{}, ErrSessionNotFound
				}
				return repo
			},
			currentUserID: userID,
			wantRevoked:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := tt.setup()

			var revokeCalled bool
			repo.revokeSessionByID = func(ctx context.Context, id string) error {
				revokeCalled = true
				return nil
			}

			service := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)), stubTokenGenerator{})

			err := service.Logout(context.Background(), refreshToken, tt.currentUserID)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("error = %v, want nil", err)
			}

			if revokeCalled != tt.wantRevoked {
				t.Fatalf("revokeCalled = %v, want %v", revokeCalled, tt.wantRevoked)
			}
		})
	}
}
