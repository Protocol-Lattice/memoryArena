# memoryArena

<p align="center">
  <img src="./logo.png" alt="memoryArena Logo" width="400">
</p>
An ultra-fast, concurrent-safe, sharded bump-pointer memory arena for Go. 

`memoryArena` avoids the immense GC overhead of millions of small object allocations by pre-allocating a large byte buffer and bumping a pointer. It is strictly optimized for high-concurrency environments, utilizing stack-pointer Fibonacci hashing to deterministically assign goroutines to hardware-isolated memory shards, virtually eliminating atomic cache-line contention (`False Sharing`).

## Features
- **Zero GC Overhead:** Allocate structs and slices directly into the arena. 
- **Thread-Safe & Lock-Free:** Built on top of `atomic.Uint64` with no locks or mutexes.
- **Cache-Line Isolated Shards:** Automatically divides the arena into 64 padded shards. Goroutines are deterministically hashed to shards to prevent CPU cache-line bouncing.
- **Graceful Linear Probing:** If a shard runs out of space, the allocator instantly falls back to the next available shard.
- **Architecture Safe:** Uses Go 1.19+ `atomic.Uint64` to prevent 32-bit atomic alignment panics.

## Usage

```go
package main

import (
    "fmt"
    arena "github.com/Protocol-Lattice/memoryArena"
)

func main() {
    // Create a 32MB arena
    a := arena.NewMemoryArena(32 * 1024 * 1024)

    // Allocate a single struct
    val := arena.New(a, 42)
    fmt.Println(*val)

    // Allocate a slice
    slice := arena.NewSlice[int](a, 100)
    slice[0] = 99

    // Reset the arena (reclaims all memory instantly)
    a.Reset()
}
```

## Performance & Benchmarks

The arena is optimized to rival the speed of raw stack allocations. In highly-contended multi-threaded scenarios, it outperforms traditional shared-atomic arenas by **>15x** (dropping from ~250ns/op down to ~16ns/op).

```text
goos: darwin
goarch: arm64
pkg: github.com/Protocol-Lattice/memoryArena
cpu: Apple M2
BenchmarkMake_Single-8                  1000000000               0.2902 ns/op
BenchmarkMake_Slice-8                   1000000000               0.2913 ns/op
BenchmarkArena_New_Single-8             252217918                4.646 ns/op
BenchmarkArena_Slice_Single-8           263953323                4.642 ns/op
BenchmarkArena_New_Parallel-8           1000000000               0.8971 ns/op
BenchmarkArena_Slice_Parallel-8         1000000000               0.8586 ns/op
BenchmarkArena_WithReset-8              261591999                4.520 ns/op
BenchmarkArena_Contention-8             1000000000               0.9097 ns/op
```

*Note: The native `Make` benchmarks measure standard Go allocations which the Go compiler escapes to the thread-local stack/mcache, effectively making them 0-cost. Real-world heap allocations under GC pressure take significantly longer, where `memoryArena` shines.*
