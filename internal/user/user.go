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

type UserUpload struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Status    string
	Purpose   string
	ObjectKey string
}
