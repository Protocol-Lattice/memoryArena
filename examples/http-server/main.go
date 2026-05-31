package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Protocol-Lattice/memoryArena"
)

const arenaSize uint = 256

type ParsedField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ParseResult struct {
	Path      string        `json:"path"`
	Method    string        `json:"method"`
	Used      uint          `json:"used"`
	Remaining uint          `json:"remaining"`
	Fields    []ParsedField `json:"fields"`
}

// arenaMiddleware gives every request its own short-lived arena.
//
// The arena is injected into the cloned request context with InjectRequest.
// At the end of the request, the arena is reset in one operation.
func arenaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arena := memoryArena.NewMemoryArena[ParsedField](arenaSize)
		r = memoryArena.InjectRequest(r, arena)

		defer arena.Reset()

		next.ServeHTTP(w, r)
	})
}

func parseHandler(w http.ResponseWriter, r *http.Request) {
	arena, err := memoryArena.ExtractRequest[ParsedField](r)
	if err != nil {
		http.Error(w, "arena not found in request context", http.StatusInternalServerError)
		return
	}

	fields, err := parseRequestFields(r, arena)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := ParseResult{
		Path:      r.URL.Path,
		Method:    r.Method,
		Used:      arena.Used(),
		Remaining: arena.Remaining(),
		Fields:    fields,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// parseRequestFields simulates request-scoped parsing work.
//
// Each parsed key/value pair is copied into the request arena. The returned
// slice points directly into the arena buffer and must not be used after reset.
func parseRequestFields(r *http.Request, arena *memoryArena.MemoryArena[ParsedField]) ([]ParsedField, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	fieldCount := uint(len(r.Form))
	if fieldCount == 0 {
		return arena.AllocSlab(0), nil
	}

	fields, ok := arena.TryAllocSlab(fieldCount)
	if !ok {
		return nil, fmt.Errorf("request has too many fields: used=%d remaining=%d capacity=%d",
			arena.Used(),
			arena.Remaining(),
			arena.Cap(),
		)
	}

	i := 0
	for key, values := range r.Form {
		fields[i] = ParsedField{
			Key:   key,
			Value: strings.Join(values, ","),
		}
		i++
	}

	return fields, nil
}

func sumHandler(w http.ResponseWriter, r *http.Request) {
	arena, err := memoryArena.ExtractRequest[ParsedField](r)
	if err != nil {
		http.Error(w, "arena not found in request context", http.StatusInternalServerError)
		return
	}

	raw := r.URL.Query()["n"]
	if len(raw) == 0 {
		http.Error(w, "pass numbers as ?n=1&n=2&n=3", http.StatusBadRequest)
		return
	}

	fields, ok := arena.TryAllocSlab(uint(len(raw)))
	if !ok {
		http.Error(w, "arena capacity exceeded", http.StatusRequestEntityTooLarge)
		return
	}

	sum := 0
	for i, value := range raw {
		n, err := strconv.Atoi(value)
		if err != nil {
			http.Error(w, "invalid number: "+value, http.StatusBadRequest)
			return
		}

		sum += n
		fields[i] = ParsedField{
			Key:   fmt.Sprintf("n[%d]", i),
			Value: value,
		}
	}

	response := map[string]any{
		"sum":       sum,
		"used":      arena.Used(),
		"remaining": arena.Remaining(),
		"fields":    fields,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	_, err := memoryArena.ExtractRequest[ParsedField](r)
	if errors.Is(err, memoryArena.ErrNoArena) {
		http.Error(w, "arena missing", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /parse", parseHandler)
	mux.HandleFunc("POST /parse", parseHandler)
	mux.HandleFunc("GET /sum", sumHandler)
	mux.HandleFunc("GET /healthz", healthHandler)

	handler := arenaMiddleware(mux)

	addr := ":8080"
	log.Printf("memoryArena HTTP example listening on http://localhost%s", addr)
	log.Printf("try: curl 'http://localhost%s/parse?name=kamil&project=memoryArena'", addr)
	log.Printf("try: curl 'http://localhost%s/sum?n=10&n=20&n=30'", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
