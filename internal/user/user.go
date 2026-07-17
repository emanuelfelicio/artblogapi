package user

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID
	Username    string
	Email       string
	DisplayName string
	Bio         string
	AvatarKey   *string
	BannerKey   *string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
