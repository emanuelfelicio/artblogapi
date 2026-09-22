package post

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MaxImagesPerPost = 10
	MinTitleLength   = 1
	MaxTitleLength   = 150
	DefaultPageLimit = 20
	MaxPageLimit     = 50
)

type Post struct {
	ID        uuid.UUID
	AuthorID  uuid.UUID
	Title     string
	Content   string
	Images    []PostImage
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PostImage struct {
	PostID      uuid.UUID
	UploadID    uuid.UUID
	Position    int
	ObjectKey   string
	ContentType string
	CreatedAt   time.Time
}

func ValidateTitle(title string) (string, error) {
	trimmed := strings.TrimSpace(title)
	if len(trimmed) < MinTitleLength || len(trimmed) > MaxTitleLength {
		return "", ErrInvalidPostTitle
	}
	return trimmed, nil
}

func ValidateContent(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", ErrInvalidPostContent
	}
	return trimmed, nil
}

func ValidateImagesCount(count int) error {
	if count > MaxImagesPerPost {
		return ErrMaxImagesExceeded
	}
	return nil
}

func NormalizePagination(limit, offset int32) (int32, int32) {
	if limit <= 0 || limit > MaxPageLimit {
		limit = DefaultPageLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
