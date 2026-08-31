package user

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
	mediaBaseURL  string
	defaultAvatar string
	defaultBanner string
}

func NewHandler(s UserService, l *slog.Logger, mediaBaseURL, defaultAvatar, defaultBanner string) *handler {
	return &handler{service: s, logger: l, mediaBaseURL: mediaBaseURL, defaultAvatar: defaultAvatar, defaultBanner: defaultBanner}
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
	userID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
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

	userID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
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

	userID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	if err := h.service.UpdateAvatar(c.Request.Context(), userID, req.UploadID); err != nil {
		if errors.Is(err, ErrUploadNotFound) || errors.Is(err, ErrUploadNotCompleted) || errors.Is(err, ErrUploadInvalidPurpose) {
			response.Fail(c, http.StatusUnprocessableEntity, response.InvalidCode, "upload not found or invalid")
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

	userID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	if err := h.service.UpdateBanner(c.Request.Context(), userID, req.UploadID); err != nil {
		if errors.Is(err, ErrUploadNotFound) || errors.Is(err, ErrUploadNotCompleted) || errors.Is(err, ErrUploadInvalidPurpose) {
			response.Fail(c, http.StatusUnprocessableEntity, response.InvalidCode, "upload not found or invalid")
			return
		}
		h.logger.Error("update_banner_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	response.SuccessNoContent(c, http.StatusNoContent)
}

// resolveURL builds the full public URL for a storage key. When mediaBaseURL is empty
// the raw storage key is returned as-is.
func (h *handler) resolveURL(key *string) string {
	if key == nil || *key == "" {
		return ""
	}
	if h.mediaBaseURL == "" {
		return *key
	}
	return h.mediaBaseURL + "/" + *key
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
