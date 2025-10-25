package data

import (
	"context"
	"errors"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/96malhar/realworld-backend/internal/validator"
	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Article struct {
	ID             int64     `json:"-"`
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Body           string    `json:"body,omitempty"`
	TagList        []string  `json:"tagList"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	FavoritesCount int       `json:"favoritesCount"`
	Favorited      bool      `json:"favorited"`
	AuthorID       int64     `json:"-"`
	Author         Profile   `json:"author"`
	Version        int       `json:"-"`
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

	// Insert tags into tags table using bulk insert
	if len(article.TagList) > 0 {
		if err = s.InsertTags(article.TagList...); err != nil {
			return err
		}
	}
	return nil
}

// GetIDBySlug retrieves just the article ID by its slug.
// This is a lightweight alternative to GetBySlug when only the ID is needed.
func (s *ArticleStore) GetIDBySlug(slug string) (int64, error) {
	query := `SELECT id FROM articles WHERE slug = $1`

	var articleID int64

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	err := s.db.QueryRow(ctx, query, slug).Scan(&articleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrRecordNotFound
		}
		return 0, err
	}

	return articleID, nil
}

// GetBySlug retrieves an article by its slug.
func (s *ArticleStore) GetBySlug(slug string, currentUser *User) (*Article, error) {
	query := `
		SELECT a.id, a.slug, a.title, a.description, a.body, a.tag_list, a.created_at, a.updated_at, 
		       a.favorites_count, a.version, u.id, u.username, u.bio, u.image
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
		&article.Version,
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
		SET title = $1, description = $2, body = $3, slug = $4, updated_at = (NOW() AT TIME ZONE 'UTC'), version = version + 1
		WHERE id = $5 AND version = $6
		RETURNING updated_at, version
	`

	args := []any{
		article.Title,
		article.Description,
		article.Body,
		article.Slug,
		article.ID,
		article.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	err := s.db.QueryRow(ctx, query, args...).Scan(&article.UpdatedAt, &article.Version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEditConflict
		}
		return err
	}

	if len(article.TagList) > 0 {
		if err = s.InsertTags(article.TagList...); err != nil {
			return err
		}

	}

	return nil
}

func (s *ArticleStore) InsertTags(tags ...string) error {
	query := `INSERT INTO tags (tag) SELECT UNNEST($1::text[]) ON CONFLICT (tag) DO NOTHING`

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	_, err := s.db.Exec(ctx, query, tags)
	if err != nil {
		return err
	}

	return nil
}

// ArticleFilters holds filtering and pagination parameters for listing articles
type ArticleFilters struct {
	Tag       string
	Author    string
	Favorited string
	Limit     int
	Offset    int
}

var alphanumericRX = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// Validate checks that the ArticleFilters fields are valid.
// Note: Pagination parameters (Limit and Offset) are validated and normalized
// by the readPagination helper before reaching this method.
func (f ArticleFilters) Validate(v *validator.Validator) {
	// Validate tag length and characters if provided
	if f.Tag != "" {
		v.Check(len(f.Tag) <= 50, "Tag must not be more than 50 characters")
		v.Check(len(f.Tag) >= 1, "Tag must not be empty")
		v.Check(alphanumericRX.MatchString(f.Tag), "Tag must contain only alphanumeric characters")
	}

	// Validate author username length and characters if provided
	if f.Author != "" {
		v.Check(len(f.Author) <= 50, "Author must not be more than 50 characters")
		v.Check(len(f.Author) >= 1, "Author must not be empty")
		v.Check(alphanumericRX.MatchString(f.Author), "Author must contain only alphanumeric characters")
	}

	// Validate favorited username length and characters if provided
	if f.Favorited != "" {
		v.Check(len(f.Favorited) <= 50, "Favorited username must not be more than 50 characters")
		v.Check(len(f.Favorited) >= 1, "Favorited username must not be empty")
		v.Check(alphanumericRX.MatchString(f.Favorited), "Favorited username must contain only alphanumeric characters")
	}
}

// List retrieves articles with optional filtering and pagination.
// Returns articles ordered by most recent first (created_at DESC).
// Uses JOINs to efficiently fetch favorited and following status in a single query.
func (s *ArticleStore) List(filters ArticleFilters, currentUser *User) ([]Article, int, error) {
	// Use -1 for anonymous users (will never match real user IDs, so JOINs return NULL/false)
	userID := int64(-1)
	if currentUser != nil && !currentUser.IsAnonymous() {
		userID = currentUser.ID
	}

	// Build base query using Squirrel - always include favorited and following columns
	// Note: body is excluded from list results for performance
	qb := sq.Select(
		"a.id", "a.slug", "a.title", "a.description", "a.tag_list",
		"a.created_at", "a.updated_at", "a.author_id", "a.version", "a.favorites_count",
		"u.username", "u.bio", "u.image",
		"COALESCE(fav.user_id IS NOT NULL, false) AS favorited",
		"COALESCE(fol.follower_id IS NOT NULL, false) AS following",
	).
		From("articles a").
		Join("users u ON a.author_id = u.id").
		LeftJoin("favorites fav ON a.id = fav.article_id AND fav.user_id = ?", userID).
		LeftJoin("follows fol ON a.author_id = fol.followed_id AND fol.follower_id = ?", userID).
		PlaceholderFormat(sq.Dollar)

	// Add WHERE conditions based on filters
	if filters.Tag != "" {
		qb = qb.Where("? = ANY(a.tag_list)", filters.Tag)
	}
	if filters.Author != "" {
		qb = qb.Where("u.username = ?", filters.Author)
	}
	if filters.Favorited != "" {
		qb = qb.Where(sq.Expr(`EXISTS (
			SELECT 1 FROM favorites fav_filter
			JOIN users fu ON fav_filter.user_id = fu.id
			WHERE fav_filter.article_id = a.id AND fu.username = ?
		)`, filters.Favorited))
	}

	// Add ordering and pagination
	query, args, err := qb.
		OrderBy("a.created_at DESC").
		Limit(uint64(filters.Limit)).
		Offset(uint64(filters.Offset)).
		ToSql()

	if err != nil {
		return nil, 0, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	// Execute query
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var article Article
		var author Profile
		var favorited, following bool

		err := rows.Scan(
			&article.ID,
			&article.Slug,
			&article.Title,
			&article.Description,
			&article.TagList,
			&article.CreatedAt,
			&article.UpdatedAt,
			&article.AuthorID,
			&article.Version,
			&article.FavoritesCount,
			&author.Username,
			&author.Bio,
			&author.Image,
			&favorited,
			&following,
		)
		if err != nil {
			return nil, 0, err
		}

		article.Favorited = favorited
		// Don't set following to true if current user is the author
		if currentUser != nil && article.AuthorID == currentUser.ID {
			author.Following = false
		} else {
			author.Following = following
		}

		article.Author = author
		articles = append(articles, article)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	// Get total count for the same filters (without pagination)
	totalCount, err := s.countArticles(filters)
	if err != nil {
		return nil, 0, err
	}

	return articles, totalCount, nil
}

// countArticles returns the total count of articles matching the filters
func (s *ArticleStore) countArticles(filters ArticleFilters) (int, error) {
	// Build count query using Squirrel
	qb := sq.Select("COUNT(a.id)").
		From("articles a").
		Join("users u ON a.author_id = u.id").
		PlaceholderFormat(sq.Dollar)

	// Add WHERE conditions based on filters (same as List method)
	if filters.Tag != "" {
		qb = qb.Where("? = ANY(a.tag_list)", filters.Tag)
	}
	if filters.Author != "" {
		qb = qb.Where("u.username = ?", filters.Author)
	}
	if filters.Favorited != "" {
		qb = qb.Where(sq.Expr(`EXISTS (
			SELECT 1 FROM favorites fav
			JOIN users fu ON fav.user_id = fu.id
			WHERE fav.article_id = a.id AND fu.username = ?
		)`, filters.Favorited))
	}

	query, args, err := qb.ToSql()
	if err != nil {
		return 0, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	var count int
	err = s.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
