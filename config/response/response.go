package response

import (
	"net/http"

	"github.com/emanuelfelicio/artblogapi/config/validation"
	"github.com/gin-gonic/gin"
)

type Response struct {
	Data  any        `json:"data,omitempty"`
	Error *ErrorInfo `json:"error,omitempty"`
}

type ErrorInfo struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details any       `json:"details,omitempty"`
}

func Fail(c *gin.Context, status int, code ErrorCode, message string, details any) {
	c.JSON(status, Response{Error: &ErrorInfo{Code: code, Message: message, Details: details}})
}

func Success(c *gin.Context, status int, data any) {
	c.JSON(status, Response{Data: data})
}

func ValidationFail(c *gin.Context, fieldErros []validation.FieldError) {
	Fail(c, http.StatusBadRequest, ValidationCode, "invalid inputs", fieldErros)
}
