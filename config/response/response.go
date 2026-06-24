package response

import (
	"net/http"

	"github.com/emanuelfelicio/artblogapi/config/validation"
	"github.com/gin-gonic/gin"
)

type Response[T any] struct {
	Data T `json:"data"`
}

type ErrorResponse[D any] struct {
	Error *ErrorInfo[D] `json:"error"`
}

type ErrorInfo[D any] struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details D         `json:"details,omitempty"`
}

func Fail(c *gin.Context, status int, code ErrorCode, message string) {
	c.JSON(status, ErrorResponse[any]{
		Error: &ErrorInfo[any]{
			Code:    code,
			Message: message,
		},
	})
}

func FailWithDetails[D any](c *gin.Context, status int, code ErrorCode, message string, details D) {
	c.JSON(status, ErrorResponse[D]{
		Error: &ErrorInfo[D]{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func Success[T any](c *gin.Context, status int, data T) {
	c.JSON(status, Response[T]{Data: data})
}

func SuccessNoContent(c *gin.Context, status int) {
	c.Status(status)
}

func ValidationFail(c *gin.Context, fieldErros []validation.FieldError) {
	FailWithDetails(c, http.StatusBadRequest, ValidationCode, "invalid inputs", fieldErros)
}
