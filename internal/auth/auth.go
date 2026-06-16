package auth

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
	PasswordHash string
	IsActive     bool
}

type Auth struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
	RefreshTTL   int
}

type AuthPrincipal struct {
	UserID string
}

type Session struct {
	ID        string
	UserID    uuid.UUID
	ExpiresAt time.Time
	CreatedAt time.Time
	Revoked   bool
	UserAgent string
	IP        string
	DeviceID  string
}

func NewUser(username, email, passwordHash string) (User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return User{}, err
	}

	return User{
		ID:           id,
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		IsActive:     true,
	}, nil
}
