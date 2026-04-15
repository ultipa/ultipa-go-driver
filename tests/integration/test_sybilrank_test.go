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

func TestSybilRankAlgoInfo(t *testing.T) {
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
		if fmt.Sprintf("%v", m["name"]) == "algo.sybilrank" {
			t.Logf("\n=== algo.sybilrank ===")
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
}

// setupSybilGraph: social network with trusted and sybil clusters.
// Trusted cluster: T1-T2-T3 (triangle, high trust)
// Bridge: T3-B1
// Sybil cluster: B1-S1-S2-S3 (sybil ring connected through B1)
// Isolated: I1
func setupSybilGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	graphName := fmt.Sprintf("test_sb_%d", time.Now().UnixMilli())
	noAuthClient.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	_ = noAuthClient.UseGraph(ctx, graphName)

	session, _ := noAuthClient.StartBulkImport(ctx, graphName, nil)
	nodes := []*gqldb.NodeData{
		{Labels: []string{"User"}, Properties: map[string]interface{}{"name": "T1"}}, // 0 trusted
		{Labels: []string{"User"}, Properties: map[string]interface{}{"name": "T2"}}, // 1 trusted
		{Labels: []string{"User"}, Properties: map[string]interface{}{"name": "T3"}}, // 2 trusted
		{Labels: []string{"User"}, Properties: map[string]interface{}{"name": "B1"}}, // 3 bridge
		{Labels: []string{"User"}, Properties: map[string]interface{}{"name": "S1"}}, // 4 sybil
		{Labels: []string{"User"}, Properties: map[string]interface{}{"name": "S2"}}, // 5 sybil
		{Labels: []string{"User"}, Properties: map[string]interface{}{"name": "S3"}}, // 6 sybil
		{Labels: []string{"User"}, Properties: map[string]interface{}{"name": "I1"}}, // 7 isolated
	}
	nr, _ := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	ids := nr.NodeIDs

	biEdge := func(from, to string) []*gqldb.EdgeData {
		return []*gqldb.EdgeData{
			{Label: "FRIEND", FromNodeID: from, ToNodeID: to},
			{Label: "FRIEND", FromNodeID: to, ToNodeID: from},
		}
	}
	var edges []*gqldb.EdgeData
	// Trusted triangle
	edges = append(edges, biEdge(ids[0], ids[1])...)
	edges = append(edges, biEdge(ids[1], ids[2])...)
	edges = append(edges, biEdge(ids[2], ids[0])...)
	// Bridge
	edges = append(edges, biEdge(ids[2], ids[3])...)
	// Sybil ring
	edges = append(edges, biEdge(ids[3], ids[4])...)
	edges = append(edges, biEdge(ids[4], ids[5])...)
	edges = append(edges, biEdge(ids[5], ids[6])...)
	edges = append(edges, biEdge(ids[6], ids[3])...)

	noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Graph: %s, T1=%s T2=%s T3=%s B1=%s S1=%s S2=%s S3=%s I1=%s",
		graphName, ids[0], ids[1], ids[2], ids[3], ids[4], ids[5], ids[6], ids[7])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func sbLog(t *testing.T, resp *gqldb.Response) {
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

func TestSybilBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupSybilGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// Use T1 and T2 as trusted seeds
	q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s,%s'}) YIELD nodeId, trust, rank`, ids[0], ids[1])
	resp, err := noAuthClient.Gql(ctx, q, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	sbLog(t, resp)

	if resp.RowCount != 8 {
		t.Errorf("Expected 8 rows, got %d", resp.RowCount)
	}

	trusts := make(map[string]float64)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		tr, _ := row.Get(1)
		trusts[fmt.Sprintf("%v", nid)] = tr.(float64)
	}

	// Trusted nodes should have higher trust than sybil nodes
	t1Trust := trusts[ids[0]]
	s1Trust := trusts[ids[4]]
	i1Trust := trusts[ids[7]]
	t.Logf("T1(trusted)=%v, S1(sybil)=%v, I1(isolated)=%v", t1Trust, s1Trust, i1Trust)

	if t1Trust < s1Trust {
		t.Errorf("Trusted T1 (%v) should have higher trust than sybil S1 (%v)", t1Trust, s1Trust)
	}

	// All trust scores >= 0
	for nid, tr := range trusts {
		if tr < 0 {
			t.Errorf("Node %s: trust should be >= 0, got %v", nid, tr)
		}
	}
}

// =============================================================================
// 2. trustedNodes parameter — required, various formats
// =============================================================================

func TestSybilTrustedNodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupSybilGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("single_trusted", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s'}) YIELD nodeId, trust, rank`, ids[0])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("Single trusted node:")
		sbLog(t, resp)
	})

	t.Run("all_trusted_cluster", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s,%s,%s'}) YIELD nodeId, trust, rank`, ids[0], ids[1], ids[2])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("Three trusted nodes:")
		sbLog(t, resp)
	})

	t.Run("missing_trustedNodes", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.sybilrank() YIELD nodeId, trust`, qc)
		if err != nil {
			t.Logf("Missing trustedNodes error (expected): %v", err)
		} else {
			t.Error("Expected error for missing required parameter")
		}
	})

	t.Run("empty_trustedNodes", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.sybilrank({trustedNodes: ''}) YIELD nodeId, trust`, qc)
		if err != nil {
			t.Logf("Empty trustedNodes error: %v", err)
		} else {
			t.Log("Empty trustedNodes accepted")
		}
	})

	t.Run("nonexistent_trusted", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.sybilrank({trustedNodes: 'n:999999'}) YIELD nodeId, trust`, qc)
		if err != nil {
			t.Logf("Nonexistent trusted node error: %v", err)
		} else {
			t.Logf("Nonexistent trusted: %d rows", resp.RowCount)
			sbLog(t, resp)
		}
	})
}

// =============================================================================
// 3. iterations parameter — 0=auto, positive, negative
// =============================================================================

func TestSybilIterations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupSybilGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}
	trusted := ids[0]

	t.Run("iterations_0_auto", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', iterations: 0}) YIELD nodeId, trust`, trusted)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		// auto = ceil(log2(8)) = 3
		t.Log("iterations=0 (auto = ceil(log2(n))):")
		sbLog(t, resp)
	})

	t.Run("iterations_1", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', iterations: 1}) YIELD nodeId, trust`, trusted)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("iterations=1:")
		sbLog(t, resp)
	})

	t.Run("iterations_20", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', iterations: 20}) YIELD nodeId, trust`, trusted)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("iterations=20:")
		sbLog(t, resp)
	})

	t.Run("iterations_negative", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', iterations: -1}) YIELD nodeId, trust`, trusted)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			errStr := fmt.Sprintf("%v", err)
			if strings.Contains(errStr, "EOF") {
				t.Fatal("SERVER CRASHED with iterations=-1")
			}
			t.Logf("iterations=-1 error: %v", err)
		} else {
			t.Logf("iterations=-1: %d rows (should validate)", resp.RowCount)
		}
	})

	t.Run("iterations_negative_large", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', iterations: -999}) YIELD nodeId, trust`, trusted)
		resp, err := noAuthClient.Gql(ctx, q, qc)
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
// 4. limit + order
// =============================================================================

func TestSybilLimitOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupSybilGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}
	trusted := ids[0]

	tests := []struct {
		name     string
		limit    int
		expected int64
	}{
		{"limit_1", 1, 1},
		{"limit_3", 3, 3},
		{"limit_8", 8, 8},
		{"limit_minus1", -1, 8},
		{"limit_0", 0, 0},
		{"limit_100", 100, 8},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', limit: %d}) YIELD nodeId, trust`, trusted, tc.limit)
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

	t.Run("order_desc", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', order: 'desc'}) YIELD nodeId, trust, rank`, trusted)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=desc (most trusted first):")
		sbLog(t, resp)
		var prev float64 = math.MaxFloat64
		for _, row := range resp.Rows {
			tr, _ := row.Get(1)
			trust := tr.(float64)
			if trust > prev {
				t.Errorf("Not descending: %v > %v", trust, prev)
			}
			prev = trust
		}
	})

	t.Run("order_asc", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', order: 'asc'}) YIELD nodeId, trust, rank`, trusted)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=asc (most sybil first):")
		sbLog(t, resp)
	})

	t.Run("order_invalid", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', order: 'invalid'}) YIELD nodeId, trust`, trusted)
		_, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Logf("Invalid order error: %v", err)
		} else {
			t.Error("Expected error")
		}
	})

	t.Run("limit_negative_large", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', limit: -999}) YIELD nodeId, trust`, trusted)
		resp, err := noAuthClient.Gql(ctx, q, qc)
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
// 5. Run modes
// =============================================================================

func TestSybilRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupSybilGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}
	trusted := ids[0]

	t.Run("stream", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank.stream({trustedNodes: '%s', order: 'desc'}) YIELD nodeId, trust, rank`, trusted)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream:")
		sbLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank.stats({trustedNodes: '%s'}) YIELD nodeCount, trustedCount, minTrust, maxTrust`, trusted)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats:")
		sbLog(t, resp)
	})

	t.Run("write", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank.write({trustedNodes: '%s'}, {db: {property: 'trust_s'}}) YIELD task_id`, trusted)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		sbLog(t, resp)
		time.Sleep(1 * time.Second)
		vResp, _ := noAuthClient.Gql(ctx, `MATCH (n:User) RETURN n.name, n.trust_s ORDER BY n.name`, qc)
		if vResp != nil {
			t.Log("Written trust scores:")
			sbLog(t, vResp)
		}
	})
}

// =============================================================================
// 6. Combined
// =============================================================================

func TestSybilCombined(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupSybilGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("all_params", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s,%s', iterations: 5, limit: 3, order: 'desc'}) YIELD nodeId, trust, rank`, ids[0], ids[1])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("all params:")
		sbLog(t, resp)
		if resp.RowCount != 3 {
			t.Errorf("Expected 3 (limit=3), got %d", resp.RowCount)
		}
	})
}

// =============================================================================
// 7. Crash test
// =============================================================================

func TestSybilCrashTest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupSybilGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}
	trusted := ids[0]

	crashTests := []struct {
		name  string
		query string
	}{
		{"iterations_negative_large", fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', iterations: -999}) YIELD nodeId, trust`, trusted)},
		{"limit_negative_large", fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', limit: -999}) YIELD nodeId, trust`, trusted)},
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
// 8. MiniCircle
// =============================================================================

func TestSybilMiniCircle(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	qc := &gqldb.QueryConfig{GraphName: "miniCircle"}

	// Get a node ID from miniCircle to use as trusted
	idResp, err := testClient.Gql(ctx, `MATCH (n) RETURN id(n) LIMIT 1`, qc)
	if err != nil || idResp.RowCount == 0 {
		t.Skip("Could not get node ID from miniCircle")
	}
	nodeId, _ := idResp.Rows[0].Get(0)
	trusted := fmt.Sprintf("%v", nodeId)

	t.Run("top5", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank({trustedNodes: '%s', order: 'desc', limit: 5}) YIELD nodeId, trust, rank`, trusted)
		resp, err := testClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle top 5 trust:")
		sbLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.sybilrank.stats({trustedNodes: '%s'}) YIELD nodeCount, trustedCount, minTrust, maxTrust`, trusted)
		resp, err := testClient.Gql(ctx, q, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("miniCircle sybilrank stats:")
		sbLog(t, resp)
	})
}
