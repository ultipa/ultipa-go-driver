package gqldb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
)

// Review round 1 of the error-code batch, item 3: in HA, a write that reached
// a follower (LEADER_CHANGED) jumped to the leader; when the leader's answer
// was any other error, the jump "did not stick" and the rotation sent the
// write again, reaching the leader a second time. A write the leader stored
// and answered 5024 was made twice.
//
// LEADER_CHANGED is a refusal before running, so moving the write on stays.
// Any other error ends the call, unless the call is read-only or the error
// says the server refused it before running it (executed=false).

func haClient(t *testing.T, hosts ...string) *Client {
	t.Helper()
	cfg := NewConfigBuilder().Hosts(hosts...).Username("root").Password("root").HealthCheckInterval(0).RetryCount(0).Build()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := client.Login(context.Background(), "root", "root"); err != nil {
		t.Fatal(err)
	}
	return client
}

// The reviewer's reproducer, and the other answers a leader can give a write
// it may have run: each reaches the leader once.
func TestHA_WriteReachesTheLeaderOnce(t *testing.T) {
	ctx := context.Background()
	for name, leaderErr := range map[string]error{
		"5024 stored":       engineError(CodeWritesCommitted),
		"4016 read-only":    engineError(CodeReadOnly),
		"6020 licence":      engineError(CodeLicenseReadOnly),
		"3011 conflict":     engineError(CodeWriteConflict),
		"timeout":           statusWith(codes.DeadlineExceeded, "the request reached its time limit", "DEADLINE_EXCEEDED", "code", "0"),
		"no detail":         statusWith(codes.Internal, "something failed after the write", ""),
		"unavailable after": statusWith(codes.Unavailable, "connection reset by peer", ""),
	} {
		t.Run(name, func(t *testing.T) {
			f0 := &frNode{id: "f0", applied: 100, followerRejects: true}
			f1 := &frNode{id: "f1", applied: 100, followerRejects: true}
			leader := &frNode{id: "leader", isLeader: true, applied: 100, leaderErr: leaderErr}
			client := haClient(t, startFRNode(t, f0), startFRNode(t, f1), startFRNode(t, leader))
			_, err := client.Gql(ctx, "INSERT (:N {x:1})", nil)
			if err == nil {
				t.Fatal("the leader's error was lost")
			}
			if n := leader.gqlCalls.Load(); n != 1 {
				t.Fatalf("the leader received the write %d times; want 1 (err %v)", n, err)
			}
			if EngineCode(leaderErr) != 0 && EngineCode(err) != EngineCode(leaderErr) {
				t.Errorf("error %v, want the leader's engine code %d", err, EngineCode(leaderErr))
			}
			if f1.gqlCalls.Load() != 0 {
				t.Errorf("the write went on to f1 after the leader answered")
			}
		})
	}
}

// A read-only call may be sent again: after the jump its error lets the
// rotation go on, and it lands on the leader again.
func TestHA_ReadOnlyMayGoRoundAgain(t *testing.T) {
	ctx := context.Background()
	f0 := &frNode{id: "f0", applied: 100, followerRejects: true}
	f1 := &frNode{id: "f1", applied: 100, followerRejects: true}
	leader := &frNode{id: "leader", isLeader: true, applied: 100, leaderErrOnce: true,
		leaderErr: statusWith(codes.Unavailable, "connection reset by peer", "")}
	client := haClient(t, startFRNode(t, f0), startFRNode(t, f1), startFRNode(t, leader))
	if _, err := client.Gql(ctx, "MATCH (n) RETURN n", &QueryConfig{ReadOnly: true}); err != nil {
		t.Fatalf("read-only: %v", err)
	}
	if n := leader.gqlCalls.Load(); n != 2 {
		t.Errorf("leader calls %d, want 2", n)
	}
}

// A write the leader refused before running it (executed=false) may be sent
// again too.
func TestHA_RefusedBeforeRunMayGoRoundAgain(t *testing.T) {
	ctx := context.Background()
	f0 := &frNode{id: "f0", applied: 100, followerRejects: true}
	f1 := &frNode{id: "f1", applied: 100, followerRejects: true}
	leader := &frNode{id: "leader", isLeader: true, applied: 100, leaderErrOnce: true,
		leaderErr: statusWith(codes.ResourceExhausted, "rate limit exceeded", "RESOURCE_EXHAUSTED", "code", "0", "executed", "false")}
	client := haClient(t, startFRNode(t, f0), startFRNode(t, f1), startFRNode(t, leader))
	if _, err := client.Gql(ctx, "INSERT (:N)", nil); err != nil {
		t.Fatalf("refused before running: %v", err)
	}
	if n := leader.gqlCalls.Load(); n != 2 {
		t.Errorf("leader calls %d, want 2", n)
	}
}

// With no leader anywhere, the rotation's waits end with the context.
func TestHA_RotationWaitHonoursTheContext(t *testing.T) {
	var hosts []string
	for i := 0; i < 6; i++ {
		hosts = append(hosts, startFRNode(t, &frNode{id: fmt.Sprint("f", i), applied: 100, followerRejects: true}))
	}
	client := haClient(t, hosts...)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := client.Gql(ctx, "INSERT (:N)", nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("a write with no leader succeeded")
	}
	// Six hosts: the waits alone add up to 2.55 s (50 ms doubling, capped at 1 s).
	if elapsed > time.Second {
		t.Errorf("returned after %s; the waits ignored the context's 150 ms", elapsed)
	}
}
