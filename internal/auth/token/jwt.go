package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrJWTProviderSecretRequired = errors.New("jwt provider secret is required")
	ErrJWTProviderIssuerRequired = errors.New("jwt provider issuer is required")
	ErrJWTProviderTTLInvalid     = errors.New("jwt provider ttl must be greater than zero")
)

type jwtTokenProvider struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewJWTTokenProvider(secret []byte, issuer string, ttl time.Duration) (*jwtTokenProvider, error) {
	if len(secret) == 0 {
		return nil, ErrJWTProviderSecretRequired
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, ErrJWTProviderIssuerRequired
	}
	if ttl <= 0 {
		return nil, ErrJWTProviderTTLInvalid
	}

	return &jwtTokenProvider{secret: secret, issuer: issuer, ttl: ttl}, nil
}

func (p *jwtTokenProvider) GenerateAccessToken(user User) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    p.issuer,
		Subject:   user.ID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(p.ttl)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(p.secret)
}

func (p *jwtTokenProvider) VerifyAccessToken(token string) {

}
