// Package store defines a small, pluggable key/value persistence seam
// (Source/Store) for settings and other small state, plus a Chain adapter
// composing multiple layers into one ascending-precedence view with a
// single designated write sink. Built-in adapters live in the defaults,
// env, and xdgfile subpackages.
package store

import (
	"context"
	"errors"
)

// ErrUnsupported is returned by a Store adapter's write methods when the
// adapter (or a composed Chain) does not support writes.
var ErrUnsupported = errors.New("store: operation not supported by this adapter")

// Source is a read-only key/value source.
type Source interface {
	// Get returns the value for key, and whether it was found.
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	// Load returns every key/value pair the source currently holds.
	Load(ctx context.Context) (map[string]string, error)
}

// Store is a read/write key/value source.
type Store interface {
	Source

	// Set stages value for key. Depending on the adapter, this may or may
	// not be durable until Save is called.
	Set(ctx context.Context, key, value string) error
	// Save flushes any staged writes to durable storage.
	Save(ctx context.Context) error
	// Delete removes key.
	Delete(ctx context.Context, key string) error
}
