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
	GetPostByID(ctx context.Context, id uuid.UUID) (Post, error)
	GetPostByIDForUpdate(ctx context.Context, id uuid.UUID) (Post, error)
	UpdatePost(ctx context.Context, id uuid.UUID, title, content *string) (Post, error)
	DeletePost(ctx context.Context, id uuid.UUID) error

	InsertPostImage(ctx context.Context, postID, uploadID uuid.UUID, position int16) error
	GetPostImagesByPostID(ctx context.Context, postID uuid.UUID) ([]PostImage, error)
	GetPostImagesByPostIDs(ctx context.Context, postIDs []uuid.UUID) (map[uuid.UUID][]PostImage, error)
	DeletePostImage(ctx context.Context, postID, uploadID uuid.UUID) error
	UpdatePostImagePosition(ctx context.Context, postID, uploadID uuid.UUID, position int16) error
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

	var createdPost Post
	err = s.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		p, err := s.repo.CreatePost(txCtx, postID, authorID, validatedTitle, validatedContent)
		if err != nil {
			return err
		}

		for i, uploadID := range parsedUploadIDs {
			if err := s.media.Bind(txCtx, uploadID, authorID, storage.PurposePOSTIMAGE); err != nil {
				return err
			}

			if err := s.repo.InsertPostImage(txCtx, postID, uploadID, int16(i)); err != nil {
				return err
			}
		}

		images, err := s.repo.GetPostImagesByPostID(txCtx, postID)
		if err != nil {
			return err
		}
		p.Images = images
		createdPost = p
		return nil
	})
	if err != nil {
		return Post{}, err
	}

	return createdPost, nil
}

func (s *service) GetPost(ctx context.Context, id uuid.UUID) (Post, error) {
	post, err := s.repo.GetPostByID(ctx, id)
	if err != nil {
		return Post{}, err
	}

	images, err := s.repo.GetPostImagesByPostID(ctx, id)
	if err != nil {
		return Post{}, err
	}
	post.Images = images

	return post, nil
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

		if title != nil || content != nil {
			if _, err := s.repo.UpdatePost(txCtx, postID, title, content); err != nil {
				return err
			}
		}

		if imageUploadIDs != nil {
			currentImages, err := s.repo.GetPostImagesByPostID(txCtx, postID)
			if err != nil {
				return err
			}

			currentMap := make(map[uuid.UUID]PostImage, len(currentImages))
			for _, img := range currentImages {
				currentMap[img.UploadID] = img
			}

			desiredMap := make(map[uuid.UUID]int, len(desiredUploadIDs))
			for i, uID := range desiredUploadIDs {
				desiredMap[uID] = i
			}

			for _, img := range currentImages {
				if _, exists := desiredMap[img.UploadID]; !exists {
					if err := s.repo.DeletePostImage(txCtx, postID, img.UploadID); err != nil {
						return err
					}
					if err := s.media.Supersede(txCtx, img.UploadID); err != nil {
						return err
					}
				}
			}

			for i, uID := range desiredUploadIDs {
				if img, exists := currentMap[uID]; exists {
					if img.Position != i {
						if err := s.repo.UpdatePostImagePosition(txCtx, postID, uID, int16(i)); err != nil {
							return err
						}
					}
				} else {
					if err := s.media.Bind(txCtx, uID, authorID, storage.PurposePOSTIMAGE); err != nil {
						return err
					}
					if err := s.repo.InsertPostImage(txCtx, postID, uID, int16(i)); err != nil {
						return err
					}
				}
			}
		}

		p, err := s.repo.GetPostByID(txCtx, postID)
		if err != nil {
			return err
		}
		images, err := s.repo.GetPostImagesByPostID(txCtx, postID)
		if err != nil {
			return err
		}
		p.Images = images
		updatedPost = p
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

		images, err := s.repo.GetPostImagesByPostID(txCtx, postID)
		if err != nil {
			return err
		}

		for _, img := range images {
			if err := s.media.Supersede(txCtx, img.UploadID); err != nil {
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
