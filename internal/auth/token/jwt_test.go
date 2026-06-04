package token_test

import (
	"errors"
	"testing"
	"time"

	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/emanuelfelicio/artblogapi/internal/auth/token"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestJWT_GenerateAndVerify_Success(t *testing.T) {
	t.Parallel()

	var (
		secret = "test-secret"
		issuer = "artblog"
		ttl    = time.Hour
		user   = auth.User{ID: uuid.New()}
	)

	tokenService, err := token.NewJWT([]byte(secret), issuer, ttl)
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}

	accessToken, err := tokenService.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	if accessToken == "" {
		t.Fatalf("GenerateAccessToken() returned empty token")
	}

	principal, err := tokenService.VerifyAccessToken(accessToken)
	if err != nil {
		t.Fatalf("VerifyAccessToken() error = %v", err)
	}

	if principal.UserID != user.ID.String() {
		t.Fatalf("principal.UserID = %q, want %q", principal.UserID, user.ID.String())
	}
}

func TestJWT_VerifyAccessToken_Errors(t *testing.T) {
	t.Parallel()

	const (
		secret = "test-secret"
		issuer = "artblog"
	)

	tokenService, err := token.NewJWT([]byte(secret), issuer, time.Hour)
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}

	tests := []struct {
		name    string
		token   string
		wantErr error
	}{
		{
			name:    "expired token",
			token:   buildSignedToken(t, secret, jwt.SigningMethodHS256, issuer, time.Now().Add(-time.Hour)),
			wantErr: jwt.ErrTokenExpired,
		},
		{
			name:    "invalid issuer",
			token:   buildSignedToken(t, secret, jwt.SigningMethodHS256, "other-issuer", time.Now().Add(time.Hour)),
			wantErr: jwt.ErrTokenInvalidIssuer,
		},
		{
			name:    "invalid algorithm",
			token:   buildSignedToken(t, secret, jwt.SigningMethodHS384, issuer, time.Now().Add(time.Hour)),
			wantErr: jwt.ErrTokenSignatureInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tokenService.VerifyAccessToken(tt.token)
			if err == nil {
				t.Fatalf("VerifyAccessToken() error = nil, want %v", tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("VerifyAccessToken() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func buildSignedToken(t *testing.T, secret string, method jwt.SigningMethod, issuer string, expiresAt time.Time) string {
	t.Helper()

	token := jwt.NewWithClaims(method, jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   uuid.NewString(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	})

	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	return signedToken
}
