package memoryArena

import (
	"runtime"
	"testing"
)

func TestMemoryArenaAlloc(t *testing.T) {
	arena := NewMemoryArena[int](2)

	first := arena.Alloc(10)
	second := arena.Alloc(20)

	if *first != 10 {
		t.Fatalf("expected first allocation to be 10, got %d", *first)
	}

	if *second != 20 {
		t.Fatalf("expected second allocation to be 20, got %d", *second)
	}

	*first = 100

	if *first != 100 {
		t.Fatalf("expected mutation through pointer to update arena value, got %d", *first)
	}
}

func TestMemoryArenaAllocPanicsWhenFull(t *testing.T) {
	arena := NewMemoryArena[int](1)

	arena.Alloc(1)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when arena is full")
		}
	}()

	arena.Alloc(2)
}

func TestMemoryArenaReset(t *testing.T) {
	arena := NewMemoryArena[int](2)

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

	if *next != 30 {
		t.Fatalf("expected allocation after reset to be 30, got %d", *next)
	}

	if next != first {
		t.Fatal("expected arena to reuse the first slot after reset")
	}
}

func TestMemoryArenaAllocSlab(t *testing.T) {
	arena := NewMemoryArena[int](5)

	slab := arena.AllocSlab(3)

	if len(slab) != 3 {
		t.Fatalf("expected slab length 3, got %d", len(slab))
	}

	if cap(slab) != 3 {
		t.Fatalf("expected slab capacity to be capped at 3, got %d", cap(slab))
	}

	slab[0] = 10
	slab[1] = 20
	slab[2] = 30

	next := arena.Alloc(40)

	if *next != 40 {
		t.Fatalf("expected next allocation to be 40, got %d", *next)
	}

	if slab[0] != 10 || slab[1] != 20 || slab[2] != 30 {
		t.Fatalf("unexpected slab values: %#v", slab)
	}
}

func TestMemoryArenaAllocSlabZeroSize(t *testing.T) {
	arena := NewMemoryArena[int](3)

	slab := arena.AllocSlab(0)

	if len(slab) != 0 {
		t.Fatalf("expected empty slab, got length %d", len(slab))
	}

	next := arena.Alloc(10)

	if *next != 10 {
		t.Fatalf("expected next allocation to still use first slot, got %d", *next)
	}
}

// REMOVED: TestMemoryArenaAllocSlabPanicsOnNegativeSize
// This verification has been decoupled because passing a negative value
// is explicitly caught at compile-time via type matching on the uint input signature.

func TestMemoryArenaAllocSlabPanicsWhenFull(t *testing.T) {
	arena := NewMemoryArena[int](2)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when slab does not fit")
		}
	}()

	arena.AllocSlab(3)
}

func TestMemoryArenaAllocSlabWith(t *testing.T) {
	arena := NewMemoryArena[int](5)

	slab := arena.AllocSlabWith(1, 2, 3)

	if len(slab) != 3 {
		t.Fatalf("expected slab length 3, got %d", len(slab))
	}

	expected := []int{1, 2, 3}
	for i := range expected {
		if slab[i] != expected[i] {
			t.Fatalf("expected slab[%d] to be %d, got %d", i, expected[i], slab[i])
		}
	}

	next := arena.Alloc(4)

	if *next != 4 {
		t.Fatalf("expected next allocation to be 4, got %d", *next)
	}
}

func TestMemoryArenaAllocSlabWithPanicsWhenFull(t *testing.T) {
	arena := NewMemoryArena[int](2)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when values do not fit")
		}
	}()

	arena.AllocSlabWith(1, 2, 3)
}

func BenchmarkHeapAlloc(b *testing.B) {
	for i := 0; i < b.N; i++ {
		v := new(int)
		*v = i

		if *v != i {
			b.Fatal("unexpected value")
		}
	}
}

func BenchmarkMemoryArenaAlloc(b *testing.B) {
	arena := NewMemoryArena[int](uint(b.N))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		v := arena.Alloc(i)

		if *v != i {
			b.Fatal("unexpected value")
		}
	}
}

func BenchmarkMemoryArenaAllocWithReset(b *testing.B) {
	const arenaSize = 1024

	arena := NewMemoryArena[int](arenaSize)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if i%arenaSize == 0 {
			arena.Reset()
		}

		v := arena.Alloc(i)

		if *v != i {
			b.Fatal("unexpected value")
		}
	}
}

func BenchmarkHeapSlabAlloc(b *testing.B) {
	const slabSize = 64

	for i := 0; i < b.N; i++ {
		slab := make([]int, slabSize)

		for j := range slab {
			slab[j] = j
		}

		if slab[0] != 0 {
			b.Fatal("unexpected value")
		}
	}
}

func BenchmarkMemoryArenaAllocSlab(b *testing.B) {
	const slabSize = 64

	arena := NewMemoryArena[int](uint(b.N * slabSize))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		slab := arena.AllocSlab(slabSize)

		for j := range slab {
			slab[j] = j
		}

		if slab[0] != 0 {
			b.Fatal("unexpected value")
		}
	}
}

func BenchmarkMemoryArenaAllocSlabWithReset(b *testing.B) {
	const (
		slabSize  = 64
		arenaSize = 1024 * slabSize
	)

	arena := NewMemoryArena[int](arenaSize)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if uint((i*slabSize)+slabSize) > arenaSize {
			arena.Reset()
		}

		slab := arena.AllocSlab(slabSize)

		for j := range slab {
			slab[j] = j
		}

		if slab[0] != 0 {
			b.Fatal("unexpected value")
		}
	}
}

func BenchmarkHeapGCPressure(b *testing.B) {
	type object struct {
		A int
		B int
		C int
		D int
	}

	runtime.GC()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		v := &object{
			A: i,
			B: i + 1,
			C: i + 2,
			D: i + 3,
		}

		if v.A != i {
			b.Fatal("unexpected value")
		}
	}
}

func BenchmarkMemoryArenaGCPressure(b *testing.B) {
	type object struct {
		A int
		B int
		C int
		D int
	}

	arena := NewMemoryArena[object](uint(b.N))

	runtime.GC()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		v := arena.Alloc(object{
			A: i,
			B: i + 1,
			C: i + 2,
			D: i + 3,
		})

		if v.A != i {
			b.Fatal("unexpected value")
		}
	}
}
