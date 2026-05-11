//go:build integration

// Phase 2 dual-source default-graph cache tests (mirrors Python
// tests/integration/test_current_graph_phase2.py).
//
// Validates the driver's session.DefaultGraph cache stays in sync with
// the server's authoritative response.current_graph across the cases
// that the regex-only fallback could not cover.
//
// Server requirement: response.current_graph populated. Old servers
// (no fix) cause the compound / multi-USE tests to skip via the
// once-per-package probe below.

package integration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

var (
	phase2Probed sync.Once
	phase2Supported bool
)

// probeCurrentGraphSupport runs once per package: creates a temporary
// graph, USE's it, inspects Response.CurrentGraph; non-empty => server
// fills the field => Phase 2 server contract satisfied.
func probeCurrentGraphSupport(t *testing.T) bool {
	phase2Probed.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		probeG := fmt.Sprintf("phase2_probe_%d", time.Now().UnixNano())
		if err := testClient.CreateGraph(ctx, probeG, gqldb.GraphTypeOpen, ""); err != nil {
			t.Logf("probe: CreateGraph failed: %v", err)
			return
		}
		resp, err := testClient.Gql(ctx, fmt.Sprintf("USE GRAPH %s", probeG), nil)
		if err != nil {
			t.Logf("probe: USE GRAPH failed: %v", err)
		} else {
			phase2Supported = strings.TrimSpace(resp.CurrentGraph) != ""
		}
		// best-effort cleanup
		_ = testClient.UseGraph(ctx, "miniCircle")
		_ = testClient.DropGraph(ctx, probeG, true)
	})
	return phase2Supported
}

func currentGraph(c *gqldb.Client) string {
	s := c.GetSession()
	if s == nil {
		return ""
	}
	return s.DefaultGraph
}

func makeTwoGraphs(t *testing.T) (string, string, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := time.Now().UnixNano()
	g1 := fmt.Sprintf("phase2_a_%d", suffix)
	g2 := fmt.Sprintf("phase2_b_%d", suffix+1)
	if err := testClient.CreateGraph(ctx, g1, gqldb.GraphTypeOpen, ""); err != nil {
		t.Fatalf("CreateGraph %s: %v", g1, err)
	}
	if err := testClient.CreateGraph(ctx, g2, gqldb.GraphTypeOpen, ""); err != nil {
		_ = testClient.DropGraph(ctx, g1, true)
		t.Fatalf("CreateGraph %s: %v", g2, err)
	}
	cleanup := func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel2()
		_ = testClient.UseGraph(ctx2, "miniCircle")
		_ = testClient.DropGraph(ctx2, g1, true)
		_ = testClient.DropGraph(ctx2, g2, true)
	}
	return g1, g2, cleanup
}

// T1 — single USE GRAPH (works on all servers via regex fallback)
func TestPhase2_T1_SingleUseGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	g1, _, cleanup := makeTwoGraphs(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := testClient.Gql(ctx, fmt.Sprintf("USE GRAPH %s", g1), nil); err != nil {
		t.Fatalf("USE GRAPH %s: %v", g1, err)
	}
	if got := currentGraph(testClient); got != g1 {
		t.Errorf("T1: cache=%q, want %q", got, g1)
	}
}

// T2 — compound USE in front (only response.current_graph path catches)
func TestPhase2_T2_CompoundUseFirst(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	if !probeCurrentGraphSupport(t) {
		t.Skip("server does not populate response.current_graph")
	}
	g1, _, cleanup := makeTwoGraphs(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := testClient.Gql(ctx, fmt.Sprintf("USE GRAPH %s; SHOW GRAPHS", g1), nil); err != nil {
		t.Fatalf("compound: %v", err)
	}
	if got := currentGraph(testClient); got != g1 {
		t.Errorf("T2: cache=%q, want %q (via response.current_graph)", got, g1)
	}
}

// T3 — compound USE at end (only response.current_graph path catches)
func TestPhase2_T3_CompoundUseLast(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	if !probeCurrentGraphSupport(t) {
		t.Skip("server does not populate response.current_graph")
	}
	g1, _, cleanup := makeTwoGraphs(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := testClient.Gql(ctx, fmt.Sprintf("SHOW GRAPHS; USE GRAPH %s", g1), nil); err != nil {
		t.Fatalf("compound: %v", err)
	}
	if got := currentGraph(testClient); got != g1 {
		t.Errorf("T3: cache=%q, want %q", got, g1)
	}
}

// T4 — multiple USE in one compound (last-write-wins via response.current_graph)
func TestPhase2_T4_MultipleUseLastWins(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	if !probeCurrentGraphSupport(t) {
		t.Skip("server does not populate response.current_graph")
	}
	g1, g2, cleanup := makeTwoGraphs(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := testClient.Gql(ctx, fmt.Sprintf("USE GRAPH %s; USE GRAPH %s", g1, g2), nil); err != nil {
		t.Fatalf("multi-USE: %v", err)
	}
	if got := currentGraph(testClient); got != g2 {
		t.Errorf("T4: cache=%q, want %q (last-write-wins)", got, g2)
	}
}

// T5 — USE GRAPH inside an active transaction is rejected (6.2.4 contract:
// server forbids switching graphs during a transaction, sidesteps the
// "rollback rule" question entirely).
func TestPhase2_T5_UseInsideActiveTxRejected(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	g1, _, cleanup := makeTwoGraphs(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := testClient.Gql(ctx, fmt.Sprintf("USE GRAPH %s", g1), nil); err != nil {
		t.Fatalf("setup USE: %v", err)
	}
	if _, err := testClient.Gql(ctx, "START TRANSACTION", nil); err != nil {
		t.Fatalf("START TRANSACTION: %v", err)
	}
	defer func() {
		_, _ = testClient.Gql(ctx, "ROLLBACK", nil)
	}()
	_, err := testClient.Gql(ctx, fmt.Sprintf("USE GRAPH %s", g1), nil)
	if err == nil {
		t.Fatal("USE GRAPH inside active tx should be rejected, got no error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cannot switch graph") {
		t.Errorf("expected 'cannot switch graph' in error, got: %v", err)
	}
}

// T6 — per-request graph_name override does not change session cache
// (PostgreSQL SET LOCAL semantics; query runs against the override but
// session.DefaultGraph stays at the previous value).
func TestPhase2_T6_GraphNameOverrideDoesNotChangeSession(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	g1, g2, cleanup := makeTwoGraphs(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Plant a marker only in g2.
	if _, err := testClient.Gql(ctx, fmt.Sprintf("USE GRAPH %s", g2), nil); err != nil {
		t.Fatalf("USE g2: %v", err)
	}
	if _, err := testClient.Gql(ctx, "INSERT (n:Phase2Marker {tag:'in_g2'})", nil); err != nil {
		t.Fatalf("INSERT marker: %v", err)
	}
	if _, err := testClient.Gql(ctx, fmt.Sprintf("USE GRAPH %s", g1), nil); err != nil {
		t.Fatalf("USE g1: %v", err)
	}
	cacheBefore := currentGraph(testClient)
	if cacheBefore != g1 {
		t.Fatalf("setup: cache=%q, want %q", cacheBefore, g1)
	}

	resp, err := testClient.Gql(ctx, "MATCH (n:Phase2Marker) RETURN n.tag",
		&gqldb.QueryConfig{GraphName: g2})
	if err != nil {
		t.Fatalf("override query: %v", err)
	}
	if resp.RowCount != 1 {
		t.Errorf("override should route to g2 (1 marker row); got RowCount=%d", resp.RowCount)
	}
	if got := currentGraph(testClient); got != cacheBefore {
		t.Errorf("per-request graph_name should not modify session; cache went from %q to %q",
			cacheBefore, got)
	}
}

// Bonus — DROP self-graph clears cache (server fixed in 6.2.4; previously
// returned the dropped name instead of "").
func TestPhase2_DropSelfClearsCache(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	if !probeCurrentGraphSupport(t) {
		t.Skip("server does not populate response.current_graph")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := fmt.Sprintf("phase2_drop_%d", time.Now().UnixNano())
	if err := testClient.CreateGraph(ctx, g, gqldb.GraphTypeOpen, ""); err != nil {
		t.Fatalf("CreateGraph: %v", err)
	}
	defer func() {
		_ = testClient.UseGraph(ctx, "miniCircle")
		_ = testClient.DropGraph(ctx, g, true)
	}()

	if _, err := testClient.Gql(ctx, fmt.Sprintf("USE GRAPH %s", g), nil); err != nil {
		t.Fatalf("USE GRAPH: %v", err)
	}
	if currentGraph(testClient) != g {
		t.Fatalf("setup: cache=%q, want %q", currentGraph(testClient), g)
	}
	if _, err := testClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", g), nil); err != nil {
		t.Fatalf("DROP GRAPH: %v", err)
	}
	if got := currentGraph(testClient); got != "" {
		t.Errorf("DROP self: cache=%q, want %q", got, "")
	}
}
