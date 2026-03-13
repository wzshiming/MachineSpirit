package memory

import "context"

// Store exposes on-demand memory access.
type Store interface {
	Read(ctx context.Context, layer Layer) ([]string, error)
	Write(ctx context.Context, layer Layer, entries []string) error
}
