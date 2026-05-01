package storage

import (
	"context"
	"io"
)

// Storage is the abstraction over object storage backends. The R2
// implementation is what runs in production; an in-memory fake or
// local-disk implementation can be wired up for tests.
type Storage interface {
	// Upload writes the contents of body to the given key and returns
	// the public URL where the object is accessible.
	Upload(ctx context.Context, key, contentType string, body io.Reader) (publicURL string, err error)

	// Download returns a reader for the given key. Caller is responsible
	// for closing the reader.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the object at the given key. Idempotent — returns
	// nil if the key doesn't exist.
	Delete(ctx context.Context, key string) error

	// PublicURL returns the public URL for a given storage key without
	// performing any network call. Useful for constructing URLs to embed
	// in API responses without re-uploading.
	PublicURL(key string) string
}
