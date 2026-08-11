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
	CompleteUpload(ctx context.Context, userID uuid.UUID, uploadID uuid.UUID) error
	GetUploadStatus(ctx context.Context, userID uuid.UUID, uploadID uuid.UUID) (Upload, error)
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
		if errors.Is(err, ErrFileSizeExceeded) || errors.Is(err, ErrInvalidFileSize) || errors.Is(err, ErrInvaliImageContentType) || errors.Is(err, ErrInvalidPurpose) {
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

func (h *handler) CompleteUpload(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	var req CompleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			response.ValidationFail(c, validation.ToFieldError(ve))
			return
		}
		response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid request body")
		return
	}

	uploadID, err := uuid.Parse(req.UploadID)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ValidationCode, "invalid upload id format")
		return
	}

	err = h.service.CompleteUpload(c.Request.Context(), userID, uploadID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUploadNotFound):
			response.Fail(c, http.StatusNotFound, response.NotFoundCode, err.Error())
		case errors.Is(err, ErrUploadNotOwned):
			response.Fail(c, http.StatusForbidden, response.ForbiddenCode, err.Error())
		case errors.Is(err, ErrUploadNotPending):
			response.Fail(c, http.StatusConflict, response.ConflictCode, err.Error())
		case errors.Is(err, ErrFileNotInQuarantine):
			response.Fail(c, http.StatusUnprocessableEntity, response.ValidationCode, err.Error())
		default:
			h.logger.Error("complete_upload_failed", slog.Any("err", err))
			response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		}
		return
	}

	response.SuccessNoContent(c, http.StatusAccepted)
}

func (h *handler) GetUploadStatus(c *gin.Context) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	idParam := c.Param("id")
	uploadID, err := uuid.Parse(idParam)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ValidationCode, "invalid upload id format")
		return
	}

	upload, err := h.service.GetUploadStatus(c.Request.Context(), userID, uploadID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUploadNotFound):
			response.Fail(c, http.StatusNotFound, response.NotFoundCode, err.Error())
		case errors.Is(err, ErrUploadNotOwned):
			response.Fail(c, http.StatusForbidden, response.ForbiddenCode, err.Error())
		default:
			h.logger.Error("get_upload_status_failed", slog.Any("err", err))
			response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		}
		return
	}

	response.Success(c, http.StatusOK, UploadStatusResponse{
		ID:            upload.ID,
		Status:        upload.Status,
		FailureReason: upload.FailureReason,
	})
}
