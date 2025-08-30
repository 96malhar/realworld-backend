package data

import (
	"context"
	"errors"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/96malhar/realworld-backend/internal/validator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Article struct {
	ID             int64     `json:"-"`
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Body           string    `json:"body"`
	TagList        []string  `json:"tagList"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	FavoritesCount int       `json:"favoritesCount"`
	Favorited      bool      `json:"favorited"`
	AuthorID       int64     `json:"-"`
	Author         Profile   `json:"author"`
}

func ValidateArticle(v *validator.Validator, article *Article) {
	// check empty or whitespace only on Title and Description and body
	v.Check(validator.NotEmptyOrWhitespace(article.Title),
		"Title must not be empty or whitespace only")
	v.Check(validator.NotEmptyOrWhitespace(article.Description),
		"Description must not be empty or whitespace only")
	v.Check(validator.NotEmptyOrWhitespace(article.Body),
		"Body must not be empty or whitespace only")

	v.Check(validator.Unique(article.TagList), "TagList must not contain duplicate tags")
}

// GenerateSlug generates a URL-friendly slug from the article title.
func (a *Article) GenerateSlug() {
	slug := strings.ToLower(a.Title)
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9\-]`)
	slug = reg.ReplaceAllString(slug, "")

	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	// Append a random string to ensure uniqueness
	slug = slug + "-" + randomString(6)

	a.Slug = slug
}

// RandomString generates a random string of specified length using lowercase letters and numbers
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

type ArticleStore struct {
	db      *pgxpool.Pool
	timeout time.Duration
}

func (s *ArticleStore) Insert(article *Article) error {
	article.GenerateSlug()

	query := `
		INSERT INTO articles (slug, title, description, body, tag_list, author_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`

	args := []any{article.Slug, article.Title, article.Description, article.Body, article.TagList, article.AuthorID}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	err := s.db.QueryRow(ctx, query, args...).Scan(&article.ID, &article.CreatedAt, &article.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

// GetBySlug retrieves an article by its slug.
func (s *ArticleStore) GetBySlug(slug string, currentUser *User) (*Article, error) {
	query := `
		SELECT a.id, a.slug, a.title, a.description, a.body, a.tag_list, a.created_at, a.updated_at, 
		       a.favorites_count, u.id, u.username, u.bio, u.image
		FROM articles a
		JOIN users u ON a.author_id = u.id
		WHERE a.slug = $1
	`

	var article Article
	var author Profile

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	err := s.db.QueryRow(ctx, query, slug).Scan(
		&article.ID,
		&article.Slug,
		&article.Title,
		&article.Description,
		&article.Body,
		&article.TagList,
		&article.CreatedAt,
		&article.UpdatedAt,
		&article.FavoritesCount,
		&article.AuthorID,
		&author.Username,
		&author.Bio,
		&author.Image,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	article.Author = author

	// Check if the current user has favorited the article
	if !currentUser.IsAnonymous() {
		favorited, err := s.checkArticleFavorited(article.ID, currentUser.ID)
		if err != nil {
			return nil, err
		}
		article.Favorited = favorited
	}
	return &article, nil
}

func (s *ArticleStore) checkArticleFavorited(articleID, userID int64) (bool, error) {
	var favorited bool
	query := `SELECT EXISTS(SELECT 1 FROM favorites WHERE article_id = $1 AND user_id = $2)`

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	err := s.db.QueryRow(ctx, query, articleID, userID).Scan(&favorited)
	if err != nil {
		return false, err
	}
	return favorited, nil
}

// FavoriteBySlug favorites an article for the given user and returns the updated article.
func (s *ArticleStore) FavoriteBySlug(slug string, userID int64) (*Article, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	// Lookup the article id first
	var articleID int64
	q1 := `SELECT id FROM articles WHERE slug = $1`
	err = tx.QueryRow(ctx, q1, slug).Scan(&articleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	// Insert favorite and check if a new row was inserted.
	q2 := `INSERT INTO favorites (user_id, article_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	tag, err := tx.Exec(ctx, q2, userID, articleID)
	if err != nil {
		return nil, err
	}

	// Only increment the count if a new favorite was actually inserted.
	if tag.RowsAffected() == 1 {
		q3 := `UPDATE articles SET favorites_count = favorites_count + 1 WHERE id = $1`
		if _, err := tx.Exec(ctx, q3, articleID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Return the fresh article including favorited status
	return s.GetBySlug(slug, &User{ID: userID})
}

// UnfavoriteBySlug unfavorites an article for the given user and returns the updated article.
func (s *ArticleStore) UnfavoriteBySlug(slug string, userID int64) (*Article, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lookup the article id first
	var articleID int64
	q1 := `SELECT id FROM articles WHERE slug = $1`
	err = tx.QueryRow(ctx, q1, slug).Scan(&articleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	// Delete the favorite record.
	q2 := `DELETE FROM favorites WHERE user_id = $1 AND article_id = $2`
	tag, err := tx.Exec(ctx, q2, userID, articleID)
	if err != nil {
		return nil, err
	}

	// Only decrement the count if a favorite was actually deleted.
	if tag.RowsAffected() == 1 {
		q3 := `UPDATE articles SET favorites_count = favorites_count - 1 WHERE id = $1 AND favorites_count > 0`
		if _, err := tx.Exec(ctx, q3, articleID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Return the fresh article including favorited status
	return s.GetBySlug(slug, &User{ID: userID})
}

func (s *ArticleStore) DeleteBySlug(slug string, authorID int64) error {
	query := `
		DELETE FROM articles
		WHERE slug = $1 AND author_id = $2
	`

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	result, err := s.db.Exec(ctx, query, slug, authorID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (s *ArticleStore) Update(article *Article) error {
	query := `
		UPDATE articles
		SET title = $1, description = $2, body = $3, slug = $4, updated_at = (NOW() AT TIME ZONE 'UTC')
		WHERE id = $5
		RETURNING updated_at
	`

	args := []any{
		article.Title,
		article.Description,
		article.Body,
		article.Slug,
		article.ID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	err := s.db.QueryRow(ctx, query, args...).Scan(&article.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRecordNotFound
		}
		return err
	}

	return nil
}
