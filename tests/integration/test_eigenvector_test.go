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

func TestEigenvectorAlgoInfo(t *testing.T) {
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
		if fmt.Sprintf("%v", m["name"]) == "algo.eigenvector" {
			t.Logf("\n=== algo.eigenvector ===")
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
}

// setupEVGraph creates a graph for eigenvector centrality testing.
// Star: Hub→A, Hub→B, Hub→C, Hub→D; triangle A-B-C; pendant D→E
// Hub should have highest eigenvector centrality.
func setupEVGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	graphName := fmt.Sprintf("test_ev_%d", time.Now().UnixMilli())
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
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "Hub"}}, // 0
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}},   // 1
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}},   // 2
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}},   // 3
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "D"}},   // 4
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "E"}},   // 5
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
	edges = append(edges, biEdge(ids[0], ids[1])...) // Hub-A
	edges = append(edges, biEdge(ids[0], ids[2])...) // Hub-B
	edges = append(edges, biEdge(ids[0], ids[3])...) // Hub-C
	edges = append(edges, biEdge(ids[0], ids[4])...) // Hub-D
	edges = append(edges, biEdge(ids[1], ids[2])...) // A-B
	edges = append(edges, biEdge(ids[2], ids[3])...) // B-C
	edges = append(edges, biEdge(ids[4], ids[5])...) // D-E

	noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Graph: %s, Hub=%s A=%s B=%s C=%s D=%s E=%s", graphName, ids[0], ids[1], ids[2], ids[3], ids[4], ids[5])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func evLog(t *testing.T, resp *gqldb.Response) {
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
// 1. Basic run — default params, correctness
// =============================================================================

func TestEVBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector() YIELD nodeId, score, rank`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	evLog(t, resp)

	if resp.RowCount != 6 {
		t.Errorf("Expected 6 rows, got %d", resp.RowCount)
	}

	scores := make(map[string]float64)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		sc, _ := row.Get(1)
		scores[fmt.Sprintf("%v", nid)] = sc.(float64)
	}

	hubScore := scores[ids[0]]
	eScore := scores[ids[5]]
	t.Logf("Hub score=%v (should be highest), E score=%v (should be lowest)", hubScore, eScore)

	// Hub should have highest eigenvector centrality
	for nid, sc := range scores {
		if sc > hubScore && nid != ids[0] {
			t.Errorf("Node %s (score=%v) > Hub (score=%v)", nid, sc, hubScore)
		}
	}

	// All scores >= 0
	for nid, sc := range scores {
		if sc < 0 {
			t.Errorf("Node %s: score should be >= 0, got %v", nid, sc)
		}
	}
}

// =============================================================================
// 2. direction parameter — both / in / out / invalid / empty
// =============================================================================

func TestEVDirection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	results := make(map[string]string)
	for _, dir := range []string{"both", "in", "out"} {
		t.Run("direction_"+dir, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.eigenvector({direction: '%s'}) YIELD nodeId, score`, dir)
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
			evLog(t, resp)
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
		t.Log("NOTE: direction 'in' and 'out' return identical results")
	}

	t.Run("invalid_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({direction: 'invalid'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Expected error: %v", err)
		} else {
			t.Error("Expected error for invalid direction")
		}
	})

	t.Run("empty_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({direction: ''}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Empty direction error: %v", err)
		} else {
			t.Log("Empty direction accepted")
		}
	})
}

// =============================================================================
// 3. iterations parameter — boundary values
// =============================================================================

func TestEVIterations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("iterations_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({iterations: 1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("iterations=1 (may not converge):")
		evLog(t, resp)
	})

	t.Run("iterations_10", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({iterations: 10}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("iterations=10:")
		evLog(t, resp)
	})

	t.Run("iterations_1000", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({iterations: 1000}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("iterations=1000:")
		evLog(t, resp)
	})

	t.Run("iterations_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({iterations: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("iterations=0 error: %v", err)
		} else {
			t.Logf("iterations=0: %d rows", resp.RowCount)
			evLog(t, resp)
		}
	})

	t.Run("iterations_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({iterations: -1}) YIELD nodeId, score`, qc)
		if err != nil {
			errStr := fmt.Sprintf("%v", err)
			if strings.Contains(errStr, "EOF") {
				t.Fatal("SERVER CRASHED with iterations=-1")
			}
			t.Logf("iterations=-1 error (no crash): %v", err)
		} else {
			t.Logf("iterations=-1: %d rows (should validate)", resp.RowCount)
			evLog(t, resp)
		}
	})
}

// =============================================================================
// 4. tolerance parameter — boundary values
// =============================================================================

func TestEVTolerance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("tolerance_default", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({tolerance: 0.000001}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("tolerance=0.000001 (default):")
		evLog(t, resp)
	})

	t.Run("tolerance_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({tolerance: 0.5}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("tolerance=0.5 (very loose, fast convergence):")
		evLog(t, resp)
	})

	t.Run("tolerance_tiny", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({tolerance: 0.0000000001}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("tolerance=1e-10 (very tight):")
		evLog(t, resp)
	})

	t.Run("tolerance_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({tolerance: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("tolerance=0 error: %v", err)
		} else {
			t.Log("tolerance=0 (will run max iterations):")
			evLog(t, resp)
		}
	})

	t.Run("tolerance_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({tolerance: -0.1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("tolerance=-0.1 error: %v", err)
		} else {
			t.Logf("tolerance=-0.1: accepted (should validate?)")
			evLog(t, resp)
		}
	})
}

// =============================================================================
// 5. ids parameter — single, multi, empty, nonexistent
// =============================================================================

func TestEVIds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("single_node", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.eigenvector({ids: ['%s']}) YIELD nodeId, score`, ids[0])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		evLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row, got %d", resp.RowCount)
		}
	})

	t.Run("three_nodes", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.eigenvector({ids: ['%s', '%s', '%s']}) YIELD nodeId, score`, ids[0], ids[1], ids[5])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		evLog(t, resp)
		if resp.RowCount != 3 {
			t.Errorf("Expected 3 rows, got %d", resp.RowCount)
		}
	})

	t.Run("empty_ids", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({ids: []}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Empty ids error: %v", err)
		} else {
			t.Logf("Empty ids: %d rows", resp.RowCount)
		}
	})

	t.Run("nonexistent_id", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({ids: ['n:999999']}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Nonexistent ID error: %v", err)
		} else {
			t.Logf("Nonexistent ID: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 6. limit parameter
// =============================================================================

func TestEVLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	tests := []struct {
		name     string
		limit    int
		expected int64
	}{
		{"limit_1", 1, 1},
		{"limit_3", 3, 3},
		{"limit_6", 6, 6},
		{"limit_minus1_all", -1, 6},
		{"limit_0", 0, 0},
		{"limit_100", 100, 6},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.eigenvector({limit: %d}) YIELD nodeId, score`, tc.limit)
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Logf("limit=%d error: %v", tc.limit, err)
				return
			}
			if resp.RowCount != tc.expected {
				t.Errorf("limit=%d: expected %d rows, got %d", tc.limit, tc.expected, resp.RowCount)
			}
		})
	}

	t.Run("limit_negative_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({limit: -999}) YIELD nodeId, score`, qc)
		if err != nil {
			if strings.Contains(fmt.Sprintf("%v", err), "EOF") {
				t.Fatal("SERVER CRASHED with limit=-999")
			}
			t.Logf("limit=-999 error: %v", err)
		} else {
			t.Logf("limit=-999: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 7. order parameter — asc, desc, invalid, verify ordering
// =============================================================================

func TestEVOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("order_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({order: 'desc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		evLog(t, resp)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({order: 'asc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		evLog(t, resp)
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

	t.Run("order_desc_limit_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({order: 'desc', limit: 1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		evLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1, got %d", resp.RowCount)
		}
	})

	t.Run("order_invalid", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({order: 'invalid'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Invalid order error: %v", err)
		} else {
			t.Error("Expected error for invalid order")
		}
	})
}

// =============================================================================
// 8. Run modes: stream, stats, write
// =============================================================================

func TestEVRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector.stream({order: 'desc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream mode:")
		evLog(t, resp)
	})

	t.Run("stats_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats mode:")
		evLog(t, resp)
	})

	t.Run("write_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector.write({}, {db: {property: 'ev_score'}}) YIELD task_id`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		evLog(t, resp)

		time.Sleep(1 * time.Second)
		vResp, err := noAuthClient.Gql(ctx, `MATCH (n:N) RETURN n.name, n.ev_score ORDER BY n.name`, qc)
		if err != nil {
			t.Logf("Verify failed: %v", err)
			return
		}
		t.Log("Written eigenvector scores:")
		evLog(t, vResp)
	})
}

// =============================================================================
// 9. Combined parameters
// =============================================================================

func TestEVCombined(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("direction_out_limit2_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({direction: 'out', limit: 2, order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		evLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows, got %d", resp.RowCount)
		}
	})

	t.Run("ids_iterations_tolerance", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.eigenvector({ids: ['%s', '%s'], iterations: 200, tolerance: 0.0001}) YIELD nodeId, score`, ids[0], ids[5])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		evLog(t, resp)
	})

	t.Run("all_params", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.eigenvector({direction: 'both', iterations: 50, tolerance: 0.001, ids: ['%s', '%s', '%s'], limit: 2, order: 'desc'}) YIELD nodeId, score, rank`, ids[0], ids[1], ids[2])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("All params combined:")
		evLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows (limit=2), got %d", resp.RowCount)
		}
	})
}

// =============================================================================
// 10. Crash test — extreme params
// =============================================================================

func TestEVCrashTest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	crashTests := []struct {
		name  string
		query string
	}{
		{"iterations_negative_large", `CALL algo.eigenvector({iterations: -999}) YIELD nodeId, score`},
		{"tolerance_negative_large", `CALL algo.eigenvector({tolerance: -999}) YIELD nodeId, score`},
		{"limit_negative_large", `CALL algo.eigenvector({limit: -999}) YIELD nodeId, score`},
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
				t.Logf("%s: %d rows (accepted without error)", tc.name, resp.RowCount)
			}
		})
	}
}

// =============================================================================
// 11. Correctness: eigenvector properties
// =============================================================================

func TestEVCorrectness(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({order: 'desc'}) YIELD nodeId, score`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Eigenvector centrality should be L2-normalized (sum of squares = 1)
	sumSq := 0.0
	for _, row := range resp.Rows {
		sc, _ := row.Get(1)
		score := sc.(float64)
		sumSq += score * score
	}
	t.Logf("Sum of squares: %v (should be ~1.0 if L2-normalized)", sumSq)

	// Or L1-normalized (max = 1.0)
	maxScore := 0.0
	for _, row := range resp.Rows {
		sc, _ := row.Get(1)
		score := sc.(float64)
		if score > maxScore {
			maxScore = score
		}
	}
	t.Logf("Max score: %v (should be 1.0 if max-normalized)", maxScore)

	if math.Abs(sumSq-1.0) > 0.01 && math.Abs(maxScore-1.0) > 0.01 {
		t.Logf("NOTE: scores not L2-normalized (sumSq=%v) nor max-normalized (max=%v)", sumSq, maxScore)
	}
}

// =============================================================================
// 12. Design: missing weight parameter?
// =============================================================================

func TestEVWeight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	_, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({weight: 'some_prop'}) YIELD nodeId, score`, qc)
	if err != nil {
		t.Logf("weight not supported: %v", err)
	} else {
		t.Log("weight parameter accepted")
	}
}

// =============================================================================
// 13. Naming: iterations vs maxIterations vs loop_num
// =============================================================================

func TestEVParamNaming(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupEVGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// This algo uses 'iterations', not 'maxIterations' or 'loop_num'
	t.Run("iterations_works", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({iterations: 50}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("'iterations' param failed: %v", err)
		}
	})

	t.Run("maxIterations_should_fail", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({maxIterations: 50}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("'maxIterations' not accepted (expected): %v", err)
		} else {
			t.Log("'maxIterations' also accepted (alias?)")
		}
	})
}

// =============================================================================
// 14. On miniCircle
// =============================================================================

func TestEVMiniCircle(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	qc := &gqldb.QueryConfig{GraphName: "miniCircle"}

	t.Run("top5", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.eigenvector({order: 'desc', limit: 5}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle top 5 eigenvector:")
		evLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.eigenvector.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle eigenvector stats:")
		evLog(t, resp)
	})

	t.Run("direction_comparison", func(t *testing.T) {
		for _, dir := range []string{"both", "in", "out"} {
			q := fmt.Sprintf(`CALL algo.eigenvector({direction: '%s', order: 'desc', limit: 3}) YIELD nodeId, score`, dir)
			resp, err := testClient.Gql(ctx, q, qc)
			if err != nil {
				t.Logf("direction=%s error: %v", dir, err)
				continue
			}
			t.Logf("miniCircle direction=%s top 3:", dir)
			evLog(t, resp)
		}
	})
}
