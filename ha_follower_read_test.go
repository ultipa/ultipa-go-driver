package gqldb

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
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
	// leaderErr, when set, is the leader's answer to Gql: every time, or only
	// the first time when leaderErrOnce is set.
	leaderErr     error
	leaderErrOnce bool
	leaderErrSent atomic.Bool
	// followerErr, when set, is a follower's answer to every Gql.
	followerErr error
}

func (f *frNode) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	return &pb.LoginResponse{SessionId: 1, ServerVersion: "test"}, nil
}

func (f *frNode) Gql(ctx context.Context, req *pb.GqlRequest) (*pb.GqlResponse, error) {
	f.gqlCalls.Add(1)
	if f.followerRejects && !f.isLeader {
		return nil, status.Error(codes.FailedPrecondition, "LEADER_CHANGED leader=raft://leader:7000")
	}
	if !f.isLeader && f.followerErr != nil {
		return nil, f.followerErr
	}
	if f.isLeader && f.leaderErr != nil && !(f.leaderErrOnce && f.leaderErrSent.Swap(true)) {
		return nil, f.leaderErr
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
	return startNodeWith(t, func(srv *grpc.Server) {
		pb.RegisterSessionServiceServer(srv, n)
		pb.RegisterQueryServiceServer(srv, n)
		pb.RegisterHAServiceServer(srv, n)
	})
}

// startNodeWith starts a gRPC server on a free local port with the services
// register adds, and returns its address.
func startNodeWith(t *testing.T, register func(*grpc.Server)) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	register(srv)
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

// Review round 2 of the error-code batch, item 1: a call marked
// ReadPreferenceFollower that failed on the follower went on to the leader
// whatever the error, so a write the follower had run (a stale HA status right
// after a failover: the "follower" is now the leader) reached the servers
// twice. It goes on to the leader only under the rule every other path
// follows: the call is read-only outside a transaction, or the follower
// refused it before running it (executed=false, or LEADER_CHANGED); never when
// the answer says part of it is stored. Otherwise the follower's error is the
// answer.
func TestFollowerRead_FallbackOnlyWhenSafe(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name      string
		followErr error
		config    *QueryConfig
		toLeader  bool // the call goes on to the leader
	}{
		{"write, 5024 stored", engineError(CodeWritesCommitted),
			&QueryConfig{ReadPreference: ReadPreferenceFollower}, false},
		{"write, connection dropped", statusWith(codes.Unavailable, "connection reset by peer", ""),
			&QueryConfig{ReadPreference: ReadPreferenceFollower}, false},
		{"write, old server, no detail", statusWith(codes.Internal, "something failed after the write", ""),
			&QueryConfig{ReadPreference: ReadPreferenceFollower}, false},
		{"write, detail without executed", statusWith(codes.Internal, "failed", "INTERNAL", "code", "0"),
			&QueryConfig{ReadPreference: ReadPreferenceFollower}, false},
		{"write, executed=false from another domain", foreignRefusal(codes.Unauthenticated),
			&QueryConfig{ReadPreference: ReadPreferenceFollower}, false},
		{"read-only flag inside a transaction", statusWith(codes.Unavailable, "connection reset by peer", ""),
			&QueryConfig{ReadPreference: ReadPreferenceFollower, ReadOnly: true, TransactionID: 9}, false},
		{"write, refused before it ran", expiredBeforeRun,
			&QueryConfig{ReadPreference: ReadPreferenceFollower}, true},
		{"write, LEADER_CHANGED", leaderChangedWithDetail(),
			&QueryConfig{ReadPreference: ReadPreferenceFollower}, true},
		{"write, old HA marker", statusWith(codes.FailedPrecondition, "LEADER_CHANGED leader=raft://leader:7000", ""),
			&QueryConfig{ReadPreference: ReadPreferenceFollower}, true},
		{"read-only, connection dropped", statusWith(codes.Unavailable, "connection reset by peer", ""),
			&QueryConfig{ReadPreference: ReadPreferenceFollower, ReadOnly: true}, true},
		{"read-only, 5024", engineError(CodeWritesCommitted),
			&QueryConfig{ReadPreference: ReadPreferenceFollower, ReadOnly: true}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			leader := &frNode{id: "leader", isLeader: true, applied: 100}
			follower := &frNode{id: "follower", applied: 100, followerErr: c.followErr}
			client := frClient(t, startFRNode(t, leader), startFRNode(t, follower))
			_, err := client.Gql(ctx, "INSERT (:Audit {note:'one write'})", c.config)
			if f := follower.gqlCalls.Load(); f != 1 {
				t.Fatalf("the follower received %d calls, want 1", f)
			}
			if c.toLeader {
				if n := leader.gqlCalls.Load(); n != 1 || err != nil {
					t.Fatalf("leader calls %d (want 1), err %v (want nil)", n, err)
				}
				return
			}
			if n := leader.gqlCalls.Load(); n != 0 {
				t.Fatalf("the call reached the leader too: %d calls after the follower answered %v", n, c.followErr)
			}
			if err == nil {
				t.Fatal("the follower's error was lost")
			}
			if st, ok := grpcStatusOf(err); !ok || st.Code() != status.Code(c.followErr) {
				t.Errorf("error %v, want the follower's status %v", err, status.Code(c.followErr))
			}
		})
	}
}

// leaderChangedWithDetail is the answer of a follower that has the detail.
func leaderChangedWithDetail() error {
	return statusWith(codes.FailedPrecondition, "LEADER_CHANGED leader=raft://leader:7000", ReasonLeaderChanged,
		"code", "0", "executed", "false")
}

// foreignRefusal is executed=false in an ErrorInfo of another domain, which
// the driver must not trust.
func foreignRefusal(c codes.Code) error {
	st, err := status.New(c, "refused").WithDetails(&errdetails.ErrorInfo{Reason: "SESSION_EXPIRED", Domain: "proxy.example.com",
		Metadata: map[string]string{"code": "0", "executed": "false"}})
	if err != nil {
		panic(err)
	}
	return st.Err()
}
