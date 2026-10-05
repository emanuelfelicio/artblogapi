package post

import "github.com/gin-gonic/gin"

func Routes(r *gin.RouterGroup, h *handler, authMiddleware, optionalAuthMiddleware gin.HandlerFunc) {
	posts := r.Group("/posts")
	{
		public := posts.Group("", optionalAuthMiddleware)
		public.GET("/recent", h.ListRecentPosts)
		public.GET("/author/:author_id", h.ListPostsByAuthor)
		public.GET("/:id", h.GetPost)

		protected := posts.Group("", authMiddleware)
		{
			protected.POST("", h.CreatePost)
			protected.PUT("/:id", h.UpdatePost)
			protected.DELETE("/:id", h.DeletePost)
			protected.PUT("/:id/like", h.LikePost)
			protected.DELETE("/:id/like", h.UnlikePost)
		}
	}
}
