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

// CompleteUploadRequest represents the request to complete an upload
type CompleteUploadRequest struct {
	UploadID string `json:"upload_id" binding:"required,uuid"`
}

// UploadStatusResponse represents the status response of an upload
type UploadStatusResponse struct {
	ID            uuid.UUID    `json:"id"`
	Status        UploadStatus `json:"status"`
	FailureReason *string      `json:"failure_reason"`
}
