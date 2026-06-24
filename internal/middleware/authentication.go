package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/emanuelfelicio/artblogapi/config/response"
	"github.com/emanuelfelicio/artblogapi/internal/auth/token"
	"github.com/gin-gonic/gin"
)

const ContextAuthPrincipalKey = "auth.principal"

func Authentication(provider *token.TokenProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			err := fmt.Errorf("missing authorization header")
			response.Fail(c, http.StatusUnauthorized, response.UnauthorizedCode, err.Error())
			c.Error(err)
			c.Abort()
			return
		}

		raw := strings.TrimPrefix(auth, "Bearer ")

		principal, err := provider.VerifyAccessToken(raw)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, response.UnauthorizedCode, "invalid token")
			c.Abort()
			return
		}

		c.Set(ContextAuthPrincipalKey, principal)
		c.Next()
	}
}
