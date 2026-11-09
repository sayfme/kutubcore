package repository

import "errors"

// ErrNotFound is returned when a record does not exist (or is soft-deleted).
var ErrNotFound = errors.New("not found")
