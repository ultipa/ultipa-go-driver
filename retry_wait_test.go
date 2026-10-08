package gqldb

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Review round 1 of the error-code batch, item 7: two mutations of the 5020
// retry survived every test: ignoring an error from the wait (the context
// ended during it), and weakening the check that stops before a wait that
// would pass the context's deadline. These pin both, through the client.

func TestRetryRead_AWaitCutShortEndsTheCall(t *testing.T) {
	loading := engineError(CodeFulltextIndexLoading)
	client, node, _ := scriptClient(t, 3, loading)
	client.retrySleep = func(context.Context, time.Duration) error { return context.Canceled }
	_, err := client.Gql(context.Background(), "MATCH (n) RETURN n", &QueryConfig{ReadOnly: true})
	if EngineCode(err) != CodeFulltextIndexLoading {
		t.Errorf("error %v, want the 5020 error", err)
	}
	if got := node.calls.Load(); got != 1 {
		t.Errorf("%d calls after the wait was cut short, want 1", got)
	}
}

func TestRetryRead_NoWaitPastTheContextsDeadline(t *testing.T) {
	loading := engineError(CodeFulltextIndexLoading)
	client, node, waits := scriptClient(t, 3, loading)
	// The waits are recorded, not slept: 100 ms fits in the 150 ms left, the
	// next one, 200 ms, does not.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := client.Gql(ctx, "MATCH (n) RETURN n", &QueryConfig{ReadOnly: true})
	if EngineCode(err) != CodeFulltextIndexLoading {
		t.Errorf("error %v, want the 5020 error", err)
	}
	if got := node.calls.Load(); got != 2 {
		t.Errorf("%d calls, want 2", got)
	}
	if fmt.Sprint(*waits) != "[100ms]" {
		t.Errorf("waits %v, want [100ms]", *waits)
	}
}
