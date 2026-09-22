package user

import "github.com/gin-gonic/gin"

func Routes(r *gin.RouterGroup, h *handler, authMiddleware gin.HandlerFunc) {
	users := r.Group("/users")
	{
		users.GET("/:username", h.GetPublicProfile)
		users.POST("/batch", h.BatchGetPublicProfiles)

		protected := users.Group("/me", authMiddleware)
		{
			protected.GET("", h.GetMyProfile)
			protected.PUT("", h.UpdateProfile)
			protected.PUT("/avatar", h.UpdateAvatar)
			protected.DELETE("/avatar", h.DeleteAvatar)
			protected.PUT("/banner", h.UpdateBanner)
			protected.DELETE("/banner", h.DeleteBanner)
		}
	}
}
