package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/emanuelfelicio/artblogapi/config/response"
	"github.com/emanuelfelicio/artblogapi/config/validation"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type handler struct {
	service *service
	logger  *slog.Logger
}

func NewHandler(s *service, l *slog.Logger) *handler {
	return &handler{service: s, logger: l}
}

func (h *handler) Register(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		if validationErr, ok := errors.AsType[validator.ValidationErrors](err); ok {
			fieldErrors := validation.ToFieldError(validationErr)
			response.ValidationFail(c, fieldErrors)
		} else {
			response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid request body", nil)
		}
		return
	}

	reqCtx := c.Request.Context()
	authResult, err := h.service.Register(reqCtx, req.Username, req.Email, req.Password)
	if err != nil {
		if e, u := errors.Is(err, ErrEmailAlreadyExists), errors.Is(err, ErrUsernameAlreadyExists); e || u {
			response.Fail(c, http.StatusConflict, response.ConflictCode, "registration conflict", map[string]bool{
				"email_exists":    e,
				"username_exists": u,
			})

		} else {
			h.logger.Error("request_failed", slog.Any("erro", err))
			response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "error trying to register user", nil)
		}
		return
	}

	res := RegisterResponse{
		AccessToken: authResult.AccessToken,
		TokenType:   authResult.TokenType,
		ExpiresIn:   authResult.ExpiresIn,
	}

	response.Success(c, http.StatusCreated, res)
}

func (h *handler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		if validationErr, ok := errors.AsType[validator.ValidationErrors](err); ok {
			fieldErrors := validation.ToFieldError(validationErr)
			response.ValidationFail(c, fieldErrors)
		} else {
			response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid request body", nil)
		}
		return
	}

	reqCtx := c.Request.Context()
	authResult, err := h.service.Login(reqCtx, req.Credential, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			response.Fail(c, http.StatusUnauthorized, response.UnauthorizedCode, "invalid credentials", nil)
			return
		}

		if errors.Is(err, ErrUserInactive) {
			response.Fail(c, http.StatusForbidden, response.ForbiddenCode, "user inactive", nil)
			return
		}

		h.logger.Error("request_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "error trying to login user", nil)
		return
	}

	res := LoginResponse{
		AccessToken: authResult.AccessToken,
		TokenType:   authResult.TokenType,
		ExpiresIn:   authResult.ExpiresIn,
	}

	response.Success(c, http.StatusOK, res)
}
