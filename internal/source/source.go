package source

import "context"

// Source defines the interface for fetching canonical configuration.
// Implementations can be local files, remote URLs, or other backends.
type Source interface {
	// Fetch returns the raw bytes of the configuration.
	// ctx carries a deadline or cancellation signal for the fetch operation.
	// Returns an error if the source is unreachable or the file is missing.
	Fetch(ctx context.Context) ([]byte, error)

	// String returns a human-readable identifier for the source (e.g., path or URL).
	String() string
}
