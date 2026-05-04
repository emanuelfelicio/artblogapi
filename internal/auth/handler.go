package auth

import (
	"errors"
	"net/http"

	"github.com/emanuelfelicio/artblogapi/config/response"
	"github.com/gin-gonic/gin"
)

type handler struct {
	service *service
}

func NewHandler(s *service) *handler {
	return &handler{service: s}
}

func (h *handler) Register(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body", nil)
		return
	}

	reqCtx := c.Request.Context()
	authResult, err := h.service.Register(reqCtx, req.Username, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) || errors.Is(err, ErrUsernameAlreadyExists) {
			response.Fail(c, http.StatusConflict, "CONFLICT", err.Error(), nil)
			return
		}
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "error trying to register user", nil)
		return
	}

	res := RegisterResponse{
		AccessToken: authResult.AccessToken,
		User: RegisterUserResponse{
			ID:       authResult.User.ID.String(),
			Username: authResult.User.Username,
			Email:    authResult.User.Email,
		},
	}

	response.Success(c, http.StatusCreated, res)
}
