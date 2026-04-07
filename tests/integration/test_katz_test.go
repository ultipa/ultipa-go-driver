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

func TestKatzAlgoInfo(t *testing.T) {
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
		if fmt.Sprintf("%v", m["name"]) == "algo.katz" {
			t.Logf("\n=== algo.katz ===")
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
}

// setupKatzGraph: Star + chain. Hub→A,B,C,D; D→E→F.
// Hub has most incoming paths → highest katz (direction=in).
func setupKatzGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	graphName := fmt.Sprintf("test_katz_%d", time.Now().UnixMilli())
	noAuthClient.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	_ = noAuthClient.UseGraph(ctx, graphName)

	session, _ := noAuthClient.StartBulkImport(ctx, graphName, nil)

	nodes := []*gqldb.NodeData{
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "Hub"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "D"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "E"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "F"}},
	}
	nr, _ := noAuthClient.InsertNodes(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	ids := nr.NodeIDs

	biEdge := func(from, to string) []*gqldb.EdgeData {
		return []*gqldb.EdgeData{
			{Label: "LINK", FromNodeID: from, ToNodeID: to},
			{Label: "LINK", FromNodeID: to, ToNodeID: from},
		}
	}
	var edges []*gqldb.EdgeData
	edges = append(edges, biEdge(ids[0], ids[1])...)
	edges = append(edges, biEdge(ids[0], ids[2])...)
	edges = append(edges, biEdge(ids[0], ids[3])...)
	edges = append(edges, biEdge(ids[0], ids[4])...)
	edges = append(edges, biEdge(ids[4], ids[5])...)
	edges = append(edges, biEdge(ids[5], ids[6])...)

	noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Graph: %s, Hub=%s A=%s B=%s C=%s D=%s E=%s F=%s", graphName, ids[0], ids[1], ids[2], ids[3], ids[4], ids[5], ids[6])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func katzLog(t *testing.T, resp *gqldb.Response) {
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

func TestKatzBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.katz() YIELD nodeId, score, rank`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	katzLog(t, resp)

	if resp.RowCount != 7 {
		t.Errorf("Expected 7 rows, got %d", resp.RowCount)
	}

	scores := make(map[string]float64)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		sc, _ := row.Get(1)
		scores[fmt.Sprintf("%v", nid)] = sc.(float64)
	}

	hubScore := scores[ids[0]]
	fScore := scores[ids[6]]
	t.Logf("Hub score=%v, F score=%v", hubScore, fScore)

	// All scores >= 0
	for nid, sc := range scores {
		if sc < 0 {
			t.Errorf("Node %s: score should be >= 0, got %v", nid, sc)
		}
	}

	// Hub should have highest katz (most connections = most walks)
	for nid, sc := range scores {
		if sc > hubScore && nid != ids[0] {
			t.Logf("NOTE: Node %s (score=%v) > Hub (score=%v)", nid, sc, hubScore)
		}
	}
}

// =============================================================================
// 2. alpha parameter — attenuation factor
// =============================================================================

func TestKatzAlpha(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("alpha_default_0.1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({alpha: 0.1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("alpha=0.1 (default):")
		katzLog(t, resp)
	})

	t.Run("alpha_small_0.01", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({alpha: 0.01}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("alpha=0.01 (less weight on distant walks):")
		katzLog(t, resp)
	})

	t.Run("alpha_large_0.5", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({alpha: 0.5}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("alpha=0.5 error (may exceed 1/spectralRadius): %v", err)
			return
		}
		t.Log("alpha=0.5:")
		katzLog(t, resp)
	})

	t.Run("alpha_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({alpha: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("alpha=0 error: %v", err)
		} else {
			t.Log("alpha=0 (no walk contribution, all scores = beta):")
			katzLog(t, resp)
		}
	})

	t.Run("alpha_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({alpha: 1.0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("alpha=1.0 error (expected, exceeds 1/spectralRadius): %v", err)
		} else {
			t.Log("alpha=1.0 (may diverge):")
			katzLog(t, resp)
		}
	})

	t.Run("alpha_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({alpha: -0.1}) YIELD nodeId, score`, qc)
		if err != nil {
			errStr := fmt.Sprintf("%v", err)
			if strings.Contains(errStr, "EOF") {
				t.Fatal("SERVER CRASHED with alpha=-0.1")
			}
			t.Logf("alpha=-0.1 error: %v", err)
		} else {
			t.Logf("alpha=-0.1 accepted (should validate?)")
			katzLog(t, resp)
		}
	})
}

// =============================================================================
// 3. beta parameter — neighborhood weight
// =============================================================================

func TestKatzBeta(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("beta_default_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({beta: 1.0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("beta=1.0 (default):")
		katzLog(t, resp)
	})

	t.Run("beta_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({beta: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("beta=0 error: %v", err)
		} else {
			t.Log("beta=0 (no initial weight):")
			katzLog(t, resp)
		}
	})

	t.Run("beta_large_10", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({beta: 10.0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("beta=10.0:")
		katzLog(t, resp)
	})

	t.Run("beta_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({beta: -1.0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("beta=-1.0 error: %v", err)
		} else {
			t.Logf("beta=-1.0 accepted (should validate?)")
			katzLog(t, resp)
		}
	})
}

// =============================================================================
// 4. iterations parameter
// =============================================================================

func TestKatzIterations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("iterations_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({iterations: 1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("iterations=1:")
		katzLog(t, resp)
	})

	t.Run("iterations_100", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({iterations: 100}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("iterations=100:")
		katzLog(t, resp)
	})

	t.Run("iterations_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({iterations: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("iterations=0 error: %v", err)
		} else {
			t.Logf("iterations=0: %d rows", resp.RowCount)
			katzLog(t, resp)
		}
	})

	t.Run("iterations_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({iterations: -1}) YIELD nodeId, score`, qc)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({iterations: -999}) YIELD nodeId, score`, qc)
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
// 5. tolerance parameter
// =============================================================================

func TestKatzTolerance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("tolerance_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({tolerance: 0.5}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("tolerance=0.5:")
		katzLog(t, resp)
	})

	t.Run("tolerance_tiny", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({tolerance: 0.0000000001}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("tolerance=1e-10:")
		katzLog(t, resp)
	})

	t.Run("tolerance_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({tolerance: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("tolerance=0 error: %v", err)
		} else {
			t.Log("tolerance=0:")
			katzLog(t, resp)
		}
	})

	t.Run("tolerance_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({tolerance: -0.1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("tolerance=-0.1 error: %v", err)
		} else {
			t.Logf("tolerance=-0.1 accepted (should validate?), %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 6. direction parameter — default is 'in' (unique among algorithms)
// =============================================================================

func TestKatzDirection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	results := make(map[string]string)
	for _, dir := range []string{"both", "in", "out"} {
		t.Run("direction_"+dir, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.katz({direction: '%s'}) YIELD nodeId, score`, dir)
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
			katzLog(t, resp)
			s := ""
			for _, row := range resp.Rows {
				nid, _ := row.Get(0)
				sc, _ := row.Get(1)
				s += fmt.Sprintf("%v:%.4f,", nid, sc.(float64))
			}
			results[dir] = s
		})
	}

	if results["in"] == results["out"] && results["in"] != "" {
		t.Log("NOTE: direction 'in' and 'out' return identical results (systemic issue)")
	} else if results["in"] != results["out"] {
		t.Log("GOOD: direction 'in' and 'out' return DIFFERENT results!")
	}

	t.Run("invalid_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.katz({direction: 'invalid'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Expected error: %v", err)
		} else {
			t.Error("Expected error for invalid direction")
		}
	})
}

// =============================================================================
// 7. ids parameter
// =============================================================================

func TestKatzIds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("single_node", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.katz({ids: ['%s']}) YIELD nodeId, score`, ids[0])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		katzLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row, got %d", resp.RowCount)
		}
	})

	t.Run("three_nodes", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.katz({ids: ['%s', '%s', '%s']}) YIELD nodeId, score`, ids[0], ids[4], ids[6])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		katzLog(t, resp)
		if resp.RowCount != 3 {
			t.Errorf("Expected 3 rows, got %d", resp.RowCount)
		}
	})

	t.Run("empty_ids", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({ids: []}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Empty ids error: %v", err)
		} else {
			t.Logf("Empty ids: %d rows", resp.RowCount)
		}
	})

	t.Run("nonexistent_id", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({ids: ['n:999999']}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Nonexistent ID error: %v", err)
		} else {
			t.Logf("Nonexistent ID: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 8. limit + order
// =============================================================================

func TestKatzLimitOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupKatzGraph(t, ctx)
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
		{"limit_minus1", -1, 7},
		{"limit_0", 0, 0},
		{"limit_100", 100, 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.katz({limit: %d}) YIELD nodeId, score`, tc.limit)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({order: 'desc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		katzLog(t, resp)
		var prev float64 = math.MaxFloat64
		for _, row := range resp.Rows {
			sc, _ := row.Get(1)
			score := sc.(float64)
			if score > prev {
				t.Errorf("Not descending: %v > %v", score, prev)
			}
			prev = score
		}
	})

	t.Run("order_asc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({order: 'asc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		katzLog(t, resp)
		var prev float64 = -1
		for _, row := range resp.Rows {
			sc, _ := row.Get(1)
			score := sc.(float64)
			if score < prev {
				t.Errorf("Not ascending: %v < %v", score, prev)
			}
			prev = score
		}
	})

	t.Run("order_invalid", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.katz({order: 'invalid'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Invalid order error: %v", err)
		} else {
			t.Error("Expected error for invalid order")
		}
	})
}

// =============================================================================
// 9. Run modes
// =============================================================================

func TestKatzRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz.stream({order: 'desc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream:")
		katzLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats:")
		katzLog(t, resp)
	})

	t.Run("write", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz.write({}, {db: {property: 'katz_s'}}) YIELD task_id`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		katzLog(t, resp)
		time.Sleep(1 * time.Second)
		vResp, _ := noAuthClient.Gql(ctx, `MATCH (n:N) RETURN n.name, n.katz_s ORDER BY n.name`, qc)
		if vResp != nil {
			t.Log("Written scores:")
			katzLog(t, vResp)
		}
	})
}

// =============================================================================
// 10. Combined parameters
// =============================================================================

func TestKatzCombined(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("alpha_beta_iterations", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({alpha: 0.05, beta: 2.0, iterations: 50, tolerance: 0.00001}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("alpha=0.05, beta=2.0, iterations=50, tolerance=1e-5:")
		katzLog(t, resp)
	})

	t.Run("all_params", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.katz({alpha: 0.05, beta: 1.0, iterations: 30, tolerance: 0.001, direction: 'both', ids: ['%s', '%s', '%s'], limit: 2, order: 'desc'}) YIELD nodeId, score, rank`, ids[0], ids[4], ids[6])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("all params:")
		katzLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 (limit=2), got %d", resp.RowCount)
		}
	})
}

// =============================================================================
// 11. Crash test
// =============================================================================

func TestKatzCrashTest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	crashTests := []struct {
		name  string
		query string
	}{
		{"alpha_negative", `CALL algo.katz({alpha: -999}) YIELD nodeId, score`},
		{"beta_negative_large", `CALL algo.katz({beta: -999}) YIELD nodeId, score`},
		{"iterations_negative_large", `CALL algo.katz({iterations: -999}) YIELD nodeId, score`},
		{"tolerance_negative_large", `CALL algo.katz({tolerance: -999}) YIELD nodeId, score`},
		{"limit_negative_large", `CALL algo.katz({limit: -999}) YIELD nodeId, score`},
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
// 12. Alpha impact: higher alpha → more weight on distant walks
// =============================================================================

func TestKatzAlphaImpact(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	alphas := []float64{0.01, 0.05, 0.1, 0.2}
	hubScores := make(map[float64]float64)
	for _, a := range alphas {
		q := fmt.Sprintf(`CALL algo.katz({alpha: %v, direction: 'both'}) YIELD nodeId, score`, a)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Logf("alpha=%v error: %v", a, err)
			continue
		}
		for _, row := range resp.Rows {
			nid, _ := row.Get(0)
			sc, _ := row.Get(1)
			if fmt.Sprintf("%v", nid) == ids[0] {
				hubScores[a] = sc.(float64)
				t.Logf("alpha=%v: Hub score=%v", a, sc)
			}
		}
	}

	// Higher alpha should generally increase scores (more walk contribution)
	if hubScores[0.01] > hubScores[0.2] && hubScores[0.01] > 0 {
		t.Logf("NOTE: Hub score decreases with higher alpha (unexpected)")
	}
}

// =============================================================================
// 13. Design: missing weight? naming consistency?
// =============================================================================

func TestKatzDesign(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupKatzGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("weight_not_supported", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.katz({weight: 'some_prop'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("weight not supported: %v", err)
		} else {
			t.Log("weight accepted")
		}
	})

	// Check if maxIterations works as alias
	t.Run("maxIterations_alias", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.katz({maxIterations: 50}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("maxIterations not accepted: %v", err)
		} else {
			t.Log("maxIterations accepted as alias")
		}
	})
}

// =============================================================================
// 14. MiniCircle
// =============================================================================

func TestKatzMiniCircle(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	qc := &gqldb.QueryConfig{GraphName: "miniCircle"}

	t.Run("top5", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.katz({order: 'desc', limit: 5}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle top 5 katz:")
		katzLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.katz.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle katz stats:")
		katzLog(t, resp)
	})

	t.Run("direction_comparison", func(t *testing.T) {
		for _, dir := range []string{"both", "in", "out"} {
			q := fmt.Sprintf(`CALL algo.katz({direction: '%s', order: 'desc', limit: 3}) YIELD nodeId, score`, dir)
			resp, err := testClient.Gql(ctx, q, qc)
			if err != nil {
				t.Logf("direction=%s error: %v", dir, err)
				continue
			}
			t.Logf("miniCircle direction=%s top 3:", dir)
			katzLog(t, resp)
		}
	})
}
