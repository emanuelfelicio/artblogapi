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

// GetPublicProfile godoc
//
//	@Summary		Get public profile
//	@Description	Returns public profile information for a given username.
//	@Tags			users
//	@Produce		json
//	@Param			username	path		string	true	"Username"
//	@Success		200			{object}	response.Response[PublicProfileResponse]
//	@Failure		404			{object}	response.ErrorResponse[any]
//	@Failure		500			{object}	response.ErrorResponse[any]
//	@Router			/users/{username} [get]
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

// GetMyProfile godoc
//
//	@Summary		Get my profile
//	@Description	Returns the full profile of the authenticated user.
//	@Tags			users
//	@Security		BearerAuth
//	@Produce		json
//	@Success		200	{object}	response.Response[MyProfileResponse]
//	@Failure		401	{object}	response.ErrorResponse[any]
//	@Failure		404	{object}	response.ErrorResponse[any]
//	@Failure		500	{object}	response.ErrorResponse[any]
//	@Router			/users/me [get]
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

// UpdateProfile godoc
//
//	@Summary		Update profile
//	@Description	Updates the display name and/or bio of the authenticated user.
//	@Tags			users
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		UpdateProfileRequest	true	"Update profile request"
//	@Success		200		{object}	response.Response[MyProfileResponse]
//	@Failure		400		{object}	response.ErrorResponse[[]validation.FieldError]
//	@Failure		401		{object}	response.ErrorResponse[any]
//	@Failure		500		{object}	response.ErrorResponse[any]
//	@Router			/users/me [put]
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

// UpdateAvatar godoc
//
//	@Summary		Update avatar
//	@Description	Sets the avatar of the authenticated user from a completed upload.
//	@Tags			users
//	@Security		BearerAuth
//	@Accept			json
//	@Param			request	body	UpdateAvatarRequest	true	"Update avatar request"
//	@Success		204		"No Content"
//	@Failure		400		{object}	response.ErrorResponse[[]validation.FieldError]
//	@Failure		401		{object}	response.ErrorResponse[any]
//	@Failure		422		{object}	response.ErrorResponse[any]
//	@Failure		500		{object}	response.ErrorResponse[any]
//	@Router			/users/me/avatar [put]
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

// UpdateBanner godoc
//
//	@Summary		Update banner
//	@Description	Sets the banner of the authenticated user from a completed upload.
//	@Tags			users
//	@Security		BearerAuth
//	@Accept			json
//	@Param			request	body	UpdateBannerRequest	true	"Update banner request"
//	@Success		204		"No Content"
//	@Failure		400		{object}	response.ErrorResponse[[]validation.FieldError]
//	@Failure		401		{object}	response.ErrorResponse[any]
//	@Failure		422		{object}	response.ErrorResponse[any]
//	@Failure		500		{object}	response.ErrorResponse[any]
//	@Router			/users/me/banner [put]
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

// mustUserID extracts the authenticated user ID from context. All failure
// branches return 500 because the middleware is responsible for setting the
// principal; reaching here without it indicates a wiring bug, not a client error.
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

// resolveURL builds the full CDN URL for a storage key. When cdnBase is empty
// (e.g. local dev without a CDN), the raw storage key is returned as-is.
func (h *handler) resolveURL(key *string) string {
	if key == nil {
		return ""
	}
	if h.cdnBase == "" {
		return *key
	}
	return h.cdnBase + "/" + *key
}

// toPublicResponse maps the domain model to the public API response.
// Falls back to configured defaults when the user has not set a custom avatar or banner.
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

// toMyResponse maps the domain model to the authenticated user API response.
// Falls back to configured defaults when the user has not set a custom avatar or banner.
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
