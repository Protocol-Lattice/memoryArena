package arena

import (
	"sync/atomic"
	"unsafe"
)

const numShards = 64

type arenaShard struct {
	ptr   atomic.Uint64
	limit uint64
	_     [112]byte // pad to 128 bytes to prevent false sharing on ARM64
}

type MemoryArena struct {
	buffer []byte
	shards [numShards]arenaShard
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

// align rounds ptr up to alignment
func align(ptr, alignment uint64) uint64 {
	mask := alignment - 1
	return (ptr + mask) & ^mask
}

func (a *MemoryArena) Alloc(size, alignment uint64) unsafe.Pointer {
	addr := uintptr(unsafe.Pointer(&size))
	// Fibonacci hash to evenly distribute stack addresses
	hash := uint64(addr) * 0x9E3779B185EBCA87
	startIdx := uintptr(hash >> 58) & (numShards - 1)
	
	if alignment <= 8 {
		allocSize := (size + 7) &^ 7
		
		shard := &a.shards[startIdx]
		if shard.ptr.Load() <= shard.limit {
			next := shard.ptr.Add(allocSize)
			if next <= shard.limit {
				return unsafe.Pointer(&a.buffer[next-allocSize])
			}
		}

		return a.allocFallback(allocSize, startIdx)
	}

	return a.allocSlow(size, alignment, startIdx)
}

//go:noinline
func (a *MemoryArena) allocFallback(allocSize uint64, startIdx uintptr) unsafe.Pointer {
	for i := uintptr(1); i < numShards; i++ {
		idx := (startIdx + i) & (numShards - 1)
		shard := &a.shards[idx]
		
		if shard.ptr.Load() <= shard.limit {
			next := shard.ptr.Add(allocSize)
			if next <= shard.limit {
				return unsafe.Pointer(&a.buffer[next-allocSize])
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
				break // Not enough space in this shard, try the next one
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
}

func (a *MemoryArena) AllocatedBytes() uint64 {
	var total uint64
	shardSize := uint64(len(a.buffer)) / numShards
	for i := range a.shards {
		used := a.shards[i].ptr.Load() - (uint64(i) * shardSize)
		// Since we use unconditional Add, ptr can overflow shardSize when full.
		if used > shardSize {
			used = shardSize
		}
		total += used
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
