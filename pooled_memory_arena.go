package memoryArena

import (
	"sync"
)

type PooledMemoryArena[T any] struct {
	pool sync.Pool
	size uint
}

// NewPooledMemoryArena manages a pool of single-threaded MemoryArena instances.
// This design eliminates lock contention entirely for high-concurrency loops.
func NewPooledMemoryArena[T any](arenaSize uint) *PooledMemoryArena[T] {
	return &PooledMemoryArena[T]{
		size: arenaSize,
		pool: sync.Pool{
			New: func() any {
				return NewMemoryArena[T](arenaSize)
			},
		},
	}
}

// Execute checks out a thread-isolated arena instance, runs the provided closure execution block,
// resets the arena at block exit, and returns the clean arena safely back to the pool.
func (p *PooledMemoryArena[T]) Execute(action func(arena *MemoryArena[T])) {
	arena := p.pool.Get().(*MemoryArena[T])

	// Reset must happen after action, including panic paths, so writes made during
	// the execution block do not leak into the next pool checkout.
	defer func() {
		arena.Reset()
		p.pool.Put(arena)
	}()

	action(arena)
}
