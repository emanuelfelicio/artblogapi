package comment

import (
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.RouterGroup, h *handler, authMiddleware gin.HandlerFunc) {
	posts := r.Group("/posts/:post_id/comments")
	posts.GET("", h.List)
	protected := posts.Group("", authMiddleware)
	protected.POST("", h.Create)
	protected.PUT("/:id", h.Update)
	protected.DELETE("/:id", h.Delete)
}
