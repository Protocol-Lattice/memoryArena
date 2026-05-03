package arena

import (
	"sync"
	"testing"
)

// --- Baselines (Go heap) ---

func BenchmarkMake_Single(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = new(int)
	}
}

func BenchmarkMake_Slice(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = make([]int, 16)
	}
}

// --- Arena: single-thread ---

func BenchmarkArena_New_Single(b *testing.B) {
	a := NewMemoryArena(1024 * 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%1024 == 0 { // reset periodically
			a.Reset()
		}
		_ = New(a, 42)
	}
}

func BenchmarkArena_Slice_Single(b *testing.B) {
	a := NewMemoryArena(1024 * 1024) // 1MB

	const batch = 1024 // how many allocations before reset

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%batch == 0 {
			a.Reset()
		}
		_ = NewSlice[int](a, 16)
	}
}

// --- Arena: parallel (contention test) ---

func BenchmarkArena_New_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		a := NewMemoryArena(1024 * 1024)

		i := 0
		for pb.Next() {
			if i&4095 == 0 { // reset every ~4K allocs
				a.Reset()
			}
			_ = New(a, 42)
			i++
		}
	})
}

func BenchmarkArena_Slice_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		a := NewMemoryArena(1024 * 1024 * 16)
		i := 0
		for pb.Next() {
			if i&4095 == 0 {
				a.Reset()
			}
			_ = NewSlice[int](a, 16)
			i++
		}
	})
}

// --- Reset scenario (important real-world case) ---

func BenchmarkArena_WithReset(b *testing.B) {
	a := NewMemoryArena(1024 * 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%1000 == 0 {
			a.Reset() // manual reset
		}
		_ = New(a, 42)
	}
}

// --- False sharing / contention stress ---

func BenchmarkArena_Contention(b *testing.B) {
	a := NewMemoryArena(1024 * 1024 * 32)

	var wg sync.WaitGroup
	workers := 8

	b.ResetTimer()

	// Max allocations per batch to avoid OOM.
	// 32MB can hold 4M ints. We use a batch of 2M to be safe.
	batch := 2000000 

	for i := 0; i < b.N; {
		ops := batch
		if b.N-i < ops {
			ops = b.N - i
		}

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < ops/workers; j++ {
					_ = New(a, 42)
				}
			}()
		}

		wg.Wait()
		a.Reset()
		
		// If ops wasn't a perfect multiple of workers, 
		// we might do slightly fewer allocations, but this is close enough for the benchmark.
		i += ops
	}
}
