# gRPC request-scoped arena example

This example shows how to use `memoryArena` as request-scoped scratch memory in a gRPC server.

A unary interceptor creates one `MemoryArena[ParsedField]` per RPC, injects it into the `context.Context`, lets the service handler allocate temporary parser state from it, and resets the arena after the handler returns.

## Files

```text
examples/grpc-server/
├── proto/arenaexample/parser.proto
└── server/main.go
```

## Generate protobuf code

From the repository root:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

protoc \
  --go_out=. \
  --go-grpc_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  examples/grpc-server/proto/arenaexample/parser.proto
```

This generates:

```text
examples/grpc-server/proto/arenaexample/parser.pb.go
examples/grpc-server/proto/arenaexample/parser_grpc.pb.go
```

## Run

```sh
go get google.golang.org/grpc
go run ./examples/grpc-server/server
```

## Try it with grpcurl

```sh
grpcurl -plaintext \
  -d '{"fields":[{"key":"name","value":"kamil"},{"key":"project","value":"memoryArena"}]}' \
  localhost:50051 \
  arenaexample.v1.ParserService/Parse
```

```sh
grpcurl -plaintext \
  -d '{"numbers":[10,20,30]}' \
  localhost:50051 \
  arenaexample.v1.ParserService/Sum
```

## Pattern

```go
func arenaUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	arena := memoryArena.NewMemoryArena[ParsedField](256)
	ctx = memoryArena.Inject(ctx, arena)

	defer arena.Reset()

	return handler(ctx, req)
}
```

Inside the service:

```go
arena, err := memoryArena.Extract[ParsedField](ctx)
if err != nil {
	return nil, status.Errorf(codes.Internal, "arena not found in context: %v", err)
}

fields, ok := arena.TryAllocSlab(uint(len(req.GetFields())))
if !ok {
	return nil, status.Error(codes.ResourceExhausted, "arena capacity exceeded")
}
```

The arena is valid only during the RPC. Do not store arena-backed pointers or slices in long-lived structs, goroutines, caches, or responses that outlive the handler.
