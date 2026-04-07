//go:build integration

package integration

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// TestArticleRankAlgoInfo prints the algorithm spec.
func TestArticleRankAlgoInfo(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := noAuthClient.Gql(ctx, `show algos`, nil)
	if err != nil {
		t.Fatalf("show algos failed: %v", err)
	}
	found := false
	for _, row := range resp.Rows {
		val, _ := row.Get(0)
		m, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		name := fmt.Sprintf("%v", m["name"])
		if name == "algo.articlerank" || name == "algo.article_rank" {
			found = true
			t.Logf("\n=== %s ===", name)
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
	if !found {
		t.Fatal("articlerank algorithm not found in show algos")
	}
}

// setupArticleRankGraph creates a citation-like graph for ArticleRank testing.
// A is cited by B,C,D (hub); B is cited by C; D is cited by E; E is isolated sink.
// A→B, A→C, A→D, B→C, C→D, D→E (directed citation graph)
func setupArticleRankGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}

	graphName := fmt.Sprintf("test_ar_%d", time.Now().UnixMilli())
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
		{Labels: []string{"Paper"}, Properties: map[string]interface{}{"name": "A"}},
		{Labels: []string{"Paper"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"Paper"}, Properties: map[string]interface{}{"name": "C"}},
		{Labels: []string{"Paper"}, Properties: map[string]interface{}{"name": "D"}},
		{Labels: []string{"Paper"}, Properties: map[string]interface{}{"name": "E"}},
	}
	nr, err := noAuthClient.InsertNodes(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}

	ids := nr.NodeIDs
	edges := []*gqldb.EdgeData{
		{Label: "CITES", FromNodeID: ids[0], ToNodeID: ids[1], Properties: map[string]interface{}{"w_int": int64(3), "w_f64": float64(1.5)}},
		{Label: "CITES", FromNodeID: ids[0], ToNodeID: ids[2], Properties: map[string]interface{}{"w_int": int64(1), "w_f64": float64(0.8)}},
		{Label: "CITES", FromNodeID: ids[0], ToNodeID: ids[3], Properties: map[string]interface{}{"w_int": int64(2), "w_f64": float64(2.0)}},
		{Label: "CITES", FromNodeID: ids[1], ToNodeID: ids[2], Properties: map[string]interface{}{"w_int": int64(5), "w_f64": float64(3.5)}},
		{Label: "CITES", FromNodeID: ids[2], ToNodeID: ids[3], Properties: map[string]interface{}{"w_int": int64(1), "w_f64": float64(0.5)}},
		{Label: "CITES", FromNodeID: ids[3], ToNodeID: ids[4], Properties: map[string]interface{}{"w_int": int64(4), "w_f64": float64(1.2)}},
	}
	_, err = noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	if err != nil {
		t.Fatalf("InsertEdges failed: %v", err)
	}
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Graph: %s, nodes: A=%s B=%s C=%s D=%s E=%s", graphName, ids[0], ids[1], ids[2], ids[3], ids[4])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func arLog(t *testing.T, resp *gqldb.Response) {
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
// 1. Basic run — default params, verify all nodes get scores
// =============================================================================

func TestArticleRankBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupArticleRankGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("default_params", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank() YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		arLog(t, resp)
		if resp.RowCount != 5 {
			t.Errorf("Expected 5 rows, got %d", resp.RowCount)
		}
		// All scores should be > 0
		for _, row := range resp.Rows {
			sc, _ := row.Get(1)
			score := sc.(float64)
			if score <= 0 {
				t.Errorf("Score should be > 0, got %v", score)
			}
		}
	})
}

// =============================================================================
// 2. dampingFactor parameter
// =============================================================================

func TestArticleRankDampingFactor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupArticleRankGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// Default damping factor is typically 0.85
	t.Run("damping_0.85_default", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({dampingFactor: 0.85}) YIELD nodeId, score`, qc)
		if err != nil {
			// Try alternative param name
			resp, err = noAuthClient.Gql(ctx, `CALL algo.articlerank({damping: 0.85}) YIELD nodeId, score`, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
		}
		t.Log("dampingFactor=0.85:")
		arLog(t, resp)
	})

	t.Run("damping_0.5", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({dampingFactor: 0.5}) YIELD nodeId, score`, qc)
		if err != nil {
			resp, err = noAuthClient.Gql(ctx, `CALL algo.articlerank({damping: 0.5}) YIELD nodeId, score`, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
		}
		t.Log("dampingFactor=0.5:")
		arLog(t, resp)
	})

	t.Run("damping_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({dampingFactor: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			resp, err = noAuthClient.Gql(ctx, `CALL algo.articlerank({damping: 0}) YIELD nodeId, score`, qc)
			if err != nil {
				t.Logf("damping=0 error: %v", err)
				return
			}
		}
		t.Log("dampingFactor=0 (all scores should be equal):")
		arLog(t, resp)
	})

	t.Run("damping_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({dampingFactor: 1.0}) YIELD nodeId, score`, qc)
		if err != nil {
			resp, err = noAuthClient.Gql(ctx, `CALL algo.articlerank({damping: 1.0}) YIELD nodeId, score`, qc)
			if err != nil {
				t.Logf("damping=1.0 error: %v", err)
				return
			}
		}
		t.Log("dampingFactor=1.0:")
		arLog(t, resp)
	})
}

// =============================================================================
// 3. maxIterations / tolerance (convergence)
// =============================================================================

func TestArticleRankConvergence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupArticleRankGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("maxIterations_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({maxIterations: 1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("maxIterations error: %v", err)
			return
		}
		t.Log("maxIterations=1:")
		arLog(t, resp)
	})

	t.Run("maxIterations_100", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({maxIterations: 100}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("maxIterations error: %v", err)
			return
		}
		t.Log("maxIterations=100:")
		arLog(t, resp)
	})

	t.Run("tolerance", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({tolerance: 0.0001}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("tolerance error: %v", err)
			return
		}
		t.Log("tolerance=0.0001:")
		arLog(t, resp)
	})
}

// =============================================================================
// 4. weight parameter (int / float64 / nonexistent)
// =============================================================================

func TestArticleRankWeight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupArticleRankGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("no_weight", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank() YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("No weight:")
		arLog(t, resp)
	})

	t.Run("weight_int", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({weight: 'w_int'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("weight=w_int error: %v", err)
			return
		}
		t.Log("weight=w_int:")
		arLog(t, resp)
	})

	t.Run("weight_float64", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({weight: 'w_f64'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("weight=w_f64 error: %v", err)
			return
		}
		t.Log("weight=w_f64:")
		arLog(t, resp)
	})

	t.Run("weight_nonexistent", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({weight: 'nonexistent'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("weight=nonexistent error: %v", err)
		} else {
			t.Log("weight=nonexistent:")
			arLog(t, resp)
		}
	})
}

// =============================================================================
// 5. normalized parameter
// =============================================================================

func TestArticleRankNormalized(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupArticleRankGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("normalized_true", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({normalized: true}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("normalized error: %v", err)
			return
		}
		t.Log("normalized=true:")
		arLog(t, resp)
		for _, row := range resp.Rows {
			sc, _ := row.Get(1)
			score := sc.(float64)
			if score < 0 || score > 1.0 {
				t.Errorf("Normalized score out of [0,1]: %v", score)
			}
		}
	})

	t.Run("normalized_false", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({normalized: false}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("normalized=false error: %v", err)
			return
		}
		t.Log("normalized=false:")
		arLog(t, resp)
	})
}

// =============================================================================
// 6. ids parameter (node subset)
// =============================================================================

func TestArticleRankIds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupArticleRankGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("single_node", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.articlerank({ids: ['%s']}) YIELD nodeId, score`, ids[0])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Logf("ids error: %v", err)
			return
		}
		arLog(t, resp)
	})

	t.Run("two_nodes", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.articlerank({ids: ['%s', '%s']}) YIELD nodeId, score`, ids[0], ids[2])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Logf("ids error: %v", err)
			return
		}
		arLog(t, resp)
	})

	t.Run("nonexistent_id", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({ids: ['n:999999']}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("nonexistent id error: %v", err)
		} else {
			arLog(t, resp)
		}
	})
}

// =============================================================================
// 7. limit + order
// =============================================================================

func TestArticleRankLimitOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupArticleRankGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("order_desc_limit_3", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({order: 'desc', limit: 3}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		arLog(t, resp)
		if resp.RowCount != 3 {
			t.Errorf("Expected 3 rows, got %d", resp.RowCount)
		}
		// Verify descending
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({order: 'asc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		arLog(t, resp)
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

	t.Run("limit_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({limit: 1, order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		arLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row, got %d", resp.RowCount)
		}
	})
}

// =============================================================================
// 8. Run modes: stream, stats, write
// =============================================================================

func TestArticleRankRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupArticleRankGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank.stream({order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream mode:")
		arLog(t, resp)
	})

	t.Run("stats_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats mode:")
		arLog(t, resp)
	})

	t.Run("write_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank.write({}, {db: {property: 'ar_score'}}) YIELD task_id, nodesWritten`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		arLog(t, resp)

		// Check nodesWritten
		if resp.RowCount > 0 {
			nw, _ := resp.Rows[0].Get(1)
			if nw == nil {
				t.Log("NOTE: nodesWritten is nil (same as degree/betweenness write bug)")
			} else {
				t.Logf("nodesWritten=%v", nw)
			}
		}

		// Verify written property
		time.Sleep(500 * time.Millisecond)
		vResp, err := noAuthClient.Gql(ctx, `MATCH (n:Paper) RETURN n.name, n.ar_score ORDER BY n.name`, qc)
		if err != nil {
			t.Logf("Verify failed: %v", err)
			return
		}
		t.Log("Written scores:")
		arLog(t, vResp)
	})
}

// =============================================================================
// 9. Combined parameters
// =============================================================================

func TestArticleRankCombined(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupArticleRankGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("damping_weight_normalized", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({dampingFactor: 0.85, weight: 'w_int', normalized: true}) YIELD nodeId, score`, qc)
		if err != nil {
			// Try without weight if not supported
			resp, err = noAuthClient.Gql(ctx, `CALL algo.articlerank({dampingFactor: 0.85, normalized: true}) YIELD nodeId, score`, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
		}
		t.Log("damping=0.85, weight=w_int, normalized=true:")
		arLog(t, resp)
	})

	t.Run("ids_limit_order", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.articlerank({ids: ['%s', '%s', '%s'], order: 'desc', limit: 2}) YIELD nodeId, score`, ids[0], ids[1], ids[2])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Logf("ids+limit+order error: %v", err)
			return
		}
		arLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows, got %d", resp.RowCount)
		}
	})

	t.Run("weight_float64_maxIter", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({weight: 'w_f64', maxIterations: 50}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("weight_f64+maxIter error: %v", err)
			return
		}
		t.Log("weight=w_f64, maxIterations=50:")
		arLog(t, resp)
	})
}

// =============================================================================
// 10. Comparison with PageRank (sanity check)
// =============================================================================

func TestArticleRankVsPageRank(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupArticleRankGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("articlerank_scores", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articlerank({order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("articlerank failed: %v", err)
		}
		t.Log("ArticleRank:")
		arLog(t, resp)
	})

	t.Run("pagerank_scores", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("pagerank error (may not exist): %v", err)
			return
		}
		t.Log("PageRank (for comparison):")
		arLog(t, resp)
	})
}
