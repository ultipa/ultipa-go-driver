//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// The drivers' follow-up to QA round 33 (oct 5), against a real server.
//
// At LSQB SF10 an End took 64 s and the client's 60 s deadline failed it while
// the load succeeded. These tests need a server that holds an End on a graph
// named slow* for a few seconds and slows each discard batch: a test build with
// the engine's own test seams switched on (DRVOCT5_SLOW_END=4s,
// DRVOCT5_DISCARD_BATCH=50, DRVOCT5_DISCARD_DELAY=200ms); set
// GQLDB_SLOW_BULK=1 to run them. The sign-in tests need a server whose
// sign-ins last 3 s (-session-ttl 3s) at GQLDB_TTL_HOST.

func needSlowBulkServer(t *testing.T) {
	t.Helper()
	if testClient == nil {
		t.Skip("no server with sign-in at GQLDB_HOST")
	}
	if os.Getenv("GQLDB_SLOW_BULK") == "" {
		t.Skip("needs a server that holds End and slows a discard (test-hook build); set GQLDB_SLOW_BULK=1")
	}
}

// computeGraph creates an open graph with compute on (End hands its load to
// compute, where the test server holds it) and drops it at the end.
func computeGraph(t *testing.T, ctx context.Context, client *gqldb.Client, name string) {
	t.Helper()
	if err := client.CreateGraph(ctx, name, gqldb.GraphTypeOpen, "bulk wait test"); err != nil {
		t.Fatalf("CreateGraph %s: %v", name, err)
	}
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = client.DropGraph(cctx, name, true)
	})
	if _, err := client.Gql(ctx, "ALTER GRAPH "+name+" SET COMPUTE ENABLED", &gqldb.QueryConfig{GraphName: name}); err != nil {
		t.Fatalf("enable compute on %s: %v", name, err)
	}
	time.Sleep(2 * time.Second)
}

// loadSession starts a bulk import session on graph and writes nodes and edges
// through it.
func loadSession(t *testing.T, ctx context.Context, client *gqldb.Client, graph string, nodes, edges int) string {
	t.Helper()
	session, err := client.StartBulkImport(ctx, graph, nil)
	if err != nil {
		t.Fatalf("StartBulkImport: %v", err)
	}
	ns := make([]*gqldb.NodeData, nodes)
	for i := range ns {
		ns[i] = &gqldb.NodeData{ID: fmt.Sprintf("n%d", i), Labels: []string{"V"}}
	}
	if _, err := client.InsertNodes(ctx, graph, ns, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID}); err != nil {
		t.Fatalf("InsertNodes: %v", err)
	}
	if edges > 0 {
		es := make([]*gqldb.EdgeData, edges)
		for i := range es {
			es[i] = &gqldb.EdgeData{Label: "E", FromNodeID: fmt.Sprintf("n%d", i%nodes), ToNodeID: fmt.Sprintf("n%d", (i*7+3)%nodes)}
		}
		if _, err := client.InsertEdges(ctx, graph, es, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID}); err != nil {
			t.Fatalf("InsertEdges: %v", err)
		}
	}
	return session.SessionID
}

// serverReportsStates reports whether the server names a bulk import
// session's state (gqldb-grpc push 10 and later). An older server (push 8)
// answers End and Abort in one blocking request.
func serverReportsStates(t *testing.T, client *gqldb.Client, sid string) bool {
	t.Helper()
	st, err := client.GetBulkImportStatus(context.Background(), sid)
	if err != nil {
		t.Fatalf("GetBulkImportStatus: %v", err)
	}
	return st.State != gqldb.BulkImportStateUnspecified
}

// requireStates skips a test of the wait that bulk import states make
// possible when the server is older than them, after aborting the test's
// session: on such a server (push 8) an open session holds one of the
// server's 20 pooled connections until End, Abort or the 30-minute idle
// cleanup, and sessions that skipped tests left open wedged it.
func requireStates(t *testing.T, client *gqldb.Client, sid string) {
	t.Helper()
	skipWithoutStates(t, client, sid, serverReportsStates(t, client, sid))
}

// skipWithoutStates is requireStates with the server's answer given.
func skipWithoutStates(t *testing.T, client *gqldb.Client, sid string, states bool) {
	t.Helper()
	if states {
		return
	}
	// Abort is quick on every server, and a skipped test needs none of the
	// session's data. Its own bound: Abort has no deadline on push 8.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := client.AbortBulkImport(ctx, sid); err != nil {
		t.Logf("AbortBulkImport %s before skipping: %v", sid, err)
	}
	t.Skip("the server reports no bulk import states (older than gqldb-grpc push 10, such as push 8): " +
		"End is one blocking request there; TestBulkWait_OlderServerEndIgnoresTheTotalWait covers it")
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano()%1_000_000_000)
}

// End held 4 s, one request lasting at most 1 s: the server answers each "still
// running" (ENDING) before the deadline, the driver reports it and sends End
// again, and the call returns ENDED. Before: one End with no deadline.
func TestBulkWait_EndReportsProgressUntilEnded(t *testing.T) {
	needSlowBulkServer(t)
	ctx := context.Background()
	graph := uniqueName("slow_go_end")
	computeGraph(t, ctx, testClient, graph)
	sid := loadSession(t, ctx, testClient, graph, 500, 1000)
	requireStates(t, testClient, sid)

	var reports []gqldb.BulkImportProgress
	res, err := testClient.EndBulkImport(ctx, sid, gqldb.BulkImportWaitOptions{
		ProgressInterval: time.Second,
		OnProgress:       func(p gqldb.BulkImportProgress) { reports = append(reports, p) },
	})
	if err != nil {
		t.Fatalf("EndBulkImport: %v", err)
	}
	if !res.Success || res.State != gqldb.BulkImportStateEnded || res.TotalRecords != 1500 {
		t.Errorf("result %+v, want success, ENDED, 1500 records", res)
	}
	if len(reports) < 2 || res.Attempts < 3 {
		t.Errorf("%d progress reports over %d attempts, want at least 2 over 3", len(reports), res.Attempts)
	}
	for _, p := range reports {
		if p.State != gqldb.BulkImportStateEnding || !p.InProgress || p.SessionID != sid {
			t.Errorf("report %+v, want ENDING in progress", p)
		}
	}
	t.Logf("End: %d attempts in %v, %d reports", res.Attempts, res.Waited, len(reports))
}

// Without the early answer each request runs to its deadline; the driver reads
// the status (ENDING) and sends End again.
func TestBulkWait_EndAfterDeadlineWithoutEarlyAnswer(t *testing.T) {
	needSlowBulkServer(t)
	ctx := context.Background()
	graph := uniqueName("slow_go_dl")
	computeGraph(t, ctx, testClient, graph)
	sid := loadSession(t, ctx, testClient, graph, 500, 1000)
	requireStates(t, testClient, sid)

	var reports []gqldb.BulkImportProgress
	res, err := testClient.EndBulkImport(ctx, sid, gqldb.BulkImportWaitOptions{
		ProgressInterval: time.Second, DisableEarlyAnswer: true,
		OnProgress: func(p gqldb.BulkImportProgress) { reports = append(reports, p) },
	})
	if err != nil {
		t.Fatalf("EndBulkImport: %v", err)
	}
	if !res.Success || res.State != gqldb.BulkImportStateEnded || res.Attempts < 2 {
		t.Errorf("result %+v, want success, ENDED after at least 2 attempts", res)
	}
	if len(reports) == 0 || reports[0].State != gqldb.BulkImportStateEnding {
		t.Errorf("reports %+v, want the status's ENDING", reports)
	}
}

// The total wait passes while the End runs: the error says it is still running
// and has not failed; End called again returns its outcome.
func TestBulkWait_EndTotalWaitThenCalledAgain(t *testing.T) {
	needSlowBulkServer(t)
	ctx := context.Background()
	graph := uniqueName("slow_go_total")
	computeGraph(t, ctx, testClient, graph)
	sid := loadSession(t, ctx, testClient, graph, 500, 1000)
	requireStates(t, testClient, sid)

	_, err := testClient.EndBulkImport(ctx, sid, gqldb.BulkImportWaitOptions{
		TotalTimeout: 1500 * time.Millisecond, ProgressInterval: time.Second,
	})
	var ip *gqldb.BulkImportInProgressError
	if !errors.As(err, &ip) || ip.State != gqldb.BulkImportStateEnding || !strings.Contains(err.Error(), "has not failed") {
		t.Fatalf("err %v (%T), want *BulkImportInProgressError in ENDING", err, err)
	}
	res, err := testClient.EndBulkImport(ctx, sid)
	if err != nil {
		t.Fatalf("EndBulkImport again: %v", err)
	}
	if !res.Success || res.State != gqldb.BulkImportStateEnded {
		t.Errorf("End again: %+v, want success ENDED", res)
	}
	st, err := testClient.GetBulkImportStatus(ctx, sid)
	if err != nil || st.IsActive || st.State != gqldb.BulkImportStateEnded {
		t.Errorf("status %+v, %v; want inactive ENDED", st, err)
	}
}

// Abort discards what the session wrote; the discard (slowed) reports its
// progress and the result says what it removed. Before: Abort returned
// "Bulk import aborted successfully" whatever the server said.
func TestBulkWait_AbortDiscardsWithProgress(t *testing.T) {
	needSlowBulkServer(t)
	ctx := context.Background()
	graph := uniqueName("test_go_abort")
	if err := testClient.CreateGraph(ctx, graph, gqldb.GraphTypeOpen, "abort"); err != nil {
		t.Fatal(err)
	}
	defer dropTestGraph(graph)
	sid := loadSession(t, ctx, testClient, graph, 300, 300)
	requireStates(t, testClient, sid)

	var reports []gqldb.BulkImportProgress
	res, err := testClient.AbortBulkImport(ctx, sid, gqldb.BulkImportWaitOptions{
		ProgressInterval: time.Second,
		OnProgress:       func(p gqldb.BulkImportProgress) { reports = append(reports, p) },
	})
	if err != nil {
		t.Fatalf("AbortBulkImport: %v", err)
	}
	if !res.Success || res.State != gqldb.BulkImportStateAborted || res.NodesRemoved != 300 ||
		!strings.Contains(res.Message, "discarded 300 nodes") {
		t.Errorf("result %+v, want ABORTED, 300 nodes removed", res)
	}
	if len(reports) == 0 || reports[0].State != gqldb.BulkImportStateDiscarding || reports[0].Operation != "abort" {
		t.Errorf("reports %+v, want DISCARDING", reports)
	}
	resp, err := testClient.Gql(ctx, "MATCH (n) RETURN count(n) AS cnt", &gqldb.QueryConfig{GraphName: graph})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := resp.SingleInt(); err != nil || n != 0 {
		t.Errorf("%d nodes left after the abort (%v), want 0", n, err)
	}
}

// Item 5: a second session on a graph is refused; no session (and no other
// session's id) is returned.
func TestBulkWait_StartOnABusyGraphIsRefused(t *testing.T) {
	if testClient == nil {
		t.Skip("no server with sign-in at GQLDB_HOST")
	}
	ctx := context.Background()
	graph := uniqueName("test_go_busy")
	if err := testClient.CreateGraph(ctx, graph, gqldb.GraphTypeOpen, "busy"); err != nil {
		t.Fatal(err)
	}
	defer dropTestGraph(graph)
	first, err := testClient.StartBulkImport(ctx, graph, nil)
	if err != nil {
		t.Fatalf("first StartBulkImport: %v", err)
	}
	second, err := testClient.StartBulkImport(ctx, graph, nil)
	if err == nil || second != nil {
		t.Fatalf("second StartBulkImport returned %+v, %v; want an error and no session", second, err)
	}
	if !strings.Contains(err.Error(), "already active") {
		t.Errorf("the refusal does not carry the server's reason: %v", err)
	}
	// From the session-binding server fix on, the refusal carries 5025
	// GRAPH_BUSY; an older server answers it with no code.
	if code := gqldb.EngineCode(err); code != 0 && (code != gqldb.CodeGraphBusy || gqldb.ErrorReason(err) != gqldb.ReasonGraphBusy) {
		t.Errorf("the refusal carries code %d reason %q, want 5025 GRAPH_BUSY", code, gqldb.ErrorReason(err))
	}
	t.Logf("busy Start: code %d reason %q", gqldb.EngineCode(err), gqldb.ErrorReason(err))
	if _, err := testClient.AbortBulkImport(ctx, first.SessionID); err != nil {
		t.Errorf("AbortBulkImport: %v", err)
	}
}

// ttlClient signs in to the server whose sign-ins last 3 s.
func ttlClient(t *testing.T) *gqldb.Client {
	t.Helper()
	host := os.Getenv("GQLDB_TTL_HOST")
	if host == "" {
		t.Skip("needs a server whose sign-ins last 3 s (-session-ttl 3s); set GQLDB_TTL_HOST")
	}
	client, err := gqldb.NewClient(gqldb.NewConfigBuilder().Hosts(host).Username(testUsername).Password(testPassword).
		Timeout(30 * time.Second).HealthCheckInterval(0).Build())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := client.Login(ctx, testUsername, testPassword); err != nil {
		t.Fatalf("Login: %v", err)
	}
	return client
}

// Item 2: Abort after the sign-in expired signs in again and discards. Before,
// Abort was not wrapped and failed with the sign-in refusal.
func TestBulkWait_AbortAfterSignInExpired(t *testing.T) {
	client := ttlClient(t)
	ctx := context.Background()
	graph := uniqueName("test_go_ttl_abort")
	if err := client.CreateGraph(ctx, graph, gqldb.GraphTypeOpen, "ttl"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DropGraph(context.Background(), graph, true) })
	sid := loadSession(t, ctx, client, graph, 20, 0)
	requireStates(t, client, sid)
	time.Sleep(4 * time.Second)

	res, err := client.AbortBulkImport(ctx, sid)
	if err != nil {
		t.Fatalf("AbortBulkImport after the sign-in expired: %v", err)
	}
	if !res.Success || res.State != gqldb.BulkImportStateAborted || res.NodesRemoved != 20 {
		t.Errorf("result %+v, want ABORTED with 20 nodes removed", res)
	}
}

// Item 6: a transaction begun before the sign-in expired is refused after the
// driver signs in again; the driver says so, sends nothing more for it, and
// begins no transaction by itself.
func TestTransaction_AfterSignInExpired(t *testing.T) {
	client := ttlClient(t)
	ctx := context.Background()
	graph := uniqueName("test_go_ttl_tx")
	if err := client.CreateGraph(ctx, graph, gqldb.GraphTypeOpen, "ttl"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DropGraph(context.Background(), graph, true) })

	tx, err := client.BeginTransaction(ctx, graph, false, 60)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Gql(ctx, "INSERT (:A {v: 1})", &gqldb.QueryConfig{GraphName: graph, TransactionID: tx.ID}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(4 * time.Second)

	_, err = client.Gql(ctx, "INSERT (:A {v: 2})", &gqldb.QueryConfig{GraphName: graph, TransactionID: tx.ID})
	var tse *gqldb.TransactionSignInExpiredError
	if !errors.As(err, &tse) || tse.TransactionID != tx.ID {
		t.Fatalf("statement after the sign-in expired: %v (%T), want *TransactionSignInExpiredError", err, err)
	}
	t.Logf("statement: %v", err)
	if _, err := client.Commit(ctx, tx.ID); !errors.As(err, &tse) {
		t.Errorf("Commit: %v, want *TransactionSignInExpiredError", err)
	}
	if !tx.IsSignInLost() || tx.IsActive() {
		t.Errorf("tx signInLost=%v active=%v", tx.IsSignInLost(), tx.IsActive())
	}

	// The new sign-in has no transaction: the driver began none.
	txs, err := client.ListTransactions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 0 {
		t.Errorf("the new sign-in holds %d transactions, want 0", len(txs))
	}
	// Nothing of the transaction is stored or seen, from this client or another.
	other := ttlClient(t)
	countA := func(c *gqldb.Client) int64 {
		t.Helper()
		resp, err := c.Gql(ctx, "MATCH (n:A) RETURN count(n) AS cnt", &gqldb.QueryConfig{GraphName: graph})
		if err != nil {
			t.Fatalf("count: %v", err)
		}
		n, err := resp.SingleInt()
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := countA(client); n != 0 {
		t.Errorf("this client sees %d nodes of the dead transaction, want 0", n)
	}
	// A plain write afterwards is stored: it does not join the dead
	// transaction (before, it reported success and was never stored).
	if _, err := client.Gql(ctx, "INSERT (:A {v: 3})", &gqldb.QueryConfig{GraphName: graph}); err != nil {
		t.Fatalf("a plain write after signing in again: %v", err)
	}
	if n := countA(other); n != 1 {
		t.Errorf("another client sees %d nodes after a plain write, want 1", n)
	}
	// A new transaction can be begun and committed (before: [3010] the session
	// already has an active transaction).
	tx2, err := client.BeginTransaction(ctx, graph, false, 60)
	if err != nil {
		t.Fatalf("BeginTransaction after signing in again: %v", err)
	}
	if _, err := client.Gql(ctx, "INSERT (:A {v: 4})", &gqldb.QueryConfig{GraphName: graph, TransactionID: tx2.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Commit(ctx, tx2.ID); err != nil {
		t.Fatalf("Commit of the new transaction: %v", err)
	}
	if n := countA(other); n != 2 {
		t.Errorf("another client sees %d nodes after the new transaction, want 2", n)
	}
}

// The owner's decision (5 October): no total limit by default. With the
// default configuration the End waits until the session is ENDED however many
// requests that takes: here a 4 s hold spans at least three 1 s requests, and
// no still-running error is returned.
func TestBulkWait_DefaultWaitHasNoTotalLimit(t *testing.T) {
	needSlowBulkServer(t)
	if d := gqldb.DefaultConfig().BulkImportWaitTimeout; d != 0 {
		t.Fatalf("default BulkImportWaitTimeout %v, want 0 (no limit)", d)
	}
	host := os.Getenv("GQLDB_HOST")
	client, err := gqldb.NewClient(gqldb.NewConfigBuilder().Hosts(host).Username(testUsername).Password(testPassword).
		HealthCheckInterval(0).Build())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	if _, err := client.Login(ctx, testUsername, testPassword); err != nil {
		t.Fatal(err)
	}
	graph := uniqueName("slow_go_nolimit")
	computeGraph(t, ctx, client, graph)
	sid := loadSession(t, ctx, client, graph, 500, 1000)
	requireStates(t, client, sid)

	var reports int
	res, err := client.EndBulkImport(ctx, sid, gqldb.BulkImportWaitOptions{
		ProgressInterval: time.Second,
		OnProgress:       func(gqldb.BulkImportProgress) { reports++ },
	})
	if err != nil {
		t.Fatalf("EndBulkImport with the default total wait: %v", err)
	}
	if !res.Success || res.State != gqldb.BulkImportStateEnded || res.Attempts < 3 || reports < 2 {
		t.Errorf("result %+v after %d reports, want ENDED after at least 3 requests", res, reports)
	}
}

// Review round 1.

// newTTLClient signs in to the server whose sign-ins last 3 s, with the given
// client session id ("" for a generated one).
func newTTLClient(t *testing.T, host, csid string) *gqldb.Client {
	t.Helper()
	cfg := gqldb.NewConfigBuilder().Hosts(host).Username(testUsername).Password(testPassword).
		Timeout(30 * time.Second).HealthCheckInterval(0).Build()
	cfg.SessionID = csid
	client, err := gqldb.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := client.Login(context.Background(), testUsername, testPassword); err != nil {
		t.Fatalf("Login: %v", err)
	}
	return client
}

// Two clients share a client session id they set (Config.SessionID, offered
// for cross-channel session continuity). A's transaction is lost with its
// sign-in while B has a live transaction under the shared id. When A signs in
// again, the driver must not send anything under the shared id: a ROLLBACK
// there ended B's transaction on a server without the session-binding fix
// (B's commit failed with "transaction is not active" and its write was lost).
func TestBulkWait_SharedSessionIDKeepsTheOtherClientsTransaction(t *testing.T) {
	host := os.Getenv("GQLDB_TTL_HOST")
	if host == "" {
		t.Skip("needs a server whose sign-ins last 3 s (-session-ttl 3s); set GQLDB_TTL_HOST")
	}
	ctx := context.Background()
	shared := fmt.Sprintf("go-shared-%d", time.Now().UnixNano())
	a := newTTLClient(t, host, shared)
	b := newTTLClient(t, host, shared)
	graph := uniqueName("test_go_shared")
	if err := b.CreateGraph(ctx, graph, gqldb.GraphTypeOpen, "shared"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = newTTLClient(t, host, "").DropGraph(context.Background(), graph, true) })
	q := &gqldb.QueryConfig{GraphName: graph}

	ta, err := a.BeginTransaction(ctx, graph, false, 120)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Gql(ctx, "INSERT (:A {v: 1})", &gqldb.QueryConfig{GraphName: graph, TransactionID: ta.ID}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ { // A's sign-in lapses; B stays signed in
		time.Sleep(time.Second)
		_, _ = b.Gql(ctx, "RETURN 1", q)
	}
	// A server without the fix keeps A's dead transaction open under the
	// shared id, and B's plain ROLLBACK joins and ends it; a fixed server
	// ended it with A's sign-in and answers that there is none.
	_, rbErr := b.Gql(ctx, "ROLLBACK", q)
	t.Logf("B's plain ROLLBACK: %v", rbErr)
	tb, err := b.BeginTransaction(ctx, graph, false, 120)
	if err != nil {
		t.Fatalf("B's BeginTransaction on the shared id: %v", err)
	}
	if _, err := b.Gql(ctx, "INSERT (:B {v: 1})", &gqldb.QueryConfig{GraphName: graph, TransactionID: tb.ID}); err != nil {
		t.Fatal(err)
	}

	// A's next call meets its expired sign-in: the driver signs in again.
	if _, err := a.Gql(ctx, "RETURN 1", q); err != nil {
		t.Fatalf("A's call after its sign-in expired: %v", err)
	}
	if got := a.ClientSessionID(); got != shared {
		t.Errorf("A's configured client session id changed to %q", got)
	}
	_, err = a.Gql(ctx, "INSERT (:A {v: 2})", &gqldb.QueryConfig{GraphName: graph, TransactionID: ta.ID})
	var tse *gqldb.TransactionSignInExpiredError
	if !errors.As(err, &tse) {
		t.Errorf("A's lost transaction: %v, want *TransactionSignInExpiredError", err)
	}

	if _, err := b.Commit(ctx, tb.ID); err != nil {
		t.Fatalf("B's commit after A signed in again: %v", err)
	}
	resp, err := newTTLClient(t, host, "").Gql(ctx, "MATCH (n:B) RETURN count(n) AS cnt", q)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := resp.SingleInt(); n != 1 {
		t.Errorf("B's node stored %d times, want 1", n)
	}
}

// A progress callback that panics does not end the wait: End goes on to its
// final state. On a server older than the states End is one request and the
// callback is never called.
func TestBulkWait_ProgressCallbackPanicDoesNotEndTheWait(t *testing.T) {
	needSlowBulkServer(t)
	ctx := context.Background()
	graph := uniqueName("slow_go_cbpanic")
	computeGraph(t, ctx, testClient, graph)
	sid := loadSession(t, ctx, testClient, graph, 200, 200)
	states := serverReportsStates(t, testClient, sid)
	calls := 0
	res, err := testClient.EndBulkImport(ctx, sid, gqldb.BulkImportWaitOptions{
		ProgressInterval: time.Second,
		OnProgress: func(gqldb.BulkImportProgress) {
			calls++
			panic("callback bug")
		},
	})
	if err != nil {
		t.Fatalf("EndBulkImport: %v", err)
	}
	if !res.Success {
		t.Errorf("result %+v, want success", res)
	}
	if states && (res.State != gqldb.BulkImportStateEnded || calls < 2) {
		t.Errorf("result %+v after %d callback calls, want ENDED after at least 2", res, calls)
	}
	if !states && calls != 0 {
		t.Errorf("an older server: the callback ran %d times, want 0", calls)
	}
	t.Logf("End: state %q, %d attempts, %d callback calls (each panicked), states=%v", res.State, res.Attempts, calls, states)
}

// A 10 ms progress interval is raised to 100 ms; End still reaches its final
// state, with at most about one request a second.
func TestBulkWait_TinyProgressInterval(t *testing.T) {
	needSlowBulkServer(t)
	ctx := context.Background()
	graph := uniqueName("slow_go_tiny")
	computeGraph(t, ctx, testClient, graph)
	sid := loadSession(t, ctx, testClient, graph, 200, 200)
	states := serverReportsStates(t, testClient, sid)
	res, err := testClient.EndBulkImport(ctx, sid, gqldb.BulkImportWaitOptions{ProgressInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("EndBulkImport with a 10 ms interval: %v", err)
	}
	if !res.Success || (states && res.State != gqldb.BulkImportStateEnded) {
		t.Errorf("result %+v, want success (ENDED on a server with states)", res)
	}
	if float64(res.Attempts) > res.Waited.Seconds()+2 {
		t.Errorf("%d requests in %v: more than one a second", res.Attempts, res.Waited)
	}
	t.Logf("End with a 10 ms interval: %d attempts in %v, state %q", res.Attempts, res.Waited.Round(time.Millisecond), res.State)
}

// On a server older than the states (push 8) End is one request that answers
// when the End is done: a total wait shorter than the End is not honoured, and
// the call succeeds. Skipped on a server with states.
func TestBulkWait_OlderServerEndIgnoresTheTotalWait(t *testing.T) {
	needSlowBulkServer(t)
	ctx := context.Background()
	graph := uniqueName("slow_go_old")
	computeGraph(t, ctx, testClient, graph)
	sid := loadSession(t, ctx, testClient, graph, 200, 200)
	if serverReportsStates(t, testClient, sid) {
		_, _ = testClient.EndBulkImport(ctx, sid)
		t.Skip("the server reports bulk import states (gqldb-grpc push 10 or later); this test is for an older one")
	}
	reports := 0
	res, err := testClient.EndBulkImport(ctx, sid, gqldb.BulkImportWaitOptions{TotalTimeout: time.Second,
		ProgressInterval: time.Second, OnProgress: func(gqldb.BulkImportProgress) { reports++ }})
	if err != nil {
		t.Fatalf("EndBulkImport on an older server: %v", err)
	}
	if !res.Success || res.State != gqldb.BulkImportStateUnspecified || res.Attempts != 1 || reports != 0 ||
		res.Waited < 3*time.Second {
		t.Errorf("result %+v, %d reports; want one blocking request of about 4 s that succeeds", res, reports)
	}
	t.Logf("older server: End took %v in %d request (total wait 1 s not honoured)", res.Waited.Round(time.Millisecond), res.Attempts)
}

// Review round 2.

// The helper that skips the state tests on a server without bulk import
// states aborts the test's session before it skips. On such a server (push 8)
// an open session holds one of the server's 20 pooled connections until End,
// Abort or the 30-minute idle cleanup, and the sessions skipped tests left
// open wedged it. The server here reports states, so the helper is told it
// does not.
func TestBulkWait_SkipWithoutStatesAbortsTheSessionFirst(t *testing.T) {
	if testClient == nil {
		t.Skip("no server with sign-in at GQLDB_HOST")
	}
	ctx := context.Background()
	graph := uniqueName("test_go_skip_states")
	if err := testClient.CreateGraph(ctx, graph, gqldb.GraphTypeOpen, "skip test"); err != nil {
		t.Fatalf("CreateGraph %s: %v", graph, err)
	}
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = testClient.DropGraph(cctx, graph, true)
	})
	sid := loadSession(t, ctx, testClient, graph, 20, 0)
	states := serverReportsStates(t, testClient, sid)
	t.Run("helper", func(t *testing.T) { skipWithoutStates(t, testClient, sid, false) })
	st, err := testClient.GetBulkImportStatus(ctx, sid)
	switch {
	case states && err != nil:
		t.Fatalf("GetBulkImportStatus after the helper skipped: %v", err)
	case states && (st.IsActive || !st.State.IsFinal()):
		t.Fatalf("the session is still open after the helper skipped: active %v, state %q", st.IsActive, st.State)
	case !states && err == nil && st.IsActive:
		t.Fatalf("the session is still open after the helper skipped")
	}
}
