package user

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID             uuid.UUID
	Username       string
	Email          string
	DisplayName    string
	Bio            string
	AvatarKey      *string
	BannerKey      *string
	AvatarUploadID *uuid.UUID
	BannerUploadID *uuid.UUID
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UserSummary struct {
	ID          uuid.UUID
	Username    string
	DisplayName string
	AvatarKey   *string
}
