package configs

import (
	"dainxor/atv/models"
	"dainxor/atv/types"
	"errors"
)

type dbError struct{}

var DBErr dbError
var (
	errNotFound       = errors.New("document not found")
	errNotModified    = errors.New("document not modified")
	errNotDeleted     = errors.New("document not deleted")
	errAlreadyExists  = errors.New("document already exists")
	errInvalidInput   = errors.New("invalid input")
	errInternal       = errors.New("internal server error")
	errNotImplemented = errors.New("not implemented")
)

func (dbError) NotFound() error       { return errNotFound }
func (dbError) NotModified() error    { return errNotModified }
func (dbError) NotDeleted() error     { return errNotDeleted }
func (dbError) AlreadyExists() error  { return errAlreadyExists }
func (dbError) InvalidInput() error   { return errInvalidInput }
func (dbError) Internal() error       { return errInternal }
func (dbError) NotImplemented() error { return errNotImplemented }

/*
Database Accessor Interface

Provides a standard interface for database operations.
This interface outlines the methods required for handling a database
through the common DB object in configs package.

You can check the implementation of the mongo accessor (mongo_db.go) as an example.
*/
type InterfaceDBAccessor interface {
	Connect(dbName, connectionString string) error
	Disconnect() error
	Migrate(models ...models.DBModelInterface) error

	CreateFilter(filter ...types.SPair[string]) any

	InsertOne(element models.DBModelInterface) types.Result[models.DBID]
	InsertMany(elements ...models.DBModelInterface) types.Result[[]models.DBID]

	FindOne(filter any, model models.DBModelInterface) types.Result[models.DBModelInterface]
	FindMany(filter any, model models.DBModelInterface) types.Result[[]models.DBModelInterface]

	UpdateOne(filter any, update models.DBModelInterface) error
	UpdateMany(filter any, update models.DBModelInterface) error

	PatchOne(filter any, update models.DBModelInterface) error
	PatchOneAndUnset(filter any, update models.DBModelInterface, fields ...string) error
	PatchMany(filter any, update models.DBModelInterface) error

	SoftDeleteOne(filter any, model models.DBModelInterface) error
	SoftDeleteMany(filter any, model models.DBModelInterface) error

	PermanentDeleteOne(filter any, model models.DBModelInterface) error
	PermanentDeleteMany(filter any, model models.DBModelInterface) error
}
