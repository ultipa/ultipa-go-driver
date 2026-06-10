//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// toInt64 coerces a GetByName-returned value (which may be int64, uint64,
// int, float64, ...) into an int64 for count comparisons.
func toInt64(t *testing.T, v interface{}) int64 {
	t.Helper()
	switch n := v.(type) {
	case int64:
		return n
	case uint64:
		return int64(n)
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case uint32:
		return int64(n)
	case float64:
		return int64(n)
	default:
		t.Fatalf("count value has unexpected type %T (%v)", v, v)
		return 0
	}
}

// markerCount runs the Marker count query and returns the count, failing the
// test on any error.
func markerCount(t *testing.T, ctx context.Context, client *gqldb.Client) int64 {
	t.Helper()
	resp, err := client.Gql(ctx, "MATCH (n:Marker) RETURN count(n) AS c", nil)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if resp.RowCount < 1 || len(resp.Rows) < 1 {
		t.Fatalf("count query returned no rows")
	}
	v, err := resp.GetByName(resp.Rows[0], "c")
	if err != nil {
		t.Fatalf("reading count column failed: %v", err)
	}
	return toInt64(t, v)
}

// TestSessionReconnect_RestoresGqlSwitchedGraph pins the session-expiry
// reconnect fix: after the session is invalidated and the driver
// auto-reconnects, it must restore the graph that was active — including one
// switched via Gql("USE GRAPH ...") (not just explicit UseGraph()). The old
// code restored a stale storedGraph and silently ran the retried query against
// the wrong/default graph (empty rows, no error); this test fails under that
// behavior.
func TestSessionReconnect_RestoresGqlSwitchedGraph(t *testing.T) {
	client, err := newReconnectClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	username, password := testUsername, testPassword
	if _, err = client.Login(ctx, username, password); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	g := fmt.Sprintf("test_reconnect_graph_%d", time.Now().UnixNano())
	if err = client.CreateGraph(ctx, g, gqldb.GraphTypeOpen, "reconnect graph restore"); err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer func() {
		_ = client.UseGraph(ctx, "miniCircle")
		_ = client.DropGraph(ctx, g, true)
	}()

	// Distinguishing data lives ONLY in g.
	if _, err = client.Gql(ctx, "INSERT (:Marker {_id:'m1'})", &gqldb.QueryConfig{GraphName: g}); err != nil {
		t.Fatalf("INSERT into g failed: %v", err)
	}

	// Switch the active graph via gql USE GRAPH — the path that used to be
	// dropped on reconnect (storedGraph never tracked it).
	if _, err = client.Gql(ctx, "USE GRAPH "+g, nil); err != nil {
		t.Fatalf("USE GRAPH g failed: %v", err)
	}
	sess := client.GetSession()
	if sess == nil {
		t.Fatalf("expected an active session before logout")
	}
	if sess.DefaultGraph != g {
		t.Fatalf("authoritative active graph should be %q after gql USE GRAPH, got %q", g, sess.DefaultGraph)
	}
	sessionBefore := sess.ID

	// Invalidate the session server-side. The next query must trigger
	// auto-reconnect (re-login + graph restore) then retry.
	if err = client.Logout(ctx); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	count := markerCount(t, ctx, client)

	// Reconnect must actually have fired: a fresh, logged-in session with a
	// different id (guards against the query trivially running on the
	// pre-logout session without exercising reconnect).
	if !client.IsLoggedIn() {
		t.Fatalf("must be re-logged-in after reconnect")
	}
	sessAfter := client.GetSession()
	if sessAfter == nil {
		t.Fatalf("session must exist after reconnect")
	}
	if sessAfter.ID == sessionBefore {
		t.Fatalf("reconnect must have created a NEW session id (still %d)", sessionBefore)
	}

	if count != 1 {
		t.Fatalf("after reconnect the gql-switched graph %q must be restored "+
			"(marker visible, count=1) — not the login/default graph; got count=%d", g, count)
	}
}

// TestSessionReconnect_ConcurrentSingleFlight pins the single-flight reconnect:
// after logout, many concurrent callers each hit UNAUTHENTICATED. They must
// trigger exactly one re-login (no double-relogin / race) and every caller must
// observe graph g restored (count=1).
func TestSessionReconnect_ConcurrentSingleFlight(t *testing.T) {
	client, err := newReconnectClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	username, password := testUsername, testPassword
	if _, err = client.Login(ctx, username, password); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	g := fmt.Sprintf("test_reconnect_concurrent_%d", time.Now().UnixNano())
	if err = client.CreateGraph(ctx, g, gqldb.GraphTypeOpen, "concurrent reconnect"); err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer func() {
		_ = client.UseGraph(ctx, "miniCircle")
		_ = client.DropGraph(ctx, g, true)
	}()

	if _, err = client.Gql(ctx, "INSERT (:Marker {_id:'m1'})", &gqldb.QueryConfig{GraphName: g}); err != nil {
		t.Fatalf("INSERT into g failed: %v", err)
	}
	if _, err = client.Gql(ctx, "USE GRAPH "+g, nil); err != nil {
		t.Fatalf("USE GRAPH g failed: %v", err)
	}

	// Invalidate — the next calls trigger reconnect.
	if err = client.Logout(ctx); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	const n = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	counts := make([]int64, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			resp, qErr := client.Gql(ctx, "MATCH (n:Marker) RETURN count(n) AS c", nil)
			if qErr != nil {
				errs[idx] = qErr
				return
			}
			if len(resp.Rows) < 1 {
				errs[idx] = fmt.Errorf("no rows")
				return
			}
			v, gErr := resp.GetByName(resp.Rows[0], "c")
			if gErr != nil {
				errs[idx] = gErr
				return
			}
			counts[idx] = toInt64(t, v)
		}(i)
	}
	close(start) // release all at once → concurrent session expiry
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("concurrent caller %d errored: %v", i, errs[i])
		}
		if counts[i] != 1 {
			t.Fatalf("concurrent caller %d must see graph g restored (count=1), got %d", i, counts[i])
		}
	}
	if !client.IsLoggedIn() {
		t.Fatalf("client must be logged in after concurrent reconnect")
	}
}
