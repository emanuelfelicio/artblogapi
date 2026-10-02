package auth

import "github.com/gin-gonic/gin"

func Routes(r *gin.RouterGroup, h *handler, authMiddleware gin.HandlerFunc, rateLimitMiddleware gin.HandlerFunc) {
	if rateLimitMiddleware == nil {
		rateLimitMiddleware = func(c *gin.Context) {
			c.Next()
		}
	}

	auth := r.Group("/auth")
	{
		auth.POST("/register", rateLimitMiddleware, h.Register)
		auth.POST("/login", rateLimitMiddleware, h.Login)
		auth.POST("/refresh", rateLimitMiddleware, h.Refresh)
		auth.POST("/logout", authMiddleware, h.Logout)
	}
}
