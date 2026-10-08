package gqldb

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Every call the driver sends carries the header x-gqldb-client with its
// language and version. The server leaves error messages as they were built
// for a client that sends it; without it, the server puts an invisible U+2060
// into the phrases older drivers resend on (review round 3, item 4).

// headerServer records the x-gqldb-client values of every call it receives.
type headerServer struct {
	pb.UnimplementedSessionServiceServer
	pb.UnimplementedQueryServiceServer
	mu   sync.Mutex
	seen map[string][]string // method -> header values
}

func (h *headerServer) record(ctx context.Context, method string) {
	md, _ := metadata.FromIncomingContext(ctx)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seen[method] = append(h.seen[method], md.Get("x-gqldb-client")...)
	if len(md.Get("x-gqldb-client")) == 0 {
		h.seen[method] = append(h.seen[method], "")
	}
}

func (h *headerServer) Login(context.Context, *pb.LoginRequest) (*pb.LoginResponse, error) {
	return &pb.LoginResponse{SessionId: 1, ServerVersion: "test"}, nil
}

func (h *headerServer) Gql(context.Context, *pb.GqlRequest) (*pb.GqlResponse, error) {
	return &pb.GqlResponse{}, nil
}

func (h *headerServer) GqlStream(_ *pb.GqlRequest, stream pb.QueryService_GqlStreamServer) error {
	return stream.Send(&pb.GqlResponse{})
}

func TestClientHeader_OnEveryCall(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h := &headerServer{seen: map[string][]string{}}
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			h.record(ctx, info.FullMethod)
			return handler(ctx, req)
		}),
		grpc.StreamInterceptor(func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			h.record(ss.Context(), info.FullMethod)
			return handler(srv, ss)
		}),
	)
	pb.RegisterSessionServiceServer(srv, h)
	pb.RegisterQueryServiceServer(srv, h)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	client, err := NewClient(NewConfigBuilder().Hosts(lis.Addr().String()).HealthCheckInterval(0).Build())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := client.Login(ctx, "u", "p"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := client.Gql(ctx, "RETURN 1", nil); err != nil {
		t.Fatalf("gql: %v", err)
	}
	if err := client.GqlStream(ctx, "RETURN 1", nil, func(*Response) error { return nil }); err != nil {
		t.Fatalf("stream: %v", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for _, method := range []string{"/gqldb.SessionService/Login", "/gqldb.QueryService/Gql", "/gqldb.QueryService/GqlStream"} {
		values := h.seen[method]
		if len(values) == 0 {
			t.Errorf("%s was not called", method)
			continue
		}
		for _, v := range values {
			if !strings.HasPrefix(v, "go/") || len(v) <= len("go/") {
				t.Errorf("%s carried x-gqldb-client %q, want go/<version>", method, v)
			}
		}
	}
	if ClientHeaderValue() != "go/"+driverVersion() {
		t.Errorf("ClientHeaderValue %q", ClientHeaderValue())
	}
}
