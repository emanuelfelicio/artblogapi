package auth

import "errors"

var (
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrUserInactive          = errors.New("user is inactive")

	ErrTokenExpired    = errors.New("token expired")
	ErrTokenReuse      = errors.New("token reuse detected")
	ErrDeviceMismatch  = errors.New("device mismatch")
	ErrSessionNotFound = errors.New("session not found")
)

type DomainError interface {
	error
	IsDomainError() bool
}

type domainErr struct {
	err error
}

func (e *domainErr) Error() string       { return e.err.Error() }
func (e *domainErr) Unwrap() error       { return e.err }
func (e *domainErr) IsDomainError() bool { return true }

func WrapDomainErr(err error) error {
	if err == nil {
		return nil
	}
	return &domainErr{err: err}
}

func IsDomainErr(err error) bool {
	var de DomainError
	return errors.As(err, &de)
}
