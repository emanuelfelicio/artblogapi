package comment

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/emanuelfelicio/artblogapi/config/response"
	"github.com/emanuelfelicio/artblogapi/config/validation"
	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type handler struct {
	service Service
	logger  *slog.Logger
}

func NewHandler(service Service, logger *slog.Logger) *handler {
	return &handler{service: service, logger: logger}
}

// List godoc
//
//	@Summary		List comments
//	@Description	Returns non-deleted comments for a post, newest first.
//	@Tags			comments
//	@Produce		json
//	@Param			post_id	path		string	true	"Post UUID"
//	@Param			limit	query		int		false	"Number of comments (default 20, max 50)"
//	@Param			offset	query		int		false	"Offset for pagination"
//	@Success		200		{object}	response.Response[[]CommentResponse]
//	@Failure		400		{object}	response.ErrorResponse[any]
//	@Failure		404		{object}	response.ErrorResponse[any]
//	@Router			/posts/{post_id}/comments [get]
func (h *handler) List(c *gin.Context) {
	postID, err := uuid.Parse(c.Param("post_id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid post id")
		return
	}
	limit, offset := parsePagination(c)
	comments, err := h.service.ListByPost(c.Request.Context(), postID, limit, offset)
	if err != nil {
		h.handleError(c, err, "list_comments_failed")
		return
	}
	result := make([]CommentResponse, 0, len(comments))
	for _, item := range comments {
		result = append(result, toResponse(item))
	}
	response.Success(c, http.StatusOK, result)
}

// Create godoc
//
//	@Summary		Create comment
//	@Tags			comments
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			post_id	path		string			true	"Post UUID"
//	@Param			request	body		CommentRequest	true	"Comment request"
//	@Success		201		{object}	response.Response[CommentResponse]
//	@Failure		400		{object}	response.ErrorResponse[any]
//	@Failure		401		{object}	response.ErrorResponse[any]
//	@Failure		404		{object}	response.ErrorResponse[any]
//	@Router			/posts/{post_id}/comments [post]
func (h *handler) Create(c *gin.Context) {
	postID, err := uuid.Parse(c.Param("post_id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid post id")
		return
	}
	var req CommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if validationErr, ok := errors.AsType[validator.ValidationErrors](err); ok {
			response.ValidationFail(c, validation.ToFieldError(validationErr))
		} else {
			response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid body")
		}
		return
	}
	if _, err := ValidateContent(req.Content); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ValidationCode, "invalid comment content")
		return
	}
	authorID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}
	item, err := h.service.Create(c.Request.Context(), postID, authorID, req.Content)
	if err != nil {
		h.handleError(c, err, "create_comment_failed")
		return
	}
	response.Success(c, http.StatusCreated, toResponse(item))
}

// Update godoc
//
//	@Summary		Update comment
//	@Tags			comments
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			post_id	path		string			true	"Post UUID"
//	@Param			id		path		string			true	"Comment UUID"
//	@Param			request	body		CommentRequest	true	"Comment request"
//	@Success		200		{object}	response.Response[CommentResponse]
//	@Failure		400		{object}	response.ErrorResponse[any]
//	@Failure		401		{object}	response.ErrorResponse[any]
//	@Failure		403		{object}	response.ErrorResponse[any]
//	@Failure		404		{object}	response.ErrorResponse[any]
//	@Router			/posts/{post_id}/comments/{id} [put]
func (h *handler) Update(c *gin.Context) {
	postID, commentID, ok := h.parseIDs(c)
	if !ok {
		return
	}
	var req CommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if validationErr, ok := errors.AsType[validator.ValidationErrors](err); ok {
			response.ValidationFail(c, validation.ToFieldError(validationErr))
		} else {
			response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid body")
		}
		return
	}
	if _, err := ValidateContent(req.Content); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ValidationCode, "invalid comment content")
		return
	}
	authorID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}
	item, err := h.service.Update(c.Request.Context(), postID, commentID, authorID, req.Content)
	if err != nil {
		h.handleError(c, err, "update_comment_failed")
		return
	}
	response.Success(c, http.StatusOK, toResponse(item))
}

// Delete godoc
//
//	@Summary		Delete comment
//	@Description	Soft-deletes a comment owned by the authenticated user.
//	@Tags			comments
//	@Security		BearerAuth
//	@Param			post_id	path	string	true	"Post UUID"
//	@Param			id		path	string	true	"Comment UUID"
//	@Success		204
//	@Failure		400	{object}	response.ErrorResponse[any]
//	@Failure		401	{object}	response.ErrorResponse[any]
//	@Failure		403	{object}	response.ErrorResponse[any]
//	@Failure		404	{object}	response.ErrorResponse[any]
//	@Router			/posts/{post_id}/comments/{id} [delete]
func (h *handler) Delete(c *gin.Context) {
	postID, commentID, ok := h.parseIDs(c)
	if !ok {
		return
	}
	authorID, err := auth.GetUserID(c)
	if err != nil {
		h.logger.Error("auth_error", slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
		return
	}
	if err := h.service.Delete(c.Request.Context(), postID, commentID, authorID); err != nil {
		h.handleError(c, err, "delete_comment_failed")
		return
	}
	response.SuccessNoContent(c, http.StatusNoContent)
}

func (h *handler) parseIDs(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	postID, err := uuid.Parse(c.Param("post_id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid post id")
		return uuid.Nil, uuid.Nil, false
	}
	commentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ParseCode, "invalid comment id")
		return uuid.Nil, uuid.Nil, false
	}
	return postID, commentID, true
}

func (h *handler) handleError(c *gin.Context, err error, logMessage string) {
	switch {
	case errors.Is(err, ErrInvalidContent):
		response.Fail(c, http.StatusBadRequest, response.ValidationCode, "invalid comment content")
	case errors.Is(err, ErrPostNotFound), errors.Is(err, ErrCommentNotFound), errors.Is(err, ErrCommentDeleted):
		response.Fail(c, http.StatusNotFound, response.NotFoundCode, "comment or post not found")
	case errors.Is(err, ErrCommentForbidden):
		response.Fail(c, http.StatusForbidden, response.ForbiddenCode, "forbidden")
	default:
		h.logger.Error(logMessage, slog.Any("err", err))
		response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal error")
	}
}

func parsePagination(c *gin.Context) (int32, int32) {
	var limit, offset int32
	if value, err := strconv.Atoi(c.Query("limit")); err == nil {
		limit = int32(value)
	}
	if value, err := strconv.Atoi(c.Query("offset")); err == nil {
		offset = int32(value)
	}
	return limit, offset
}

func toResponse(item Comment) CommentResponse {
	return CommentResponse{ID: item.ID.String(), PostID: item.PostID.String(), AuthorID: item.AuthorID.String(), Content: item.Content, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
