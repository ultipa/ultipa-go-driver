package gqldb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// detailError is a gRPC error as the server sends it: a status with one
// ErrorInfo carrying the engine code and the reason.
func detailError(c codes.Code, msg, reason string, code int) error {
	st, err := status.New(c, msg).WithDetails(&errdetails.ErrorInfo{
		Reason:   reason,
		Domain:   ErrorInfoDomain,
		Metadata: map[string]string{"code": fmt.Sprint(code)},
	})
	if err != nil {
		panic(err)
	}
	return st.Err()
}

// engineError is the server's error for an engine code, with the status and
// reason the server gives it.
func engineError(code int) error {
	switch code {
	case CodeFulltextIndexLoading:
		return detailError(codes.FailedPrecondition, "[5020] fulltext index idx_doc is loading into memory (42%); retry shortly", ReasonFulltextIndexLoading, code)
	case CodeWritesCommitted:
		return detailError(codes.FailedPrecondition, "[5024] the statement committed; delivering its results was cancelled", ReasonWritesCommitted, code)
	case CodeReadOnly:
		return detailError(codes.FailedPrecondition, "[4016] database is read-only", "READ_ONLY", code)
	case CodeLicenseReadOnly:
		return detailError(codes.FailedPrecondition, "[6020] license is in read-only mode", "LICENSE_READ_ONLY", code)
	case CodeWriteConflict:
		return detailError(codes.Aborted, "[3011] write conflict: re-read it and retry", "WRITE_CONFLICT", code)
	case CodeLicenseGraphLimit:
		return detailError(codes.FailedPrecondition, "[6012] database limit exceeded", "LICENSE_GRAPH_LIMIT", code)
	}
	panic(fmt.Sprintf("no test error for %d", code))
}

func TestErrorDetail_CodeAndReasonOnTheError(t *testing.T) {
	for code, reason := range map[int]string{
		CodeFulltextIndexLoading: ReasonFulltextIndexLoading,
		CodeWritesCommitted:      ReasonWritesCommitted,
		CodeReadOnly:             "READ_ONLY",
		CodeLicenseReadOnly:      "LICENSE_READ_ONLY",
		CodeWriteConflict:        "WRITE_CONFLICT",
		CodeLicenseGraphLimit:    "LICENSE_GRAPH_LIMIT",
	} {
		t.Run(reason, func(t *testing.T) {
			raw := engineError(code)
			ge := NewError(0, "query failed", raw)
			if ge.Code != code || ge.Reason != reason {
				t.Errorf("GqldbError code=%d reason=%q, want %d %q", ge.Code, ge.Reason, code, reason)
			}
			// Through any wrapping, and on the raw gRPC error.
			for name, err := range map[string]error{"raw": raw, "wrapped": fmt.Errorf("outer: %w", ge)} {
				if got := EngineCode(err); got != code {
					t.Errorf("%s: EngineCode = %d, want %d", name, got, code)
				}
				if got := ErrorReason(err); got != reason {
					t.Errorf("%s: ErrorReason = %q, want %q", name, got, reason)
				}
			}
		})
	}
}

func TestErrorDetail_NeverFromTheText(t *testing.T) {
	cases := map[string]error{
		"no detail, code in the text": status.Error(codes.FailedPrecondition, "[5020] fulltext index idx_doc is loading; retry shortly"),
		"another domain": func() error {
			st, _ := status.New(codes.FailedPrecondition, "[5020] x").WithDetails(&errdetails.ErrorInfo{
				Reason: ReasonFulltextIndexLoading, Domain: "example.com", Metadata: map[string]string{"code": "5020"}})
			return st.Err()
		}(),
		"not a gRPC error": errors.New("[5020] fulltext index idx_doc is loading"),
		"nil":              nil,
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			if code, reason := EngineCode(err), ErrorReason(err); code != 0 || reason != "" {
				t.Errorf("code=%d reason=%q, want 0 and empty", code, reason)
			}
			if ge := NewError(0, "query failed", err); ge.Code != 0 || ge.Reason != "" {
				t.Errorf("GqldbError code=%d reason=%q, want 0 and empty", ge.Code, ge.Reason)
			}
		})
	}
	// A code the caller gives wins over the detail.
	if ge := NewError(42, "x", engineError(CodeFulltextIndexLoading)); ge.Code != 42 || ge.Reason != "" {
		t.Errorf("explicit code: code=%d reason=%q, want 42 and empty", ge.Code, ge.Reason)
	}
}

func TestRetryBackoff(t *testing.T) {
	base := 100 * time.Millisecond
	want := []time.Duration{100, 200, 400, 800, 1600, 2000, 2000}
	for i, w := range want {
		if got := retryBackoff(base, i); got != w*time.Millisecond {
			t.Errorf("attempt %d: %v, want %v", i, got, w*time.Millisecond)
		}
	}
	if got := retryBackoff(0, 3); got != 0 {
		t.Errorf("zero base: %v, want 0", got)
	}
	if got := retryBackoff(5*time.Second, 0); got != maxRetryBackoff {
		t.Errorf("a base above the cap: %v, want %v", got, maxRetryBackoff)
	}
}

// The retry rule on its own: which requests and which codes, and when it stops.
func TestRetryRead_Rules(t *testing.T) {
	readOnly := &QueryConfig{ReadOnly: true}
	noSleep := func(context.Context, time.Duration) error { return nil }
	always := func() bool { return true }
	run := func(ctx context.Context, cfg *QueryConfig, retries int, errs ...error) (int, error) {
		calls := 0
		err := retryRead(ctx, cfg, retries, 100*time.Millisecond, noSleep, always, func() error {
			calls++
			if calls <= len(errs) {
				return errs[calls-1]
			}
			return errs[len(errs)-1]
		})
		return calls, err
	}
	ctx := context.Background()
	loading := engineError(CodeFulltextIndexLoading)

	if calls, err := run(ctx, readOnly, 3, loading, loading, nil); calls != 3 || err != nil {
		t.Errorf("5020 then success: %d calls, %v; want 3 calls and success", calls, err)
	}
	if calls, err := run(ctx, readOnly, 3, loading); calls != 4 || EngineCode(err) != CodeFulltextIndexLoading {
		t.Errorf("5020 every time: %d calls, %v; want 4 calls ending in 5020", calls, err)
	}
	for name, cfg := range map[string]*QueryConfig{
		"no config":        nil,
		"not read-only":    {},
		"in a transaction": {ReadOnly: true, TransactionID: 7},
	} {
		if calls, _ := run(ctx, cfg, 3, loading); calls != 1 {
			t.Errorf("5020, %s: %d calls, want 1", name, calls)
		}
	}
	if calls, _ := run(ctx, readOnly, 0, loading); calls != 1 {
		t.Errorf("5020 with retries off: %d calls, want 1", calls)
	}
	// A write conflict is returned, never sent again, in auto-commit as in a
	// transaction (write_conflict_test.go).
	for _, code := range []int{CodeWritesCommitted, CodeReadOnly, CodeLicenseReadOnly, CodeLicenseGraphLimit, CodeWriteConflict} {
		for name, cfg := range map[string]*QueryConfig{"read-only": readOnly, "auto-commit write": {}} {
			if calls, err := run(ctx, cfg, 3, engineError(code)); calls != 1 || EngineCode(err) != code {
				t.Errorf("%d, %s: %d calls (%v), want 1", code, name, calls, err)
			}
		}
	}
	for name, cfg := range map[string]*QueryConfig{"write in a transaction": {TransactionID: 7}, "read-only in a transaction": {ReadOnly: true, TransactionID: 7}} {
		if calls, err := run(ctx, cfg, 3, engineError(CodeWriteConflict)); calls != 1 || EngineCode(err) != CodeWriteConflict {
			t.Errorf("3011, %s: %d calls (%v), want 1", name, calls, err)
		}
	}
	textOnly := status.Error(codes.FailedPrecondition, "[5020] fulltext index idx_doc is loading; retry shortly")
	if calls, _ := run(ctx, readOnly, 3, textOnly); calls != 1 {
		t.Errorf("5020 in the text only: %d calls, want 1", calls)
	}

	// It never waits past the context's deadline.
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	calls := 0
	_ = retryRead(short, readOnly, 3, 100*time.Millisecond, sleepContext, always, func() error { calls++; return loading })
	if calls != 1 {
		t.Errorf("deadline closer than the wait: %d calls, want 1", calls)
	}
	// It asks before each retry.
	calls = 0
	_ = retryRead(ctx, readOnly, 3, 0, noSleep, func() bool { return false }, func() error { calls++; return loading })
	if calls != 1 {
		t.Errorf("canRetry false: %d calls, want 1", calls)
	}
}

// scriptNode is a gRPC server whose Gql and GqlStream answer with the errors
// in errs, one per call (nil: success), repeating the last one.
type scriptNode struct {
	pb.UnimplementedSessionServiceServer
	pb.UnimplementedQueryServiceServer
	mu          sync.Mutex
	errs        []error
	rowsBefore  int // rows a stream sends before its error
	calls       atomic.Int32
	streamCalls atomic.Int32
}

func (n *scriptNode) next(call int32) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.errs) == 0 {
		return nil
	}
	if int(call) <= len(n.errs) {
		return n.errs[call-1]
	}
	return n.errs[len(n.errs)-1]
}

func (n *scriptNode) Login(context.Context, *pb.LoginRequest) (*pb.LoginResponse, error) {
	return &pb.LoginResponse{SessionId: 1, ServerVersion: "test"}, nil
}

func (n *scriptNode) Gql(context.Context, *pb.GqlRequest) (*pb.GqlResponse, error) {
	if err := n.next(n.calls.Add(1)); err != nil {
		return nil, err
	}
	return &pb.GqlResponse{}, nil
}

func (n *scriptNode) GqlStream(_ *pb.GqlRequest, stream pb.QueryService_GqlStreamServer) error {
	err := n.next(n.streamCalls.Add(1))
	for i := 0; i < n.rowsBefore && err != nil; i++ {
		if serr := stream.Send(&pb.GqlResponse{}); serr != nil {
			return serr
		}
	}
	return err
}

// scriptClient starts a scriptNode answering with errs and a client logged in
// to it whose waits between retries are recorded rather than slept.
func scriptClient(t *testing.T, retries int, errs ...error) (*Client, *scriptNode, *[]time.Duration) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	node := &scriptNode{errs: errs}
	srv := grpc.NewServer()
	pb.RegisterSessionServiceServer(srv, node)
	pb.RegisterQueryServiceServer(srv, node)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	cfg := NewConfigBuilder().Hosts(lis.Addr().String()).Username("root").Password("root").
		HealthCheckInterval(0).RetryCount(retries).RetryDelay(100 * time.Millisecond).Build()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	var waits []time.Duration
	var wmu sync.Mutex
	client.retrySleep = func(_ context.Context, d time.Duration) error {
		wmu.Lock()
		waits = append(waits, d)
		wmu.Unlock()
		return nil
	}
	if _, err := client.Login(context.Background(), "root", "root"); err != nil {
		t.Fatal(err)
	}
	return client, node, &waits
}

// Over a real gRPC connection: the detail arrives, a read-only Gql is retried
// on 5020 with the doubling wait, and nothing else is.
func TestGql_RetriesIndexLoadingForReadsOnly(t *testing.T) {
	ctx := context.Background()
	loading := engineError(CodeFulltextIndexLoading)
	readOnly := &QueryConfig{ReadOnly: true}

	t.Run("5020 read-only, then success", func(t *testing.T) {
		client, node, waits := scriptClient(t, 3, loading, loading, nil)
		if _, err := client.Gql(ctx, "MATCH (n WHERE ~name CONTAINS 'a') RETURN n", readOnly); err != nil {
			t.Fatalf("want success after two retries, got %v", err)
		}
		if got := node.calls.Load(); got != 3 {
			t.Errorf("%d calls, want 3", got)
		}
		if fmt.Sprint(*waits) != "[100ms 200ms]" {
			t.Errorf("waits %v, want [100ms 200ms]", *waits)
		}
	})
	t.Run("5020 read-only, every time", func(t *testing.T) {
		client, node, waits := scriptClient(t, 3, loading)
		_, err := client.Gql(ctx, "MATCH (n) RETURN n", readOnly)
		var ge *GqldbError
		if !errors.As(err, &ge) || ge.Code != CodeFulltextIndexLoading || ge.Reason != ReasonFulltextIndexLoading {
			t.Fatalf("want a GqldbError with 5020 FULLTEXT_INDEX_LOADING, got %#v", err)
		}
		if got := node.calls.Load(); got != 4 {
			t.Errorf("%d calls, want 4 (1 + RetryCount 3)", got)
		}
		if fmt.Sprint(*waits) != "[100ms 200ms 400ms]" {
			t.Errorf("waits %v, want [100ms 200ms 400ms]", *waits)
		}
	})
	t.Run("5020 without read-only", func(t *testing.T) {
		client, node, _ := scriptClient(t, 3, loading)
		_, err := client.Gql(ctx, "MATCH (n) RETURN n", nil)
		if EngineCode(err) != CodeFulltextIndexLoading || node.calls.Load() != 1 {
			t.Errorf("%d calls (%v), want 1 ending in 5020", node.calls.Load(), err)
		}
	})
	t.Run("retries off", func(t *testing.T) {
		client, node, _ := scriptClient(t, 0, loading)
		_, _ = client.Gql(ctx, "MATCH (n) RETURN n", readOnly)
		if got := node.calls.Load(); got != 1 {
			t.Errorf("%d calls, want 1", got)
		}
	})
	for _, code := range []int{CodeWritesCommitted, CodeReadOnly, CodeLicenseReadOnly, CodeLicenseGraphLimit, CodeWriteConflict} {
		for name, cfg := range map[string]*QueryConfig{"read-only": readOnly, "auto-commit write": nil} {
			t.Run(fmt.Sprintf("%d %s is never sent again", code, name), func(t *testing.T) {
				client, node, _ := scriptClient(t, 3, engineError(code))
				_, err := client.Gql(ctx, "INSERT (:N {x: 1})", cfg)
				if EngineCode(err) != code {
					t.Errorf("error %v, want engine code %d", err, code)
				}
				if got := node.calls.Load(); got != 1 {
					t.Errorf("%d calls, want 1", got)
				}
			})
		}
	}
	t.Run("3011 in a transaction is never sent again", func(t *testing.T) {
		client, node, _ := scriptClient(t, 3, engineError(CodeWriteConflict))
		_, err := client.Gql(ctx, "INSERT (:N {x: 1})", &QueryConfig{TransactionID: 7})
		if EngineCode(err) != CodeWriteConflict || node.calls.Load() != 1 {
			t.Errorf("%d calls (%v), want 1 ending in 3011", node.calls.Load(), err)
		}
	})
}

func TestGqlStream_RetriesIndexLoadingOnlyBeforeRows(t *testing.T) {
	ctx := context.Background()
	loading := engineError(CodeFulltextIndexLoading)
	readOnly := &QueryConfig{ReadOnly: true}
	discard := func(*Response) error { return nil }

	t.Run("before any row", func(t *testing.T) {
		client, node, _ := scriptClient(t, 3, loading, nil)
		if err := client.GqlStream(ctx, "MATCH (n) RETURN n", readOnly, discard); err != nil {
			t.Fatalf("want success after one retry, got %v", err)
		}
		if got := node.streamCalls.Load(); got != 2 {
			t.Errorf("%d calls, want 2", got)
		}
	})
	t.Run("after a row", func(t *testing.T) {
		client, node, _ := scriptClient(t, 3, loading, nil)
		node.rowsBefore = 1
		err := client.GqlStream(ctx, "MATCH (n) RETURN n", readOnly, discard)
		if EngineCode(err) != CodeFulltextIndexLoading {
			t.Fatalf("want the 5020 error, got %v", err)
		}
		if got := node.streamCalls.Load(); got != 1 {
			t.Errorf("%d calls, want 1: rows had reached the callback", got)
		}
	})
	t.Run("5024 is never sent again", func(t *testing.T) {
		client, node, _ := scriptClient(t, 3, engineError(CodeWritesCommitted))
		_ = client.GqlStream(ctx, "INSERT (:N) RETURN 1", readOnly, discard)
		if got := node.streamCalls.Load(); got != 1 {
			t.Errorf("%d calls, want 1", got)
		}
	})
}

// FT-8 end to end: a follower's FAILED_PRECONDITION whose text names an index
// "leader_changed_idx" is an ordinary error, not a leader change; the reason
// LEADER_CHANGED is one, whatever the message says.
func TestLeaderRouting_DecidedByReason(t *testing.T) {
	ctx := context.Background()
	newPair := func(t *testing.T, followerErr error) (*Client, *scriptNode, *scriptNode) {
		t.Helper()
		start := func(errs ...error) (string, *scriptNode) {
			lis, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			node := &scriptNode{errs: errs}
			srv := grpc.NewServer()
			pb.RegisterSessionServiceServer(srv, node)
			pb.RegisterQueryServiceServer(srv, node)
			go func() { _ = srv.Serve(lis) }()
			t.Cleanup(srv.Stop)
			return lis.Addr().String(), node
		}
		fAddr, follower := start(followerErr)
		lAddr, leader := start()
		cfg := NewConfigBuilder().Hosts(fAddr, lAddr).Username("root").Password("root").HealthCheckInterval(0).Build()
		client, err := NewClient(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		if _, err := client.Login(ctx, "root", "root"); err != nil {
			t.Fatal(err)
		}
		return client, follower, leader
	}

	t.Run("reason LEADER_CHANGED rotates", func(t *testing.T) {
		client, follower, leader := newPair(t, detailError(codes.FailedPrecondition, "this server is not the leader", ReasonLeaderChanged, 0))
		if _, err := client.Gql(ctx, "INSERT (:N {x: 1})", nil); err != nil {
			t.Fatalf("want the write to reach the leader, got %v", err)
		}
		if follower.calls.Load() < 1 || leader.calls.Load() != 1 {
			t.Errorf("follower %d, leader %d calls; want the write retried on the leader once", follower.calls.Load(), leader.calls.Load())
		}
	})
	t.Run("an index named leader_changed_idx does not", func(t *testing.T) {
		indexErr := detailError(codes.FailedPrecondition,
			"[5020] fulltext index leader_changed_idx is loading into memory; retry shortly", ReasonFulltextIndexLoading, CodeFulltextIndexLoading)
		client, follower, leader := newPair(t, indexErr)
		_, err := client.Gql(ctx, "INSERT (:N {x: 1})", nil)
		if EngineCode(err) != CodeFulltextIndexLoading {
			t.Fatalf("want the 5020 error, got %v", err)
		}
		if follower.calls.Load() != 1 || leader.calls.Load() != 0 {
			t.Errorf("follower %d, leader %d calls; want 1 and 0", follower.calls.Load(), leader.calls.Load())
		}
	})
}
