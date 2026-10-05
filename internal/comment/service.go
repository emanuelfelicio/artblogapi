package comment

import (
	"context"

	"github.com/google/uuid"
)

type Service interface {
	Create(context.Context, uuid.UUID, uuid.UUID, string) (Comment, error)
	ListByPost(context.Context, uuid.UUID, int32, int32) ([]Comment, error)
	Update(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (Comment, error)
	Delete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) Create(ctx context.Context, postID, authorID uuid.UUID, content string) (Comment, error) {
	content, err := ValidateContent(content)
	if err != nil {
		return Comment{}, err
	}
	if err := s.repo.PostExists(ctx, postID); err != nil {
		return Comment{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Comment{}, err
	}
	return s.repo.Create(ctx, id, postID, authorID, content)
}

func (s *service) ListByPost(ctx context.Context, postID uuid.UUID, limit, offset int32) ([]Comment, error) {
	limit, offset = NormalizePagination(limit, offset)
	if err := s.repo.PostExists(ctx, postID); err != nil {
		return nil, err
	}
	return s.repo.ListByPost(ctx, postID, limit, offset)
}

func (s *service) Update(ctx context.Context, postID, commentID, authorID uuid.UUID, content string) (Comment, error) {
	content, err := ValidateContent(content)
	if err != nil {
		return Comment{}, err
	}
	var result Comment
	err = s.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.repo.GetForUpdate(txCtx, commentID, postID)
		if err != nil {
			return err
		}
		if current.AuthorID != authorID {
			return ErrCommentForbidden
		}
		if current.DeletedAt != nil {
			return ErrCommentDeleted
		}
		result, err = s.repo.Update(txCtx, commentID, postID, content)
		return err
	})
	return result, err
}

func (s *service) Delete(ctx context.Context, postID, commentID, authorID uuid.UUID) error {
	return s.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.repo.GetForUpdate(txCtx, commentID, postID)
		if err != nil {
			return err
		}
		if current.AuthorID != authorID {
			return ErrCommentForbidden
		}
		if current.DeletedAt != nil {
			return ErrCommentDeleted
		}
		return s.repo.SoftDelete(txCtx, commentID, postID)
	})
}
