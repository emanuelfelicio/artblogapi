package post

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/emanuelfelicio/artblogapi/config/response"
	"github.com/emanuelfelicio/artblogapi/config/validation"
	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/emanuelfelicio/artblogapi/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type PostService interface {
	CreatePost(ctx context.Context, authorID uuid.UUID, title, content string, imageUploadIDs []string) (Post, error)
	GetPost(ctx context.Context, id uuid.UUID) (Post, error)
	ListRecentPosts(ctx context.Context, limit, offset int32) ([]Post, error)
	ListPostsByAuthor(ctx context.Context, authorID uuid.UUID, limit, offset int32) ([]Post, error)
	UpdatePost(ctx context.Context, postID, authorID uuid.UUID, title, content *string, imageUploadIDs []string) (Post, error)
	DeletePost(ctx context.Context, postID, authorID uuid.UUID) error
}

type handler struct {
	service       PostService
	logger        *slog.Logger
	mediaBaseURL  string
	defaultAvatar string
}

func NewHandler(s PostService, l *slog.Logger, mediaBaseURL, defaultAvatar string) *handler {
	return &handler{
		service:       s,
		logger:        l,
		mediaBaseURL:  mediaBaseURL,
		defaultAvatar: defaultAvatar,
	}
}

// CreatePost godoc
//
//	@Summary		Create post
//	@Description	Creates a new post with optional carousel images (max 10).
//	@Tags			posts
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreatePostRequest	true	"Create post request"
//	@Success		201		{object}	response.Response[PostResponse]
//	@Failure		400		{object}	response.ErrorResponse[any]
//	@Failure		401		{object}	response.ErrorResponse[any]
//	@Failure		500		{object}	response.ErrorResponse[any]
//	@Router			/posts [post]
func (h *handler) CreatePost(c *gin.Context) {
	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if validationErr, ok := errors.AsType[validator.ValidationErrors](err); ok {
			response.ValidationFail(c, validation.ToFieldError(validationErr))
		} else {
			response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid body")
		}
		return
	}

	userID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	p, err := h.service.CreatePost(c.Request.Context(), userID, req.Title, req.Content, req.ImageUploadIDs)
	if err != nil {
		h.handleServiceError(c, err, "create_post_failed")
		return
	}

	response.Success(c, http.StatusCreated, h.toPostResponse(p))
}

// GetPost godoc
//
//	@Summary		Get post details
//	@Description	Retrieves post details including author and ordered carousel images.
//	@Tags			posts
//	@Produce		json
//	@Param			id	path		string	true	"Post ID"
//	@Success		200	{object}	response.Response[PostResponse]
//	@Failure		400	{object}	response.ErrorResponse[any]
//	@Failure		404	{object}	response.ErrorResponse[any]
//	@Failure		500	{object}	response.ErrorResponse[any]
//	@Router			/posts/{id} [get]
func (h *handler) GetPost(c *gin.Context) {
	idStr := c.Param("id")
	postID, err := uuid.Parse(idStr)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid post id")
		return
	}

	p, err := h.service.GetPost(c.Request.Context(), postID)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			response.Fail(c, http.StatusNotFound, response.NotFoundCode, "post not found")
			return
		}
		h.logger.Error("get_post_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	response.Success(c, http.StatusOK, h.toPostResponse(p))
}

// ListRecentPosts godoc
//
//	@Summary		List recent posts
//	@Description	Returns a paginated list of recent posts with images.
//	@Tags			posts
//	@Produce		json
//	@Param			limit	query		int	false	"Number of posts to return (default 20, max 50)"
//	@Param			offset	query		int	false	"Offset for pagination (default 0)"
//	@Success		200		{object}	response.Response[[]PostResponse]
//	@Failure		500		{object}	response.ErrorResponse[any]
//	@Router			/posts/recent [get]
func (h *handler) ListRecentPosts(c *gin.Context) {
	var limit, offset int32
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil {
			limit = int32(l)
		}
	}
	if oStr := c.Query("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil {
			offset = int32(o)
		}
	}

	posts, err := h.service.ListRecentPosts(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("list_recent_posts_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	res := make([]PostResponse, 0, len(posts))
	for _, p := range posts {
		res = append(res, h.toPostResponse(p))
	}

	response.Success(c, http.StatusOK, res)
}

// ListPostsByAuthor godoc
//
//	@Summary		List posts by author
//	@Description	Returns a paginated list of posts by a specific author.
//	@Tags			posts
//	@Produce		json
//	@Param			author_id	path		string	true	"Author UUID"
//	@Param			limit		query		int		false	"Number of posts to return (default 20, max 50)"
//	@Param			offset		query		int		false	"Offset for pagination (default 0)"
//	@Success		200			{object}	response.Response[[]PostResponse]
//	@Failure		400			{object}	response.ErrorResponse[any]
//	@Failure		500			{object}	response.ErrorResponse[any]
//	@Router			/posts/author/{author_id} [get]
func (h *handler) ListPostsByAuthor(c *gin.Context) {
	authorIDStr := c.Param("author_id")
	authorID, err := uuid.Parse(authorIDStr)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid author id")
		return
	}

	var limit, offset int32
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil {
			limit = int32(l)
		}
	}
	if oStr := c.Query("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil {
			offset = int32(o)
		}
	}

	posts, err := h.service.ListPostsByAuthor(c.Request.Context(), authorID, limit, offset)
	if err != nil {
		h.logger.Error("list_posts_by_author_failed", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	res := make([]PostResponse, 0, len(posts))
	for _, p := range posts {
		res = append(res, h.toPostResponse(p))
	}

	response.Success(c, http.StatusOK, res)
}

// UpdatePost godoc
//
//	@Summary		Update post
//	@Description	Updates post title, content, and/or reconciles the image carousel.
//	@Tags			posts
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Post ID"
//	@Param			request	body		UpdatePostRequest	true	"Update post request"
//	@Success		200		{object}	response.Response[PostResponse]
//	@Failure		400		{object}	response.ErrorResponse[any]
//	@Failure		401		{object}	response.ErrorResponse[any]
//	@Failure		403		{object}	response.ErrorResponse[any]
//	@Failure		404		{object}	response.ErrorResponse[any]
//	@Failure		500		{object}	response.ErrorResponse[any]
//	@Router			/posts/{id} [put]
func (h *handler) UpdatePost(c *gin.Context) {
	idStr := c.Param("id")
	postID, err := uuid.Parse(idStr)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid post id")
		return
	}

	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if validationErr, ok := errors.AsType[validator.ValidationErrors](err); ok {
			response.ValidationFail(c, validation.ToFieldError(validationErr))
		} else {
			response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid body")
		}
		return
	}

	userID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	p, err := h.service.UpdatePost(c.Request.Context(), postID, userID, req.Title, req.Content, req.ImageUploadIDs)
	if err != nil {
		h.handleServiceError(c, err, "update_post_failed")
		return
	}

	response.Success(c, http.StatusOK, h.toPostResponse(p))
}

// DeletePost godoc
//
//	@Summary		Delete post
//	@Description	Deletes a post and marks associated media as superseded.
//	@Tags			posts
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Post ID"
//	@Success		204	"No Content"
//	@Failure		400	{object}	response.ErrorResponse[any]
//	@Failure		401	{object}	response.ErrorResponse[any]
//	@Failure		403	{object}	response.ErrorResponse[any]
//	@Failure		404	{object}	response.ErrorResponse[any]
//	@Failure		500	{object}	response.ErrorResponse[any]
//	@Router			/posts/{id} [delete]
func (h *handler) DeletePost(c *gin.Context) {
	idStr := c.Param("id")
	postID, err := uuid.Parse(idStr)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid post id")
		return
	}

	userID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}

	if err := h.service.DeletePost(c.Request.Context(), postID, userID); err != nil {
		h.handleServiceError(c, err, "delete_post_failed")
		return
	}

	response.SuccessNoContent(c, http.StatusNoContent)
}

func (h *handler) handleServiceError(c *gin.Context, err error, logMsg string) {
	switch {
	case errors.Is(err, ErrPostNotFound):
		response.Fail(c, http.StatusNotFound, response.NotFoundCode, "post not found")
	case errors.Is(err, ErrPostForbidden):
		response.Fail(c, http.StatusForbidden, "FORBIDDEN", "forbidden: you cannot modify this post")
	case errors.Is(err, ErrMaxImagesExceeded):
		response.Fail(c, http.StatusBadRequest, response.ValidationCode, "maximum 10 images allowed per post")
	case errors.Is(err, ErrDuplicateUploadID):
		response.Fail(c, http.StatusBadRequest, response.ValidationCode, "duplicate image upload id")
	case errors.Is(err, ErrInvalidPostTitle):
		response.Fail(c, http.StatusBadRequest, response.ValidationCode, "invalid post title")
	case errors.Is(err, ErrInvalidPostContent):
		response.Fail(c, http.StatusBadRequest, response.ValidationCode, "invalid post content")
	case errors.Is(err, storage.ErrUploadNotFound):
		response.Fail(c, http.StatusBadRequest, response.NotFoundCode, "image upload not found")
	case errors.Is(err, storage.ErrUploadNotCompleted):
		response.Fail(c, http.StatusBadRequest, "UPLOAD_NOT_READY", "image upload is not ready (must be completed)")
	case errors.Is(err, storage.ErrUploadInvalidPurpose):
		response.Fail(c, http.StatusBadRequest, "INVALID_PURPOSE", "image upload purpose must be POST_IMAGE")
	default:
		h.logger.Error(logMsg, slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
	}
}

func (h *handler) toPostResponse(p Post) PostResponse {
	var avatarURL *string
	if p.Author.AvatarKey != nil && *p.Author.AvatarKey != "" {
		url := h.resolveURL(p.Author.AvatarKey)
		avatarURL = &url
	} else if h.defaultAvatar != "" {
		avatarURL = &h.defaultAvatar
	}

	images := make([]PostImageResponse, 0, len(p.Images))
	for _, img := range p.Images {
		images = append(images, PostImageResponse{
			UploadID: img.UploadID.String(),
			URL:      h.resolveURL(&img.ObjectKey),
			Position: img.Position,
		})
	}

	return PostResponse{
		ID:      p.ID.String(),
		Title:   p.Title,
		Content: p.Content,
		Author: PostAuthorResponse{
			ID:          p.Author.ID.String(),
			Username:    p.Author.Username,
			DisplayName: p.Author.DisplayName,
			AvatarURL:   avatarURL,
		},
		Images:    images,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func (h *handler) resolveURL(key *string) string {
	if key == nil || *key == "" {
		return ""
	}
	if h.mediaBaseURL == "" {
		return *key
	}
	return h.mediaBaseURL + "/" + *key
}
