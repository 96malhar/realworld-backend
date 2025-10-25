package data

import (
	"context"
	"time"

	"github.com/96malhar/realworld-backend/internal/validator"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	ArticleID int64     `json:"-"`
	AuthorID  int64     `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Author    Profile   `json:"author"`
}

func ValidateComment(v *validator.Validator, comment *Comment) {
	v.Check(validator.NotEmptyOrWhitespace(comment.Body),
		"Body must not be empty or whitespace only")
}

type CommentStore struct {
	db      *pgxpool.Pool
	timeout time.Duration
}

// Insert creates a new comment for an article.
func (s *CommentStore) Insert(comment *Comment) error {
	query := `
		INSERT INTO comments (body, article_id, author_id)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`

	args := []any{comment.Body, comment.ArticleID, comment.AuthorID}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	err := s.db.QueryRow(ctx, query, args...).Scan(&comment.ID, &comment.CreatedAt, &comment.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}
