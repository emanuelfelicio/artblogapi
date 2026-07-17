package user

import "time"

type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name" binding:"omitempty,max=60"`
	Bio         *string `json:"bio" binding:"omitempty,max=500"`
}

type UpdateAvatarRequest struct {
	UploadID string `json:"upload_id" binding:"required,uuid"`
}

type UpdateBannerRequest struct {
	UploadID string `json:"upload_id" binding:"required,uuid"`
}

type PublicProfileResponse struct {
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	AvatarURL   string    `json:"avatar_url"`
	BannerURL   string    `json:"banner_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type MyProfileResponse struct {
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	Bio         string    `json:"bio"`
	AvatarURL   string    `json:"avatar_url"`
	BannerURL   string    `json:"banner_url"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
