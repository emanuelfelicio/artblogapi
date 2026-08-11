package storage

import "github.com/google/uuid"

// InitUploadRequest represents the request body for init endpoint
type InitUploadRequest struct {
	Purpose     string `json:"purpose" binding:"required"`
	FileSize    int32  `json:"file_size" binding:"required,min=1"`
	ContentType string `json:"content_type" binding:"required"`
}

// InitUploadResponse is the response returned after creating an upload record
type InitUploadResponse struct {
	UploadID  uuid.UUID `json:"upload_id"`
	UploadURL string    `json:"upload_url"`
}
