package memoryArena

import (
	"context"

	"golang.org/x/sync/semaphore"
)

type ConcurrentMemoryArena[T any] struct {
	sem   *semaphore.Weighted
	arena *MemoryArena[T]
}

// NewConcurrentMemoryArena creates a thread-safe arena wrapper.
func NewConcurrentMemoryArena[T any](size uint) *ConcurrentMemoryArena[T] {
	return &ConcurrentMemoryArena[T]{
		sem:   semaphore.NewWeighted(1),
		arena: NewMemoryArena[T](size),
	}
}

// Alloc locks the arena, copies obj into the next free slot, and returns a pointer to it.
//
// It panics if the arena is full.
func (c *ConcurrentMemoryArena[T]) Alloc(obj T) *T {
	if err := c.sem.Acquire(context.Background(), 1); err != nil {
		panic(err)
	}
	defer c.sem.Release(1)

	return c.arena.Alloc(obj)
}

// AllocContext attempts to lock the arena using ctx, copies obj into the next free slot,
// and returns a pointer to it.
//
// It returns ctx.Err() if the context is canceled or times out before the arena is acquired.
// It panics if the arena is full after acquisition.
func (c *ConcurrentMemoryArena[T]) AllocContext(ctx context.Context, obj T) (*T, error) {
	if err := c.sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer c.sem.Release(1)

	return c.arena.Alloc(obj), nil
}

// AllocSlab locks the arena, reserves size contiguous slots, and returns them.
//
// The returned slice is capacity-bounded, so append cannot overwrite later arena allocations.
// It panics if the slab does not fit.
func (c *ConcurrentMemoryArena[T]) AllocSlab(size uint) []T {
	if err := c.sem.Acquire(context.Background(), 1); err != nil {
		panic(err)
	}
	defer c.sem.Release(1)

	return c.arena.AllocSlab(size)
}

// AllocSlabContext attempts to lock the arena using ctx, reserves size contiguous slots,
// and returns them.
//
// It returns ctx.Err() if the context is canceled or times out before the arena is acquired.
// It panics if the slab does not fit after acquisition.
func (c *ConcurrentMemoryArena[T]) AllocSlabContext(ctx context.Context, size uint) ([]T, error) {
	if err := c.sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer c.sem.Release(1)

	return c.arena.AllocSlab(size), nil
}

// AllocSlabWith locks the arena, reserves a slab, copies values into it,
// and returns the allocated slab.
//
// The returned slice is capacity-bounded, so append cannot overwrite later arena allocations.
// It panics if the values do not fit.
func (c *ConcurrentMemoryArena[T]) AllocSlabWith(values ...T) []T {
	if err := c.sem.Acquire(context.Background(), 1); err != nil {
		panic(err)
	}
	defer c.sem.Release(1)

	return c.arena.AllocSlabWith(values...)
}

// AllocSlabWithContext attempts to lock the arena using ctx, reserves a slab,
// copies values into it, and returns the allocated slab.
//
// It returns ctx.Err() if the context is canceled or times out before the arena is acquired.
// It panics if the values do not fit after acquisition.
func (c *ConcurrentMemoryArena[T]) AllocSlabWithContext(ctx context.Context, values ...T) ([]T, error) {
	if err := c.sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer c.sem.Release(1)

	return c.arena.AllocSlabWith(values...), nil
}

// Reset locks the arena, zeroes the allocated region, and resets the allocation offset.
//
// Use Reset only when no goroutine is still reading or writing values previously returned
// by the arena.
func (c *ConcurrentMemoryArena[T]) Reset() {
	if err := c.sem.Acquire(context.Background(), 1); err != nil {
		panic(err)
	}
	defer c.sem.Release(1)

	c.arena.Reset()
}

// ResetContext attempts to lock the arena using ctx, zeroes the allocated region,
// and resets the allocation offset.
//
// It returns ctx.Err() if the context is canceled or times out before the arena is acquired.
func (c *ConcurrentMemoryArena[T]) ResetContext(ctx context.Context) error {
	if err := c.sem.Acquire(ctx, 1); err != nil {
		return err
	}
	defer c.sem.Release(1)

	c.arena.Reset()
	return nil
}
