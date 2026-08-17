package storage

import "errors"

var (
	ErrFileSizeExceeded       = errors.New("file_size exceeds maximum allowed for this purpose")
	ErrInvalidFileSize        = errors.New("invalide file size")
	ErrInvalidPurpose         = errors.New("invalid upload purpose")
	ErrInvaliImageContentType = errors.New("invalid image content_type")
	ErrUploadNotFound         = errors.New("upload not found")
	ErrUploadNotPending       = errors.New("upload is not in PENDING status")
	ErrUploadNotOwned         = errors.New("upload does not belong to this user")
	ErrFileNotFound           = errors.New("file not found")
)
