package gqldb

import (
	"context"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Review round 2 of the error-code batch, item 3: a request of several
// statements whose first statement committed and whose second failed (5020
// here) was answered as a plain 5020, and an application that asked again
// stored the first statement twice. The server now marks such an answer, and
// every [5024] answer, with the ErrorInfo metadata partly_stored=true: some or
// all of the request's changes are stored. No path of the driver sends a
// request with that mark again, whatever else the error says: not the 5020
// retry, not the re-login resend, not the stream fallback, not HA.

func partlyStoredErr(c codes.Code, msg, reason, code string, extra ...string) error {
	return statusWith(c, msg, reason, append([]string{"code", code, "partly_stored", "true"}, extra...)...)
}

var (
	stored5020 = partlyStoredErr(codes.FailedPrecondition,
		"[5020] some statements' changes are stored: a statement failed after one or more of the request's statements had committed",
		ReasonFulltextIndexLoading, "5020")
	// The server never sends executed=false with the mark; a driver must
	// still refuse the pair.
	storedButRefused = partlyStoredErr(codes.Unauthenticated, "session expired", "SESSION_EXPIRED", "7023", "executed", "false")
)

func TestPartlyStored(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want bool
	}{
		"marked":                {stored5020, true},
		"marked, wrapped":       {NewError(0, "query failed", stored5020), true},
		"5024 marked":           {partlyStoredErr(codes.FailedPrecondition, "[5024] stored", ReasonWritesCommitted, "5024"), true},
		"5020 without the mark": {engineError(CodeFulltextIndexLoading), false},
		"other value":           {statusWith(codes.FailedPrecondition, "x", "X", "code", "1", "partly_stored", "false"), false},
		"no detail":             {statusWith(codes.FailedPrecondition, "[5020] partly_stored=true", ""), false},
		"foreign domain":        {foreignPartlyStored(), false},
		"nil":                   {nil, false},
	} {
		if got := PartlyStored(c.err); got != c.want {
			t.Errorf("%s: PartlyStored = %v, want %v", name, got, c.want)
		}
	}
}

func foreignPartlyStored() error {
	return withForeignDetail(codes.FailedPrecondition, map[string]string{"code": "5020", "partly_stored": "true"})
}

// The 5020 retry: a read-only request marked partly stored is not asked again.
func TestPartlyStored_No5020Retry(t *testing.T) {
	calls := 0
	readOnly := &QueryConfig{ReadOnly: true}
	err := retryRead(context.Background(), readOnly, 3, time.Millisecond, noSleepRetry, func() bool { return true },
		func() error { calls++; return stored5020 })
	if calls != 1 || err == nil {
		t.Fatalf("a 5020 marked partly stored was sent %d times, want 1 (err %v)", calls, err)
	}
	if mayRetryRead(readOnly, stored5020) {
		t.Error("mayRetryRead allows a 5020 marked partly stored")
	}
}

func noSleepRetry(context.Context, time.Duration) error { return nil }

// The re-login resend: refused before running and read-only both lose to the mark.
func TestPartlyStored_NoReloginResend(t *testing.T) {
	ctx := context.Background()
	for name, c := range map[string]struct {
		err    error
		config *QueryConfig
	}{
		"executed=false with the mark": {storedButRefused, nil},
		"read-only with the mark":      {partlyStoredErr(codes.Unauthenticated, "session expired", "SESSION_EXPIRED", "7023"), &QueryConfig{ReadOnly: true}},
	} {
		t.Run(name, func(t *testing.T) {
			client, node := sessionClient(t, c.err)
			_, err := client.Gql(ctx, "INSERT (:Audit {note:'one write'})", c.config)
			if n := node.calls.Load(); n != 1 || err == nil {
				t.Fatalf("sent %d times, want 1 (err %v)", n, err)
			}
			if node.logins.Load() != 2 {
				t.Errorf("logins %d, want 2: the client still signs in again for the next call", node.logins.Load())
			}
			if !PartlyStored(err) {
				t.Errorf("the returned error lost the mark: %v", err)
			}
		})
	}
}

// The stream fallback: a row-limit answer marked partly stored is not fetched again.
func TestPartlyStored_NoStreamFallback(t *testing.T) {
	for name, err := range map[string]error{
		"read-only":                   partlyStoredErr(codes.ResourceExhausted, rowLimitText, "RESULT_ROW_LIMIT", "0"),
		"executed=false and the mark": partlyStoredErr(codes.ResourceExhausted, rowLimitText, "RESULT_ROW_LIMIT", "0", "executed", "false"),
	} {
		t.Run(name, func(t *testing.T) {
			client, node := rowLimitClient(t, err)
			_, gerr := client.Gql(context.Background(), "MATCH (n) RETURN n", &QueryConfig{ReadOnly: true})
			if node.gqlCalls.Load() != 1 || node.streamCalls.Load() != 0 || gerr == nil {
				t.Fatalf("gql %d stream %d err %v; want 1, 0 and the error", node.gqlCalls.Load(), node.streamCalls.Load(), gerr)
			}
		})
	}
}

// HA: neither the follower-read fallback nor the rotation sends a marked
// answer to another server, even for a read-only call.
func TestPartlyStored_NotSentToAnotherServer(t *testing.T) {
	ctx := context.Background()
	marked := partlyStoredErr(codes.Unavailable, "connection reset by peer", "UNAVAILABLE", "0")

	leader := &frNode{id: "leader", isLeader: true, applied: 100}
	follower := &frNode{id: "follower", applied: 100, followerErr: marked}
	client := frClient(t, startFRNode(t, leader), startFRNode(t, follower))
	if _, err := client.Gql(ctx, "MATCH (n) RETURN n", &QueryConfig{ReadPreference: ReadPreferenceFollower, ReadOnly: true}); err == nil {
		t.Error("follower read: the marked error was lost")
	}
	if leader.gqlCalls.Load() != 0 {
		t.Errorf("follower read: a marked answer went on to the leader (%d calls)", leader.gqlCalls.Load())
	}

	// Even a LEADER_CHANGED answer loses to the mark (the server never sends
	// the two together; the driver must still not move such a write).
	leader2 := &frNode{id: "leader", isLeader: true, applied: 100}
	follower2 := &frNode{id: "follower", applied: 100, followerErr: partlyStoredErr(codes.FailedPrecondition,
		"LEADER_CHANGED", ReasonLeaderChanged, "0", "executed", "false")}
	client2 := frClient(t, startFRNode(t, leader2), startFRNode(t, follower2))
	if _, err := client2.Gql(ctx, "INSERT (:A)", &QueryConfig{ReadPreference: ReadPreferenceFollower}); err == nil {
		t.Error("follower write: the marked LEADER_CHANGED was lost")
	}
	if leader2.gqlCalls.Load() != 0 {
		t.Errorf("follower write: a marked LEADER_CHANGED went on to the leader (%d calls)", leader2.gqlCalls.Load())
	}

	f0 := &frNode{id: "f0", applied: 100, followerRejects: true}
	f1 := &frNode{id: "f1", applied: 100, followerRejects: true}
	haLeader := &frNode{id: "leader", isLeader: true, applied: 100, leaderErrOnce: true, leaderErr: marked}
	ha := haClient(t, startFRNode(t, f0), startFRNode(t, f1), startFRNode(t, haLeader))
	if _, err := ha.Gql(ctx, "MATCH (n) RETURN n", &QueryConfig{ReadOnly: true}); err == nil {
		t.Error("HA: the marked error was lost")
	}
	if n := haLeader.gqlCalls.Load(); n != 1 {
		t.Errorf("HA: the leader received a marked read-only call %d times, want 1", n)
	}
}

// withForeignDetail is a status whose ErrorInfo comes from another domain.
func withForeignDetail(c codes.Code, md map[string]string) error {
	return withForeignDetailReason(c, "X", md)
}

// withForeignDetailReason is withForeignDetail with the given reason.
func withForeignDetailReason(c codes.Code, reason string, md map[string]string) error {
	st, err := status.New(c, "from a proxy").WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: "proxy.example.com", Metadata: md})
	if err != nil {
		panic(err)
	}
	return st.Err()
}
