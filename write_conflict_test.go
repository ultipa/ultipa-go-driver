package gqldb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Write conflicts (engine code 3011): nothing is sent again by the driver.
//
// A real gRPC server runs in this process, so the errors cross the wire with
// their details exactly as the server sends them. The fake stores one node for
// each auto-commit Gql it answers with success, and the writes of a
// transaction only when its Commit succeeds. A conflict stores nothing, unless
// its detail says part of the request is stored. It also keeps a transaction
// opened with GQL text (START TRANSACTION ... COMMIT, no transaction id): a
// conflict ends it with nothing stored, and a request sent after that runs on
// its own.
//
// The rule (GUIDE.md, "Write conflicts"): a write conflict is recognised from
// the server's detail (code 3011 or the reason WRITE_CONFLICT); for a request
// without a transaction id never from the text "[3011]" or the status Aborted
// alone. A server older than the detail answers a conflict with status Aborted
// and a message that starts with "[3011]": that is accepted only for a call
// that carries a transaction id (a statement of a transaction begun with
// BeginTransaction, or its Commit), only when the answer has no detail at all,
// and only to return a *WriteConflictError and so let WithTransactionRetry run
// the whole body again. The driver never sends a request again after a write
// conflict: not an
// auto-commit request (the server already runs a conflicting auto-commit
// statement again by itself), not a statement of a transaction, not a request
// of a transaction opened with GQL text. WithTransactionRetry runs the whole
// body again.

const (
	conflictText  = `[3011] write conflict: node "k" was modified by another transaction after this one read it; re-read it and retry`
	conflictWrite = "MATCH (n:P WHERE n.pid='k') SET n.v = 1"
	conflictRead  = "MATCH (n:P WHERE n.pid='k') RETURN n.v"
)

var (
	conflict             = statusWith(codes.Aborted, conflictText, "WRITE_CONFLICT", "code", "3011")
	conflictReasonOnly   = statusWith(codes.Aborted, conflictText, "WRITE_CONFLICT", "code", "0")
	conflictPartlyStored = statusWith(codes.Aborted, conflictText, "WRITE_CONFLICT", "code", "3011", "partly_stored", "true")
	// A server older than the detail: the status and the words only.
	conflictNoDetail = statusWith(codes.Aborted, conflictText, "")
	// The words in a message whose detail says something else.
	conflictWordsOnly = statusWith(codes.AlreadyExists, "node '[3011]' already exists", "ALREADY_EXISTS", "code", "5017")
	// How an older server words a conflict, and answers that only look like it.
	legacyStatementText = "[3011] failed to execute MERGE: write conflict: another transaction holds a key this " +
		"statement needs, and this transaction already holds one that would have to be released first; retry the transaction"
	legacyCommitText = `[3011] failed to apply write buffer: write conflict: node "k" was modified by another ` +
		"transaction after this one read it; re-read it and retry"
	legacyStatement = statusWith(codes.Aborted, legacyStatementText, "")
	legacyCommit    = statusWith(codes.Aborted, legacyCommitText, "")
	lookalikes      = map[string]error{
		"[3011] not at the start":       statusWith(codes.Aborted, "write conflict [3011] on node k", ""),
		"[30110]":                       statusWith(codes.Aborted, "[30110] something else", ""),
		"not Aborted":                   statusWith(codes.Internal, legacyStatementText, ""),
		"the detail says otherwise":     statusWith(codes.Aborted, legacyStatementText, "INTERNAL", "code", "5017"),
		"the detail says partly stored": conflictPartlyStored,
	}
	// A COMMIT with no transaction open.
	noActiveTransaction = statusWith(codes.FailedPrecondition, "[5017] no active transaction to commit", "INTERNAL", "code", "5017")
)

type conflictNode struct {
	pb.UnimplementedSessionServiceServer
	pb.UnimplementedQueryServiceServer
	pb.UnimplementedTransactionServiceServer

	mu           sync.Mutex
	gqlErrors    []error // the next Gql / GqlStream answers; nil means success
	commitErrors []error
	rowsBefore   int
	calls        int
	begins       int
	commits      int
	stored       int
	pending      map[uint64]int
	nextTx       uint64
	// A transaction opened with GQL text (START TRANSACTION), which the
	// requests in it name by no transaction id.
	sessionTx      bool
	sessionPending int
}

func (n *conflictNode) answer(req *pb.GqlRequest) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls++
	text := strings.ToUpper(strings.TrimSpace(req.GetGql()))
	if req.GetTransactionId() == 0 && text == "START TRANSACTION" {
		n.sessionTx = true
		return nil
	}
	if len(n.gqlErrors) > 0 {
		err := n.gqlErrors[0]
		n.gqlErrors = n.gqlErrors[1:]
		if err != nil {
			if req.GetTransactionId() == 0 && n.sessionTx {
				// The conflict ends the session's transaction; nothing of it
				// is stored.
				n.sessionTx, n.sessionPending = false, 0
			}
			if PartlyStored(err) {
				n.stored++
			}
			return err
		}
	}
	writes := 0
	if strings.Contains(req.GetGql(), " SET ") || strings.Contains(req.GetGql(), "INSERT") {
		writes = 1
	}
	commits := strings.HasSuffix(text, "COMMIT")
	switch {
	case req.GetTransactionId() != 0:
		n.pending[req.GetTransactionId()] += writes
	case n.sessionTx:
		n.sessionPending += writes
		if commits {
			n.stored += n.sessionPending
			n.sessionTx, n.sessionPending = false, 0
		}
	default:
		// Outside any transaction the request runs on its own: its write is
		// stored, and a trailing COMMIT finds no transaction.
		n.stored += writes
		if commits {
			return noActiveTransaction
		}
	}
	return nil
}

func (n *conflictNode) Login(context.Context, *pb.LoginRequest) (*pb.LoginResponse, error) {
	return &pb.LoginResponse{SessionId: 1, ServerVersion: "test"}, nil
}

func (n *conflictNode) Logout(context.Context, *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	return &pb.LogoutResponse{Success: true}, nil
}

func (n *conflictNode) Gql(_ context.Context, req *pb.GqlRequest) (*pb.GqlResponse, error) {
	if err := n.answer(req); err != nil {
		return nil, err
	}
	return &pb.GqlResponse{}, nil
}

func (n *conflictNode) GqlStream(req *pb.GqlRequest, stream pb.QueryService_GqlStreamServer) error {
	if err := n.answer(req); err != nil {
		for i := 0; i < n.rowsBefore; i++ {
			if serr := stream.Send(&pb.GqlResponse{}); serr != nil {
				return serr
			}
		}
		return err
	}
	return stream.Send(&pb.GqlResponse{})
}

func (n *conflictNode) Begin(context.Context, *pb.BeginRequest) (*pb.BeginResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.begins++
	n.nextTx++
	return &pb.BeginResponse{TransactionId: n.nextTx}, nil
}

func (n *conflictNode) Commit(_ context.Context, req *pb.CommitRequest) (*pb.CommitResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.commits++
	writes := n.pending[req.GetTransactionId()]
	delete(n.pending, req.GetTransactionId())
	if len(n.commitErrors) > 0 {
		err := n.commitErrors[0]
		n.commitErrors = n.commitErrors[1:]
		if err != nil {
			return nil, err
		}
	}
	n.stored += writes
	return &pb.CommitResponse{Success: true}, nil
}

func (n *conflictNode) Rollback(_ context.Context, req *pb.RollbackRequest) (*pb.RollbackResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.pending, req.GetTransactionId())
	return &pb.RollbackResponse{Success: true}, nil
}

func (n *conflictNode) counts() (calls, stored int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.calls, n.stored
}

// conflictClient starts a conflictNode and a client signed in to it, whose
// waits between retries are recorded instead of slept.
func conflictClient(t *testing.T, retryCount int, node *conflictNode) (*Client, *[]time.Duration) {
	t.Helper()
	node.pending = map[uint64]int{}
	node.nextTx = 100
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterSessionServiceServer(srv, node)
	pb.RegisterQueryServiceServer(srv, node)
	pb.RegisterTransactionServiceServer(srv, node)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	cfg := NewConfigBuilder().Hosts(lis.Addr().String()).HealthCheckInterval(0).RetryCount(retryCount).Build()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	waits := &[]time.Duration{}
	client.retrySleep = func(_ context.Context, d time.Duration) error {
		*waits = append(*waits, d)
		return nil
	}
	if _, err := client.Login(context.Background(), "admin", "pw"); err != nil {
		t.Fatal(err)
	}
	return client, waits
}

func TestWriteConflict_AutoCommitGql(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name       string
		errs       []error
		config     *QueryConfig
		wantStored int
		wantWCE    bool // the error is a *WriteConflictError
	}{
		{"returned, not sent again", []error{conflict}, nil, 0, true},
		{"the reason alone is a conflict", []error{conflictReasonOnly}, nil, 0, true},
		{"marked read-only: returned, not sent again", []error{conflict}, &QueryConfig{ReadOnly: true}, 0, true},
		{"inside a transaction: not sent again", []error{conflict}, &QueryConfig{TransactionID: 7}, 0, true},
		{"marked partly stored: not sent again, not a conflict", []error{conflictPartlyStored}, nil, 1, false},
		{"server without the detail: not sent again, not a conflict", []error{conflictNoDetail}, nil, 0, false},
		{"words in another error: not a conflict", []error{conflictWordsOnly}, nil, 0, false},
	}
	for _, retries := range []int{0, 3, 10} {
		for _, c := range cases {
			t.Run(fmt.Sprintf("RetryCount %d, %s", retries, c.name), func(t *testing.T) {
				node := &conflictNode{gqlErrors: c.errs}
				client, waits := conflictClient(t, retries, node)
				_, err := client.Gql(ctx, conflictWrite, c.config)
				if err == nil {
					t.Fatal("no error")
				}
				calls, stored := node.counts()
				if calls != 1 || stored != c.wantStored {
					t.Errorf("calls, stored = %d, %d, want 1, %d", calls, stored, c.wantStored)
				}
				if len(*waits) != 0 {
					t.Errorf("waits = %v, want none", *waits)
				}
				var wce *WriteConflictError
				if got := errors.As(err, &wce); got != c.wantWCE {
					t.Errorf("*WriteConflictError = %v, want %v (%v)", got, c.wantWCE, err)
				}
				if IsWriteConflict(err) != c.wantWCE {
					t.Errorf("IsWriteConflict = %v, want %v", IsWriteConflict(err), c.wantWCE)
				}
				if c.config != nil && c.config.TransactionID != 0 {
					if !strings.Contains(err.Error(), "run the whole transaction again") {
						t.Errorf("a conflict in a transaction does not say to run the whole transaction again: %v", err)
					}
				} else if strings.Contains(err.Error(), "run the whole transaction again") {
					t.Errorf("an auto-commit conflict talks about a transaction: %v", err)
				}
			})
		}
	}
}

func TestWriteConflict_CodesFromTheDetail(t *testing.T) {
	node := &conflictNode{gqlErrors: []error{conflict}}
	client, _ := conflictClient(t, 0, node)
	_, err := client.Gql(context.Background(), conflictWrite, nil)
	var ge *GqldbError
	if !errors.As(err, &ge) {
		t.Fatalf("a write conflict hides the *GqldbError: %T %v", err, err)
	}
	if EngineCode(err) != CodeWriteConflict || ErrorReason(err) != ReasonWriteConflict {
		t.Errorf("EngineCode, ErrorReason = %d, %q", EngineCode(err), ErrorReason(err))
	}
	// The constant as shipped in 6.2.14 (GUIDE.md, "What code means on each error type").
	if WriteConflictCode != 3011 {
		t.Errorf("WriteConflictCode = %d, want 3011", WriteConflictCode)
	}
	if !errors.Is(err, ErrWriteConflict) {
		t.Error("errors.Is(err, ErrWriteConflict) = false")
	}
	if calls, _ := node.counts(); calls != 1 {
		t.Errorf("RetryCount 0: %d calls, want 1", calls)
	}
	if st, _ := status.FromError(errors.Unwrap(ge)); st.Code() != codes.Aborted {
		t.Errorf("status = %v, want Aborted", st.Code())
	}
}

func TestWriteConflict_Stream(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name       string
		rowsBefore int
		config     *QueryConfig
		wantRows   int
	}{
		{"before any row: returned, not sent again", 0, nil, 0},
		{"after a row: not sent again", 1, nil, 1},
		{"inside a transaction: not sent again", 0, &QueryConfig{TransactionID: 7}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			node := &conflictNode{gqlErrors: []error{conflict}, rowsBefore: c.rowsBefore}
			client, _ := conflictClient(t, 3, node)
			rows := 0
			err := client.GqlStream(ctx, conflictWrite, c.config, func(*Response) error { rows++; return nil })
			if err == nil {
				t.Fatal("no error")
			}
			if calls, stored := node.counts(); calls != 1 || stored != 0 || rows != c.wantRows {
				t.Errorf("calls, stored, rows = %d, %d, %d, want 1, 0, %d", calls, stored, rows, c.wantRows)
			}
			var wce *WriteConflictError
			if !errors.As(err, &wce) {
				t.Errorf("a stream's conflict is not a *WriteConflictError: %v", err)
			}
		})
	}
}

// A transaction opened with GQL text carries no transaction id, so the
// driver cannot tell its requests from auto-commit ones. The request that
// loses the conflict, sent again, would run outside the transaction: its write
// stored on its own and the rest of the transaction lost.
func TestWriteConflict_SessionTransaction(t *testing.T) {
	ctx := context.Background()
	for _, last := range []string{
		"MATCH (n:P WHERE n.pid='k2') SET n.v = 7 COMMIT",
		"MATCH (n:P WHERE n.pid='k2') SET n.v = 7",
	} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s, stream %v", last, stream), func(t *testing.T) {
				node := &conflictNode{gqlErrors: []error{nil, conflict}}
				client, waits := conflictClient(t, 3, node)
				if _, err := client.Gql(ctx, "START TRANSACTION", nil); err != nil {
					t.Fatal(err)
				}
				if _, err := client.Gql(ctx, conflictWrite, nil); err != nil {
					t.Fatal(err)
				}
				before, _ := node.counts()
				var err error
				if stream {
					err = client.GqlStream(ctx, last, nil, func(*Response) error { return nil })
				} else {
					_, err = client.Gql(ctx, last, nil)
				}
				calls, stored := node.counts()
				if calls-before != 1 || stored != 0 {
					t.Errorf("calls, stored = %d, %d, want 1, 0 (%v)", calls-before, stored, err)
				}
				var wce *WriteConflictError
				if !errors.As(err, &wce) || EngineCode(err) != CodeWriteConflict {
					t.Errorf("the conflict did not reach the caller: %T %v", err, err)
				}
				if len(*waits) != 0 {
					t.Errorf("waits = %v, want none", *waits)
				}
			})
		}
	}
}

func TestWriteConflict_Transactions(t *testing.T) {
	ctx := context.Background()

	t.Run("commit says to run the whole transaction again", func(t *testing.T) {
		node := &conflictNode{commitErrors: []error{conflict}}
		client, _ := conflictClient(t, 3, node)
		tx, err := client.BeginTransaction(ctx, "g", false, 30)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Gql(ctx, conflictWrite, &QueryConfig{TransactionID: tx.ID}); err != nil {
			t.Fatal(err)
		}
		_, err = client.Commit(ctx, tx.ID)
		var wce *WriteConflictError
		if !errors.As(err, &wce) || !strings.Contains(err.Error(), "run the whole transaction again") {
			t.Errorf("commit conflict = %v", err)
		}
		if EngineCode(err) != CodeWriteConflict {
			t.Errorf("EngineCode = %d", EngineCode(err))
		}
		if node.commits != 1 || node.stored != 0 {
			t.Errorf("commits, stored = %d, %d, want 1, 0", node.commits, node.stored)
		}
	})

	t.Run("an older server's commit conflict is a write conflict", func(t *testing.T) {
		node := &conflictNode{commitErrors: []error{legacyCommit}}
		client, _ := conflictClient(t, 3, node)
		tx, err := client.BeginTransaction(ctx, "g", false, 30)
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Commit(ctx, tx.ID)
		var wce *WriteConflictError
		if !errors.As(err, &wce) || EngineCode(err) != CodeWriteConflict || ErrorReason(err) != ReasonWriteConflict ||
			!strings.Contains(err.Error(), "run the whole transaction again") {
			t.Errorf("err = %T %v (EngineCode %d), want a *WriteConflictError", err, err, EngineCode(err))
		}
	})

	increment := func(client *Client, runs *int) func(uint64) error {
		return func(txID uint64) error {
			*runs++
			if _, err := client.Gql(ctx, conflictRead, &QueryConfig{TransactionID: txID}); err != nil {
				return err
			}
			_, err := client.Gql(ctx, conflictWrite, &QueryConfig{TransactionID: txID})
			return err
		}
	}

	for _, c := range []struct {
		name                  string
		gqlErrs, commitErrs   []error
		retries               int
		wantErr               bool
		wantBegins, wantRuns  int
		wantStored, wantCalls int
	}{
		{"commit conflict: the whole body runs again", nil, []error{conflict}, 2, false, 2, 2, 1, 4},
		{"statement conflict: the whole body runs again", []error{nil, conflict}, nil, 1, false, 2, 2, 1, 4},
		{"retry is opt-in", nil, []error{conflict}, 0, true, 1, 1, 0, 2},
		{"an older server's commit conflict: the whole body runs again", nil, []error{legacyCommit}, 2, false, 2, 2, 1, 4},
		{"an older server's statement conflict: the whole body runs again", []error{nil, legacyStatement}, nil, 1, false, 2, 2, 1, 4},
		{"no retry for a lookalike without the detail", nil, []error{lookalikes["[3011] not at the start"]}, 3, true, 1, 1, 0, 2},
		{"retry is bounded", nil, []error{conflict, conflict, conflict, conflict, conflict}, 2, true, 3, 3, 0, 6},
	} {
		t.Run(c.name, func(t *testing.T) {
			node := &conflictNode{gqlErrors: c.gqlErrs, commitErrors: c.commitErrs}
			client, _ := conflictClient(t, 3, node)
			runs := 0
			err := client.WithTransactionRetry(ctx, "g", false, c.retries, increment(client, &runs))
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, want error: %v", err, c.wantErr)
			}
			calls, stored := node.counts()
			if node.begins != c.wantBegins || runs != c.wantRuns || stored != c.wantStored || calls != c.wantCalls {
				t.Errorf("begins, runs, stored, calls = %d, %d, %d, %d, want %d, %d, %d, %d",
					node.begins, runs, stored, calls, c.wantBegins, c.wantRuns, c.wantStored, c.wantCalls)
			}
		})
	}
}

// A server older than the detail: status Aborted and a message that starts
// with "[3011]" make a *WriteConflictError only for a call that carries a
// transaction id; nothing is ever sent again.
func TestWriteConflict_OlderServer(t *testing.T) {
	ctx := context.Background()
	call := func(client *Client, stream bool, query string, config *QueryConfig) error {
		if stream {
			return client.GqlStream(ctx, query, config, func(*Response) error { return nil })
		}
		_, err := client.Gql(ctx, query, config)
		return err
	}
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("a statement of a transaction (stream %v)", stream), func(t *testing.T) {
			node := &conflictNode{gqlErrors: []error{legacyStatement}}
			client, waits := conflictClient(t, 3, node)
			err := call(client, stream, conflictWrite, &QueryConfig{TransactionID: 7})
			var wce *WriteConflictError
			if !errors.As(err, &wce) || EngineCode(err) != CodeWriteConflict ||
				!strings.Contains(err.Error(), "run the whole transaction again") {
				t.Errorf("err = %T %v, want a *WriteConflictError", err, err)
			}
			if calls, stored := node.counts(); calls != 1 || stored != 0 || len(*waits) != 0 {
				t.Errorf("calls, stored, waits = %d, %d, %v, want 1, 0, none", calls, stored, *waits)
			}
		})
		t.Run(fmt.Sprintf("no transaction id: a plain error (stream %v)", stream), func(t *testing.T) {
			node := &conflictNode{gqlErrors: []error{legacyStatement}}
			client, _ := conflictClient(t, 3, node)
			err := call(client, stream, conflictWrite, nil)
			if err == nil || IsWriteConflict(err) {
				t.Errorf("err = %v, want a plain error", err)
			}
			if calls, _ := node.counts(); calls != 1 {
				t.Errorf("calls = %d, want 1", calls)
			}
		})
	}
	t.Run("a transaction opened with GQL text: a plain error, one call, nothing stored", func(t *testing.T) {
		node := &conflictNode{gqlErrors: []error{nil, legacyCommit}}
		client, _ := conflictClient(t, 3, node)
		for _, q := range []string{"START TRANSACTION", conflictWrite} {
			if _, err := client.Gql(ctx, q, nil); err != nil {
				t.Fatal(err)
			}
		}
		_, err := client.Gql(ctx, "MATCH (n:P WHERE n.pid='k2') SET n.v = 7 COMMIT", nil)
		if calls, stored := node.counts(); calls != 3 || stored != 0 || err == nil || IsWriteConflict(err) {
			t.Errorf("calls, stored = %d, %d, err = %v, want 3, 0 and a plain error", calls, stored, err)
		}
	})
	for name, answer := range lookalikes {
		t.Run("not a conflict, statement: "+name, func(t *testing.T) {
			node := &conflictNode{gqlErrors: []error{answer}}
			client, _ := conflictClient(t, 3, node)
			if err := call(client, false, conflictWrite, &QueryConfig{TransactionID: 7}); err == nil || IsWriteConflict(err) {
				t.Errorf("err = %v, want a plain error", err)
			}
		})
		t.Run("not a conflict, commit: "+name, func(t *testing.T) {
			node := &conflictNode{commitErrors: []error{answer}}
			client, _ := conflictClient(t, 3, node)
			tx, err := client.BeginTransaction(ctx, "g", false, 30)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Commit(ctx, tx.ID); err == nil || IsWriteConflict(err) {
				t.Errorf("err = %v, want a plain error", err)
			}
		})
	}
	t.Run("WithTransactionRetry: an auto-commit request in the body is not the transaction's", func(t *testing.T) {
		node := &conflictNode{gqlErrors: []error{legacyStatement}}
		client, _ := conflictClient(t, 3, node)
		runs := 0
		err := client.WithTransactionRetry(ctx, "g", false, 3, func(uint64) error {
			runs++
			_, err := client.Gql(ctx, conflictWrite, nil)
			return err
		})
		if calls, _ := node.counts(); err == nil || IsWriteConflict(err) || runs != 1 || node.begins != 1 || calls != 1 {
			t.Errorf("err = %v, runs = %d, begins = %d, calls = %d, want a plain error and 1, 1, 1", err, runs, node.begins, calls)
		}
	})
}
