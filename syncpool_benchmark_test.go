package memoryArena

import (
	"strconv"
	"sync"
	"testing"
)

var syncPoolBenchmarkSink byte

func BenchmarkArenaVsSyncPoolLargeRepeatedAllocations(b *testing.B) {
	benchmarks := []struct {
		name string
		size uint
	}{
		{name: "4KiB", size: 4 << 10},
		{name: "64KiB", size: 64 << 10},
		{name: "1MiB", size: 1 << 20},
		{name: "4MiB", size: 4 << 20},
		{name: "16MiB", size: 16 << 20},
		{name: "64MiB", size: 64 << 20},
		{name: "100MiB", size: 100 << 20},
		{name: "1GiB", size: 1 << 30},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.Run("HeapMake", func(b *testing.B) {
				benchmarkHeapMakeBytes(b, int(bm.size))
			})

			b.Run("ArenaSlabReuse", func(b *testing.B) {
				benchmarkArenaSlabReuseBytes(b, bm.size)
			})

			b.Run("SyncPoolSliceReuse", func(b *testing.B) {
				benchmarkSyncPoolSliceReuseBytes(b, int(bm.size))
			})
		})
	}
}

func BenchmarkArenaVsSyncPoolRepeatedLargeBatches(b *testing.B) {
	benchmarks := []struct {
		items int
	}{
		{items: 1_024},       // 8 KiB on 64-bit platforms.
		{items: 16_384},      // 128 KiB on 64-bit platforms.
		{items: 65_536},      // 512 KiB on 64-bit platforms.
		{items: 1_048_576},   // 8 MiB on 64-bit platforms.
		{items: 13_107_200},  // 100 MiB on 64-bit platforms.
		{items: 134_217_728}, // 1 GiB on 64-bit platforms.
	}

	for _, bm := range benchmarks {
		name := strconv.Itoa(bm.items) + "Items"

		b.Run(name, func(b *testing.B) {
			b.Run("ArenaBatchReuse", func(b *testing.B) {
				arena := NewMemoryArena[int](uint(bm.items))

				b.ReportAllocs()
				b.SetBytes(int64(bm.items * 8))
				b.ResetTimer()

				var sum int
				for i := 0; i < b.N; i++ {
					slab := arena.AllocSlab(uint(bm.items))
					for j := range slab {
						slab[j] = j
					}
					sum += slab[len(slab)-1]
					arena.Reset()
				}

				syncPoolBenchmarkSink = byte(sum)
			})

			b.Run("SyncPoolBatchReuse", func(b *testing.B) {
				pool := sync.Pool{
					New: func() any {
						return make([]int, bm.items)
					},
				}

				b.ReportAllocs()
				b.SetBytes(int64(bm.items * 8))
				b.ResetTimer()

				var sum int
				for i := 0; i < b.N; i++ {
					buf := pool.Get().([]int)[:bm.items]
					for j := range buf {
						buf[j] = j
					}
					sum += buf[len(buf)-1]
					clear(buf)
					pool.Put(buf)
				}

				syncPoolBenchmarkSink = byte(sum)
			})
		})
	}
}

func benchmarkHeapMakeBytes(b *testing.B, size int) {
	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()

	var sum byte
	for i := 0; i < b.N; i++ {
		buf := make([]byte, size)
		touchBytes(buf)
		sum ^= buf[len(buf)-1]
	}

	syncPoolBenchmarkSink = sum
}

func benchmarkArenaSlabReuseBytes(b *testing.B, size uint) {
	arena := NewMemoryArena[byte](size)

	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()

	var sum byte
	for i := 0; i < b.N; i++ {
		buf := arena.AllocSlab(size)
		touchBytes(buf)
		sum ^= buf[len(buf)-1]
		arena.Reset()
	}

	syncPoolBenchmarkSink = sum
}

func benchmarkSyncPoolSliceReuseBytes(b *testing.B, size int) {
	pool := sync.Pool{
		New: func() any {
			return make([]byte, size)
		},
	}

	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()

	var sum byte
	for i := 0; i < b.N; i++ {
		buf := pool.Get().([]byte)[:size]
		touchBytes(buf)
		sum ^= buf[len(buf)-1]
		clear(buf)
		pool.Put(buf)
	}

	syncPoolBenchmarkSink = sum
}

func touchBytes(buf []byte) {
	// Touch one byte per cache line to model real repeated large temporary writes
	// without turning the benchmark into a pure memset throughput test.
	for i := 0; i < len(buf); i += 64 {
		buf[i] = byte(i)
	}
	buf[len(buf)-1] = byte(len(buf))
}
