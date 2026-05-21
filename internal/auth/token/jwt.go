package token

import (
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

type AuthPrincipal struct {
	UserID string
}

type JWTTokenService struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewJWT(secret []byte, issuer string, ttl time.Duration) (*JWTTokenService, error) {
	if len(secret) == 0 {
		return nil, ErrJWTProviderSecretRequired
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, ErrJWTProviderIssuerRequired
	}
	if ttl <= 0 {
		return nil, ErrJWTProviderTTLInvalid
	}

	return &JWTTokenService{secret: secret, issuer: issuer, ttl: ttl}, nil
}

func (p *JWTTokenService) GenerateAccessToken(user auth.User) (string, error) {
	now := time.Now()
	claims := AccessClaims{jwt.RegisteredClaims{
		Issuer:    p.issuer,
		Subject:   user.ID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(p.ttl)),
	}}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(p.secret)
}

func (p *JWTTokenService) VerifyAccessToken(token string) (AuthPrincipal, error) {
	parser := jwt.NewParser(jwt.WithIssuer(p.issuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	claims := &AccessClaims{}
	parsedToken, err := parser.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) { return p.secret, nil })

	if err != nil {
		return AuthPrincipal{}, err
	}

	if parsedToken.Valid == false {
		return AuthPrincipal{}, ErrInvalidToken
	}

	return AuthPrincipal{UserID: claims.Subject}, nil
}
