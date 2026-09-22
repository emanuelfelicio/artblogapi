package post

import "github.com/gin-gonic/gin"

func Routes(r *gin.RouterGroup, h *handler, authMiddleware gin.HandlerFunc) {
	posts := r.Group("/posts")
	{
		posts.GET("/recent", h.ListRecentPosts)
		posts.GET("/author/:author_id", h.ListPostsByAuthor)
		posts.GET("/:id", h.GetPost)

		protected := posts.Group("", authMiddleware)
		{
			protected.POST("", h.CreatePost)
			protected.PUT("/:id", h.UpdatePost)
			protected.DELETE("/:id", h.DeletePost)
		}
	}
}
