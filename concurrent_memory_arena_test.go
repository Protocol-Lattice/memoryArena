package memoryArena

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestConcurrentMemoryArena_Concurrency(t *testing.T) {
	const (
		numGoroutines = 10
		allocsPerG    = 100
		totalSize     = numGoroutines * allocsPerG
	)

	arena := NewConcurrentMemoryArena[int](uint(totalSize))

	var wg sync.WaitGroup
	errs := make(chan string, totalSize)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)

		go func(gID int) {
			defer wg.Done()

			for j := 0; j < allocsPerG; j++ {
				val := gID*allocsPerG + j
				p := arena.Alloc(val)

				if got := *p; got != val {
					errs <- fmt.Sprintf("expected %d, got %d", val, got)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	assertConcurrentArenaFull(t, arena)
}

func TestConcurrentMemoryArenaSlab_Concurrency(t *testing.T) {
	const (
		numGoroutines = 5
		slabSize      = 4
		totalSize     = numGoroutines * slabSize
	)

	arena := NewConcurrentMemoryArena[int](uint(totalSize))

	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)

		go func(gID int) {
			defer wg.Done()

			_ = arena.AllocSlab(uint(slabSize / 2))

			values := []int{gID, gID}
			_ = arena.AllocSlabWith(values...)
		}(i)
	}

	wg.Wait()

	assertConcurrentArenaFull(t, arena)
}

func TestConcurrentMemoryArenaAllocContextCanceledBeforeAcquire(t *testing.T) {
	arena := NewConcurrentMemoryArena[int](1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ptr, err := arena.AllocContext(ctx, 42)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if ptr != nil {
		t.Fatalf("expected nil pointer when context is canceled, got %v", ptr)
	}
}

func TestConcurrentMemoryArenaAllocSlabContextCanceledBeforeAcquire(t *testing.T) {
	arena := NewConcurrentMemoryArena[int](4)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	slab, err := arena.AllocSlabContext(ctx, 2)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if slab != nil {
		t.Fatalf("expected nil slab when context is canceled, got %#v", slab)
	}
}

func TestConcurrentMemoryArenaAllocSlabWithContextCanceledBeforeAcquire(t *testing.T) {
	arena := NewConcurrentMemoryArena[int](4)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	slab, err := arena.AllocSlabWithContext(ctx, 1, 2)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if slab != nil {
		t.Fatalf("expected nil slab when context is canceled, got %#v", slab)
	}
}

func TestConcurrentMemoryArenaResetContextCanceledBeforeAcquire(t *testing.T) {
	arena := NewConcurrentMemoryArena[int](1)
	ptr := arena.Alloc(99)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := arena.ResetContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if *ptr != 99 {
		t.Fatalf("expected failed reset to leave allocated value unchanged, got %d", *ptr)
	}
}

func TestConcurrentMemoryArenaAllocContextCanceledWhileWaitingForSemaphore(t *testing.T) {
	arena := NewConcurrentMemoryArena[int](4)
	holdConcurrentArenaSemaphore(t, arena)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var ptr *int
	var err error

	go func() {
		defer close(done)
		ptr, err = arena.AllocContext(ctx, 42)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for AllocContext")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled while waiting for semaphore, got %v", err)
	}

	if ptr != nil {
		t.Fatalf("expected nil pointer when context is canceled, got %v", ptr)
	}

	if arena.arena.offset != 0 {
		t.Fatalf("expected canceled allocation not to advance offset, got %d", arena.arena.offset)
	}
}

func TestConcurrentMemoryArenaAllocSlabContextCanceledWhileWaitingForSemaphore(t *testing.T) {
	arena := NewConcurrentMemoryArena[int](4)
	holdConcurrentArenaSemaphore(t, arena)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var slab []int
	var err error

	go func() {
		defer close(done)
		slab, err = arena.AllocSlabContext(ctx, 2)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for AllocSlabContext")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled while waiting for semaphore, got %v", err)
	}

	if slab != nil {
		t.Fatalf("expected nil slab when context is canceled, got %#v", slab)
	}

	if arena.arena.offset != 0 {
		t.Fatalf("expected canceled slab allocation not to advance offset, got %d", arena.arena.offset)
	}
}

func TestConcurrentMemoryArenaAllocSlabWithContextCanceledWhileWaitingForSemaphore(t *testing.T) {
	arena := NewConcurrentMemoryArena[int](4)
	holdConcurrentArenaSemaphore(t, arena)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var slab []int
	var err error

	go func() {
		defer close(done)
		slab, err = arena.AllocSlabWithContext(ctx, 1, 2)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for AllocSlabWithContext")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled while waiting for semaphore, got %v", err)
	}

	if slab != nil {
		t.Fatalf("expected nil slab when context is canceled, got %#v", slab)
	}

	if arena.arena.offset != 0 {
		t.Fatalf("expected canceled slab allocation not to advance offset, got %d", arena.arena.offset)
	}
}

func TestConcurrentMemoryArenaResetContextCanceledWhileWaitingForSemaphore(t *testing.T) {
	arena := NewConcurrentMemoryArena[int](4)
	ptr := arena.Alloc(99)

	holdConcurrentArenaSemaphore(t, arena)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() {
		done <- arena.ResetContext(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled while waiting for semaphore, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ResetContext")
	}

	if *ptr != 99 {
		t.Fatalf("expected canceled reset not to clear allocated value, got %d", *ptr)
	}
}

func TestConcurrentMemoryArenaContextMethodsSucceed(t *testing.T) {
	arena := NewConcurrentMemoryArena[int](6)
	ctx := context.Background()

	ptr, err := arena.AllocContext(ctx, 10)
	if err != nil {
		t.Fatalf("AllocContext returned unexpected error: %v", err)
	}

	if *ptr != 10 {
		t.Fatalf("expected allocated value 10, got %d", *ptr)
	}

	slab, err := arena.AllocSlabContext(ctx, 2)
	if err != nil {
		t.Fatalf("AllocSlabContext returned unexpected error: %v", err)
	}

	if len(slab) != 2 || cap(slab) != 2 {
		t.Fatalf("expected slab len/cap 2/2, got %d/%d", len(slab), cap(slab))
	}

	slabWith, err := arena.AllocSlabWithContext(ctx, 1, 2, 3)
	if err != nil {
		t.Fatalf("AllocSlabWithContext returned unexpected error: %v", err)
	}

	expected := []int{1, 2, 3}
	for i := range expected {
		if slabWith[i] != expected[i] {
			t.Fatalf("expected slabWith[%d] to be %d, got %d", i, expected[i], slabWith[i])
		}
	}

	if err := arena.ResetContext(ctx); err != nil {
		t.Fatalf("ResetContext returned unexpected error: %v", err)
	}

	if *ptr != 0 {
		t.Fatalf("expected reset to zero previously allocated value, got %d", *ptr)
	}
}

func TestConcurrentMemoryArenaResetClearsAllocatedValues(t *testing.T) {
	arena := NewConcurrentMemoryArena[int](2)

	first := arena.Alloc(10)
	second := arena.Alloc(20)

	arena.Reset()

	if *first != 0 {
		t.Fatalf("expected first value to be zero after reset, got %d", *first)
	}

	if *second != 0 {
		t.Fatalf("expected second value to be zero after reset, got %d", *second)
	}

	next := arena.Alloc(30)
	if next != first {
		t.Fatal("expected reset to reuse the first slot")
	}

	if *next != 30 {
		t.Fatalf("expected allocation after reset to be 30, got %d", *next)
	}
}

func assertConcurrentArenaFull[T any](t *testing.T, arena *ConcurrentMemoryArena[T]) {
	t.Helper()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected concurrent arena to be full")
		}
	}()

	var zero T
	_ = arena.Alloc(zero)
}

func holdConcurrentArenaSemaphore[T any](t *testing.T, arena *ConcurrentMemoryArena[T]) {
	t.Helper()

	if err := arena.sem.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("failed to acquire arena semaphore: %v", err)
	}

	t.Cleanup(func() {
		arena.sem.Release(1)
	})
}

func BenchmarkConcurrentMemoryArenaAlloc_Contended(b *testing.B) {
	arena := NewConcurrentMemoryArena[int](uint(b.N))

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = arena.Alloc(42)
		}
	})
}

func BenchmarkConcurrentMemoryArenaAlloc_Uncontended(b *testing.B) {
	arena := NewConcurrentMemoryArena[int](uint(b.N))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = arena.Alloc(i)
	}
}

func BenchmarkConcurrentMemoryArenaAllocSlab_Contended(b *testing.B) {
	const slabSize = 64

	arena := NewConcurrentMemoryArena[int](uint(b.N * slabSize))

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = arena.AllocSlab(uint(slabSize))
		}
	})
}
