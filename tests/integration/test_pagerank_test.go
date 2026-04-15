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

func TestPRAlgoInfo(t *testing.T) {
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
		if fmt.Sprintf("%v", m["name"]) == "algo.pagerank" {
			t.Logf("\n=== algo.pagerank ===")
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
}

// setupPRGraph: directed citation graph. A→B,C,D; B→C; C→D; D→E.
// D and E should have highest PageRank (most "votes").
func setupPRGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	graphName := fmt.Sprintf("test_pr_%d", time.Now().UnixMilli())
	noAuthClient.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	_ = noAuthClient.UseGraph(ctx, graphName)

	session, _ := noAuthClient.StartBulkImport(ctx, graphName, nil)
	nodes := []*gqldb.NodeData{
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "D"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "E"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "F"}}, // isolated
	}
	nr, _ := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	ids := nr.NodeIDs

	edges := []*gqldb.EdgeData{
		{Label: "CITES", FromNodeID: ids[0], ToNodeID: ids[1]}, // A→B
		{Label: "CITES", FromNodeID: ids[0], ToNodeID: ids[2]}, // A→C
		{Label: "CITES", FromNodeID: ids[0], ToNodeID: ids[3]}, // A→D
		{Label: "CITES", FromNodeID: ids[1], ToNodeID: ids[2]}, // B→C
		{Label: "CITES", FromNodeID: ids[2], ToNodeID: ids[3]}, // C→D
		{Label: "CITES", FromNodeID: ids[3], ToNodeID: ids[4]}, // D→E
	}
	noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Graph: %s, A=%s B=%s C=%s D=%s E=%s F(iso)=%s", graphName, ids[0], ids[1], ids[2], ids[3], ids[4], ids[5])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func prLog(t *testing.T, resp *gqldb.Response) {
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

func TestPRBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupPRGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank() YIELD nodeId, score`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	prLog(t, resp)

	if resp.RowCount != 6 {
		t.Errorf("Expected 6 rows, got %d", resp.RowCount)
	}

	scores := make(map[string]float64)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		sc, _ := row.Get(1)
		scores[fmt.Sprintf("%v", nid)] = sc.(float64)
	}

	// All scores > 0 (even dangling nodes get (1-d)/n)
	for nid, sc := range scores {
		if sc <= 0 {
			t.Errorf("Node %s: score should be > 0, got %v", nid, sc)
		}
	}

	// Sum of all scores should be ~1.0 (or ~n depending on normalization)
	sum := 0.0
	for _, sc := range scores {
		sum += sc
	}
	t.Logf("Sum of scores: %v (should be ~1.0 or ~%d)", sum, len(scores))

	// E should have high PageRank (pointed to by D which is pointed to by A,C)
	eScore := scores[ids[4]]
	aScore := scores[ids[0]]
	t.Logf("A(source)=%v, E(sink)=%v", aScore, eScore)
}

// =============================================================================
// 2. damping parameter
// =============================================================================

func TestPRDamping(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupPRGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("damping_0.85", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({damping: 0.85}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("damping=0.85 (default):")
		prLog(t, resp)
	})

	t.Run("damping_0.5", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({damping: 0.5}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("damping=0.5 (more uniform):")
		prLog(t, resp)
	})

	t.Run("damping_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({damping: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("damping=0 error: %v", err)
		} else {
			t.Log("damping=0 (all scores equal = 1/n):")
			prLog(t, resp)
		}
	})

	t.Run("damping_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({damping: 1.0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("damping=1.0 error: %v", err)
		} else {
			t.Log("damping=1.0 (no random jump):")
			prLog(t, resp)
		}
	})

	t.Run("damping_negative", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({damping: -0.1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("damping=-0.1 error: %v", err)
		} else {
			t.Error("damping=-0.1 should error")
		}
	})

	t.Run("damping_gt1", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({damping: 1.5}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("damping=1.5 error: %v", err)
		} else {
			t.Error("damping=1.5 should error")
		}
	})
}

// =============================================================================
// 3. loop_num parameter (iterations)
// =============================================================================

func TestPRLoopNum(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupPRGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("loop_num_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({loop_num: 1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("loop_num=1:")
		prLog(t, resp)
	})

	t.Run("loop_num_100", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({loop_num: 100}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("loop_num=100:")
		prLog(t, resp)
	})

	t.Run("loop_num_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({loop_num: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("loop_num=0 error: %v", err)
		} else {
			t.Logf("loop_num=0: %d rows", resp.RowCount)
			prLog(t, resp)
		}
	})

	t.Run("loop_num_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({loop_num: -1}) YIELD nodeId, score`, qc)
		if err != nil {
			errStr := fmt.Sprintf("%v", err)
			if strings.Contains(errStr, "EOF") {
				t.Fatal("SERVER CRASHED with loop_num=-1")
			}
			t.Logf("loop_num=-1 error: %v", err)
		} else {
			t.Logf("loop_num=-1: %d rows (should validate)", resp.RowCount)
		}
	})

	t.Run("loop_num_negative_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({loop_num: -999}) YIELD nodeId, score`, qc)
		if err != nil {
			errStr := fmt.Sprintf("%v", err)
			if strings.Contains(errStr, "EOF") {
				t.Fatal("SERVER CRASHED with loop_num=-999")
			}
			t.Logf("loop_num=-999 error: %v", err)
		} else {
			t.Logf("loop_num=-999: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 4. tolerance parameter
// =============================================================================

func TestPRTolerance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupPRGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("tolerance_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({tolerance: 0.5}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("tolerance=0.5:")
		prLog(t, resp)
	})

	t.Run("tolerance_tiny", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({tolerance: 0.0000000001}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("tolerance=1e-10:")
		prLog(t, resp)
	})

	t.Run("tolerance_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({tolerance: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("tolerance=0 error: %v", err)
		} else {
			t.Log("tolerance=0:")
			prLog(t, resp)
		}
	})

	t.Run("tolerance_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({tolerance: -0.1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("tolerance=-0.1 error: %v", err)
		} else {
			t.Logf("tolerance=-0.1 accepted (should validate?), %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 5. limit + order
// =============================================================================

func TestPRLimitOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupPRGraph(t, ctx)
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
		{"limit_minus1", -1, 6},
		{"limit_0", 0, 0},
		{"limit_100", 100, 6},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.pagerank({limit: %d}) YIELD nodeId, score`, tc.limit)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=desc:")
		prLog(t, resp)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({order: 'asc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=asc:")
		prLog(t, resp)
	})

	t.Run("order_invalid", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({order: 'invalid'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Invalid order error: %v", err)
		} else {
			t.Error("Expected error")
		}
	})

	t.Run("limit_negative_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({limit: -999}) YIELD nodeId, score`, qc)
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
// 6. Run modes
// =============================================================================

func TestPRRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupPRGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank.stream({order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream:")
		prLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats:")
		prLog(t, resp)
	})

	t.Run("write", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank.write({}, {db: {property: 'pr_s'}}) YIELD task_id`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		prLog(t, resp)
		time.Sleep(1 * time.Second)
		vResp, _ := noAuthClient.Gql(ctx, `MATCH (n:N) RETURN n.name, n.pr_s ORDER BY n.name`, qc)
		if vResp != nil {
			t.Log("Written PR scores:")
			prLog(t, vResp)
		}
	})
}

// =============================================================================
// 7. Combined
// =============================================================================

func TestPRCombined(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupPRGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("all_params", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({damping: 0.9, loop_num: 50, tolerance: 0.00001, limit: 3, order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("damping=0.9, loop_num=50, tolerance=1e-5, limit=3, order=desc:")
		prLog(t, resp)
		if resp.RowCount != 3 {
			t.Errorf("Expected 3, got %d", resp.RowCount)
		}
	})
}

// =============================================================================
// 8. Crash test
// =============================================================================

func TestPRCrashTest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupPRGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	crashTests := []struct {
		name  string
		query string
	}{
		{"damping_negative", `CALL algo.pagerank({damping: -999}) YIELD nodeId, score`},
		{"loop_num_negative_large", `CALL algo.pagerank({loop_num: -999}) YIELD nodeId, score`},
		{"tolerance_negative_large", `CALL algo.pagerank({tolerance: -999}) YIELD nodeId, score`},
		{"limit_negative_large", `CALL algo.pagerank({limit: -999}) YIELD nodeId, score`},
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
// 9. Score sum check (should be ~1.0 for stochastic matrix)
// =============================================================================

func TestPRScoreSum(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupPRGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank() YIELD nodeId, score`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	sum := 0.0
	for _, row := range resp.Rows {
		sc, _ := row.Get(1)
		sum += sc.(float64)
	}
	t.Logf("Sum of PageRank scores: %v", sum)

	// Standard PageRank: sum = 1.0 (probability distribution)
	// Some implementations: sum = n
	if math.Abs(sum-1.0) < 0.01 {
		t.Log("Sum ≈ 1.0 (probability normalized)")
	} else if math.Abs(sum-float64(resp.RowCount)) < 0.01 {
		t.Logf("Sum ≈ %d (count normalized)", resp.RowCount)
	} else {
		t.Logf("Sum = %v (non-standard normalization)", sum)
	}
}

// =============================================================================
// 10. Design: missing direction/ids/weight?
// =============================================================================

func TestPRDesign(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupPRGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("direction_not_in_spec", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({direction: 'out'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("direction not supported: %v", err)
		} else {
			t.Log("direction accepted")
		}
	})

	t.Run("ids_not_in_spec", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({ids: ['n:0']}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("ids not supported: %v", err)
		} else {
			t.Log("ids accepted")
		}
	})

	t.Run("weight_not_in_spec", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({weight: 'some_prop'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("weight not supported: %v", err)
		} else {
			t.Log("weight accepted")
		}
	})

	// Check maxIterations alias
	t.Run("maxIterations_alias", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({maxIterations: 50}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("maxIterations not accepted (uses loop_num): %v", err)
		} else {
			t.Log("maxIterations accepted as alias")
		}
	})
}

// =============================================================================
// 11. MiniCircle
// =============================================================================

func TestPRMiniCircle(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	qc := &gqldb.QueryConfig{GraphName: "miniCircle"}

	t.Run("top5", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.pagerank({order: 'desc', limit: 5}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle top 5 PageRank:")
		prLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.pagerank.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle PR stats:")
		prLog(t, resp)
	})
}
