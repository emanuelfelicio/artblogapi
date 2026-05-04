package auth

import "github.com/gin-gonic/gin"

func Routes(r *gin.RouterGroup, h *handler) {

	auth := r.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login")
	}
}
