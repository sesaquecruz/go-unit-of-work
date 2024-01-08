// Package uow provides an implementation of the Unit of Work pattern for managing database transactions.
//
// The Unit of Work allows you to encapsulate a set of database operations within a single transaction.
// This package contains an interface and implementation of a Unit of Work.
package uow

import (
	"context"
	"database/sql"
	"errors"
)

var (
	ErrRepositoryNotRegistered     = errors.New("repository not registered")
	ErrRepositoryAlreadyRegistered = errors.New("repository already registered")
	ErrInvalidRepositoryType       = errors.New("invalid repository type")
)

type RepositoryName string
type Repository any
type RepositoryFactory func(tx *sql.Tx) Repository

// Transaction interface.
type TX interface {
	Get(name RepositoryName) (Repository, error)
}

// Unit of Work interface.
type UOW interface {
	Register(name RepositoryName, factory RepositoryFactory) error
	Remove(name RepositoryName) error
	Has(name RepositoryName) bool
	Clear()
	Do(ctx context.Context, fn func(ctx context.Context, tx TX) error) error
}

// Transaction implementation.
type Transaction struct {
	tx           *sql.Tx
	repositories map[RepositoryName]RepositoryFactory
}

// Create a new transaction. Return a pointer to a transaction.
func NewTransaction(tx *sql.Tx, repositories map[RepositoryName]RepositoryFactory) *Transaction {
	return &Transaction{
		tx:           tx,
		repositories: repositories,
	}
}

// Return repository of type T if any found.
// In case of type cast error returns ErrInvalidRepositoryType.
func GetAs[T any](t TX, name RepositoryName) (T, error) {
	repository, err := t.Get(name)
	var res T
	if err != nil {
		return res, err
	}
	res, ok := repository.(T)
	if !ok {
		return res, ErrInvalidRepositoryType
	}

	return res, nil
}

// Given a repository name returns a repository. Return an error if the repository does not exist.
func (t *Transaction) Get(name RepositoryName) (Repository, error) {
	if repository, ok := t.repositories[name]; ok {
		return repository(t.tx), nil
	}

	return nil, ErrRepositoryNotRegistered
}

// Unit of Work implementation
type UnitOfWork struct {
	db           *sql.DB
	repositories map[RepositoryName]RepositoryFactory
}

// Create a new unit of work. Return a pointer to a unit of work.
func NewUnitOfWork(db *sql.DB) *UnitOfWork {
	return &UnitOfWork{
		db:           db,
		repositories: make(map[RepositoryName]RepositoryFactory),
	}
}

// Register a repository factory with the given repository name.
// Return an error if a repository name already registered.
func (u *UnitOfWork) Register(name RepositoryName, factory RepositoryFactory) error {
	if _, ok := u.repositories[name]; ok {
		return ErrRepositoryAlreadyRegistered
	}

	u.repositories[name] = factory
	return nil
}

// Remove a repository factory with the given repository name.
// Return an error if the repository name does not registered.
func (u *UnitOfWork) Remove(name RepositoryName) error {
	if _, ok := u.repositories[name]; !ok {
		return ErrRepositoryNotRegistered
	}

	delete(u.repositories, name)
	return nil
}

// Verify if a repository name already registered.
// Return true if the repository name is registered, otherwise return false.
func (u *UnitOfWork) Has(name RepositoryName) bool {
	_, ok := u.repositories[name]
	return ok
}

// Remove all registered repository name and factories.
func (u *UnitOfWork) Clear() {
	u.repositories = make(map[RepositoryName]RepositoryFactory)
}

// Executes the provided function (fn) within a transactional context.
//
// The tx parameter in fn gives access to the repositories registered in the Unit Of Work.
// All operations performed on the repositories are executed within the same transaction.
//
// The ctx parameter of Do is passed to fn when it is called.
//
// If an error occurs, the transaction is rolled back and the error is returned.
// Otherwise, the transaction is committed, and nil is returned.
func (u *UnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx TX) error) error {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = fn(ctx, NewTransaction(tx, u.repositories))
	if err != nil {
		return err
	}

	err = tx.Commit()
	return err
}
