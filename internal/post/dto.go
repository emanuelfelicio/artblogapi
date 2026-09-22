package post

import "time"

type CreatePostRequest struct {
	Title          string   `json:"title" binding:"required,min=1,max=150"`
	Content        string   `json:"content" binding:"required"`
	ImageUploadIDs []string `json:"image_upload_ids" binding:"omitempty,max=10,dive,uuid"`
}

type UpdatePostRequest struct {
	Title          *string  `json:"title" binding:"omitempty,min=1,max=150"`
	Content        *string  `json:"content" binding:"omitempty"`
	ImageUploadIDs []string `json:"image_upload_ids" binding:"omitempty,max=10,dive,uuid"`
}

type PostResponse struct {
	ID        string              `json:"id"`
	Title     string              `json:"title"`
	Content   string              `json:"content"`
	Author    PostAuthorResponse  `json:"author"`
	Images    []PostImageResponse `json:"images"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type PostAuthorResponse struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
}

type PostImageResponse struct {
	UploadID string `json:"upload_id"`
	URL      string `json:"url"`
	Position int    `json:"position"`
}
