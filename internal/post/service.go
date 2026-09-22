package post

import (
	"context"

	"github.com/emanuelfelicio/artblogapi/internal/storage"
	"github.com/google/uuid"
)

type MediaBinder interface {
	Bind(ctx context.Context, uploadID, userID uuid.UUID, purpose storage.UploadPurpose) error
	Supersede(ctx context.Context, uploadID uuid.UUID) error
}

type Repository interface {
	WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error

	CreatePost(ctx context.Context, id, authorID uuid.UUID, title, content string) (Post, error)
	GetPostWithImages(ctx context.Context, id uuid.UUID) (Post, error)
	GetPostByIDForUpdate(ctx context.Context, id uuid.UUID) (Post, error)
	UpdatePost(ctx context.Context, id uuid.UUID, title, content *string) (Post, error)
	DeletePost(ctx context.Context, id uuid.UUID) error

	BatchInsertPostImages(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID, positions []int16) error
	DeletePostImages(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID) error
	UpdatePostImagePositions(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID, positions []int16) error
	DeletePostImagesByPostID(ctx context.Context, postID uuid.UUID) ([]uuid.UUID, error)
	GetPostImagesByPostIDs(ctx context.Context, postIDs []uuid.UUID) (map[uuid.UUID][]PostImage, error)
	ListRecentPosts(ctx context.Context, limit, offset int32) ([]Post, error)
	ListPostsByAuthor(ctx context.Context, authorID uuid.UUID, limit, offset int32) ([]Post, error)
}

type service struct {
	repo  Repository
	media MediaBinder
}

func NewService(repo Repository, media MediaBinder) *service {
	return &service{
		repo:  repo,
		media: media,
	}
}

func (s *service) CreatePost(ctx context.Context, authorID uuid.UUID, title, content string, imageUploadIDs []string) (Post, error) {
	validatedTitle, err := ValidateTitle(title)
	if err != nil {
		return Post{}, err
	}
	validatedContent, err := ValidateContent(content)
	if err != nil {
		return Post{}, err
	}

	if err := ValidateImagesCount(len(imageUploadIDs)); err != nil {
		return Post{}, err
	}

	parsedUploadIDs, err := parseUniqueUploadIDs(imageUploadIDs)
	if err != nil {
		return Post{}, err
	}

	postID, err := uuid.NewV7()
	if err != nil {
		return Post{}, err
	}

	positions := make([]int16, len(parsedUploadIDs))
	for i := range parsedUploadIDs {
		positions[i] = int16(i)
	}

	var createdPost Post
	err = s.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		_, err := s.repo.CreatePost(txCtx, postID, authorID, validatedTitle, validatedContent)
		if err != nil {
			return err
		}

		for _, uploadID := range parsedUploadIDs {
			if err := s.media.Bind(txCtx, uploadID, authorID, storage.PurposePOSTIMAGE); err != nil {
				return err
			}
		}

		if len(parsedUploadIDs) > 0 {
			if err := s.repo.BatchInsertPostImages(txCtx, postID, parsedUploadIDs, positions); err != nil {
				return err
			}
		}

		fullPost, err := s.repo.GetPostWithImages(txCtx, postID)
		if err != nil {
			return err
		}
		createdPost = fullPost
		return nil
	})
	if err != nil {
		return Post{}, err
	}

	return createdPost, nil
}

func (s *service) GetPost(ctx context.Context, id uuid.UUID) (Post, error) {
	return s.repo.GetPostWithImages(ctx, id)
}

func (s *service) ListRecentPosts(ctx context.Context, limit, offset int32) ([]Post, error) {
	limit, offset = NormalizePagination(limit, offset)

	posts, err := s.repo.ListRecentPosts(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	if err := s.attachImagesToPosts(ctx, posts); err != nil {
		return nil, err
	}

	return posts, nil
}

func (s *service) ListPostsByAuthor(ctx context.Context, authorID uuid.UUID, limit, offset int32) ([]Post, error) {
	limit, offset = NormalizePagination(limit, offset)

	posts, err := s.repo.ListPostsByAuthor(ctx, authorID, limit, offset)
	if err != nil {
		return nil, err
	}

	if err := s.attachImagesToPosts(ctx, posts); err != nil {
		return nil, err
	}

	return posts, nil
}

func (s *service) UpdatePost(ctx context.Context, postID, authorID uuid.UUID, title, content *string, imageUploadIDs []string) (Post, error) {
	if title != nil {
		validTitle, err := ValidateTitle(*title)
		if err != nil {
			return Post{}, err
		}
		title = &validTitle
	}
	if content != nil {
		validContent, err := ValidateContent(*content)
		if err != nil {
			return Post{}, err
		}
		content = &validContent
	}

	var desiredUploadIDs []uuid.UUID
	if imageUploadIDs != nil {
		if err := ValidateImagesCount(len(imageUploadIDs)); err != nil {
			return Post{}, err
		}
		parsed, err := parseUniqueUploadIDs(imageUploadIDs)
		if err != nil {
			return Post{}, err
		}
		desiredUploadIDs = parsed
	}

	var updatedPost Post
	err := s.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		currentPost, err := s.repo.GetPostByIDForUpdate(txCtx, postID)
		if err != nil {
			return err
		}

		if currentPost.AuthorID != authorID {
			return ErrPostForbidden
		}

		postResult := currentPost
		if title != nil || content != nil {
			p, err := s.repo.UpdatePost(txCtx, postID, title, content)
			if err != nil {
				return err
			}
			postResult = p
		}

		if imageUploadIDs != nil {
			imagesMap, err := s.repo.GetPostImagesByPostIDs(txCtx, []uuid.UUID{postID})
			if err != nil {
				return err
			}
			currentImages := imagesMap[postID]

			current := make(map[uuid.UUID]PostImage, len(currentImages))
			for _, img := range currentImages {
				current[img.UploadID] = img
			}

			desired := make(map[uuid.UUID]struct{}, len(desiredUploadIDs))
			for _, uID := range desiredUploadIDs {
				desired[uID] = struct{}{}
			}

			var toDeleteIDs []uuid.UUID
			for _, img := range currentImages {
				if _, exists := desired[img.UploadID]; !exists {
					toDeleteIDs = append(toDeleteIDs, img.UploadID)
				}
			}

			if len(toDeleteIDs) > 0 {
				if err := s.repo.DeletePostImages(txCtx, postID, toDeleteIDs); err != nil {
					return err
				}
				for _, uID := range toDeleteIDs {
					if err := s.media.Supersede(txCtx, uID); err != nil {
						return err
					}
				}
			}

			var toInsertIDs []uuid.UUID
			var toInsertPositions []int16
			var toUpdateIDs []uuid.UUID
			var toUpdatePositions []int16

			for i, uID := range desiredUploadIDs {
				if img, exists := current[uID]; exists {
					if img.Position != i {
						toUpdateIDs = append(toUpdateIDs, uID)
						toUpdatePositions = append(toUpdatePositions, int16(i))
					}
				} else {
					toInsertIDs = append(toInsertIDs, uID)
					toInsertPositions = append(toInsertPositions, int16(i))
				}
			}

			for _, uID := range toInsertIDs {
				if err := s.media.Bind(txCtx, uID, authorID, storage.PurposePOSTIMAGE); err != nil {
					return err
				}
			}

			if len(toInsertIDs) > 0 {
				if err := s.repo.BatchInsertPostImages(txCtx, postID, toInsertIDs, toInsertPositions); err != nil {
					return err
				}
			}

			if len(toUpdateIDs) > 0 {
				if err := s.repo.UpdatePostImagePositions(txCtx, postID, toUpdateIDs, toUpdatePositions); err != nil {
					return err
				}
			}

			finalImagesMap, err := s.repo.GetPostImagesByPostIDs(txCtx, []uuid.UUID{postID})
			if err != nil {
				return err
			}
			postResult.Images = finalImagesMap[postID]
		} else {
			imagesMap, err := s.repo.GetPostImagesByPostIDs(txCtx, []uuid.UUID{postID})
			if err != nil {
				return err
			}
			postResult.Images = imagesMap[postID]
		}

		updatedPost = postResult
		return nil
	})
	if err != nil {
		return Post{}, err
	}

	return updatedPost, nil
}

func (s *service) DeletePost(ctx context.Context, postID, authorID uuid.UUID) error {
	return s.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		currentPost, err := s.repo.GetPostByIDForUpdate(txCtx, postID)
		if err != nil {
			return err
		}

		if currentPost.AuthorID != authorID {
			return ErrPostForbidden
		}

		uploadIDs, err := s.repo.DeletePostImagesByPostID(txCtx, postID)
		if err != nil {
			return err
		}

		for _, uploadID := range uploadIDs {
			if err := s.media.Supersede(txCtx, uploadID); err != nil {
				return err
			}
		}

		return s.repo.DeletePost(txCtx, postID)
	})
}

func (s *service) attachImagesToPosts(ctx context.Context, posts []Post) error {
	if len(posts) == 0 {
		return nil
	}

	postIDs := make([]uuid.UUID, len(posts))
	for i, p := range posts {
		postIDs[i] = p.ID
	}

	imagesMap, err := s.repo.GetPostImagesByPostIDs(ctx, postIDs)
	if err != nil {
		return err
	}

	for i := range posts {
		if imgs, ok := imagesMap[posts[i].ID]; ok {
			posts[i].Images = imgs
		} else {
			posts[i].Images = []PostImage{}
		}
	}
	return nil
}

func parseUniqueUploadIDs(rawIDs []string) ([]uuid.UUID, error) {
	if len(rawIDs) == 0 {
		return nil, nil
	}

	seen := make(map[uuid.UUID]struct{}, len(rawIDs))
	parsed := make([]uuid.UUID, 0, len(rawIDs))

	for _, raw := range rawIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[id]; exists {
			return nil, ErrDuplicateUploadID
		}
		seen[id] = struct{}{}
		parsed = append(parsed, id)
	}

	return parsed, nil
}
