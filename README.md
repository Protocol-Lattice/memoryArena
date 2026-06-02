<img width="1024" height="1024" alt="ChatGPT Image 2 cze 2026 o 14_11_49" src="https://github.com/user-attachments/assets/d756a6bf-23fe-4a6d-9c1d-3887080a175f" />

A small, typed memory arena for Go.

`memoryArena` pre-allocates a fixed-size backing slice and hands out values from it by advancing an offset. Instead of allocating many short-lived objects individually on the heap, you can allocate them from an arena and reuse the whole backing buffer with `Reset`.

```go
import "github.com/Protocol-Lattice/memoryArena"
```

## Why use this?

Go’s garbage collector is excellent, but some workloads create large numbers of short-lived temporary values. Examples include parsers, request-scoped buffers, simulation ticks, batch processing, and hot loops.

An arena changes the ownership model:

- allocate many values from one backing buffer;
- reuse memory all at once with `Reset`;
- avoid individual object lifetime management;
- reduce garbage collector pressure in scoped workloads.

This package is intentionally **safe and Go-native**. It uses typed `[]T` storage instead of raw `unsafe.Pointer` memory.

## Features

- Generic `MemoryArena[T]` backed by `[]T`.
- Fast single-value allocation with `Alloc`.
- Non-panicking allocation with `TryAlloc`.
- Contiguous slab allocation with `AllocSlab`.
- Non-panicking slab allocation with `TryAllocSlab`.
- Copy-in slab allocation with `AllocSlabWith`.
- Non-panicking copy-in slab allocation with `TryAllocSlabWith`.
- Capacity helpers: `Used`, `Remaining`, and `Cap`.
- `Reset` clears only the allocated region before reuse.
- `ConcurrentMemoryArena[T]` for synchronized shared allocation.
- Context-aware concurrent allocation methods.
- `PooledMemoryArena[T]` for high-concurrency scoped workloads.
- Context helpers for request-scoped arena ownership.
- HTTP request helpers for injecting and extracting arenas from `*http.Request`.

## Install

```sh
go get github.com/Protocol-Lattice/memoryArena
```

## Quick start

```go
package main

import (
	"fmt"

	"github.com/Protocol-Lattice/memoryArena"
)

func main() {
	arena := memoryArena.NewMemoryArena[int](4)

	first := arena.Alloc(10)
	second := arena.Alloc(20)

	fmt.Println(*first)
	fmt.Println(*second)

	arena.Reset()

	next := arena.Alloc(30)
	fmt.Println(*next)
}
```

## Core API

### Create an arena

```go
arena := memoryArena.NewMemoryArena[int](1024)
```

This creates an arena that can hold `1024` values of type `int`.

The arena is fixed-size. If you allocate past capacity with `Alloc`, it panics. Use the `Try*` APIs when you want explicit capacity handling.

## Single-value allocation

```go
arena := memoryArena.NewMemoryArena[string](2)

name := arena.Alloc("kamil")

fmt.Println(*name)
```

`Alloc` copies the value into the next free slot and returns a pointer to that slot.

```go
ptr := arena.Alloc("hello")
```

The returned pointer points directly into the arena buffer.

## Non-panicking allocation

Use `TryAlloc` when capacity exhaustion should be handled manually.

```go
arena := memoryArena.NewMemoryArena[int](2)

first, ok := arena.TryAlloc(10)
if !ok {
	return
}

second, ok := arena.TryAlloc(20)
if !ok {
	return
}

third, ok := arena.TryAlloc(30)
if !ok {
	fmt.Println("arena is full")
}

_, _, _ = first, second, third
```

Failed `Try*` allocations do not advance the arena offset.

## Capacity helpers

```go
arena := memoryArena.NewMemoryArena[int](4)

fmt.Println(arena.Cap())       // 4
fmt.Println(arena.Used())      // 0
fmt.Println(arena.Remaining()) // 4

_ = arena.Alloc(10)

fmt.Println(arena.Used())      // 1
fmt.Println(arena.Remaining()) // 3
```

## Slab allocation

Use `AllocSlab` when you need a contiguous region inside the arena.

```go
arena := memoryArena.NewMemoryArena[int](8)

slab := arena.AllocSlab(3)

slab[0] = 10
slab[1] = 20
slab[2] = 30

next := arena.Alloc(40)

fmt.Println(slab)
fmt.Println(*next)
```

The returned slab is capacity-bounded, so appending to it cannot silently overwrite later arena allocations.

```go
slab := arena.AllocSlab(3)

// cap(slab) == len(slab)
```

## Non-panicking slab allocation

```go
arena := memoryArena.NewMemoryArena[int](2)

slab, ok := arena.TryAllocSlab(4)
if !ok {
	fmt.Println("slab does not fit")
	return
}

_ = slab
```

## Slab allocation with values

Use `AllocSlabWith` when you already have values to copy into the arena.

```go
arena := memoryArena.NewMemoryArena[string](4)

names := arena.AllocSlabWith("alice", "bob", "carol")

fmt.Println(names)
```

Use `TryAllocSlabWith` when you want non-panicking behavior.

```go
arena := memoryArena.NewMemoryArena[string](2)

names, ok := arena.TryAllocSlabWith("alice", "bob", "carol")
if !ok {
	fmt.Println("values do not fit")
	return
}

_ = names
```

## Reset behavior

`Reset` clears the allocated portion of the arena and moves the offset back to zero.

```go
arena := memoryArena.NewMemoryArena[int](2)

first := arena.Alloc(10)
second := arena.Alloc(20)

arena.Reset()

fmt.Println(*first)  // 0
fmt.Println(*second) // 0

next := arena.Alloc(30)
fmt.Println(*next) // 30
```

Pointers and slabs returned before `Reset` still point into the backing buffer, but their previous values are no longer live.

Treat old pointers and slabs as invalid after reset.

## Mixed-type allocation

The preferred usage is strongly typed:

```go
arena := memoryArena.NewMemoryArena[int](16)
```

If you need mixed values, you can use `any`:

```go
arena := memoryArena.NewMemoryArena[any](4)

a := arena.Alloc(123)
b := arena.Alloc("hello")

fmt.Println((*a).(int))
fmt.Println((*b).(string))
```

This allows heterogeneous storage, but the returned pointers are `*any`, so reading values requires type assertions.

```go
value := (*a).(int)
```

For maximum type safety and performance, prefer concrete `T` arenas when possible.

## Concurrent usage

`MemoryArena[T]` is intentionally not thread-safe. It is designed for single-owner hot paths.

For concurrent workloads, use one of these models:

| Type | Best for | Synchronization model |
| --- | --- | --- |
| `MemoryArena[T]` | Single-owner hot paths | None |
| `ConcurrentMemoryArena[T]` | Shared arena across goroutines | Single semaphore gate |
| `PooledMemoryArena[T]` | Request/job scoped concurrent workloads | `sync.Pool` of isolated arenas |

## Shared synchronized arena

`ConcurrentMemoryArena[T]` wraps a regular arena with a single semaphore gate. All allocations and resets are synchronized.

```go
package main

import (
	"sync"

	"github.com/Protocol-Lattice/memoryArena"
)

func main() {
	arena := memoryArena.NewConcurrentMemoryArena[int](1024)

	var wg sync.WaitGroup

	for workerID := 0; workerID < 8; workerID++ {
		wg.Add(1)

		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < 100; i++ {
				value := workerID*100 + i
				ptr := arena.Alloc(value)

				_ = ptr
			}
		}(workerID)
	}

	wg.Wait()
}
```

The synchronized arena protects allocation and reset operations. It does not protect later reads or writes through returned pointers and slices.

If multiple goroutines access the same returned pointer or slab, coordinate that access yourself.

## Context-aware concurrent allocation

Use context-aware methods when waiting for the arena gate should respect cancellation or deadlines.

```go
ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
defer cancel()

arena := memoryArena.NewConcurrentMemoryArena[int](1024)

ptr, err := arena.AllocContext(ctx, 42)
if err != nil {
	return err
}

_ = ptr
```

Context-aware slab allocation is also available:

```go
slab, err := arena.AllocSlabContext(ctx, 4)
if err != nil {
	return err
}

_ = slab
```

Copy-in slab allocation:

```go
slab, err := arena.AllocSlabWithContext(ctx, 1, 2, 3, 4)
if err != nil {
	return err
}

_ = slab
```

Context-aware reset:

```go
if err := arena.ResetContext(ctx); err != nil {
	return err
}
```

## Pooled arena

`PooledMemoryArena[T]` is designed for high-concurrency scoped workloads.

Instead of many goroutines fighting over one shared arena, each `Execute` call checks out an isolated arena from a `sync.Pool`, runs your function, resets the arena, and returns it to the pool.

```go
pool := memoryArena.NewPooledMemoryArena[int](256)

pool.Execute(func(arena *memoryArena.MemoryArena[int]) {
	ptr := arena.Alloc(123)
	slab := arena.AllocSlabWith(1, 2, 3, 4)

	_, _ = ptr, slab
})
```

Values allocated inside the closure should not escape the closure.

Good use cases:

- HTTP request handlers;
- worker jobs;
- parser passes;
- temporary batch processing;
- high-concurrency loops.

`Execute` resets the arena even if the closure panics. The panic still bubbles up to the caller.

## Context helpers

You can attach an arena to a `context.Context`.

```go
ctx := context.Background()
arena := memoryArena.NewMemoryArena[int](128)

ctx = memoryArena.Inject(ctx, arena)

got, err := memoryArena.Extract[int](ctx)
if err != nil {
	return err
}

_ = got
```

If no arena exists, the stored arena is nil, or the generic type does not match, `Extract` returns `ErrNoArena`.

```go
got, err := memoryArena.Extract[string](ctx)
if errors.Is(err, memoryArena.ErrNoArena) {
	// no string arena found
}

_ = got
```

For performance-sensitive code, prefer passing `*MemoryArena[T]` explicitly instead of storing it in context.

## HTTP request helpers

Use `InjectRequest` and `ExtractRequest` for request-scoped arena ownership.

```go
func handler(w http.ResponseWriter, r *http.Request) {
	arena := memoryArena.NewMemoryArena[int](128)

	r = memoryArena.InjectRequest(r, arena)

	got, err := memoryArena.ExtractRequest[int](r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = got
}
```

`InjectRequest` returns a cloned request with the arena attached to the request context. It does not mutate the original request.

## API reference

### `MemoryArena`

```go
func NewMemoryArena[T any](size uint) *MemoryArena[T]
```

Creates a fixed-size arena for values of type `T`.

```go
func (arena *MemoryArena[T]) Alloc(obj T) *T
```

Copies `obj` into the next free slot and returns a pointer to it. Panics if the arena is full.

```go
func (arena *MemoryArena[T]) TryAlloc(obj T) (*T, bool)
```

Copies `obj` into the next free slot. Returns `nil, false` if the arena is full.

```go
func (arena *MemoryArena[T]) AllocSlab(size uint) []T
```

Reserves `size` contiguous slots and returns them as a capacity-bounded slice. Panics if the slab does not fit.

```go
func (arena *MemoryArena[T]) TryAllocSlab(size uint) ([]T, bool)
```

Reserves `size` contiguous slots. Returns `nil, false` if the slab does not fit.

```go
func (arena *MemoryArena[T]) AllocSlabWith(values ...T) []T
```

Reserves a slab, copies `values` into it, and returns the allocated slab. Panics if the values do not fit.

```go
func (arena *MemoryArena[T]) TryAllocSlabWith(values ...T) ([]T, bool)
```

Reserves a slab, copies `values` into it, and returns the allocated slab. Returns `nil, false` if the values do not fit.

```go
func (arena *MemoryArena[T]) Reset()
```

Clears the allocated region and resets the offset to zero.

```go
func (arena *MemoryArena[T]) Used() uint
```

Returns the number of allocated slots.

```go
func (arena *MemoryArena[T]) Remaining() uint
```

Returns the number of available slots.

```go
func (arena *MemoryArena[T]) Cap() uint
```

Returns the total arena capacity.

### `ConcurrentMemoryArena`

```go
func NewConcurrentMemoryArena[T any](size uint) *ConcurrentMemoryArena[T]
```

Creates a synchronized arena.

```go
func (arena *ConcurrentMemoryArena[T]) Alloc(obj T) *T
```

Synchronizes access, allocates one value, and returns a pointer to it.

```go
func (arena *ConcurrentMemoryArena[T]) AllocContext(ctx context.Context, obj T) (*T, error)
```

Attempts to acquire the arena gate using `ctx`, then allocates one value.

```go
func (arena *ConcurrentMemoryArena[T]) AllocSlab(size uint) []T
```

Synchronizes access and allocates a contiguous slab.

```go
func (arena *ConcurrentMemoryArena[T]) AllocSlabContext(ctx context.Context, size uint) ([]T, error)
```

Attempts to acquire the arena gate using `ctx`, then allocates a contiguous slab.

```go
func (arena *ConcurrentMemoryArena[T]) AllocSlabWith(values ...T) []T
```

Synchronizes access, reserves a slab, and copies values into it.

```go
func (arena *ConcurrentMemoryArena[T]) AllocSlabWithContext(ctx context.Context, values ...T) ([]T, error)
```

Attempts to acquire the arena gate using `ctx`, reserves a slab, and copies values into it.

```go
func (arena *ConcurrentMemoryArena[T]) Reset()
```

Synchronizes access and resets the underlying arena.

```go
func (arena *ConcurrentMemoryArena[T]) ResetContext(ctx context.Context) error
```

Attempts to acquire the arena gate using `ctx`, then resets the underlying arena.

### `PooledMemoryArena`

```go
func NewPooledMemoryArena[T any](arenaSize uint) *PooledMemoryArena[T]
```

Creates a pool of reusable arenas.

```go
func (pool *PooledMemoryArena[T]) Execute(action func(arena *MemoryArena[T]))
```

Checks out an arena, runs `action`, resets the arena, and returns it to the pool.

### Context helpers

```go
func Inject[T any](ctx context.Context, arena *MemoryArena[T]) context.Context
```

Returns a child context containing the arena.

```go
func Extract[T any](ctx context.Context) (*MemoryArena[T], error)
```

Extracts a typed arena from context.

### HTTP request helpers

```go
func InjectRequest[T any](r *http.Request, arena *MemoryArena[T]) *http.Request
```

Returns a cloned request with the arena injected into its context.

```go
func ExtractRequest[T any](r *http.Request) (*MemoryArena[T], error)
```

Extracts a typed arena from the request context.

## Safety notes

- `MemoryArena[T]` is not thread-safe.
- Use `ConcurrentMemoryArena[T]` if multiple goroutines must allocate from one shared arena.
- Use `PooledMemoryArena[T]` when each task/request can use an isolated temporary arena.
- Returned pointers and slabs point directly into the arena buffer.
- Do not use returned values after `Reset`.
- Do not use values allocated inside `PooledMemoryArena.Execute` after the closure returns.
- `Reset` clears allocated slots, so old pointers may observe zero values.
- `AllocSlab` and `AllocSlabWith` return capacity-bounded slices.
- Failed `Try*` allocations do not advance the offset.
- Arena capacity is fixed.
- Use `MemoryArena[any]` only when heterogeneous storage is needed and type assertions are acceptable.

## Benchmarks

Run all benchmarks:

```sh
go test -bench=. -benchmem
```

Run tests with the race detector:

```sh
go test -race ./...
```

Run all tests:

```sh
go test ./...
```

The benchmark suite covers:

- heap allocation vs arena allocation;
- reset-heavy reuse;
- slab allocation;
- heap slice allocation vs arena slab allocation;
- GC pressure comparisons;
- contended concurrent allocation;
- contended concurrent slab allocation;
- pooled arena execution under parallel contention.

## When to use this

Use `memoryArena` when:

- values have a shared scoped lifetime;
- you can reset all temporary values together;
- you want typed arena-style allocation without unsafe raw memory;
- you want fewer heap allocations in hot paths;
- you are building parsers, request-scoped buffers, simulations, temporary batches, or worker-local scratch memory.

## When not to use this

Do not use an arena when:

- values need independent lifetimes;
- values must be freed individually;
- values escape unpredictably;
- capacity is unknown and unbounded;
- regular heap allocation is already simple and fast enough;
- the code becomes harder to reason about because of reset-based lifetime management.

## License

MIT
