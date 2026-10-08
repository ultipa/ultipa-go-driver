package gqldb

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// The drivers' follow-up to QA round 33 (oct 5): End and Abort wait for a
// final state, Abort signs in again, a 5024 commit is "stored in part", the
// commit's warnings reach the caller, and a transaction whose sign-in expired
// is reported as such and never begun again by the driver.

// bulkFake answers SessionService, BulkImportService, TransactionService and
// QueryService the way the server does, from scripts the test sets.
type bulkFake struct {
	pb.UnimplementedSessionServiceServer
	pb.UnimplementedBulkImportServiceServer
	pb.UnimplementedTransactionServiceServer
	pb.UnimplementedQueryServiceServer

	mu         sync.Mutex
	logins     int
	loginFn    func(n int) error // a non-nil error refuses that sign-in
	statusFn   func(n int) (*pb.GetBulkImportStatusResponse, error)
	endFn      func(ctx context.Context, n int) (*pb.EndBulkImportResponse, error)
	abortFn    func(ctx context.Context, n int) (*pb.AbortBulkImportResponse, error)
	commitFn   func(n int) (*pb.CommitResponse, error)
	gqlFn      func(n int, req *pb.GqlRequest) (*pb.GqlResponse, error)
	statusN    int
	endN       int
	abortN     int
	commitN    int
	rollbackN  int
	beginN     int
	gqlN       int
	endCaps    []string // the capabilities header of each End/Abort
	endSession []string // the session-id header of each End/Abort
	gqlLog     []string // "query|x-ultipa-session-id" of each Gql
}

func capsOf(ctx context.Context) (string, string) {
	md, _ := metadata.FromIncomingContext(ctx)
	return strings.Join(md.Get("x-gqldb-capabilities"), ","), strings.Join(md.Get("session-id"), ",")
}

func (f *bulkFake) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	f.mu.Lock()
	f.logins++
	n, fn := f.logins, f.loginFn
	f.mu.Unlock()
	if fn != nil {
		if err := fn(n); err != nil {
			return nil, err
		}
	}
	return &pb.LoginResponse{SessionId: uint64(100 + n), ServerVersion: "test"}, nil
}

func (f *bulkFake) GetBulkImportStatus(ctx context.Context, req *pb.GetBulkImportStatusRequest) (*pb.GetBulkImportStatusResponse, error) {
	f.mu.Lock()
	f.statusN++
	n, fn := f.statusN, f.statusFn
	f.mu.Unlock()
	if fn == nil {
		return &pb.GetBulkImportStatusResponse{IsActive: true, State: pb.BulkImportState_BULK_IMPORT_STATE_ACTIVE}, nil
	}
	return fn(n)
}

func (f *bulkFake) EndBulkImport(ctx context.Context, req *pb.EndBulkImportRequest) (*pb.EndBulkImportResponse, error) {
	c, sid := capsOf(ctx)
	f.mu.Lock()
	f.endN++
	n, fn := f.endN, f.endFn
	f.endCaps = append(f.endCaps, c)
	f.endSession = append(f.endSession, sid)
	f.mu.Unlock()
	return fn(ctx, n)
}

func (f *bulkFake) AbortBulkImport(ctx context.Context, req *pb.AbortBulkImportRequest) (*pb.AbortBulkImportResponse, error) {
	c, sid := capsOf(ctx)
	f.mu.Lock()
	f.abortN++
	n, fn := f.abortN, f.abortFn
	f.endCaps = append(f.endCaps, c)
	f.endSession = append(f.endSession, sid)
	f.mu.Unlock()
	return fn(ctx, n)
}

func (f *bulkFake) Begin(ctx context.Context, req *pb.BeginRequest) (*pb.BeginResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.beginN++
	return &pb.BeginResponse{TransactionId: uint64(7 + f.beginN)}, nil
}

func (f *bulkFake) Commit(ctx context.Context, req *pb.CommitRequest) (*pb.CommitResponse, error) {
	f.mu.Lock()
	f.commitN++
	n, fn := f.commitN, f.commitFn
	f.mu.Unlock()
	if fn == nil {
		return &pb.CommitResponse{Success: true}, nil
	}
	return fn(n)
}

func (f *bulkFake) Rollback(ctx context.Context, req *pb.RollbackRequest) (*pb.RollbackResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rollbackN++
	return &pb.RollbackResponse{Success: true}, nil
}

func (f *bulkFake) Gql(ctx context.Context, req *pb.GqlRequest) (*pb.GqlResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	f.mu.Lock()
	f.gqlLog = append(f.gqlLog, req.Gql+"|"+strings.Join(md.Get("x-ultipa-session-id"), ","))
	if req.Gql == "ROLLBACK" {
		f.mu.Unlock()
		return &pb.GqlResponse{}, nil
	}
	f.gqlN++
	n, fn := f.gqlN, f.gqlFn
	f.mu.Unlock()
	if fn == nil {
		return &pb.GqlResponse{}, nil
	}
	return fn(n, req)
}

func (f *bulkFake) counts() (status, end, abort, commit, rollback, begin, gql, logins int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statusN, f.endN, f.abortN, f.commitN, f.rollbackN, f.beginN, f.gqlN, f.logins
}

func startBulkFake(t *testing.T, f *bulkFake) *Client {
	t.Helper()
	return startBulkFakeWith(t, f, nil)
}

// startBulkFakeWith is startBulkFake with a hook that changes the config.
func startBulkFakeWith(t *testing.T, f *bulkFake, configure func(*Config)) *Client {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterSessionServiceServer(srv, f)
	pb.RegisterBulkImportServiceServer(srv, f)
	pb.RegisterTransactionServiceServer(srv, f)
	pb.RegisterQueryServiceServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	cfg := NewConfigBuilder().Hosts(lis.Addr().String()).HealthCheckInterval(0).Build()
	if configure != nil {
		configure(cfg)
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := client.Login(context.Background(), "admin", "pw"); err != nil {
		t.Fatal(err)
	}
	return client
}

// earlyAnswerFake answers End the way the server answers a client with the
// capability: in progress for the first `running` calls, then ENDED.
func earlyAnswerEnd(running int) func(ctx context.Context, n int) (*pb.EndBulkImportResponse, error) {
	return func(ctx context.Context, n int) (*pb.EndBulkImportResponse, error) {
		if n <= running {
			return &pb.EndBulkImportResponse{InProgress: true, State: pb.BulkImportState_BULK_IMPORT_STATE_ENDING,
				Message: "End of bulk import session s1 is still running (ending): it has not failed"}, nil
		}
		return &pb.EndBulkImportResponse{Success: true, TotalRecords: 42, State: pb.BulkImportState_BULK_IMPORT_STATE_ENDED,
			Message: "bulk import completed successfully"}, nil
	}
}

// waitForDeadline answers nothing: it holds the call until its deadline, as a
// server without the early answer does while End runs.
func waitForDeadline(ctx context.Context) error {
	<-ctx.Done()
	return status.FromContextError(ctx.Err()).Err()
}

// An End the server answers "still running" is sent again, with the capability
// header, until the state is final; each report reaches OnProgress.
func TestEndBulkImport_ResendsWhileInProgress(t *testing.T) {
	f := &bulkFake{endFn: earlyAnswerEnd(2)}
	client := startBulkFake(t, f)
	var reports []BulkImportProgress
	res, err := client.EndBulkImport(context.Background(), "s1", BulkImportWaitOptions{
		ProgressInterval: 2 * time.Second,
		OnProgress:       func(p BulkImportProgress) { reports = append(reports, p) },
	})
	if err != nil {
		t.Fatalf("EndBulkImport: %v", err)
	}
	if !res.Success || res.State != BulkImportStateEnded || res.TotalRecords != 42 || res.Attempts != 3 {
		t.Errorf("result %+v, want success, ENDED, 42 records, 3 attempts", res)
	}
	if len(reports) != 2 || reports[0].State != BulkImportStateEnding || !reports[0].InProgress || reports[0].Operation != "end" {
		t.Errorf("progress reports %+v, want 2 in-progress ENDING reports", reports)
	}
	for i, c := range f.endCaps {
		if c != "bulk-progress" {
			t.Errorf("End %d sent capabilities %q, want bulk-progress", i+1, c)
		}
	}
}

// A request whose deadline passes with no answer (no early answer asked for)
// is followed by a status read; End is sent again while the session ends.
func TestEndBulkImport_DeadlineThenStatusThenResend(t *testing.T) {
	f := &bulkFake{
		statusFn: func(n int) (*pb.GetBulkImportStatusResponse, error) {
			if n == 1 {
				return &pb.GetBulkImportStatusResponse{IsActive: true, State: pb.BulkImportState_BULK_IMPORT_STATE_ACTIVE}, nil
			}
			return &pb.GetBulkImportStatusResponse{IsActive: true, State: pb.BulkImportState_BULK_IMPORT_STATE_ENDING,
				Message: "bulk import session s1 (ending)"}, nil
		},
		endFn: func(ctx context.Context, n int) (*pb.EndBulkImportResponse, error) {
			if n == 1 {
				return nil, waitForDeadline(ctx)
			}
			return &pb.EndBulkImportResponse{Success: true, State: pb.BulkImportState_BULK_IMPORT_STATE_ENDED}, nil
		},
	}
	client := startBulkFake(t, f)
	var reports []BulkImportProgress
	res, err := client.EndBulkImport(context.Background(), "s1", BulkImportWaitOptions{
		ProgressInterval: 300 * time.Millisecond, DisableEarlyAnswer: true,
		OnProgress: func(p BulkImportProgress) { reports = append(reports, p) },
	})
	if err != nil {
		t.Fatalf("EndBulkImport: %v", err)
	}
	if !res.Success || res.State != BulkImportStateEnded || res.Attempts != 2 {
		t.Errorf("result %+v, want success, ENDED, 2 attempts", res)
	}
	if len(reports) != 1 || reports[0].State != BulkImportStateEnding {
		t.Errorf("progress reports %+v, want the status's ENDING", reports)
	}
	for i, c := range f.endCaps {
		if c != "" {
			t.Errorf("End %d asked for %q with DisableEarlyAnswer", i+1, c)
		}
	}
}

// An older server reports no state: End is sent once, without the header,
// under the caller's context alone, exactly as before.
func TestEndBulkImport_OlderServerOneCall(t *testing.T) {
	f := &bulkFake{
		statusFn: func(int) (*pb.GetBulkImportStatusResponse, error) {
			return &pb.GetBulkImportStatusResponse{IsActive: true}, nil
		},
		endFn: func(ctx context.Context, n int) (*pb.EndBulkImportResponse, error) {
			if _, ok := ctx.Deadline(); ok {
				return nil, status.Error(codes.Internal, "the request carried a deadline the caller did not set")
			}
			return &pb.EndBulkImportResponse{Success: true, TotalRecords: 3, Message: "bulk import completed successfully"}, nil
		},
	}
	client := startBulkFake(t, f)
	res, err := client.EndBulkImport(context.Background(), "s1", BulkImportWaitOptions{ProgressInterval: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("EndBulkImport: %v", err)
	}
	if !res.Success || res.State != BulkImportStateUnspecified || res.Attempts != 1 {
		t.Errorf("result %+v, want success, no state, 1 attempt", res)
	}
	if _, end, _, _, _, _, _, _ := f.counts(); end != 1 || f.endCaps[0] != "" {
		t.Errorf("an older server got %d Ends, capabilities %q; want 1 without the header", end, f.endCaps)
	}
}

// When the total wait passes while the server still ends the session, the
// caller gets a *BulkImportInProgressError that says it has not failed.
func TestEndBulkImport_TotalWaitPasses(t *testing.T) {
	f := &bulkFake{endFn: earlyAnswerEnd(1 << 30)}
	client := startBulkFake(t, f)
	t0 := time.Now()
	_, err := client.EndBulkImport(context.Background(), "s1", BulkImportWaitOptions{
		TotalTimeout: 1500 * time.Millisecond, ProgressInterval: 200 * time.Millisecond,
	})
	var ip *BulkImportInProgressError
	if !errors.As(err, &ip) || !errors.Is(err, ErrBulkImportInProgress) {
		t.Fatalf("err %v (%T), want *BulkImportInProgressError", err, err)
	}
	if ip.State != BulkImportStateEnding || ip.Operation != "end" || ip.SessionID != "s1" ||
		!strings.Contains(ip.Error(), "has not failed") {
		t.Errorf("error %+v: %v", ip, ip)
	}
	if took := time.Since(t0); took > 4*time.Second {
		t.Errorf("the wait took %v past a 1.5 s total", took)
	}
	if _, end, _, _, _, _, _, _ := f.counts(); end > 3 {
		t.Errorf("End sent %d times in 1.5 s: the 1 s pause between sends is not kept", end)
	}
}

// Abort signs in again on an expired sign-in and is sent again when the server
// refused it before running it; its result is the server's.
func TestAbortBulkImport_SignsInAgainAndResends(t *testing.T) {
	f := &bulkFake{
		abortFn: func(ctx context.Context, n int) (*pb.AbortBulkImportResponse, error) {
			if n == 1 {
				return nil, statusWith(codes.Unauthenticated,
					"sign-in session expired (7023): sign in again; this is not a bulk import or transaction session",
					"SESSION_EXPIRED", "code", "7023", "executed", "false")
			}
			if n == 2 {
				return &pb.AbortBulkImportResponse{InProgress: true, State: pb.BulkImportState_BULK_IMPORT_STATE_DISCARDING,
					Progress: 50, ProgressTotal: 100, NodesRemoved: 50}, nil
			}
			return &pb.AbortBulkImportResponse{Success: true, State: pb.BulkImportState_BULK_IMPORT_STATE_ABORTED,
				Progress: 100, ProgressTotal: 100, NodesRemoved: 100, EdgesRemoved: 7,
				Message: "bulk import aborted; discarded 100 nodes and 7 edges in 1s"}, nil
		},
	}
	client := startBulkFake(t, f)
	var reports []BulkImportProgress
	res, err := client.AbortBulkImport(context.Background(), "s1", BulkImportWaitOptions{
		OnProgress: func(p BulkImportProgress) { reports = append(reports, p) },
	})
	if err != nil {
		t.Fatalf("AbortBulkImport: %v", err)
	}
	if !res.Success || res.State != BulkImportStateAborted || res.NodesRemoved != 100 || res.EdgesRemoved != 7 ||
		!strings.Contains(res.Message, "discarded 100 nodes") {
		t.Errorf("result %+v", res)
	}
	if len(reports) != 1 || reports[0].Operation != "abort" || reports[0].NodesRemoved != 50 || reports[0].ProgressTotal != 100 {
		t.Errorf("progress reports %+v", reports)
	}
	_, _, abort, _, _, _, _, logins := f.counts()
	if abort != 3 || logins != 2 {
		t.Errorf("abort sent %d times and %d sign-ins, want 3 and 2", abort, logins)
	}
	if f.endSession[0] == f.endSession[1] {
		t.Errorf("the Abort after signing in again carried the old sign-in %q", f.endSession[1])
	}
}

// An Abort the server answers success=false (its End is running) is reported
// as such: the driver no longer says "aborted successfully" whatever happened.
func TestAbortBulkImport_ReportsTheServersAnswer(t *testing.T) {
	f := &bulkFake{abortFn: func(ctx context.Context, n int) (*pb.AbortBulkImportResponse, error) {
		return &pb.AbortBulkImportResponse{Success: false, State: pb.BulkImportState_BULK_IMPORT_STATE_ENDING,
			Message: "the session cannot be aborted: its End is running"}, nil
	}}
	client := startBulkFake(t, f)
	res, err := client.AbortBulkImport(context.Background(), "s1")
	if err != nil {
		t.Fatalf("AbortBulkImport: %v", err)
	}
	if res.Success || res.State != BulkImportStateEnding || !strings.Contains(res.Message, "cannot be aborted") {
		t.Errorf("result %+v, want the server's refusal", res)
	}
}

// A commit answered 5024 marked partly_stored is "stored in part": not rolled
// back, not sent again, not retried by WithTransactionRetry, not followed by a
// rollback.
func TestCommit_PartlyStoredIsNotRolledBack(t *testing.T) {
	stored := partlyStoredErr(codes.FailedPrecondition,
		"[5024] the transaction's label changes are stored; its node changes are not", ReasonWritesCommitted, "5024")
	f := &bulkFake{commitFn: func(int) (*pb.CommitResponse, error) { return nil, stored }}
	client := startBulkFake(t, f)
	ctx := context.Background()

	tx, err := client.BeginTransaction(ctx, "g", false, 30)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Commit(ctx, tx.ID)
	var pc *PartlyCommittedError
	if !errors.As(err, &pc) || !errors.Is(err, ErrPartlyCommitted) {
		t.Fatalf("err %v (%T), want *PartlyCommittedError", err, err)
	}
	if EngineCode(err) != CodeWritesCommitted || ErrorReason(err) != ReasonWritesCommitted || IsWriteConflict(err) {
		t.Errorf("code %d reason %q conflict %v", EngineCode(err), ErrorReason(err), IsWriteConflict(err))
	}
	if !strings.Contains(err.Error(), "label changes are stored") || strings.Contains(err.Error(), "rolled back;") {
		t.Errorf("message %q", err)
	}
	if tx.IsRolledBack() || !tx.IsPartlyCommitted() || tx.IsActive() {
		t.Errorf("tx rolledBack=%v partlyCommitted=%v active=%v", tx.IsRolledBack(), tx.IsPartlyCommitted(), tx.IsActive())
	}

	err = client.WithTransactionRetry(ctx, "g", false, 3, func(uint64) error { return nil })
	if !errors.As(err, &pc) {
		t.Fatalf("WithTransactionRetry: %v, want *PartlyCommittedError", err)
	}
	_, _, _, commit, rollback, begin, _, _ := f.counts()
	if commit != 2 || rollback != 0 || begin != 2 {
		t.Errorf("commits %d, rollbacks %d, begins %d; want 2, 0, 2 (each commit once, never retried or rolled back)", commit, rollback, begin)
	}
}

// The commit's warnings reach the caller.
func TestCommit_WarningsReachTheCaller(t *testing.T) {
	f := &bulkFake{commitFn: func(int) (*pb.CommitResponse, error) {
		return &pb.CommitResponse{Success: true, Message: "committed; warning: index idx_a missed a change",
			Warnings: []string{"index idx_a missed a change: out of use until ALTER INDEX idx_a REBUILD"}}, nil
	}}
	client := startBulkFake(t, f)
	ctx := context.Background()
	tx, err := client.BeginTransaction(ctx, "g", false, 30)
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.CommitWithResult(ctx, tx.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := "index idx_a missed a change: out of use until ALTER INDEX idx_a REBUILD"
	if !res.Success || len(res.Warnings) != 1 || res.Warnings[0] != want {
		t.Errorf("result %+v", res)
	}
	if w := tx.Warnings(); len(w) != 1 || w[0] != want || !tx.IsCommitted() {
		t.Errorf("tx warnings %q committed %v", w, tx.IsCommitted())
	}
}

// A statement of a transaction whose sign-in expired: the driver signs in
// again, does not send the statement again, reports it clearly, and neither it
// nor WithTransaction begins another transaction.
func TestTransaction_SignInExpired(t *testing.T) {
	expired := statusWith(codes.Unauthenticated,
		"sign-in session expired (7023): sign in again; this is not a bulk import or transaction session",
		"SESSION_EXPIRED", "code", "7023", "executed", "false")
	var expire atomic.Bool
	f := &bulkFake{gqlFn: func(n int, req *pb.GqlRequest) (*pb.GqlResponse, error) {
		if req.TransactionId != 0 && expire.CompareAndSwap(true, false) {
			return nil, expired
		}
		if req.TransactionId != 0 && req.TransactionId != 8 && req.TransactionId != 9 {
			return nil, status.Error(codes.NotFound, "transaction not found")
		}
		return &pb.GqlResponse{}, nil
	}}
	client := startBulkFake(t, f)
	ctx := context.Background()

	tx, err := client.BeginTransaction(ctx, "g", false, 30)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Gql(ctx, "INSERT (:A)", &QueryConfig{TransactionID: tx.ID}); err != nil {
		t.Fatal(err)
	}
	expire.Store(true)
	_, err = client.Gql(ctx, "INSERT (:B)", &QueryConfig{TransactionID: tx.ID})
	var tse *TransactionSignInExpiredError
	if !errors.As(err, &tse) || !errors.Is(err, ErrTransactionSignInExpired) || tse.TransactionID != tx.ID {
		t.Fatalf("err %v (%T), want *TransactionSignInExpiredError for %d", err, err, tx.ID)
	}
	if !strings.Contains(err.Error(), "begin a new transaction") || !strings.Contains(err.Error(), "none of its changes were committed") {
		t.Errorf("message %q", err)
	}
	if !tx.IsSignInLost() || tx.IsActive() {
		t.Errorf("tx signInLost=%v active=%v", tx.IsSignInLost(), tx.IsActive())
	}
	// The server keeps the dead transaction under the client's session id:
	// the client takes a new id, and asks once, with the old id, to roll the
	// dead transaction back.
	if client.ClientSessionID() == tx.ClientSessionID {
		t.Errorf("the client kept its session id %q after losing a transaction", tx.ClientSessionID)
	}
	f.mu.Lock()
	rollbacks := 0
	for _, l := range f.gqlLog {
		if l == "ROLLBACK|"+tx.ClientSessionID {
			rollbacks++
		}
	}
	f.mu.Unlock()
	if rollbacks != 1 {
		t.Errorf("ROLLBACK with the old client session id sent %d times, want 1 (log %q)", rollbacks, f.gqlLog)
	}
	_, _, _, _, _, begin, gql, logins := f.counts()
	if gql != 2 || logins != 2 || begin != 1 {
		t.Errorf("gql %d logins %d begins %d; want 2 (the statement not sent again), 2, 1", gql, logins, begin)
	}

	// Commit and Rollback of it are refused without being sent.
	if _, err := client.Commit(ctx, tx.ID); !errors.As(err, &tse) {
		t.Errorf("Commit: %v, want *TransactionSignInExpiredError", err)
	}
	if _, err := client.Rollback(ctx, tx.ID); !errors.As(err, &tse) {
		t.Errorf("Rollback: %v, want *TransactionSignInExpiredError", err)
	}
	if _, _, _, commit, rollback, _, _, _ := f.counts(); commit != 0 || rollback != 0 {
		t.Errorf("commit %d rollback %d sent for a lost transaction", commit, rollback)
	}

	// WithTransaction does not begin another one by itself.
	expire.Store(true)
	err = client.WithTransactionRetry(ctx, "g", false, 3, func(txID uint64) error {
		_, err := client.Gql(ctx, "INSERT (:C)", &QueryConfig{TransactionID: txID})
		return err
	})
	if !errors.As(err, &tse) {
		t.Fatalf("WithTransactionRetry: %v, want *TransactionSignInExpiredError", err)
	}
	if _, _, _, commit, _, begin, _, _ := f.counts(); begin != 2 || commit != 0 {
		t.Errorf("begins %d commits %d; want 2 (one per call, none by the driver) and 0", begin, commit)
	}

	// Plain calls keep working on the new sign-in.
	if _, err := client.Gql(ctx, "RETURN 1", nil); err != nil {
		t.Errorf("a plain query after signing in again: %v", err)
	}
}

// A transaction begun under a sign-in that a new one replaced (another
// goroutine signed in again) is refused without being sent.
func TestTransaction_BegunUnderAnEarlierSignIn(t *testing.T) {
	f := &bulkFake{}
	client := startBulkFake(t, f)
	ctx := context.Background()
	tx, err := client.BeginTransaction(ctx, "g", false, 30)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Login(ctx, "admin", "pw"); err != nil {
		t.Fatal(err)
	}
	_, err = client.Gql(ctx, "INSERT (:A)", &QueryConfig{TransactionID: tx.ID})
	var tse *TransactionSignInExpiredError
	if !errors.As(err, &tse) {
		t.Fatalf("err %v, want *TransactionSignInExpiredError", err)
	}
	if _, _, _, _, _, _, gql, _ := f.counts(); gql != 0 {
		t.Errorf("the statement was sent %d times", gql)
	}
}

// Review round 1.

// A client session id the application set may be shared with other clients:
// after a re-login that finds a lost transaction, the driver keeps it and sends
// no ROLLBACK under it (which would end another client's live transaction).
// The lost transaction is still reported.
func TestTransaction_SignInExpired_ConfiguredSessionIDIsKept(t *testing.T) {
	expired := statusWith(codes.Unauthenticated,
		"sign-in session expired (7023): sign in again; this is not a bulk import or transaction session",
		"SESSION_EXPIRED", "code", "7023", "executed", "false")
	var expire atomic.Bool
	f := &bulkFake{gqlFn: func(n int, req *pb.GqlRequest) (*pb.GqlResponse, error) {
		if req.TransactionId != 0 && expire.CompareAndSwap(true, false) {
			return nil, expired
		}
		return &pb.GqlResponse{}, nil
	}}
	client := startBulkFakeWith(t, f, func(c *Config) { c.SessionID = "shared-csid-1" })
	ctx := context.Background()
	tx, err := client.BeginTransaction(ctx, "g", false, 30)
	if err != nil {
		t.Fatal(err)
	}
	expire.Store(true)
	_, err = client.Gql(ctx, "INSERT (:B)", &QueryConfig{TransactionID: tx.ID})
	var tse *TransactionSignInExpiredError
	if !errors.As(err, &tse) || tse.TransactionID != tx.ID {
		t.Fatalf("err %v (%T), want *TransactionSignInExpiredError for %d", err, err, tx.ID)
	}
	if got := client.ClientSessionID(); got != "shared-csid-1" {
		t.Errorf("the configured client session id changed to %q", got)
	}
	f.mu.Lock()
	log := append([]string(nil), f.gqlLog...)
	f.mu.Unlock()
	for _, l := range log {
		if strings.HasPrefix(l, "ROLLBACK|") {
			t.Errorf("a ROLLBACK was sent under the shared client session id (log %q)", log)
		}
	}
	if _, err := client.Commit(ctx, tx.ID); !errors.As(err, &tse) {
		t.Errorf("Commit of the lost transaction: %v, want *TransactionSignInExpiredError", err)
	}
	if _, err := client.Gql(ctx, "RETURN 1", nil); err != nil {
		t.Errorf("a plain query after signing in again: %v", err)
	}
	if _, _, _, commit, rollback, _, _, logins := f.counts(); commit != 0 || rollback != 0 || logins != 2 {
		t.Errorf("commit %d rollback %d logins %d; want 0, 0, 2", commit, rollback, logins)
	}
}

// A progress callback that panics does not end the wait: the panic is
// recovered, logged once, and End goes on to its final state; the callback is
// still called for each report.
func TestEndBulkImport_ProgressCallbackPanicDoesNotEndTheWait(t *testing.T) {
	var logged []string
	var logMu sync.Mutex
	saved := bulkLogf
	bulkLogf = func(format string, args ...interface{}) {
		logMu.Lock()
		defer logMu.Unlock()
		logged = append(logged, format)
	}
	t.Cleanup(func() { bulkLogf = saved })

	f := &bulkFake{endFn: earlyAnswerEnd(3)}
	client := startBulkFake(t, f)
	calls := 0
	res, err := client.EndBulkImport(context.Background(), "s1", BulkImportWaitOptions{
		ProgressInterval: 2 * time.Second,
		OnProgress: func(BulkImportProgress) {
			calls++
			panic("callback bug")
		},
	})
	if err != nil {
		t.Fatalf("EndBulkImport: %v", err)
	}
	if !res.Success || res.State != BulkImportStateEnded || res.Attempts != 4 {
		t.Errorf("result %+v, want success, ENDED, 4 attempts", res)
	}
	if calls != 3 {
		t.Errorf("the callback ran %d times, want 3 (once per report, after a panic too)", calls)
	}
	logMu.Lock()
	defer logMu.Unlock()
	if len(logged) != 1 || !strings.Contains(logged[0], "OnProgress") {
		t.Errorf("log lines %q, want exactly one about OnProgress", logged)
	}
}

// A tiny progress interval is raised to 100 ms, and the status read after a
// missed deadline gets a deadline of its own: a status that takes 150 ms to
// answer still lets the wait reach ENDED.
func TestEndBulkImport_TinyIntervalAndTheStatusReadsOwnDeadline(t *testing.T) {
	var endDeadline atomic.Int64
	f := &bulkFake{
		statusFn: func(n int) (*pb.GetBulkImportStatusResponse, error) {
			if n == 1 {
				return &pb.GetBulkImportStatusResponse{IsActive: true, State: pb.BulkImportState_BULK_IMPORT_STATE_ACTIVE}, nil
			}
			time.Sleep(150 * time.Millisecond)
			return &pb.GetBulkImportStatusResponse{IsActive: true, State: pb.BulkImportState_BULK_IMPORT_STATE_ENDING}, nil
		},
		endFn: func(ctx context.Context, n int) (*pb.EndBulkImportResponse, error) {
			if n == 1 {
				if d, ok := ctx.Deadline(); ok {
					endDeadline.Store(int64(time.Until(d)))
				}
				return nil, waitForDeadline(ctx)
			}
			return &pb.EndBulkImportResponse{Success: true, State: pb.BulkImportState_BULK_IMPORT_STATE_ENDED}, nil
		},
	}
	client := startBulkFake(t, f)
	res, err := client.EndBulkImport(context.Background(), "s1", BulkImportWaitOptions{
		ProgressInterval: 10 * time.Millisecond, DisableEarlyAnswer: true,
	})
	if err != nil {
		t.Fatalf("EndBulkImport with a 10 ms interval: %v", err)
	}
	if res.State != BulkImportStateEnded || res.Attempts != 2 {
		t.Errorf("result %+v, want ENDED after 2 attempts", res)
	}
	if d := time.Duration(endDeadline.Load()); d < 60*time.Millisecond {
		t.Errorf("the End request carried a %v deadline; a 10 ms interval should be raised to 100 ms", d)
	}
}

// When the total wait ends the status read that follows a missed deadline,
// the caller is told the work is still running, not handed the request's
// deadline error.
func TestEndBulkImport_TotalWaitEndsTheStatusRead(t *testing.T) {
	f := &bulkFake{
		statusFn: func(n int) (*pb.GetBulkImportStatusResponse, error) {
			if n == 1 {
				return &pb.GetBulkImportStatusResponse{IsActive: true, State: pb.BulkImportState_BULK_IMPORT_STATE_ENDING}, nil
			}
			time.Sleep(2 * time.Second)
			return &pb.GetBulkImportStatusResponse{IsActive: true, State: pb.BulkImportState_BULK_IMPORT_STATE_ENDING}, nil
		},
		endFn: func(ctx context.Context, n int) (*pb.EndBulkImportResponse, error) { return nil, waitForDeadline(ctx) },
	}
	client := startBulkFake(t, f)
	_, err := client.EndBulkImport(context.Background(), "s1", BulkImportWaitOptions{
		TotalTimeout: time.Second, ProgressInterval: 300 * time.Millisecond, DisableEarlyAnswer: true,
	})
	var ip *BulkImportInProgressError
	if !errors.As(err, &ip) || ip.State != BulkImportStateEnding {
		t.Fatalf("err %v (%T), want *BulkImportInProgressError in ENDING", err, err)
	}
}

// The "stopped waiting" error says what the last state means. ACTIVE: the
// server had not reported the End as started; it may not have reached the
// server, and then the session stays open and the idle cleanup discards it
// unless End is sent again. No advice to poll the status.
func TestBulkImportInProgressError_SaysWhatTheStateMeans(t *testing.T) {
	f := &bulkFake{endFn: func(ctx context.Context, n int) (*pb.EndBulkImportResponse, error) {
		return nil, waitForDeadline(ctx)
	}}
	client := startBulkFake(t, f)
	_, err := client.EndBulkImport(context.Background(), "s1", BulkImportWaitOptions{TotalTimeout: time.Second})
	var ip *BulkImportInProgressError
	if !errors.As(err, &ip) {
		t.Fatalf("err %v (%T), want *BulkImportInProgressError", err, err)
	}
	msg := ip.Error()
	if ip.State != BulkImportStateActive || ip.Attempts != 1 ||
		!strings.Contains(msg, "before the server reported the End as started (last state ACTIVE)") ||
		!strings.Contains(msg, "may not have reached the server") || !strings.Contains(msg, "idle cleanup") ||
		strings.Contains(msg, "GetBulkImportStatus") || strings.Contains(msg, "still running") {
		t.Errorf("ACTIVE: %+v: %s", ip, msg)
	}

	cases := []struct {
		e         BulkImportInProgressError
		want, not []string
	}{
		{BulkImportInProgressError{SessionID: "s", Operation: "end", State: BulkImportStateActive},
			[]string{"before the request was sent", "call EndBulkImport again to keep its data", "idle cleanup"},
			[]string{"GetBulkImportStatus", "may not have reached"}},
		{BulkImportInProgressError{SessionID: "s", Operation: "abort", State: BulkImportStateActive, Attempts: 2},
			[]string{"reported the discard as started", "holds the graph", "call AbortBulkImport again"}, []string{"GetBulkImportStatus"}},
		{BulkImportInProgressError{SessionID: "s", Operation: "end", State: BulkImportStateEnding, Progress: 3, ProgressTotal: 9, Attempts: 4},
			[]string{"still running", "3 of 9 records", "has not failed", "call EndBulkImport again to wait for its outcome"},
			[]string{"GetBulkImportStatus", "idle"}},
		{BulkImportInProgressError{SessionID: "s", Operation: "abort", State: BulkImportStateDiscarding, Attempts: 4},
			[]string{"still running", "call AbortBulkImport again", "repeated Abort"}, []string{"GetBulkImportStatus"}},
		{BulkImportInProgressError{SessionID: "s", Operation: "end", State: BulkImportStateEnded, Attempts: 4},
			[]string{"reached state ENDED", "call EndBulkImport again for its outcome"}, []string{"still running"}},
	}
	for _, c := range cases {
		m := c.e.Error()
		for _, w := range c.want {
			if !strings.Contains(m, w) {
				t.Errorf("%s/%s: %q lacks %q", c.e.Operation, c.e.State, m, w)
			}
		}
		for _, n := range c.not {
			if strings.Contains(m, n) {
				t.Errorf("%s/%s: %q says %q", c.e.Operation, c.e.State, m, n)
			}
		}
	}
}

// Review round 2.

// A statement of a transaction answered UNAUTHENTICATED is never sent again,
// and the caller gets the lost-transaction error, also when the first attempt
// to sign in again fails (here the connection drops during the sign-in). The
// message says so; the next call signs in again.
func TestTransaction_SignInExpired_SigningInAgainFails(t *testing.T) {
	expired := statusWith(codes.Unauthenticated,
		"sign-in session expired (7023): sign in again; this is not a bulk import or transaction session",
		"SESSION_EXPIRED", "code", "7023", "executed", "false")
	var expirePlain atomic.Bool
	var sentMu sync.Mutex
	var sent []uint64
	f := &bulkFake{
		loginFn: func(n int) error {
			if n == 2 {
				return status.Error(codes.Unavailable, "connection reset during sign-in")
			}
			return nil
		},
		gqlFn: func(n int, req *pb.GqlRequest) (*pb.GqlResponse, error) {
			if req.TransactionId != 0 {
				sentMu.Lock()
				sent = append(sent, req.TransactionId)
				sentMu.Unlock()
				return nil, expired
			}
			if expirePlain.CompareAndSwap(true, false) {
				return nil, expired
			}
			return &pb.GqlResponse{}, nil
		},
	}
	client := startBulkFake(t, f)
	ctx := context.Background()
	tx, err := client.BeginTransaction(ctx, "g", false, 30)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Gql(ctx, "INSERT (:A)", &QueryConfig{TransactionID: tx.ID})
	var tse *TransactionSignInExpiredError
	if !errors.As(err, &tse) || tse.TransactionID != tx.ID {
		t.Fatalf("err %v (%T), want *TransactionSignInExpiredError for %d", err, err, tx.ID)
	}
	msg := err.Error()
	for _, want := range []string{"signing in again failed too", "connection reset during sign-in", "none of its changes were committed"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not say %q", msg, want)
		}
	}
	if strings.Contains(msg, "the driver signed in again") {
		t.Errorf("message %q claims the driver signed in again", msg)
	}
	sentMu.Lock()
	n := len(sent)
	sentMu.Unlock()
	if n != 1 {
		t.Errorf("the statement of the lost transaction was sent %d times, want 1", n)
	}
	if !tx.IsSignInLost() {
		t.Error("the transaction is not marked lost")
	}
	if _, _, _, _, _, _, _, logins := f.counts(); logins != 2 {
		t.Errorf("logins %d, want 2", logins)
	}

	// Nothing of it is sent any more; the next plain call signs in and works.
	if _, err := client.Commit(ctx, tx.ID); !errors.As(err, &tse) {
		t.Errorf("Commit: %v, want *TransactionSignInExpiredError", err)
	}
	expirePlain.Store(true)
	if _, err := client.Gql(ctx, "RETURN 1", nil); err != nil {
		t.Errorf("a plain query after the failed sign-in: %v", err)
	}
	if _, _, _, commit, _, _, _, logins := f.counts(); commit != 0 || logins != 3 {
		t.Errorf("commit %d logins %d; want 0 and 3", commit, logins)
	}
}
