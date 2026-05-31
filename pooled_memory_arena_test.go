package memoryArena

import (
	"sync"
	"testing"
)

func TestPooledMemoryArena_BasicExecution(t *testing.T) {
	pool := NewPooledMemoryArena[int](10)

	var capturedPtr *int
	pool.Execute(func(arena *MemoryArena[int]) {
		capturedPtr = arena.Alloc(42)
		if *capturedPtr != 42 {
			t.Errorf("Expected allocated value to be 42, got %d", *capturedPtr)
		}
	})

	// Outside the block, the pool instance has been returned and reset,
	// but the pointer address should still hold its zeroed out form due to Reset().
	if *capturedPtr != 0 {
		t.Errorf("Expected arena.Reset() to clear the old object value, got %d", *capturedPtr)
	}
}

func TestPooledMemoryArena_DataIsolationAndConcurrency(t *testing.T) {
	const (
		numGoroutines = 50
		iterations    = 100
	)

	pool := NewPooledMemoryArena[int](1000)
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				pool.Execute(func(arena *MemoryArena[int]) {
					// Allocate a predictable slab unique to this goroutine sequence
					baseVal := (gID * iterations) + j
					slab := arena.AllocSlabWith(baseVal, baseVal+1, baseVal+2)

					if len(slab) != 3 {
						t.Errorf("Expected slab length 3, got %d", len(slab))
					}
					if slab[0] != baseVal || slab[1] != baseVal+1 || slab[2] != baseVal+2 {
						t.Errorf("Data corruption detected. Expected structural elements starting at %d, got %v", baseVal, slab)
					}
				})
			}
		}(i)
	}

	wg.Wait()
}

func TestPooledMemoryArena_ResetOnReclaim(t *testing.T) {
	// Keep the pool size strictly small to guarantee that the same arena instance
	// is heavily reassigned across sequential execution calls.
	pool := NewPooledMemoryArena[int](10)

	pool.Execute(func(arena *MemoryArena[int]) {
		_ = arena.AllocSlabWith(1, 2, 3, 4, 5)
		if arena.offset != 5 {
			t.Errorf("Expected initial offset to be 5, got %d", arena.offset)
		}
	})

	// Execute again sequentially. The internal sync.Pool should hand back the same instance
	// or create a new one; in either case, the offset must be completely wiped to 0.
	pool.Execute(func(arena *MemoryArena[int]) {
		if arena.offset != 0 {
			t.Errorf("Expected arena offset to be reset to 0 upon pool check-out, got %d", arena.offset)
		}

		// Ensure old garbage is non-accessible
		slab := arena.AllocSlab(5)
		for _, val := range slab {
			if val != 0 {
				t.Errorf("Expected underlying buffer slots to be zeroed out, found contaminated value: %d", val)
			}
		}
	})
}

func TestPooledMemoryArena_PanicResilience(t *testing.T) {
	pool := NewPooledMemoryArena[int](10)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected an inner allocation panic to bubble up through the execution block")
		}

		// Run an extraction cycle right after a crash to verify the pool structure wasn't permanently deadlocked
		pool.Execute(func(arena *MemoryArena[int]) {
			ptr := arena.Alloc(99)
			if *ptr != 99 {
				t.Errorf("Expected pool functionality to recover post-panic, got %d", *ptr)
			}
		})
	}()

	pool.Execute(func(arena *MemoryArena[int]) {
		// Intentionally exceed the capacity threshold of 10 to throw a core panic
		_ = arena.AllocSlab(100)
	})
}

// Benchmark comparisons reflecting lock-free local allocations vs synchronized paradigms
func BenchmarkPooledMemoryArena_ContendedAlloc(b *testing.B) {
	const arenaSize = 256
	pool := NewPooledMemoryArena[int](arenaSize)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pool.Execute(func(arena *MemoryArena[int]) {
				ptr := arena.Alloc(123)
				slab := arena.AllocSlabWith(1, 2, 3, 4)
				_ = ptr
				_ = slab
			})
		}
	})
}

func BenchmarkConcurrentMemoryArena_ContendedAlloc(b *testing.B) {
	const arenaSize = 50_000_000 // Fixed high bound to handle dynamic benchmark scale
	arena := NewConcurrentMemoryArena[int](arenaSize)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ptr := arena.Alloc(123)
			_ = ptr
		}
	})
}
