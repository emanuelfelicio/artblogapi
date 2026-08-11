package storage

import "errors"

var (
	ErrFileSizeExceeded       = errors.New("file_size exceeds maximum allowed for this purpose")
	ErrInvalidFileSize        = errors.New("invalide file size")
	ErrInvalidPurpose         = errors.New("invalid upload purpose")
	ErrInvaliImageContentType = errors.New("invalid image content_type")
)
