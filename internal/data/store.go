package data

import (
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type ModelStore struct {
	Users UserStoreInterface
}

func NewModelStore(db *pgxpool.Pool) ModelStore {
	return ModelStore{
		Users: &UserStore{db: db},
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
	// Follow a user
	FollowUser(followerID, followedID int64) error
	// Unfollow a user
	UnfollowUser(followerID, followedID int64) error
	// Check if following
	IsFollowing(followerID, followedID int64) (bool, error)
	// Update an existing user record.
	Update(user *User) error
}
