package arena

import (
	"fmt"
	"testing"
	"unsafe"
)

// --- Alloc ---

func TestAlloc_ReturnsNonNilPointer(t *testing.T) {
	a := NewMemoryArena(64 * 64)
	p := a.Alloc(8, 1)
	if p == nil {
		t.Fatal("expected non-nil pointer from Alloc")
	}
}

func TestAlloc_AdvancesBumpPointer(t *testing.T) {
	a := NewMemoryArena(64 * 64)

	a.Alloc(8, 1)
	if a.AllocatedBytes() < 8 {
		t.Fatalf("expected ptr >= 8, got %d", a.AllocatedBytes())
	}

	prev := a.AllocatedBytes()
	a.Alloc(16, 1)
	if a.AllocatedBytes() <= prev {
		t.Fatalf("expected ptr to advance, got %d -> %d", prev, a.AllocatedBytes())
	}
}

func TestAlloc_PointersDoNotOverlap(t *testing.T) {
	a := NewMemoryArena(64 * 64)

	p1 := uintptr(a.Alloc(8, 1))
	p2 := uintptr(a.Alloc(8, 1))

	if p2 <= p1 {
		t.Fatalf("expected p2 > p1, got p1=%d p2=%d", p1, p2)
	}

	if p2-p1 < 8 {
		t.Fatalf("expected at least 8 bytes separation, got %d", p2-p1)
	}
}

func TestAlloc_ExactFit(t *testing.T) {
	a := NewMemoryArena(16 * 64)
	p := a.Alloc(16, 1)
	if p == nil {
		t.Fatal("expected non-nil pointer for exact-fit allocation")
	}
}

func TestAlloc_OOM_Panics(t *testing.T) {
	a := NewMemoryArena(8 * 64)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on out-of-memory allocation")
		}
	}()

	a.Alloc(9, 1)
}

func TestAlloc_OOM_AfterPartialUse(t *testing.T) {
	a := NewMemoryArena(16 * 64)

	// Allocate 10 bytes in each of the 64 shards, leaving 6 bytes remaining in each.
	for i := 0; i < 64; i++ {
		a.Alloc(10, 1)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when remaining space is too small")
		}
	}()

	// 7 bytes will not fit in any of the remaining 6-byte spaces.
	a.Alloc(7, 1)
}

// --- New ---

func TestNew_StoresAndReadsValue(t *testing.T) {
	a := NewMemoryArena(64 * 64)

	p := New(a, int64(42))
	if *p != 42 {
		t.Fatalf("expected 42, got %d", *p)
	}
}

func TestNew_MutationPersists(t *testing.T) {
	a := NewMemoryArena(64 * 64)

	p := New(a, int32(0))
	*p = 99

	if *p != 99 {
		t.Fatalf("expected 99 after mutation, got %d", *p)
	}
}

func TestNew_MultipleAllocationsDoNotOverlap(t *testing.T) {
	a := NewMemoryArena(128 * 64)

	p1 := New(a, int64(1))
	p2 := New(a, int64(2))

	if p1 == p2 {
		t.Fatal("two New calls returned the same pointer")
	}

	if *p1 != 1 || *p2 != 2 {
		t.Fatalf("values corrupted: p1=%d p2=%d", *p1, *p2)
	}
}

func TestNew_AdvancesPtrByTypeSize(t *testing.T) {
	a := NewMemoryArena(128 * 64)

	before := a.AllocatedBytes()
	New(a, int64(0))
	after := a.AllocatedBytes()

	if after-before < uint64(unsafe.Sizeof(int64(0))) {
		t.Fatalf("expected ptr to advance by at least %d, got %d",
			unsafe.Sizeof(int64(0)), after-before)
	}
}

func TestNew_StructType(t *testing.T) {
	type Point struct{ X, Y float64 }

	a := NewMemoryArena(128 * 64)

	p := New(a, Point{X: 1.5, Y: 2.5})

	if p.X != 1.5 || p.Y != 2.5 {
		t.Fatalf("expected {1.5 2.5}, got %+v", *p)
	}
}

// --- NewSlice ---

func TestNewSlice_ReturnsSlice(t *testing.T) {
	a := NewMemoryArena(256 * 64)

	s := NewSlice[int32](a, 4)

	if len(s) != 4 {
		t.Fatalf("expected len=4, got %d", len(s))
	}
}

func TestNewSlice_IsWritable(t *testing.T) {
	a := NewMemoryArena(256 * 64)

	s := NewSlice[int32](a, 4)

	for i := range s {
		s[i] = int32(i * 10)
	}

	for i := range s {
		if s[i] != int32(i*10) {
			t.Fatalf("unexpected value at %d: %d", i, s[i])
		}
	}
}

func TestNewSlice_AdvancesPtrByExpectedBytes(t *testing.T) {
	a := NewMemoryArena(256 * 64)

	before := a.AllocatedBytes()
	n := 4

	NewSlice[int32](a, n)

	after := a.AllocatedBytes()
	expected := uint64(n) * uint64(unsafe.Sizeof(int32(0)))

	if after-before < expected {
		t.Fatalf("expected ptr advance >= %d bytes, got %d", expected, after-before)
	}
}

func TestNewSlice_OOM_Panics(t *testing.T) {
	a := NewMemoryArena(8 * 64)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when NewSlice exceeds arena capacity")
		}
	}()

	NewSlice[int64](a, 4)
}

func TestAlloc_LargeArenas(t *testing.T) {
	sizes := []int{1 * 1024 * 1024, 10 * 1024 * 1024, 100 * 1024 * 1024, 1024 * 1024 * 1024}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dMB", size/(1024*1024)), func(t *testing.T) {
			a := NewMemoryArena(size)

			// Allocate in chunks until exactly full
			allocSize := uint64(32)
			count := uint64(size) / allocSize

			for i := uint64(0); i < count; i++ {
				p := a.Alloc(allocSize, 1)
				if p == nil {
					t.Fatalf("unexpected nil pointer at allocation %d", i)
				}
			}

			// Arena should be completely full now
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("expected panic when allocating past 100% capacity")
				}
			}()
			a.Alloc(8, 1)
		})
	}
}
