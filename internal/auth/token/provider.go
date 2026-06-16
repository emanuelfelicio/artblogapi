package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrJWTProviderSecretRequired = errors.New("jwt provider secret is required")
	ErrJWTProviderIssuerRequired = errors.New("jwt provider issuer is required")
	ErrJWTProviderTTLInvalid     = errors.New("jwt provider ttl must be greater than zero")
	ErrInvalidToken              = errors.New("invalid token")
)

type AccessClaims struct {
	jwt.RegisteredClaims
}

type TokenProvider struct {
	secret          []byte
	issuer          string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewTokenProvider(secret []byte, issuer string, accessTokenTTL, refreshTokenTTL time.Duration) (*TokenProvider, error) {
	if len(secret) == 0 {
		return nil, ErrJWTProviderSecretRequired
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, ErrJWTProviderIssuerRequired
	}
	if accessTokenTTL <= 0 || refreshTokenTTL <= 0 {
		return nil, ErrJWTProviderTTLInvalid
	}

	return &TokenProvider{
		secret:          secret,
		issuer:          issuer,
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}, nil
}

func (p *TokenProvider) GenerateAccessToken(user auth.User) (string, error) {
	now := time.Now()
	claims := AccessClaims{jwt.RegisteredClaims{
		Issuer:    p.issuer,
		Subject:   user.ID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(p.accessTokenTTL)),
	}}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(p.secret)
}

func (p *TokenProvider) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (p *TokenProvider) HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (p *TokenProvider) AccessTokenTTL() time.Duration {
	return p.accessTokenTTL
}

func (p *TokenProvider) RefreshTokenTTL() time.Duration {
	return p.refreshTokenTTL
}

func (p *TokenProvider) VerifyAccessToken(token string) (auth.AuthPrincipal, error) {
	parser := jwt.NewParser(jwt.WithIssuer(p.issuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	claims := &AccessClaims{}
	_, err := parser.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) { return p.secret, nil })

	if err != nil {
		return auth.AuthPrincipal{}, ErrInvalidToken
	}

	return auth.AuthPrincipal{UserID: claims.Subject}, nil
}
