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

// frNode is a minimal gRPC server for follower-read routing tests: it answers HAService.GetStatus
// with a configured role + applied index, serves Gql locally (followers serve reads), and counts
// Gql calls so a test can assert WHICH node served a read.
type frNode struct {
	pb.UnimplementedSessionServiceServer
	pb.UnimplementedQueryServiceServer
	pb.UnimplementedHAServiceServer
	id       string
	isLeader bool
	applied  uint64
	// followerRejects makes a non-leader reply LEADER_CHANGED to any Gql (the write-routing case).
	// Left false for follower-READ tests, where a follower serves the read locally.
	followerRejects bool
	gqlCalls        atomic.Int32
}

func (f *frNode) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	return &pb.LoginResponse{SessionId: 1, ServerVersion: "test"}, nil
}

func (f *frNode) Gql(ctx context.Context, req *pb.GqlRequest) (*pb.GqlResponse, error) {
	f.gqlCalls.Add(1)
	if f.followerRejects && !f.isLeader {
		return nil, status.Error(codes.FailedPrecondition, "LEADER_CHANGED leader=raft://leader:7000")
	}
	return &pb.GqlResponse{}, nil
}

func (f *frNode) GetStatus(ctx context.Context, req *pb.HAGetStatusRequest) (*pb.HAGetStatusResponse, error) {
	return &pb.HAGetStatusResponse{Status: &pb.HAStatus{
		Enabled:      true,
		NodeId:       f.id,
		IsLeader:     f.isLeader,
		AppliedIndex: f.applied,
	}}, nil
}

func startFRNode(t *testing.T, n *frNode) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	pb.RegisterSessionServiceServer(srv, n)
	pb.RegisterQueryServiceServer(srv, n)
	pb.RegisterHAServiceServer(srv, n)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func frClient(t *testing.T, hosts ...string) *Client {
	t.Helper()
	cfg := NewConfigBuilder().Hosts(hosts...).Username("root").Password("root").HealthCheckInterval(0).Build()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := client.Login(context.Background(), "root", "root"); err != nil {
		t.Fatalf("login: %v", err)
	}
	return client
}

// TestFollowerRead_RoutesToFreshFollower: a read with ReadPreferenceFollower + MaxStaleness routes
// to a follower within the bound, skipping a too-stale follower AND the leader.
func TestFollowerRead_RoutesToFreshFollower(t *testing.T) {
	ctx := context.Background()
	leader := &frNode{id: "leader", isLeader: true, applied: 100}
	fresh := &frNode{id: "fresh", applied: 98} // lag 2
	stale := &frNode{id: "stale", applied: 50} // lag 50
	client := frClient(t, startFRNode(t, leader), startFRNode(t, fresh), startFRNode(t, stale))

	if _, err := client.Gql(ctx, "MATCH (n) RETURN n", &QueryConfig{
		ReadPreference: ReadPreferenceFollower, MaxStaleness: 5,
	}); err != nil {
		t.Fatalf("follower read: %v", err)
	}
	if got := fresh.gqlCalls.Load(); got != 1 {
		t.Fatalf("fresh follower (lag 2 <= 5) should have served the read, got %d calls", got)
	}
	if got := stale.gqlCalls.Load(); got != 0 {
		t.Fatalf("stale follower (lag 50 > 5) must NOT serve, got %d calls", got)
	}
	if got := leader.gqlCalls.Load(); got != 0 {
		t.Fatalf("leader must not serve a satisfiable follower read, got %d calls", got)
	}
}

// TestFollowerRead_FallsBackToLeaderWhenAllStale: when no follower is within the bound, the read
// transparently routes to the leader.
func TestFollowerRead_FallsBackToLeaderWhenAllStale(t *testing.T) {
	ctx := context.Background()
	leader := &frNode{id: "leader", isLeader: true, applied: 100}
	f1 := &frNode{id: "f1", applied: 90} // lag 10
	f2 := &frNode{id: "f2", applied: 80} // lag 20
	client := frClient(t, startFRNode(t, leader), startFRNode(t, f1), startFRNode(t, f2))

	if _, err := client.Gql(ctx, "MATCH (n) RETURN n", &QueryConfig{
		ReadPreference: ReadPreferenceFollower, MaxStaleness: 5,
	}); err != nil {
		t.Fatalf("follower read with leader fallback: %v", err)
	}
	if got := leader.gqlCalls.Load(); got != 1 {
		t.Fatalf("leader should serve when all followers are too stale, got %d", got)
	}
	if f1.gqlCalls.Load() != 0 || f2.gqlCalls.Load() != 0 {
		t.Fatalf("no too-stale follower should serve: f1=%d f2=%d", f1.gqlCalls.Load(), f2.gqlCalls.Load())
	}
}

// TestFollowerRead_DefaultPrefRoutesToLeader: the default (nil config / ReadPreferenceLeader) keeps
// reads on the leader even when a fully-caught-up follower exists — read-your-writes preserved.
func TestFollowerRead_DefaultPrefRoutesToLeader(t *testing.T) {
	ctx := context.Background()
	leader := &frNode{id: "leader", isLeader: true, applied: 100}
	follower := &frNode{id: "follower", applied: 100} // lag 0
	client := frClient(t, startFRNode(t, leader), startFRNode(t, follower))

	if _, err := client.Gql(ctx, "MATCH (n) RETURN n", nil); err != nil {
		t.Fatalf("default read: %v", err)
	}
	if got := leader.gqlCalls.Load(); got != 1 {
		t.Fatalf("default read should go to the leader, got %d", got)
	}
	if got := follower.gqlCalls.Load(); got != 0 {
		t.Fatalf("default read must not hit a follower, got %d", got)
	}
}

// TestFollowerRead_AnyFollowerWhenUnbounded: MaxStaleness 0 accepts any healthy follower.
func TestFollowerRead_AnyFollowerWhenUnbounded(t *testing.T) {
	ctx := context.Background()
	leader := &frNode{id: "leader", isLeader: true, applied: 100}
	behind := &frNode{id: "behind", applied: 1} // lag 99, but bound is unset
	client := frClient(t, startFRNode(t, leader), startFRNode(t, behind))

	if _, err := client.Gql(ctx, "MATCH (n) RETURN n", &QueryConfig{
		ReadPreference: ReadPreferenceFollower, // MaxStaleness 0 = unbounded
	}); err != nil {
		t.Fatalf("unbounded follower read: %v", err)
	}
	if got := behind.gqlCalls.Load(); got != 1 {
		t.Fatalf("unbounded follower read should accept any healthy follower, got %d", got)
	}
	if got := leader.gqlCalls.Load(); got != 0 {
		t.Fatalf("leader must not serve when an (unbounded) follower is available, got %d", got)
	}
}

// TestProactiveRouting_JumpsToLeaderSkippingFollowers: on LEADER_CHANGED the driver asks
// HAService.GetStatus who the leader is and jumps straight there (item C7), instead of rotating
// host-by-host — so an intermediate follower is never tried.
func TestProactiveRouting_JumpsToLeaderSkippingFollowers(t *testing.T) {
	ctx := context.Background()
	f0 := &frNode{id: "f0", applied: 100, followerRejects: true}         // Hosts[0] = initial active host
	f1 := &frNode{id: "f1", applied: 100, followerRejects: true}         // Hosts[1] = rotation would try this next
	leader := &frNode{id: "leader", isLeader: true, applied: 100}        // Hosts[2] = the leader
	a0, a1, la := startFRNode(t, f0), startFRNode(t, f1), startFRNode(t, leader)

	cfg := NewConfigBuilder().Hosts(a0, a1, la).Username("root").Password("root").HealthCheckInterval(0).Build()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := client.Login(ctx, "root", "root"); err != nil {
		t.Fatalf("login: %v", err)
	}

	// A write hits f0 (active) → LEADER_CHANGED → proactive GetStatus learns the leader is Hosts[2]
	// and jumps straight there, never touching f1.
	if _, err := client.Gql(ctx, "INSERT (:N {x:1})", nil); err != nil {
		t.Fatalf("write should succeed via proactive leader jump: %v", err)
	}
	if leader.gqlCalls.Load() < 1 {
		t.Fatal("write should have landed on the leader")
	}
	if got := f1.gqlCalls.Load(); got != 0 {
		t.Fatalf("proactive routing should skip f1 and jump straight to the leader; f1 got %d calls", got)
	}
	client.mu.RLock()
	idx := client.activeHostIdx
	client.mu.RUnlock()
	if cfg.Hosts[idx%len(cfg.Hosts)] != la {
		t.Fatalf("active host should be pinned to the leader %s, got %s", la, cfg.Hosts[idx%len(cfg.Hosts)])
	}
}
