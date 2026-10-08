package gqldb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"testing"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Review round 2 of the error-code batch, item 2: a stream that had handed a
// row to the callback and then met LEADER_CHANGED was sent again to the
// leader, so the caller saw its first rows twice (and the leader ran the
// request a second time). A stream is never sent again once a row reached the
// caller, on any path: the LEADER_CHANGED branch, the jump straight to the
// leader, and the rotation. The error is returned instead.

// haStreamNode is an HA node whose GqlStream sends rows chunks, then fails
// with err (nil: it ends there).
type haStreamNode struct {
	pb.UnimplementedSessionServiceServer
	pb.UnimplementedQueryServiceServer
	pb.UnimplementedHAServiceServer
	id       string
	isLeader bool
	noHA     bool // GetStatus is unimplemented: no jump, rotation only
	rows     int
	err      error
	streams  atomic.Int32
}

func (n *haStreamNode) Login(context.Context, *pb.LoginRequest) (*pb.LoginResponse, error) {
	return &pb.LoginResponse{SessionId: 1, ServerVersion: "test"}, nil
}

func (n *haStreamNode) GqlStream(_ *pb.GqlRequest, s pb.QueryService_GqlStreamServer) error {
	n.streams.Add(1)
	for i := 0; i < n.rows; i++ {
		if err := s.Send(&pb.GqlResponse{Columns: []string{"x"}, HasMore: true}); err != nil {
			return err
		}
	}
	if n.err == nil {
		return s.Send(&pb.GqlResponse{Columns: []string{"x"}})
	}
	return n.err
}

func (n *haStreamNode) GetStatus(context.Context, *pb.HAGetStatusRequest) (*pb.HAGetStatusResponse, error) {
	if n.noHA {
		return nil, status.Error(codes.Unimplemented, "no HA")
	}
	return &pb.HAGetStatusResponse{Status: &pb.HAStatus{Enabled: true, NodeId: n.id, IsLeader: n.isLeader, AppliedIndex: 100}}, nil
}

func startHAStreamNode(t *testing.T, n *haStreamNode) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterSessionServiceServer(srv, n)
	pb.RegisterQueryServiceServer(srv, n)
	pb.RegisterHAServiceServer(srv, n)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func leaderChangedAnswers() map[string]error {
	return map[string]error{
		"detail":     leaderChangedWithDetail(),
		"old marker": statusWith(codes.FailedPrecondition, "LEADER_CHANGED leader=raft://leader:7000", ""),
	}
}

// The reviewer's reproducer: the first host streams one row, then answers
// LEADER_CHANGED. The leader must not get the stream.
func TestHAStream_LeaderChangedAfterARowIsNotSentAgain(t *testing.T) {
	ctx := context.Background()
	for name, lc := range leaderChangedAnswers() {
		for _, ro := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/read-only=%v", name, ro), func(t *testing.T) {
				f0 := &haStreamNode{id: "f0", rows: 1, err: lc}
				leader := &haStreamNode{id: "leader", isLeader: true}
				client := haClient(t, startHAStreamNode(t, f0), startHAStreamNode(t, leader))
				rows := 0
				err := client.GqlStream(ctx, "MATCH (n) RETURN n; INSERT (:A)", &QueryConfig{ReadOnly: ro},
					func(*Response) error { rows++; return nil })
				if n := leader.streams.Load(); n != 0 {
					t.Fatalf("the stream was sent again to the leader after a row reached the caller (leader streams %d, rows %d)", n, rows)
				}
				if rows != 1 || !isLeaderChangedError(err) {
					t.Errorf("rows %d (want 1), err %v (want the LEADER_CHANGED answer)", rows, err)
				}
			})
		}
	}
}

// Control: LEADER_CHANGED before any row moves the stream to the leader.
func TestHAStream_LeaderChangedBeforeARowGoesToTheLeader(t *testing.T) {
	f0 := &haStreamNode{id: "f0", err: leaderChangedWithDetail()}
	leader := &haStreamNode{id: "leader", isLeader: true}
	client := haClient(t, startHAStreamNode(t, f0), startHAStreamNode(t, leader))
	rows := 0
	if err := client.GqlStream(context.Background(), "INSERT (:A) RETURN 1", nil, func(*Response) error { rows++; return nil }); err != nil {
		t.Fatalf("stream: %v", err)
	}
	if leader.streams.Load() != 1 || rows != 1 {
		t.Errorf("leader streams %d, rows %d; want 1 and 1", leader.streams.Load(), rows)
	}
}

// A callback that returns an error wrapping a LEADER_CHANGED status (from a
// call it made itself) after it got a row: the stream is not sent again.
func TestHAStream_CallbackLeaderChangedIsNotSentAgain(t *testing.T) {
	f0 := &haStreamNode{id: "f0", rows: 2}
	leader := &haStreamNode{id: "leader", isLeader: true}
	client := haClient(t, startHAStreamNode(t, f0), startHAStreamNode(t, leader))
	nested := fmt.Errorf("nested call: %w", leaderChangedWithDetail())
	err := client.GqlStream(context.Background(), "MATCH (n) RETURN n", &QueryConfig{ReadOnly: true},
		func(*Response) error { return nested })
	if n := f0.streams.Load() + leader.streams.Load(); n != 1 {
		t.Fatalf("the stream was sent %d times after its callback failed, want 1", n)
	}
	if !errors.Is(err, nested) && !isLeaderChangedError(err) {
		t.Errorf("err %v, want the callback's error", err)
	}
}

// After the jump straight to the leader, a leader that streamed a row and
// then answered LEADER_CHANGED itself (it lost the leadership) ends the call.
func TestHAStream_JumpTargetAfterARowIsNotSentOn(t *testing.T) {
	f0 := &haStreamNode{id: "f0", err: leaderChangedWithDetail()}
	leader := &haStreamNode{id: "leader", isLeader: true, rows: 1, err: leaderChangedWithDetail()}
	f2 := &haStreamNode{id: "f2", err: leaderChangedWithDetail()}
	client := haClient(t, startHAStreamNode(t, f0), startHAStreamNode(t, leader), startHAStreamNode(t, f2))
	rows := 0
	err := client.GqlStream(context.Background(), "MATCH (n) RETURN n", &QueryConfig{ReadOnly: true},
		func(*Response) error { rows++; return nil })
	if err == nil {
		t.Fatal("the leader's error was lost")
	}
	if total := f0.streams.Load() + leader.streams.Load() + f2.streams.Load(); total != 2 || rows != 1 {
		t.Errorf("streams f0 %d leader %d f2 %d, rows %d; want 1, 1, 0 and 1 row",
			f0.streams.Load(), leader.streams.Load(), f2.streams.Load(), rows)
	}
}

// In the rotation (no HA status to jump with), a candidate that streamed a
// row and then answered LEADER_CHANGED ends the call.
func TestHAStream_RotationCandidateAfterARowIsNotSentOn(t *testing.T) {
	f0 := &haStreamNode{id: "f0", noHA: true, err: leaderChangedWithDetail()}
	f1 := &haStreamNode{id: "f1", noHA: true, rows: 1, err: leaderChangedWithDetail()}
	leader := &haStreamNode{id: "leader", noHA: true, isLeader: true}
	client := haClient(t, startHAStreamNode(t, f0), startHAStreamNode(t, f1), startHAStreamNode(t, leader))
	rows := 0
	err := client.GqlStream(context.Background(), "MATCH (n) RETURN n", &QueryConfig{ReadOnly: true},
		func(*Response) error { rows++; return nil })
	if err == nil {
		t.Fatal("the candidate's error was lost")
	}
	if n := leader.streams.Load(); n != 0 || rows != 1 {
		t.Errorf("the rotation went on after a row reached the caller: leader streams %d, rows %d", n, rows)
	}
}
