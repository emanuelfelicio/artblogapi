package response

import "github.com/gin-gonic/gin"

type Response struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Error   *ErrorInfo `json:"error,omitempty"`
}

type ErrorInfo struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Causes  []Cause `json:"causes,omitempty"`
}

type Cause struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func Fail(c *gin.Context, status int, code string, message string, causes []Cause) {
	c.JSON(status, Response{Success: false, Error: &ErrorInfo{Code: code, Message: message, Causes: causes}})
}

func Success(c *gin.Context, status int, data any) {
	c.JSON(status, Response{Success: true, Data: data})
}
