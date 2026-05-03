package arena

import (
	"sync/atomic"
	"unsafe"
)

//go:linkname runtime_procPin runtime.procPin
func runtime_procPin() int

//go:linkname runtime_procUnpin runtime.procUnpin
func runtime_procUnpin()

const (
	numShards = 64
	tlabSize  = 65536 // 64 KB per-P allocation buffer
	maxProcs  = 1024  // safe upper bound for GOMAXPROCS
)

type arenaShard struct {
	ptr   atomic.Uint64
	limit uint64
	_     [112]byte // pad to 128 bytes – prevent false sharing on ARM64
}

// perPSlab is a per-OS-thread (per-P) bump-pointer cache.
// Because Alloc pins the goroutine to its P before touching these fields,
// no atomics are needed on the hot path.
type perPSlab struct {
	ptr   uint64
	limit uint64
	_     [112]byte // same padding
}

type MemoryArena struct {
	buffer []byte
	shards [numShards]arenaShard
	perP   [maxProcs]perPSlab
}

func NewMemoryArena(size int) *MemoryArena {
	a := &MemoryArena{
		buffer: make([]byte, size),
	}
	shardSize := uint64(size) / numShards
	for i := 0; i < numShards; i++ {
		a.shards[i].ptr.Store(uint64(i) * shardSize)
		a.shards[i].limit = uint64(i+1) * shardSize
	}
	return a
}

// align rounds ptr up to alignment (must be a power of two).
func align(ptr, alignment uint64) uint64 {
	mask := alignment - 1
	return (ptr + mask) &^ mask
}

func (a *MemoryArena) Alloc(size, alignment uint64) unsafe.Pointer {
	if alignment <= 8 {
		allocSize := (size + 7) &^ 7

		// ── Per-P TLAB fast path (zero atomics) ─────────────────────────
		pid := runtime_procPin()
		slab := &a.perP[pid]
		ptr := slab.ptr
		next := ptr + allocSize
		if next <= slab.limit {
			slab.ptr = next
			runtime_procUnpin()
			return unsafe.Pointer(&a.buffer[ptr])
		}
		runtime_procUnpin()

		// TLAB exhausted – claim a fresh chunk from the sharded arena.
		return a.refillAndAlloc(allocSize)
	}

	// Slow path for exotic alignments (CAS loop).
	addr := uintptr(unsafe.Pointer(&size))
	hash := uint64(addr) * 0x9E3779B185EBCA87
	startIdx := uintptr(hash>>58) & (numShards - 1)
	return a.allocSlow(size, alignment, startIdx)
}

// refillAndAlloc grabs a fresh chunk from the sharded arena,
// installs it as the calling P's TLAB, and returns the first allocation.
// It claims at most tlabSize bytes, but never more than what the shard has.
//
//go:noinline
func (a *MemoryArena) refillAndAlloc(allocSize uint64) unsafe.Pointer {
	// Double-check current P's TLAB after pinning, in case we migrated
	// since the check in Alloc().
	pid := runtime_procPin()
	slab := &a.perP[pid]
	if slab.ptr+allocSize <= slab.limit {
		ptr := slab.ptr
		slab.ptr += allocSize
		runtime_procUnpin()
		return unsafe.Pointer(&a.buffer[ptr])
	}
	runtime_procUnpin()

	// Use local stack address for shard diversity across goroutines.
	var dummy uint64
	addr := uintptr(unsafe.Pointer(&dummy))
	hash := uint64(addr) * 0x9E3779B185EBCA87
	startIdx := uintptr(hash>>58) & (numShards - 1)


	for i := uintptr(0); i < numShards; i++ {
		idx := (startIdx + i) & (numShards - 1)
		sh := &a.shards[idx]

		for {
			cur := sh.ptr.Load()
			if cur >= sh.limit {
				break // shard is full
			}
			remaining := sh.limit - cur
			if remaining < allocSize {
				break // not enough even for this one alloc
			}
			// Claim up to tlabSize but never more than what remains.
			take := uint64(tlabSize)
			if take > remaining {
				take = remaining
			}
			if sh.ptr.CompareAndSwap(cur, cur+take) {
				// Install the claimed chunk as this P's TLAB.
				pid := runtime_procPin()
				a.perP[pid].ptr = cur + allocSize
				a.perP[pid].limit = cur + take
				runtime_procUnpin()
				return unsafe.Pointer(&a.buffer[cur])
			}
		}
	}
	panic("arena: out of memory")
}

//go:noinline
func (a *MemoryArena) allocSlow(size, alignment uint64, startIdx uintptr) unsafe.Pointer {
	for i := uintptr(0); i < numShards; i++ {
		idx := (startIdx + i) & (numShards - 1)
		shard := &a.shards[idx]

		for {
			current := shard.ptr.Load()
			aligned := align(current, alignment)
			next := align(aligned+size, 8)

			if next > shard.limit {
				break
			}
			if shard.ptr.CompareAndSwap(current, next) {
				return unsafe.Pointer(&a.buffer[aligned])
			}
		}
	}
	panic("arena: out of memory")
}

func (a *MemoryArena) Reset() {
	shardSize := uint64(len(a.buffer)) / numShards
	for i := 0; i < numShards; i++ {
		a.shards[i].ptr.Store(uint64(i) * shardSize)
	}
	// Invalidate all per-P TLABs so they refill from the reset shards.
	for i := range a.perP {
		a.perP[i].ptr = 0
		a.perP[i].limit = 0
	}
}

func (a *MemoryArena) AllocatedBytes() uint64 {
	// Shards advance their ptr by the full TLAB chunk on every refill,
	// so committed = bytes actually handed to callers + bytes still sitting
	// unused in per-P TLABs. Subtract the unused TLAB tails to get the
	// true "bytes allocated by callers" figure.
	var total uint64
	shardSize := uint64(len(a.buffer)) / numShards
	for i := range a.shards {
		committed := a.shards[i].ptr.Load()
		base := uint64(i) * shardSize
		if committed <= base {
			continue
		}
		used := committed - base
		if used > shardSize {
			used = shardSize
		}
		total += used
	}
	// Subtract unused bytes still held in per-P TLABs.
	for i := range a.perP {
		if a.perP[i].limit > a.perP[i].ptr {
			total -= a.perP[i].limit - a.perP[i].ptr
		}
	}
	return total
}

func New[T any](a *MemoryArena, value T) *T {
	var zero T
	size := uint64(unsafe.Sizeof(zero))
	align := uint64(unsafe.Alignof(zero))

	ptr := (*T)(a.Alloc(size, align))
	*ptr = value
	return ptr
}

func NewSlice[T any](a *MemoryArena, length int) []T {
	var zero T
	elemSize := unsafe.Sizeof(zero)
	elemAlign := unsafe.Alignof(zero)

	totalSize := uint64(length) * uint64(elemSize)

	ptr := a.Alloc(totalSize, uint64(elemAlign))

	// build slice header manually
	slice := unsafe.Slice((*T)(ptr), length)
	return slice
}
