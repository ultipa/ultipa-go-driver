package gqldb

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// ListTransactions signs in again, like every call around it.
//
// Found while running the 6.2.147 release gate: a client left idle past the
// server's sign-in TTL answered the listing with
//
//	list transactions failed: sign-in session expired (7023): sign in again
//
// while every neighbouring call -- Gql, BeginTransaction, GetBulkImportStatus --
// signed in again and answered. It was the one transaction call that never went
// through withAutoReconnect.
//
// A listing reads and writes nothing and carries no transaction id, so sending
// it again is safe whatever the server says about it; the rule marks it
// read-only for that reason, which is what lets the resend happen on a server
// older than the executed=false mark too. Pinned here: with the mark and
// without it the listing answers and the driver signed in once; the resend is a
// resend (the server saw two listings); a sign-in that fails leaves the original
// error rather than an empty list, which would read as "no transactions".

// listNode answers the first ListTransactions with err and later ones with one
// transaction whose SessionID is the sign-in that answered, so a test can tell
// which sign-in the second attempt used. loginErr fails every login but the
// first.
type listNode struct {
	pb.UnimplementedSessionServiceServer
	pb.UnimplementedTransactionServiceServer
	err      error
	loginErr error
	logins   atomic.Int32
	lists    atomic.Int32
}

func (n *listNode) Login(context.Context, *pb.LoginRequest) (*pb.LoginResponse, error) {
	if n.logins.Add(1) > 1 && n.loginErr != nil {
		return nil, n.loginErr
	}
	return &pb.LoginResponse{SessionId: uint64(n.logins.Load()), ServerVersion: "test"}, nil
}

func (n *listNode) Logout(context.Context, *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	return &pb.LogoutResponse{}, nil
}

func (n *listNode) ListTransactions(context.Context, *pb.ListTransactionsRequest) (*pb.ListTransactionsResponse, error) {
	if n.lists.Add(1) == 1 && n.err != nil {
		return nil, n.err
	}
	return &pb.ListTransactionsResponse{Transactions: []*pb.TransactionInfo{
		{TransactionId: 7, SessionId: uint64(n.logins.Load()), GraphName: "g"},
	}}, nil
}

func listClient(t *testing.T, n *listNode) *Client {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterSessionServiceServer(srv, n)
	pb.RegisterTransactionServiceServer(srv, n)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	cfg := NewConfigBuilder().Hosts(lis.Addr().String()).HealthCheckInterval(0).RetryCount(0).Build()
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

func TestListTransactions_ExpiredSignInIsAnswered(t *testing.T) {
	for name, err := range map[string]error{
		"the server marks it executed=false":        expiredBeforeRun,
		"a server older than the mark says nothing": statusWith(codes.Unauthenticated, "sign-in session expired (7023): sign in again", ""),
		"UNAUTHENTICATED with an unrelated message": reproducerNoDetail,
	} {
		t.Run(name, func(t *testing.T) {
			node := &listNode{err: err}
			client := listClient(t, node)
			txs, lerr := client.ListTransactions(context.Background())
			if lerr != nil {
				t.Fatalf("the listing still fails after the sign-in lapsed: %v", lerr)
			}
			if len(txs) != 1 || txs[0].TransactionID != 7 {
				t.Errorf("got %+v, want the one transaction the server holds", txs)
			}
			if got := node.logins.Load(); got != 2 {
				t.Errorf("signed in %d times, want exactly 2 (once at the start, once again)", got)
			}
			if got := node.lists.Load(); got != 2 {
				t.Errorf("the server saw %d listings, want 2 -- the resend must be a real resend", got)
			}
			if txs[0].SessionID != 2 {
				t.Errorf("the answer came from session %d, want the new sign-in's 2", txs[0].SessionID)
			}
		})
	}
}

func TestListTransactions_AFailedSignInLeavesTheOriginalError(t *testing.T) {
	node := &listNode{err: expiredBeforeRun,
		loginErr: statusWith(codes.Unavailable, "the host is down", "")}
	client := listClient(t, node)
	if _, err := client.ListTransactions(context.Background()); err == nil {
		t.Fatal("an empty list was returned where the server's refusal belongs")
	}
	if got := node.lists.Load(); got != 1 {
		t.Errorf("the server saw %d listings, want 1 -- never sent again when the sign-in did not work", got)
	}
}

func TestListTransactions_AnUnrelatedFailureIsNotRetried(t *testing.T) {
	node := &listNode{err: statusWith(codes.PermissionDenied, "[5017] not allowed", "")}
	client := listClient(t, node)
	if _, err := client.ListTransactions(context.Background()); err == nil {
		t.Fatal("the error was lost")
	}
	if node.logins.Load() != 1 || node.lists.Load() != 1 {
		t.Errorf("logins %d lists %d, want 1 and 1: only UNAUTHENTICATED signs in again",
			node.logins.Load(), node.lists.Load())
	}
}

func TestListTransactions_AWorkingListingIsSentOnce(t *testing.T) {
	node := &listNode{}
	client := listClient(t, node)
	txs, err := client.ListTransactions(context.Background())
	if err != nil || len(txs) != 1 {
		t.Fatalf("txs %+v err %v", txs, err)
	}
	if node.logins.Load() != 1 || node.lists.Load() != 1 {
		t.Errorf("logins %d lists %d, want 1 and 1: the wrapper must not repeat a call that worked",
			node.logins.Load(), node.lists.Load())
	}
}
