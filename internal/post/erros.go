package post

import "errors"

var (
	ErrPostNotFound       = errors.New("post not found")
	ErrPostForbidden      = errors.New("forbidden: you are not the author of this post")
	ErrMaxImagesExceeded  = errors.New("a post can have at most 10 images")
	ErrDuplicateUploadID  = errors.New("duplicate upload id in post images")
	ErrInvalidPostTitle   = errors.New("invalid post title")
	ErrInvalidPostContent = errors.New("invalid post content")
)
