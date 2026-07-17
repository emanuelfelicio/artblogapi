package user

import "errors"

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrUploadNotFound = errors.New("upload not found or not completed")
	ErrUserInactive   = errors.New("user is inactive")
)
