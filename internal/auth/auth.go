package auth

import (
	"errors"

	"github.com/google/uuid"
)

var (
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrUserInactive          = errors.New("user is inactive")
)

type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
	PasswordHash string
	DisplayName  string
	AvatarURL    string
	IsActive     bool
}

type Auth struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int64
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
