package user

import "errors"

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrUploadNotFound       = errors.New("upload not found or not owned by user")
	ErrUploadNotCompleted   = errors.New("upload is not completed")
	ErrUploadInvalidPurpose = errors.New("upload purpose is invalid for this resource")
)
