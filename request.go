package memoryArena

import "net/http"

// InjectRequest attaches the arena to r.Context() and returns a cloned request.
func InjectRequest[T any](r *http.Request, arena *MemoryArena[T]) *http.Request {
	return r.WithContext(Inject(r.Context(), arena))
}

// ExtractRequest retrieves the arena from r.Context().
func ExtractRequest[T any](r *http.Request) (*MemoryArena[T], error) {
	return Extract[T](r.Context())
}
