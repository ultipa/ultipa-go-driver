package gqldb

import (
	"context"
	"testing"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// Review round 2 of the error-code batch, item 7: resend-gate cases the
// review's mutations showed were not pinned by this package's own tests.
//
//   GM1: the stream fallback reads only the server's own ErrorInfo: an
//        executed=false from another domain (a proxy, say) does not let a
//        write be fetched again through GqlStream.
//   GM2: a write in an open transaction that meets LEADER_CHANGED is not
//        moved to the leader: the new leader knows nothing of the transaction.
//   And the driver signs in again on UNAUTHENTICATED alone.

func TestResendGaps_StreamFallbackIgnoresAForeignDomain(t *testing.T) {
	foreign := withForeignDetailReason(codes.ResourceExhausted, "RESULT_ROW_LIMIT",
		map[string]string{"code": "0", "executed": "false"})
	client, node := rowLimitClient(t, foreign)
	if _, err := client.Gql(context.Background(), "INSERT (:Audit {note:'one write'})", nil); err == nil {
		t.Fatal("the Gql error was lost")
	}
	if node.gqlCalls.Load() != 1 || node.streamCalls.Load() != 0 {
		t.Errorf("a foreign-domain executed=false sent the write again: gql %d stream %d", node.gqlCalls.Load(), node.streamCalls.Load())
	}
}

func TestResendGaps_ReloginIgnoresAForeignDomain(t *testing.T) {
	client, node := sessionClient(t, foreignRefusal(codes.Unauthenticated))
	if _, err := client.Gql(context.Background(), "INSERT (:Audit {note:'one write'})", nil); err == nil {
		t.Fatal("the error was lost")
	}
	if n := node.calls.Load(); n != 1 {
		t.Errorf("a foreign-domain executed=false sent the write again: %d calls", n)
	}
}

// txNode is an HA node that also begins transactions.
type txNode struct {
	frNode
	pb.UnimplementedTransactionServiceServer
}

func (n *txNode) Begin(context.Context, *pb.BeginRequest) (*pb.BeginResponse, error) {
	return &pb.BeginResponse{TransactionId: 77}, nil
}

func (n *txNode) Rollback(context.Context, *pb.RollbackRequest) (*pb.RollbackResponse, error) {
	return &pb.RollbackResponse{Success: true}, nil
}

func startTxNode(t *testing.T, n *txNode) string {
	t.Helper()
	return startNodeWith(t, func(srv *grpc.Server) {
		pb.RegisterSessionServiceServer(srv, n)
		pb.RegisterQueryServiceServer(srv, n)
		pb.RegisterHAServiceServer(srv, n)
		pb.RegisterTransactionServiceServer(srv, n)
	})
}

func TestResendGaps_TransactionWriteIsNotMovedToTheLeader(t *testing.T) {
	for name, lc := range leaderChangedAnswers() {
		t.Run(name, func(t *testing.T) {
			f0 := &txNode{frNode: frNode{id: "f0", applied: 100, followerErr: lc}}
			leader := &txNode{frNode: frNode{id: "leader", isLeader: true, applied: 100}}
			client := haClient(t, startTxNode(t, f0), startTxNode(t, leader))
			ctx := context.Background()
			tx, err := client.BeginTransaction(ctx, "g", false, 30)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Gql(ctx, "INSERT (:A)", &QueryConfig{TransactionID: tx.ID}); err == nil {
				t.Fatal("the LEADER_CHANGED error was lost")
			}
			if n := leader.gqlCalls.Load(); n != 0 {
				t.Errorf("a write in an open transaction was moved to the leader (%d calls)", n)
			}
		})
	}
}

// Only UNAUTHENTICATED makes the driver sign in again: a read-only call
// answered UNAVAILABLE, 5024 or another RESOURCE_EXHAUSTED is sent once and
// causes no sign-in.
func TestResendGaps_OnlyUnauthenticatedSignsInAgain(t *testing.T) {
	for name, err := range map[string]error{
		"unavailable":        statusWith(codes.Unavailable, "connection reset by peer", ""),
		"5024":               partlyStoredErr(codes.FailedPrecondition, "[5024] stored", ReasonWritesCommitted, "5024"),
		"resource exhausted": statusWith(codes.ResourceExhausted, "rate limit exceeded", "RESOURCE_EXHAUSTED", "code", "0"),
	} {
		t.Run(name, func(t *testing.T) {
			client, node := sessionClient(t, err)
			if _, gerr := client.Gql(context.Background(), "MATCH (n) RETURN n", &QueryConfig{ReadOnly: true}); gerr == nil {
				t.Fatal("the error was lost")
			}
			if node.calls.Load() != 1 || node.logins.Load() != 1 {
				t.Errorf("calls %d logins %d, want 1 and 1", node.calls.Load(), node.logins.Load())
			}
		})
	}
}

// After signing in again a call is sent again at most once: a server that
// refuses every call before running it gets exactly two.
func TestResendGaps_SentAgainAtMostOnce(t *testing.T) {
	client, node := sessionClient(t, expiredBeforeRun)
	node.always = true
	// Bounded, so a driver that kept sending fails here instead of spinning.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Gql(ctx, "INSERT (:Audit {note:'one write'})", nil); err == nil {
		t.Fatal("the error was lost")
	}
	if node.calls.Load() != 2 || node.logins.Load() != 2 {
		t.Errorf("calls %d logins %d, want 2 and 2", node.calls.Load(), node.logins.Load())
	}
}
