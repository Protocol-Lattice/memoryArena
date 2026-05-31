package memoryArena

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInjectExtract(t *testing.T) {
	ctx := context.Background()
	arena := NewMemoryArena[int](4)

	ctx = Inject(ctx, arena)

	got, err := Extract[int](ctx)
	if err != nil {
		t.Fatalf("Extract returned unexpected error: %v", err)
	}

	if got != arena {
		t.Fatal("expected extracted arena to match injected arena")
	}
}

func TestExtractReturnsErrNoArena(t *testing.T) {
	got, err := Extract[int](context.Background())
	if !errors.Is(err, ErrNoArena) {
		t.Fatalf("expected ErrNoArena, got %v", err)
	}

	if got != nil {
		t.Fatalf("expected nil arena, got %#v", got)
	}
}

func TestExtractReturnsErrNoArenaForNilArena(t *testing.T) {
	ctx := Inject[int](context.Background(), nil)

	got, err := Extract[int](ctx)
	if !errors.Is(err, ErrNoArena) {
		t.Fatalf("expected ErrNoArena, got %v", err)
	}

	if got != nil {
		t.Fatalf("expected nil arena, got %#v", got)
	}
}

func TestExtractReturnsErrNoArenaForDifferentGenericType(t *testing.T) {
	ctx := Inject(context.Background(), NewMemoryArena[int](4))

	got, err := Extract[string](ctx)
	if !errors.Is(err, ErrNoArena) {
		t.Fatalf("expected ErrNoArena, got %v", err)
	}

	if got != nil {
		t.Fatalf("expected nil arena, got %#v", got)
	}
}

func TestInjectRequestExtractRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/arena", nil)
	arena := NewMemoryArena[int](4)

	req = InjectRequest(req, arena)

	got, err := ExtractRequest[int](req)
	if err != nil {
		t.Fatalf("ExtractRequest returned unexpected error: %v", err)
	}

	if got != arena {
		t.Fatal("expected extracted request arena to match injected arena")
	}
}

func TestExtractRequestReturnsErrNoArena(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/arena", nil)

	got, err := ExtractRequest[int](req)
	if !errors.Is(err, ErrNoArena) {
		t.Fatalf("expected ErrNoArena, got %v", err)
	}

	if got != nil {
		t.Fatalf("expected nil arena, got %#v", got)
	}
}

func TestInjectRequestDoesNotMutateOriginalRequest(t *testing.T) {
	original := httptest.NewRequest(http.MethodPost, "/arena", nil)
	arena := NewMemoryArena[int](4)

	cloned := InjectRequest(original, arena)

	if cloned == original {
		t.Fatal("expected InjectRequest to return a cloned request")
	}

	if _, err := ExtractRequest[int](original); !errors.Is(err, ErrNoArena) {
		t.Fatalf("expected original request to remain unchanged, got %v", err)
	}

	got, err := ExtractRequest[int](cloned)
	if err != nil {
		t.Fatalf("ExtractRequest returned unexpected error from cloned request: %v", err)
	}

	if got != arena {
		t.Fatal("expected cloned request to contain injected arena")
	}
}

func TestInjectRequestPreservesRequestFields(t *testing.T) {
	original := httptest.NewRequest(http.MethodPatch, "/arena?debug=true", nil)
	arena := NewMemoryArena[int](4)

	cloned := InjectRequest(original, arena)

	if cloned.Method != original.Method {
		t.Fatalf("expected method %q, got %q", original.Method, cloned.Method)
	}

	if cloned.URL.String() != original.URL.String() {
		t.Fatalf("expected URL %q, got %q", original.URL.String(), cloned.URL.String())
	}

	if cloned.Header.Get("missing") != original.Header.Get("missing") {
		t.Fatal("expected request headers to be preserved")
	}
}
