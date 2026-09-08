package user

import (
	"errors"

	"github.com/emanuelfelicio/artblogapi/internal/storage"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrUploadNotFound       = storage.ErrUploadNotFound
	ErrUploadNotCompleted   = storage.ErrUploadNotCompleted
	ErrUploadInvalidPurpose = storage.ErrUploadInvalidPurpose
)
