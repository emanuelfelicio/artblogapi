package comment

import "errors"

var (
	ErrCommentNotFound  = errors.New("comment not found")
	ErrPostNotFound     = errors.New("post not found")
	ErrCommentForbidden = errors.New("forbidden: you are not the author of this comment")
	ErrCommentDeleted   = errors.New("comment deleted")
	ErrInvalidContent   = errors.New("invalid comment content")
)
