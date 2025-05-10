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
}
