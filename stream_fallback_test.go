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

// Review round 1 of the error-code batch, item 6: when Gql answered that a
// result was too large ("use streaming API"), the driver sent the same
// statement again through GqlStream, decided by the text and with no
// read-only check. Against a server older than the 2026-10-01 fix, a write
// whose result passed 10,000 rows was made twice.
//
// The fallback now happens only for a request that is safe to send again:
// read-only (outside a transaction), or refused before it ran.

const rowLimitText = "result set too large, use streaming API (GqlStream)"

// rowLimitNode answers every Gql with err and every GqlStream with two rows.
type rowLimitNode struct {
	pb.UnimplementedSessionServiceServer
	pb.UnimplementedQueryServiceServer
	err         error
	gqlCalls    atomic.Int32
	streamCalls atomic.Int32
}

func (n *rowLimitNode) Login(context.Context, *pb.LoginRequest) (*pb.LoginResponse, error) {
	return &pb.LoginResponse{SessionId: 1, ServerVersion: "test"}, nil
}

func (n *rowLimitNode) Gql(context.Context, *pb.GqlRequest) (*pb.GqlResponse, error) {
	n.gqlCalls.Add(1)
	return nil, n.err
}

func (n *rowLimitNode) GqlStream(_ *pb.GqlRequest, stream pb.QueryService_GqlStreamServer) error {
	n.streamCalls.Add(1)
	for i := 0; i < 2; i++ {
		row := &pb.GqlResponse{Columns: []string{"n"}, Rows: []*pb.Row{{}}}
		if err := stream.Send(row); err != nil {
			return err
		}
	}
	return nil
}

func rowLimitClient(t *testing.T, err error) (*Client, *rowLimitNode) {
	t.Helper()
	lis, lerr := net.Listen("tcp", "127.0.0.1:0")
	if lerr != nil {
		t.Fatal(lerr)
	}
	node := &rowLimitNode{err: err}
	srv := grpc.NewServer()
	pb.RegisterSessionServiceServer(srv, node)
	pb.RegisterQueryServiceServer(srv, node)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	client, cerr := NewClient(NewConfigBuilder().Hosts(lis.Addr().String()).HealthCheckInterval(0).RetryCount(0).Build())
	if cerr != nil {
		t.Fatal(cerr)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client, node
}

func TestStreamFallback_OnlyWhenSafeToSendAgain(t *testing.T) {
	ctx := context.Background()
	oldServer := statusWith(codes.ResourceExhausted, rowLimitText, "")
	newServer := statusWith(codes.ResourceExhausted, rowLimitText, "RESULT_ROW_LIMIT", "code", "0")
	refused := statusWith(codes.ResourceExhausted, rowLimitText, "RESULT_ROW_LIMIT", "code", "0", "executed", "false")
	rateLimit := statusWith(codes.ResourceExhausted, "rate limit exceeded; use streaming API", "RESOURCE_EXHAUSTED", "code", "0", "executed", "false")
	readOnly := &QueryConfig{ReadOnly: true}
	inTx := &QueryConfig{ReadOnly: true, TransactionID: 7}

	cases := []struct {
		name     string
		err      error
		config   *QueryConfig
		fallback bool
	}{
		{"old server, read-only", oldServer, readOnly, true},
		{"old server, not read-only (a write may have run)", oldServer, nil, false},
		{"old server, read-only in a transaction", oldServer, inTx, false},
		{"new server, read-only", newServer, readOnly, true},
		{"new server, not read-only", newServer, nil, false},
		{"refused before it ran", refused, nil, true},
		{"another RESOURCE_EXHAUSTED quoting the words", rateLimit, readOnly, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, node := rowLimitClient(t, c.err)
			resp, err := client.Gql(ctx, "MATCH (n) RETURN n", c.config)
			if got := node.gqlCalls.Load(); got != 1 {
				t.Errorf("Gql calls %d, want 1", got)
			}
			if c.fallback {
				if err != nil || node.streamCalls.Load() != 1 || len(resp.Rows) != 2 {
					t.Errorf("want one GqlStream giving 2 rows; err=%v streams=%d", err, node.streamCalls.Load())
				}
				return
			}
			if err == nil || node.streamCalls.Load() != 0 {
				t.Errorf("want the Gql error and no GqlStream; err=%v streams=%d", err, node.streamCalls.Load())
			}
		})
	}
}
