package auth

import (
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUsernameAlreadyExists = errors.New("username already exists")
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
	User        *User
}

func NewUser(username, email, rawPassword string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:           uuid.New(),
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		IsActive:     true,
	}, nil
}
