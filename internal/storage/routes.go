package storage

import "github.com/gin-gonic/gin"

func Routes(r *gin.RouterGroup, h *handler, authMiddleware gin.HandlerFunc) {
	uploads := r.Group("/uploads", authMiddleware)
	{
		uploads.POST("/init", h.InitUpload)
		uploads.POST("/complete", h.CompleteUpload)
		uploads.GET("/:id", h.GetUploadStatus)
	}
}
