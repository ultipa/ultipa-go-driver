package gqldb

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeNode is a minimal gRPC server implementing just enough of SessionService and
// QueryService to drive the leader-routing path: a follower replies LEADER_CHANGED
// to a write; the leader accepts it.
type fakeNode struct {
	pb.UnimplementedSessionServiceServer
	pb.UnimplementedQueryServiceServer
	isLeader   bool
	gqlCalls   atomic.Int32
	loginCalls atomic.Int32
}

func (f *fakeNode) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	f.loginCalls.Add(1)
	return &pb.LoginResponse{SessionId: 1, ServerVersion: "test"}, nil
}

func (f *fakeNode) Gql(ctx context.Context, req *pb.GqlRequest) (*pb.GqlResponse, error) {
	f.gqlCalls.Add(1)
	if !f.isLeader {
		return nil, status.Error(codes.FailedPrecondition, "LEADER_CHANGED leader=raft://other:7000")
	}
	return &pb.GqlResponse{}, nil
}

func startFakeNode(t *testing.T, isLeader bool) (string, *fakeNode) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	node := &fakeNode{isLeader: isLeader}
	srv := grpc.NewServer()
	pb.RegisterSessionServiceServer(srv, node)
	pb.RegisterQueryServiceServer(srv, node)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String(), node
}

// A write that first lands on a follower is transparently retried on the leader
// after a LEADER_CHANGED reply, and the active host is pinned to the leader after.
func TestLeaderRouting_RotatesToLeaderOnLeaderChanged(t *testing.T) {
	ctx := context.Background()
	followerAddr, follower := startFakeNode(t, false)
	leaderAddr, leader := startFakeNode(t, true)

	cfg := NewConfigBuilder().
		Hosts(followerAddr, leaderAddr). // active host starts at index 0 == follower
		Username("root").
		Password("root").
		HealthCheckInterval(0).
		Build()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if _, err := client.Login(ctx, "root", "root"); err != nil {
		t.Fatalf("login: %v", err)
	}

	if _, err := client.Gql(ctx, "INSERT (:N {x: 1})", nil); err != nil {
		t.Fatalf("write should succeed via leader rotation, got: %v", err)
	}

	if follower.gqlCalls.Load() < 1 {
		t.Fatal("expected the write to first hit the follower")
	}
	if leader.gqlCalls.Load() < 1 {
		t.Fatal("expected the write to be retried on the leader")
	}
	client.mu.RLock()
	idx := client.activeHostIdx
	client.mu.RUnlock()
	if cfg.Hosts[idx%len(cfg.Hosts)] != leaderAddr {
		t.Fatalf("active host should be the leader %s, got index %d (%s)", leaderAddr, idx, cfg.Hosts[idx%len(cfg.Hosts)])
	}
}

// With a single endpoint, a LEADER_CHANGED cannot be routed elsewhere: the driver
// surfaces the error immediately instead of spinning (the write is tried once).
func TestLeaderRouting_SingleHostDoesNotSpin(t *testing.T) {
	ctx := context.Background()
	followerAddr, follower := startFakeNode(t, false)

	cfg := NewConfigBuilder().
		Hosts(followerAddr).
		Username("root").
		Password("root").
		HealthCheckInterval(0).
		Build()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if _, err := client.Login(ctx, "root", "root"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := client.Gql(ctx, "INSERT (:N {x: 1})", nil); err == nil {
		t.Fatal("single-host LEADER_CHANGED must surface as an error, not silently succeed")
	}
	if got := follower.gqlCalls.Load(); got != 1 {
		t.Fatalf("single-host write must be attempted exactly once (no spin), got %d", got)
	}
}

// TestIsLeaderChangedError: a leader change is decided by the error detail's
// reason; the message counts only for a server that sends no detail, and only
// when it is the bare marker. The word inside any other message never counts
// (FT-8): a fulltext error naming an index "leader_changed_idx" used to send
// the request to another server.
func TestIsLeaderChangedError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"reason LEADER_CHANGED", detailError(codes.FailedPrecondition, "not the leader", ReasonLeaderChanged, 0), true},
		{"reason LEADER_CHANGED, wrapped", NewError(0, "query failed", detailError(codes.FailedPrecondition, "LEADER_CHANGED", ReasonLeaderChanged, 0)), true},
		{"bare marker, no detail", status.Error(codes.FailedPrecondition, "LEADER_CHANGED"), true},
		{"bare marker with leader address, no detail", status.Error(codes.FailedPrecondition, "LEADER_CHANGED leader=raft://x:7000"), true},
		{"index named leader_changed_idx, no detail", status.Error(codes.FailedPrecondition,
			"[5020] fulltext index leader_changed_idx is loading into memory; retry shortly"), false},
		{"index named leader_changed_idx, with detail", detailError(codes.FailedPrecondition,
			"[5020] fulltext index leader_changed_idx is loading into memory; retry shortly", ReasonFulltextIndexLoading, 5020), false},
		{"lower-case word, no detail", status.Error(codes.FailedPrecondition, "leader_changed"), false},
		{"marker text but another reason", detailError(codes.FailedPrecondition, "LEADER_CHANGED", "FAILED_PRECONDITION", 0), false},
		{"another status", detailError(codes.Unavailable, "LEADER_CHANGED", ReasonLeaderChanged, 0), false},
		{"another precondition", status.Error(codes.FailedPrecondition, "some other precondition"), false},
		{"not a gRPC error", NewError(0, "LEADER_CHANGED", nil), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isLeaderChangedError(c.err); got != c.want {
				t.Fatalf("isLeaderChangedError = %v, want %v (%v)", got, c.want, c.err)
			}
		})
	}
}
