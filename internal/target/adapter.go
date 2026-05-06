// Package target provides implementations for configuration targets.
package target

import "context"

// Adapter fetches a config file from a specific remote target.
type Adapter interface {
	// Name returns the human-readable name of this target.
	Name() string
	// Fetch fetches the raw config bytes from the remote target.
	Fetch(ctx context.Context) ([]byte, error)
}
