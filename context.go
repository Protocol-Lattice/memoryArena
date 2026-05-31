package memoryArena

import (
	"context"
	"errors"
)

// uniqueKey prevents context key collisions
type contextKey struct{}

var (
	arenaKey   = contextKey{}
	ErrNoArena = errors.New("no memory arena found in context")
)

// Inject attaches the memory arena to the request context.
// Note: To minimize heap allocations in extreme hot paths, prefer passing
// the *MemoryArena explicitly as a function parameter if possible.
func Inject[T any](ctx context.Context, a *MemoryArena[T]) context.Context {
	return context.WithValue(ctx, arenaKey, a)
}

// Extract retrieves the memory arena from the context.
func Extract[T any](ctx context.Context) (*MemoryArena[T], error) {
	a, ok := ctx.Value(arenaKey).(*MemoryArena[T])
	if !ok || a == nil {
		return nil, ErrNoArena
	}
	return a, nil
}
