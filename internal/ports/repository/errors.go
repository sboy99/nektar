package repository

import "errors"

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("repository: not found")
