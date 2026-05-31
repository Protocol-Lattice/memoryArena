package memoryArena

type MemoryArena[T any] struct {
	buffer []T
	offset uint
}

func NewMemoryArena[T any](size uint) *MemoryArena[T] {
	return &MemoryArena[T]{
		buffer: make([]T, size),
		offset: 0,
	}
}

func (arena *MemoryArena[T]) Alloc(obj T) *T {
	allocated, ok := arena.TryAlloc(obj)
	if !ok {
		panic("memory arena: not enough space")
	}

	return allocated
}

func (arena *MemoryArena[T]) TryAlloc(obj T) (*T, bool) {
	if arena.offset >= arena.Cap() {
		return nil, false
	}

	arena.buffer[arena.offset] = obj
	allocated := &arena.buffer[arena.offset]
	arena.offset++

	return allocated, true
}

func (arena *MemoryArena[T]) Reset() {
	if arena.offset > 0 {
		clear(arena.buffer[:arena.offset])
	}

	arena.offset = 0
}

func (arena *MemoryArena[T]) AllocSlab(size uint) []T {
	slab, ok := arena.TryAllocSlab(size)
	if !ok {
		panic("memory arena: not enough space for slab")
	}

	return slab
}

func (arena *MemoryArena[T]) TryAllocSlab(size uint) ([]T, bool) {
	if size > arena.Remaining() {
		return nil, false
	}

	start := arena.offset
	end := arena.offset + size

	arena.offset = end

	return arena.buffer[start:end:end], true
}

func (arena *MemoryArena[T]) AllocSlabWith(values ...T) []T {
	slab, ok := arena.TryAllocSlabWith(values...)
	if !ok {
		panic("memory arena: not enough space for slab")
	}

	return slab
}

func (arena *MemoryArena[T]) TryAllocSlabWith(values ...T) ([]T, bool) {
	slab, ok := arena.TryAllocSlab(uint(len(values)))
	if !ok {
		return nil, false
	}

	copy(slab, values)

	return slab, true
}

func (arena *MemoryArena[T]) Used() uint {
	return arena.offset
}

func (arena *MemoryArena[T]) Cap() uint {
	return uint(len(arena.buffer))
}

func (arena *MemoryArena[T]) Remaining() uint {
	return arena.Cap() - arena.Used()
}
