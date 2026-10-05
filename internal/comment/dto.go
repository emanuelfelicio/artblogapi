package comment

import "time"

type CommentRequest struct {
	Content string `json:"content" binding:"required,min=1,max=500"`
}

type CommentResponse struct {
	ID        string    `json:"id"`
	PostID    string    `json:"post_id"`
	AuthorID  string    `json:"author_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
