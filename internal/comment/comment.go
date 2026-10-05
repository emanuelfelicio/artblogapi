package comment

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MinContentLength = 1
	MaxContentLength = 500
	DefaultPageLimit = 20
	MaxPageLimit     = 50
)

type Comment struct {
	ID        uuid.UUID
	PostID    uuid.UUID
	AuthorID  uuid.UUID
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func ValidateContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(content) < MinContentLength || utf8.RuneCountInString(content) > MaxContentLength {
		return "", ErrInvalidContent
	}
	return content, nil
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
