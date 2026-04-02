package auth

import (
	"context"
)

type service struct {
	repo repository
}

func NewService(repo repository) *service {
	return &service{repo: repo}
}

func (s *service) Register(ctx context.Context, username, email, password string) (User, error) {
	exists, err := s.repo.CheckEmailExists(ctx, email)
	if err != nil {
		return User{}, err
	}
	if exists {
		return User{}, ErrEmailAlreadyExists
	}

	exists, err = s.repo.CheckUsernameExists(ctx, username)
	if err != nil {
		return User{}, err
	}
	if exists {
		return User{}, ErrUsernameAlreadyExists
	}

	user, err := NewUser(username, email, password)
	if err != nil {
		return User{}, err
	}

	return s.repo.CreateUser(ctx, user)
}
