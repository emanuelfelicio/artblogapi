package user

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/emanuelfelicio/artblogapi/config/response"
	"github.com/emanuelfelicio/artblogapi/config/validation"
	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type UserService interface {
	GetPublicProfile(ctx context.Context, username string) (User, error)
	GetMyProfile(ctx context.Context, userID uuid.UUID) (User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, displayName, bio *string) (User, error)
	UpdateAvatar(ctx context.Context, userID uuid.UUID, uploadIDStr string) error
	UpdateBanner(ctx context.Context, userID uuid.UUID, uploadIDStr string) error
}

type handler struct {
	service       UserService
	logger        *slog.Logger
	cdnBase       string
	defaultAvatar string
	defaultBanner string
}

func NewHandler(s UserService, l *slog.Logger, cdnBase, defaultAvatar, defaultBanner string) *handler {
	return &handler{service: s, logger: l, cdnBase: strings.TrimRight(cdnBase, "/"), defaultAvatar: defaultAvatar, defaultBanner: defaultBanner}
}

func (h *handler) GetPublicProfile(c *gin.Context) {
	username := c.Param("username")
	u, err := h.service.GetPublicProfile(c.Request.Context(), username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.Fail(c, http.StatusNotFound, response.NotFoundCode, "user not found")
			return
		}
		h.logger.Error("get_public_profile_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	response.Success(c, http.StatusOK, h.toPublicResponse(u))
}

func (h *handler) GetMyProfile(c *gin.Context) {
	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	u, err := h.service.GetMyProfile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.Fail(c, http.StatusNotFound, response.NotFoundCode, "user not found")
			return
		}
		h.logger.Error("get_my_profile_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	response.Success(c, http.StatusOK, h.toMyResponse(u))
}

func (h *handler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if validationErr, ok := errors.AsType[validator.ValidationErrors](err); ok {
			response.ValidationFail(c, validation.ToFieldError(validationErr))
		} else {
			response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid body")
		}
		return
	}

	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	u, err := h.service.UpdateProfile(c.Request.Context(), userID, req.DisplayName, req.Bio)
	if err != nil {
		h.logger.Error("update_profile_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	response.Success(c, http.StatusOK, h.toMyResponse(u))
}

func (h *handler) UpdateAvatar(c *gin.Context) {
	var req UpdateAvatarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if validationErr, ok := errors.AsType[validator.ValidationErrors](err); ok {
			response.ValidationFail(c, validation.ToFieldError(validationErr))
		} else {
			response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid body")
		}
		return
	}

	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	if err := h.service.UpdateAvatar(c.Request.Context(), userID, req.UploadID); err != nil {
		if errors.Is(err, ErrUploadNotFound) {
			response.Fail(c, http.StatusUnprocessableEntity, response.InvalidCode, "upload not found or not completed")
			return
		}
		h.logger.Error("update_avatar_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	response.SuccessNoContent(c, http.StatusNoContent)
}

func (h *handler) UpdateBanner(c *gin.Context) {
	var req UpdateBannerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if validationErr, ok := errors.AsType[validator.ValidationErrors](err); ok {
			response.ValidationFail(c, validation.ToFieldError(validationErr))
		} else {
			response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid body")
		}
		return
	}

	userID, ok := h.mustUserID(c)
	if !ok {
		return
	}

	if err := h.service.UpdateBanner(c.Request.Context(), userID, req.UploadID); err != nil {
		if errors.Is(err, ErrUploadNotFound) {
			response.Fail(c, http.StatusUnprocessableEntity, response.InvalidCode, "upload not found or not completed")
			return
		}
		h.logger.Error("update_banner_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	response.SuccessNoContent(c, http.StatusNoContent)
}

func (h *handler) mustUserID(c *gin.Context) (uuid.UUID, bool) {
	raw, exists := c.Get("auth.principal")
	if !exists {
		h.logger.Error("auth_principal_missing_from_context")
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return uuid.Nil, false
	}

	principal, ok := raw.(auth.AuthPrincipal)
	if !ok {
		h.logger.Error("invalid_auth_principal", slog.Any("principal", raw))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return uuid.Nil, false
	}

	userID, err := uuid.Parse(principal.UserID)
	if err != nil {
		h.logger.Error("invalid_user_id_in_principal", slog.String("user_id", principal.UserID), slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return uuid.Nil, false
	}

	return userID, true
}

func (h *handler) resolveURL(key *string) string {
	if key == nil {
		return ""
	}
	if h.cdnBase == "" {
		return *key
	}
	return h.cdnBase + "/" + *key
}

func (h *handler) toPublicResponse(u User) PublicProfileResponse {
	avatarURL := h.defaultAvatar
	if u.AvatarKey != nil {
		avatarURL = h.resolveURL(u.AvatarKey)
	}

	bannerURL := h.defaultBanner
	if u.BannerKey != nil {
		bannerURL = h.resolveURL(u.BannerKey)
	}

	return PublicProfileResponse{
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Bio:         u.Bio,
		AvatarURL:   avatarURL,
		BannerURL:   bannerURL,
		CreatedAt:   u.CreatedAt,
	}
}

func (h *handler) toMyResponse(u User) MyProfileResponse {
	avatarURL := h.defaultAvatar
	if u.AvatarKey != nil {
		avatarURL = h.resolveURL(u.AvatarKey)
	}

	bannerURL := h.defaultBanner
	if u.BannerKey != nil {
		bannerURL = h.resolveURL(u.BannerKey)
	}

	return MyProfileResponse{
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		Bio:         u.Bio,
		AvatarURL:   avatarURL,
		BannerURL:   bannerURL,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}
