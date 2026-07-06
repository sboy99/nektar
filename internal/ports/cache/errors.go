package cache

import "errors"

// ErrNotFound is returned when a cache key does not exist.
var ErrNotFound = errors.New("cache: key not found")
