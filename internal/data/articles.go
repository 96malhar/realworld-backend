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
	db *pgxpool.Pool
}

func (s *ArticleStore) Insert(article *Article) error {
	article.GenerateSlug()

	query := `
		INSERT INTO articles (slug, title, description, body, tag_list, author_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`

	args := []any{article.Slug, article.Title, article.Description, article.Body, article.TagList, article.AuthorID}
	err := s.db.QueryRow(context.Background(), query, args...,
	).Scan(&article.ID, &article.CreatedAt, &article.UpdatedAt)
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

	err := s.db.QueryRow(context.Background(), query, slug).Scan(
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
	err := s.db.QueryRow(context.Background(), query, articleID, userID).Scan(&favorited)
	if err != nil {
		return false, err
	}
	return favorited, nil
}
