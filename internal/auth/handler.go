package auth

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/emanuelfelicio/artblogapi/config/response"
	"github.com/emanuelfelicio/artblogapi/config/validation"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type RefreshCookieConfig struct {
	Name     string
	Path     string
	Domain   string
	Secure   bool
	HTTPOnly bool
	SameSite http.SameSite
}

const (
	cookieName = "refresh_token"
	cookiePath = "/"
)

func NewRefreshCookieConfig(domain string, secure bool) RefreshCookieConfig {

	var sameSite http.SameSite
	if secure {
		sameSite = http.SameSiteNoneMode
	} else {
		sameSite = http.SameSiteLaxMode
	}

	return RefreshCookieConfig{
		Name:     cookieName,
		Path:     cookiePath,
		Domain:   domain,
		Secure:   secure,
		HTTPOnly: true,
		SameSite: sameSite,
	}
}

type AuthService interface {
	Register(ctx context.Context, username, email, rawPassword, userAgent, ip, deviceID string) (Auth, error)
	Login(ctx context.Context, credential, password, userAgent, ip, deviceID string) (Auth, error)
	Refresh(ctx context.Context, refreshToken, userAgent, ip, deviceID string) (Auth, error)
	Logout(ctx context.Context, refreshToken string, currentUserID uuid.UUID) error
}

type handler struct {
	service AuthService
	logger  *slog.Logger
	cookie  RefreshCookieConfig
}

func NewHandler(s AuthService, l *slog.Logger, cookie RefreshCookieConfig) *handler {
	return &handler{service: s, logger: l, cookie: cookie}
}

// Register godoc
//
//	@Summary		Register a new user
//	@Description	Creates a new user account
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request		body		RegisterRequest	true	"Register request"
//	@Param			X-Device-ID	header		string			false	"Device ID"
//	@Success		201			{object}	response.Response[RegisterResponse]
//	@Failure		400			{object}	response.ErrorResponse[[]validation.FieldError]
//	@Failure		409			{object}	response.ErrorResponse[map[string]bool]
//	@Failure		500			{object}	response.ErrorResponse[any]
//	@Router			/auth/register [post]
func (h *handler) Register(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		if validationErr, ok := errors.AsType[validator.ValidationErrors](err); ok {
			fieldErrors := validation.ToFieldError(validationErr)
			response.ValidationFail(c, fieldErrors)
		} else {
			response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid request body")
		}
		return
	}

	reqCtx := c.Request.Context()
	userAgent := c.Request.UserAgent()
	ip := extractIP(c.Request)
	deviceID := c.Request.Header.Get("X-Device-ID")

	authResult, err := h.service.Register(reqCtx, req.Username, req.Email, req.Password, userAgent, ip, deviceID)
	if err != nil {
		if e, u := errors.Is(err, ErrEmailAlreadyExists), errors.Is(err, ErrUsernameAlreadyExists); e || u {
			response.FailWithDetails(c, http.StatusConflict, response.ConflictCode, "registration conflict", map[string]bool{
				"email_exists":    e,
				"username_exists": u,
			})

		} else {
			h.logger.Error("request_failed", slog.Any("erro", err))
			response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "error trying to register user")
		}
		return
	}

	h.setRefreshTokenCookie(c, authResult.RefreshToken, authResult.RefreshTTL)

	res := RegisterResponse{
		AccessToken: authResult.AccessToken,
		TokenType:   authResult.TokenType,
		ExpiresIn:   authResult.ExpiresIn,
	}

	response.Success(c, http.StatusCreated, res)
}

// Login godoc
//
//	@Summary		User Login
//	@Description	Authenticates user credentials and returns an access token, setting a refresh token cookie.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request		body		LoginRequest	true	"Login request"
//	@Param			X-Device-ID	header		string			false	"Device ID"
//	@Success		200			{object}	response.Response[LoginResponse]
//	@Failure		400			{object}	response.ErrorResponse[[]validation.FieldError]
//	@Failure		401			{object}	response.ErrorResponse[any]
//	@Failure		403			{object}	response.ErrorResponse[any]
//	@Failure		500			{object}	response.ErrorResponse[any]
//	@Router			/auth/login [post]
func (h *handler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		if validationErr, ok := errors.AsType[validator.ValidationErrors](err); ok {
			fieldErrors := validation.ToFieldError(validationErr)
			response.ValidationFail(c, fieldErrors)
		} else {
			response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid request body")
		}
		return
	}

	reqCtx := c.Request.Context()
	userAgent := c.Request.UserAgent()
	ip := extractIP(c.Request)
	deviceID := c.Request.Header.Get("X-Device-ID")

	authResult, err := h.service.Login(reqCtx, req.Credential, req.Password, userAgent, ip, deviceID)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			response.Fail(c, http.StatusUnauthorized, response.UnauthorizedCode, "invalid credentials")
			return
		}

		if errors.Is(err, ErrUserInactive) {
			response.Fail(c, http.StatusForbidden, response.ForbiddenCode, "user inactive")
			return
		}

		h.logger.Error("request_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "error trying to login user")
		return
	}

	h.setRefreshTokenCookie(c, authResult.RefreshToken, authResult.RefreshTTL)

	res := LoginResponse{
		AccessToken: authResult.AccessToken,
		TokenType:   authResult.TokenType,
		ExpiresIn:   authResult.ExpiresIn,
	}

	response.Success(c, http.StatusOK, res)
}

// Refresh godoc
//
//	@Summary		Refresh Access Token
//	@Description	Rotates the refresh token cookie and issues a new access token.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			X-Device-ID	header		string	false	"Device ID"
//	@Success		200			{object}	response.Response[LoginResponse]
//	@Failure		401			{object}	response.ErrorResponse[any]
//	@Failure		500			{object}	response.ErrorResponse[any]
//	@Router			/auth/refresh [post]
func (h *handler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie(h.cookie.Name)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.UnauthorizedCode, "unauthorized")
		return
	}

	reqCtx := c.Request.Context()
	userAgent := c.Request.UserAgent()
	ip := extractIP(c.Request)
	deviceID := c.Request.Header.Get("X-Device-ID")

	authResult, err := h.service.Refresh(reqCtx, refreshToken, userAgent, ip, deviceID)
	if err != nil {
		if IsDomainErr(err) {
			response.Fail(c, http.StatusUnauthorized, response.UnauthorizedCode, "unauthorized")
			return
		}

		h.logger.Error("request_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "error trying to refresh token")
		return
	}

	h.setRefreshTokenCookie(c, authResult.RefreshToken, authResult.RefreshTTL)

	res := LoginResponse{
		AccessToken: authResult.AccessToken,
		TokenType:   authResult.TokenType,
		ExpiresIn:   authResult.ExpiresIn,
	}

	response.Success(c, http.StatusOK, res)
}

// Logout godoc
//
//	@Summary		User Logout
//	@Description	Revokes the active session and clears the refresh token cookie.
//	@Tags			auth
//	@Security		BearerAuth
//	@Success		204	"No Content"
//	@Failure		401	{object}	response.ErrorResponse[any]
//	@Failure		500	{object}	response.ErrorResponse[any]
//	@Router			/auth/logout [post]
func (h *handler) Logout(c *gin.Context) {
	refreshToken, _ := c.Cookie(h.cookie.Name)

	principalAny, exists := c.Get("auth.principal")
	if !exists {
		h.logger.Error("auth_principal_missing_from_context")
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "error trying to logout")
		return
	}

	principal, ok := principalAny.(AuthPrincipal)
	if !ok {
		h.logger.Error("logout_failed_invalid_principal", slog.Any("principal", principalAny))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "error trying to logout")
		return
	}

	userID, err := uuid.Parse(principal.UserID)
	if err != nil {
		h.logger.Error("logout_failed_invalid_uuid", slog.String("user_id", principal.UserID), slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "error trying to logout")
		return
	}

	if refreshToken != "" {
		if err := h.service.Logout(c.Request.Context(), refreshToken, userID); err != nil {
			h.logger.Error("request_failed", slog.Any("err", err))
		}
	}

	h.clearRefreshTokenCookie(c)
	response.SuccessNoContent(c, http.StatusNoContent)
}

func (h *handler) setRefreshTokenCookie(c *gin.Context, token string, ttl int) {
	c.SetSameSite(h.cookie.SameSite)
	c.SetCookie(
		h.cookie.Name,
		token,
		ttl,
		h.cookie.Path,
		h.cookie.Domain,
		h.cookie.Secure,
		h.cookie.HTTPOnly,
	)
}

func (h *handler) clearRefreshTokenCookie(c *gin.Context) {
	c.SetCookie(
		h.cookie.Name,
		"",
		-1,
		h.cookie.Path,
		h.cookie.Domain,
		h.cookie.Secure,
		h.cookie.HTTPOnly,
	)
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		ip := strings.TrimSpace(ips[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
