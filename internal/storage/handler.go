package storage

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/emanuelfelicio/artblogapi/config/response"
	"github.com/emanuelfelicio/artblogapi/config/validation"
	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type StorageService interface {
	InitUpload(ctx context.Context, userID uuid.UUID, purpose string, fileSize int, contentType string) (uuid.UUID, string, error)
}

type handler struct {
	service StorageService
	logger  *slog.Logger
}

func NewHandler(s StorageService, l *slog.Logger) *handler {
	return &handler{service: s, logger: l}
}

func (h *handler) InitUpload(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	var req InitUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			response.ValidationFail(c, validation.ToFieldError(ve))
			return
		}
		response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid request body")
		return
	}

	uploadID, uploadURL, err := h.service.InitUpload(c.Request.Context(), userID, req.Purpose, int(req.FileSize), req.ContentType)
	if err != nil {
		if errors.Is(err, ErrInvalidPurpose) {
			response.Fail(c, http.StatusBadRequest, response.ValidationCode, "invalid upload purpose")
			return
		}
		if errors.Is(err, ErrFileSizeExceeded) || errors.Is(err, ErrInvalidFileSize) || errors.Is(err, ErrInvaliImageContentType) {
			response.Fail(c, http.StatusBadRequest, response.ValidationCode, err.Error())
			return
		}

		h.logger.Error("init_upload_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	response.Success(c, http.StatusCreated, InitUploadResponse{
		UploadID:  uploadID,
		UploadURL: uploadURL,
	})
}
