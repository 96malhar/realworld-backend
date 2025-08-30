package data

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type ModelStore struct {
	Users    UserStoreInterface
	Articles ArticleStoreInterface
}

func NewModelStore(db *pgxpool.Pool, timeout time.Duration) ModelStore {
	return ModelStore{
		Users:    &UserStore{db: db, timeout: timeout},
		Articles: &ArticleStore{db: db, timeout: timeout},
	}
}

type UserStoreInterface interface {
	// Insert a new record into the users table.
	Insert(user *User) error
	// GetByEmail returns a specific record from the users table.
	GetByEmail(email string) (*User, error)
	// GetByID retrieves a specific record from the users table by ID.
	GetByID(id int64) (*User, error)
	// GetByUsername retrieves a specific record from the users table by username.
	GetByUsername(username string) (*User, error)
	// FollowUser records that a user is following another user
	FollowUser(followerID, followedID int64) error
	// UnfollowUser records that a user has unfollowed another user
	UnfollowUser(followerID, followedID int64) error
	// IsFollowing checks if a user is following another user
	IsFollowing(followerID, followedID int64) (bool, error)
	// Update an existing user record.
	Update(user *User) error
}

type ArticleStoreInterface interface {
	// Insert a new record into the articles table.
	Insert(article *Article) error
	// GetBySlug retrieves a specific record from the articles table by slug.
	GetBySlug(slug string, currentUser *User) (*Article, error)
	// FavoriteBySlug favorites the article with the given slug for the user and returns the updated article.
	FavoriteBySlug(slug string, userID int64) (*Article, error)
	// UnfavoriteBySlug unfavorites the article with the given slug for the user and returns the updated article.
	UnfavoriteBySlug(slug string, userID int64) (*Article, error)
	// DeleteBySlug deletes the article with the given slug.
	DeleteBySlug(slug string, userID int64) error
	// Update an existing article record.
	Update(article *Article) error
}
