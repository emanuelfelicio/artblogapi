package testauth

import (
	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/gin-gonic/gin"
)

// WithPrincipal is a stub middleware that injects a test principal into the Gin context to simulate authentication.
func WithPrincipal(principalID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("auth.principal", auth.AuthPrincipal{UserID: principalID})
		c.Next()
	}
}

// WithoutPrincipal is a stub middleware that rejects requests with 401 Unauthorized to simulate missing authentication.
func WithoutPrincipal() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.AbortWithStatus(401)
	}
}
