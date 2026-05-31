package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/Protocol-Lattice/memoryArena"
	arenaexamplepb "github.com/Protocol-Lattice/memoryArena/examples/grpc-server/proto/arenaexample"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const arenaSize uint = 256

type ParsedField struct {
	Key   string
	Value string
}

type parserServer struct {
	arenaexamplepb.UnimplementedParserServiceServer
}

// arenaUnaryInterceptor gives each unary RPC its own short-lived arena.
//
// This is the gRPC equivalent of HTTP request middleware:
//   - create arena;
//   - inject it into context;
//   - handler extracts it;
//   - reset arena when the RPC finishes.
func arenaUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	arena := memoryArena.NewMemoryArena[ParsedField](arenaSize)
	ctx = memoryArena.Inject(ctx, arena)

	defer arena.Reset()

	return handler(ctx, req)
}

func (s *parserServer) Parse(ctx context.Context, req *arenaexamplepb.ParseRequest) (*arenaexamplepb.ParseResponse, error) {
	arena, err := memoryArena.Extract[ParsedField](ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "arena not found in context: %v", err)
	}

	fields, ok := arena.TryAllocSlab(uint(len(req.GetFields())))
	if !ok {
		return nil, status.Errorf(codes.ResourceExhausted, "arena capacity exceeded: used=%d remaining=%d capacity=%d",
			arena.Used(),
			arena.Remaining(),
			arena.Cap(),
		)
	}

	for i, field := range req.GetFields() {
		fields[i] = ParsedField{
			Key:   field.GetKey(),
			Value: field.GetValue(),
		}
	}

	return &arenaexamplepb.ParseResponse{
		Fields:    toProtoFields(fields),
		Used:      uint32(arena.Used()),
		Remaining: uint32(arena.Remaining()),
	}, nil
}

func (s *parserServer) Sum(ctx context.Context, req *arenaexamplepb.SumRequest) (*arenaexamplepb.SumResponse, error) {
	arena, err := memoryArena.Extract[ParsedField](ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "arena not found in context: %v", err)
	}

	numbers := req.GetNumbers()
	fields, ok := arena.TryAllocSlab(uint(len(numbers)))
	if !ok {
		return nil, status.Errorf(codes.ResourceExhausted, "arena capacity exceeded: used=%d remaining=%d capacity=%d",
			arena.Used(),
			arena.Remaining(),
			arena.Cap(),
		)
	}

	var sum int64
	for i, n := range numbers {
		sum += n
		fields[i] = ParsedField{
			Key:   fmt.Sprintf("numbers[%d]", i),
			Value: strconv.FormatInt(n, 10),
		}
	}

	return &arenaexamplepb.SumResponse{
		Sum:           sum,
		ParsedNumbers: toProtoFields(fields),
		Used:          uint32(arena.Used()),
		Remaining:     uint32(arena.Remaining()),
	}, nil
}

func toProtoFields(fields []ParsedField) []*arenaexamplepb.Field {
	out := make([]*arenaexamplepb.Field, len(fields))
	for i, field := range fields {
		out[i] = &arenaexamplepb.Field{
			Key:   field.Key,
			Value: field.Value,
		}
	}
	return out
}

func main() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(arenaUnaryInterceptor),
	)

	arenaexamplepb.RegisterParserServiceServer(server, &parserServer{})

	// Enables grpcurl to discover services/methods without passing parser.proto.
	reflection.Register(server)

	log.Println("memoryArena gRPC example listening on localhost:50051")
	log.Println("try: grpcurl -plaintext list localhost:50051")
	log.Println("try: grpcurl -plaintext -d '{\"fields\":[{\"key\":\"name\",\"value\":\"kamil\"}]}' localhost:50051 arenaexample.v1.ParserService/Parse")
	log.Println("try: grpcurl -plaintext -d '{\"numbers\":[10,20,30]}' localhost:50051 arenaexample.v1.ParserService/Sum")

	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
