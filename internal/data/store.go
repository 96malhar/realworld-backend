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
	Insert(user *User) error
}
