//go:build integration

package integration

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestHITSAlgoInfo(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := noAuthClient.Gql(ctx, `show algos`, nil)
	if err != nil {
		t.Fatalf("show algos failed: %v", err)
	}
	for _, row := range resp.Rows {
		val, _ := row.Get(0)
		m, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		if fmt.Sprintf("%v", m["name"]) == "algo.hits" {
			t.Logf("\n=== algo.hits ===")
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
}

// setupHITSGraph creates a directed graph for HITS testing.
// Hub nodes (A,B) point to Authority nodes (X,Y,Z).
// A→X, A→Y, A→Z, B→X, B→Y, C→Z, Z→A (feedback loop)
// A,B should be top hubs; X,Y,Z should be top authorities.
func setupHITSGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	graphName := fmt.Sprintf("test_hits_%d", time.Now().UnixMilli())
	_, err := noAuthClient.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	if err != nil {
		t.Fatalf("Create graph failed: %v", err)
	}
	_ = noAuthClient.UseGraph(ctx, graphName)

	session, err := noAuthClient.StartBulkImport(ctx, graphName, nil)
	if err != nil || !session.Success {
		t.Fatalf("StartBulkImport failed: %v / %s", err, session.Message)
	}

	nodes := []*gqldb.NodeData{
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}},  // 0 hub
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}},  // 1 hub
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}},  // 2 minor hub
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "X"}},  // 3 authority
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "Y"}},  // 4 authority
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "Z"}},  // 5 authority
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "iso"}}, // 6 isolated
	}
	nr, _ := noAuthClient.InsertNodes(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	ids := nr.NodeIDs

	// Directed edges: hubs → authorities
	edges := []*gqldb.EdgeData{
		{Label: "LINKS", FromNodeID: ids[0], ToNodeID: ids[3]}, // A→X
		{Label: "LINKS", FromNodeID: ids[0], ToNodeID: ids[4]}, // A→Y
		{Label: "LINKS", FromNodeID: ids[0], ToNodeID: ids[5]}, // A→Z
		{Label: "LINKS", FromNodeID: ids[1], ToNodeID: ids[3]}, // B→X
		{Label: "LINKS", FromNodeID: ids[1], ToNodeID: ids[4]}, // B→Y
		{Label: "LINKS", FromNodeID: ids[2], ToNodeID: ids[5]}, // C→Z
		{Label: "LINKS", FromNodeID: ids[5], ToNodeID: ids[0]}, // Z→A (feedback)
	}
	noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Graph: %s, A(hub)=%s B(hub)=%s C=%s X(auth)=%s Y(auth)=%s Z(auth)=%s iso=%s",
		graphName, ids[0], ids[1], ids[2], ids[3], ids[4], ids[5], ids[6])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func hitsLog(t *testing.T, resp *gqldb.Response) {
	t.Helper()
	t.Logf("Rows: %d, Columns: %v", resp.RowCount, resp.Columns)
	for _, row := range resp.Rows {
		vals := make([]string, len(resp.Columns))
		for i := range resp.Columns {
			v, _ := row.Get(i)
			vals[i] = fmt.Sprintf("%s=%v(%T)", resp.Columns[i], v, v)
		}
		t.Logf("  %v", vals)
	}
}

// =============================================================================
// 1. Basic run + correctness
// =============================================================================

func TestHITSBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupHITSGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.hits() YIELD nodeId, hubScore, authScore`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	hitsLog(t, resp)

	if resp.RowCount != 7 {
		t.Errorf("Expected 7 rows, got %d", resp.RowCount)
	}

	hubScores := make(map[string]float64)
	authScores := make(map[string]float64)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		hs, _ := row.Get(1)
		as, _ := row.Get(2)
		nidStr := fmt.Sprintf("%v", nid)
		hubScores[nidStr] = hs.(float64)
		authScores[nidStr] = as.(float64)
	}

	// A should be top hub (points to 3 authorities)
	// X should be top authority (pointed to by 2 hubs)
	t.Logf("A hub=%.4f auth=%.4f", hubScores[ids[0]], authScores[ids[0]])
	t.Logf("B hub=%.4f auth=%.4f", hubScores[ids[1]], authScores[ids[1]])
	t.Logf("X hub=%.4f auth=%.4f", hubScores[ids[3]], authScores[ids[3]])

	// Hubs should have higher hub scores than authorities
	if hubScores[ids[0]] < hubScores[ids[3]] {
		t.Logf("NOTE: A(hub) hubScore=%v < X(auth) hubScore=%v", hubScores[ids[0]], hubScores[ids[3]])
	}

	// Authorities should have higher auth scores than hubs (except Z→A feedback)
	if authScores[ids[3]] < authScores[ids[0]] && authScores[ids[3]] > 0 {
		t.Logf("NOTE: X(auth) authScore=%v < A(hub) authScore=%v", authScores[ids[3]], authScores[ids[0]])
	}

	// Isolated node should have 0 for both
	isoHub := hubScores[ids[6]]
	isoAuth := authScores[ids[6]]
	t.Logf("iso hub=%.4f auth=%.4f (should be 0)", isoHub, isoAuth)
	if isoHub != 0 || isoAuth != 0 {
		t.Errorf("Isolated node should have hub=0, auth=0; got hub=%v, auth=%v", isoHub, isoAuth)
	}

	// All scores >= 0
	for nid, hs := range hubScores {
		if hs < 0 {
			t.Errorf("Node %s: hubScore should be >= 0, got %v", nid, hs)
		}
	}
	for nid, as := range authScores {
		if as < 0 {
			t.Errorf("Node %s: authScore should be >= 0, got %v", nid, as)
		}
	}
}

// =============================================================================
// 2. iterations parameter — boundary values
// =============================================================================

func TestHITSIterations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHITSGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("iterations_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({iterations: 1}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("iterations=1:")
		hitsLog(t, resp)
	})

	t.Run("iterations_100", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({iterations: 100}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("iterations=100:")
		hitsLog(t, resp)
	})

	t.Run("iterations_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({iterations: 0}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Logf("iterations=0 error: %v", err)
		} else {
			t.Logf("iterations=0: %d rows", resp.RowCount)
			hitsLog(t, resp)
		}
	})

	t.Run("iterations_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({iterations: -1}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			errStr := fmt.Sprintf("%v", err)
			if strings.Contains(errStr, "EOF") {
				t.Fatal("SERVER CRASHED with iterations=-1")
			}
			t.Logf("iterations=-1 error (no crash): %v", err)
		} else {
			t.Logf("iterations=-1: %d rows (should validate)", resp.RowCount)
			hitsLog(t, resp)
		}
	})

	t.Run("iterations_negative_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({iterations: -999}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			errStr := fmt.Sprintf("%v", err)
			if strings.Contains(errStr, "EOF") {
				t.Fatal("SERVER CRASHED with iterations=-999")
			}
			t.Logf("iterations=-999 error: %v", err)
		} else {
			t.Logf("iterations=-999: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 3. tolerance parameter — boundary values
// =============================================================================

func TestHITSTolerance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHITSGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("tolerance_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({tolerance: 0.5}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("tolerance=0.5:")
		hitsLog(t, resp)
	})

	t.Run("tolerance_tiny", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({tolerance: 0.0000000001}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("tolerance=1e-10:")
		hitsLog(t, resp)
	})

	t.Run("tolerance_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({tolerance: 0}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Logf("tolerance=0 error: %v", err)
		} else {
			t.Log("tolerance=0:")
			hitsLog(t, resp)
		}
	})

	t.Run("tolerance_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({tolerance: -0.1}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Logf("tolerance=-0.1 error: %v", err)
		} else {
			t.Logf("tolerance=-0.1: accepted (should validate?)")
			hitsLog(t, resp)
		}
	})
}

// =============================================================================
// 4. limit parameter
// =============================================================================

func TestHITSLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHITSGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	tests := []struct {
		name     string
		limit    int
		expected int64
	}{
		{"limit_1", 1, 1},
		{"limit_3", 3, 3},
		{"limit_7", 7, 7},
		{"limit_minus1_all", -1, 7},
		{"limit_0", 0, 0},
		{"limit_100", 100, 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.hits({limit: %d}) YIELD nodeId, hubScore, authScore`, tc.limit)
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Logf("limit=%d error: %v", tc.limit, err)
				return
			}
			if resp.RowCount != tc.expected {
				t.Errorf("limit=%d: expected %d, got %d", tc.limit, tc.expected, resp.RowCount)
			}
		})
	}

	t.Run("limit_negative_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({limit: -999}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			if strings.Contains(fmt.Sprintf("%v", err), "EOF") {
				t.Fatal("SERVER CRASHED")
			}
			t.Logf("limit=-999 error: %v", err)
		} else {
			t.Logf("limit=-999: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 5. order parameter — sorted by authority score
// =============================================================================

func TestHITSOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHITSGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("order_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({order: 'desc'}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=desc (by authScore):")
		hitsLog(t, resp)
		// Verify descending by authScore
		var prev float64 = math.MaxFloat64
		for _, row := range resp.Rows {
			as, _ := row.Get(2)
			auth := as.(float64)
			if auth > prev {
				t.Errorf("Not descending: %v > %v", auth, prev)
			}
			prev = auth
		}
	})

	t.Run("order_asc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({order: 'asc'}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=asc:")
		hitsLog(t, resp)
		var prev float64 = -1
		for _, row := range resp.Rows {
			as, _ := row.Get(2)
			auth := as.(float64)
			if auth < prev {
				t.Errorf("Not ascending: %v < %v", auth, prev)
			}
			prev = auth
		}
	})

	t.Run("order_desc_limit_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({order: 'desc', limit: 1}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("Top authority (desc, limit=1):")
		hitsLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1, got %d", resp.RowCount)
		}
	})

	t.Run("order_invalid", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.hits({order: 'invalid'}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Logf("Invalid order error: %v", err)
		} else {
			t.Error("Expected error for invalid order")
		}
	})
}

// =============================================================================
// 6. Run modes: stream, stats, write
// =============================================================================

func TestHITSRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHITSGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits.stream({order: 'desc'}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream mode:")
		hitsLog(t, resp)
	})

	t.Run("stats_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits.stats() YIELD nodeCount, minHubScore, maxHubScore, minAuthScore, maxAuthScore`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats mode:")
		hitsLog(t, resp)
	})

	t.Run("write_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits.write({}, {db: {property: {hubScore: 'hub_s', authScore: 'auth_s'}}}) YIELD task_id`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		hitsLog(t, resp)

		time.Sleep(1 * time.Second)
		vResp, err := noAuthClient.Gql(ctx, `MATCH (n:N) RETURN n.name, n.hub_s, n.auth_s ORDER BY n.name`, qc)
		if err != nil {
			t.Logf("Verify failed: %v", err)
			return
		}
		t.Log("Written hub/auth scores:")
		hitsLog(t, vResp)
	})
}

// =============================================================================
// 7. Combined parameters
// =============================================================================

func TestHITSCombined(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHITSGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("iterations_tolerance_limit_order", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.hits({iterations: 50, tolerance: 0.0001, limit: 3, order: 'desc'}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("iterations=50, tolerance=0.0001, limit=3, order=desc:")
		hitsLog(t, resp)
		if resp.RowCount != 3 {
			t.Errorf("Expected 3, got %d", resp.RowCount)
		}
	})
}

// =============================================================================
// 8. Crash test
// =============================================================================

func TestHITSCrashTest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHITSGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	crashTests := []struct {
		name  string
		query string
	}{
		{"iterations_negative_large", `CALL algo.hits({iterations: -999}) YIELD nodeId, hubScore, authScore`},
		{"tolerance_negative_large", `CALL algo.hits({tolerance: -999}) YIELD nodeId, hubScore, authScore`},
		{"limit_negative_large", `CALL algo.hits({limit: -999}) YIELD nodeId, hubScore, authScore`},
	}
	for _, tc := range crashTests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := noAuthClient.Gql(ctx, tc.query, qc)
			if err != nil {
				errStr := fmt.Sprintf("%v", err)
				if strings.Contains(errStr, "EOF") || strings.Contains(errStr, "Unavailable") {
					t.Fatalf("SERVER CRASHED with %s", tc.name)
				}
				t.Logf("%s error (no crash): %v", tc.name, err)
			} else {
				t.Logf("%s: %d rows (accepted)", tc.name, resp.RowCount)
			}
		})
	}
}

// =============================================================================
// 9. Correctness: L2-normalization check
// =============================================================================

func TestHITSNormalization(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHITSGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.hits() YIELD nodeId, hubScore, authScore`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	hubSumSq := 0.0
	authSumSq := 0.0
	for _, row := range resp.Rows {
		hs, _ := row.Get(1)
		as, _ := row.Get(2)
		hubSumSq += hs.(float64) * hs.(float64)
		authSumSq += as.(float64) * as.(float64)
	}
	t.Logf("Hub sum of squares: %v (L2-norm → should be ~1.0)", hubSumSq)
	t.Logf("Auth sum of squares: %v (L2-norm → should be ~1.0)", authSumSq)

	// Check max normalization
	maxHub, maxAuth := 0.0, 0.0
	for _, row := range resp.Rows {
		hs, _ := row.Get(1)
		as, _ := row.Get(2)
		if hs.(float64) > maxHub {
			maxHub = hs.(float64)
		}
		if as.(float64) > maxAuth {
			maxAuth = as.(float64)
		}
	}
	t.Logf("Max hub: %v, Max auth: %v (max-norm → should be 1.0)", maxHub, maxAuth)
}

// =============================================================================
// 10. Design: missing ids/direction/weight?
// =============================================================================

func TestHITSDesign(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHITSGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("ids_not_supported", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.hits({ids: ['n:0']}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Logf("ids not supported: %v", err)
		} else {
			t.Log("ids accepted")
		}
	})

	t.Run("direction_not_supported", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.hits({direction: 'out'}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Logf("direction not supported: %v", err)
		} else {
			t.Log("direction accepted")
		}
	})

	t.Run("weight_not_supported", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.hits({weight: 'some_prop'}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Logf("weight not supported: %v", err)
		} else {
			t.Log("weight accepted")
		}
	})
}

// =============================================================================
// 11. MiniCircle
// =============================================================================

func TestHITSMiniCircle(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	qc := &gqldb.QueryConfig{GraphName: "miniCircle"}

	t.Run("top5_auth", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.hits({order: 'desc', limit: 5}) YIELD nodeId, hubScore, authScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle top 5 by authScore:")
		hitsLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.hits.stats() YIELD nodeCount, minHubScore, maxHubScore, minAuthScore, maxAuthScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle HITS stats:")
		hitsLog(t, resp)
	})
}
