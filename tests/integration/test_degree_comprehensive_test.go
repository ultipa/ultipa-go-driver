//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// setupDegreeTestGraph creates a test graph with known topology for degree tests.
// Topology: A→B(10, 1.5, 2.7), B→C(20, 3.2, 4.8), A→C(5, 0.9, 0.3), C→A(15, 2.0, 1.1)
// A: out=2, in=1; B: out=1, in=1; C: out=1, in=2
func setupDegreeTestGraph(t *testing.T, ctx context.Context) (string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}

	graphName := fmt.Sprintf("test_deg_%d", time.Now().UnixMilli())
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
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "A"}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "C"}},
	}
	nr, err := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}

	edges := []*gqldb.EdgeData{
		{Label: "KNOWS", FromNodeID: nr.NodeIDs[0], ToNodeID: nr.NodeIDs[1], Properties: map[string]interface{}{"w_int": int64(10), "w_f32": float32(1.5), "w_f64": float64(2.7)}},
		{Label: "KNOWS", FromNodeID: nr.NodeIDs[1], ToNodeID: nr.NodeIDs[2], Properties: map[string]interface{}{"w_int": int64(20), "w_f32": float32(3.2), "w_f64": float64(4.8)}},
		{Label: "KNOWS", FromNodeID: nr.NodeIDs[0], ToNodeID: nr.NodeIDs[2], Properties: map[string]interface{}{"w_int": int64(5), "w_f32": float32(0.9), "w_f64": float64(0.3)}},
		{Label: "KNOWS", FromNodeID: nr.NodeIDs[2], ToNodeID: nr.NodeIDs[0], Properties: map[string]interface{}{"w_int": int64(15), "w_f32": float32(2.0), "w_f64": float64(1.1)}},
	}
	_, err = noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	if err != nil {
		t.Fatalf("InsertEdges failed: %v", err)
	}

	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Test graph: %s, nodes: %v", graphName, nr.NodeIDs)

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, cleanup
}

func logDegreeRows(t *testing.T, resp *gqldb.Response) {
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
// 1. direction parameter: 'in', 'out', 'both'
// =============================================================================

func TestDegreeDirection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, cleanup := setupDegreeTestGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// Expected: A: out=2, in=1, both=3; B: out=1, in=1, both=2; C: out=1, in=2, both=3
	tests := []struct {
		name      string
		direction string
	}{
		{"both", "both"},
		{"in", "in"},
		{"out", "out"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.degree({direction: '%s'}) YIELD nodeId, degree, score, inDegree, outDegree`, tc.direction)
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
			logDegreeRows(t, resp)
			if resp.RowCount != 3 {
				t.Errorf("Expected 3 rows, got %d", resp.RowCount)
			}
		})
	}

	// Invalid direction
	t.Run("invalid_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.degree({direction: 'invalid'}) YIELD nodeId, degree`, qc)
		if err != nil {
			t.Logf("Expected error for invalid direction: %v", err)
		} else {
			t.Error("Expected error for invalid direction, but got success")
		}
	})
}

// =============================================================================
// 2. normalized + score_base parameters
// =============================================================================

func TestDegreeNormalized(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, cleanup := setupDegreeTestGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("normalized_false", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({normalized: false}) YIELD nodeId, degree, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		logDegreeRows(t, resp)
	})

	t.Run("normalized_true_score_base_max", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({normalized: true, score_base: 'max'}) YIELD nodeId, degree, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		logDegreeRows(t, resp)
		// Max degree is 3 (A and C), so normalized score for A,C = 1.0, B = 2/3 ≈ 0.667
		for _, row := range resp.Rows {
			sc, _ := row.Get(2)
			score, ok := sc.(float64)
			if !ok {
				t.Errorf("score should be float64, got %T", sc)
				continue
			}
			if score < 0 || score > 1.0 {
				t.Errorf("Normalized score should be in [0,1], got %v", score)
			}
		}
	})

	t.Run("normalized_true_score_base_count", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({normalized: true, score_base: 'count'}) YIELD nodeId, degree, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		logDegreeRows(t, resp)
		// node count - 1 = 2, so score = degree / 2
		for _, row := range resp.Rows {
			sc, _ := row.Get(2)
			score, ok := sc.(float64)
			if !ok {
				t.Errorf("score should be float64, got %T", sc)
				continue
			}
			if score < 0 {
				t.Errorf("Normalized score should be >= 0, got %v", score)
			}
		}
	})

	t.Run("invalid_score_base", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.degree({normalized: true, score_base: 'invalid'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Expected error for invalid score_base: %v", err)
		} else {
			t.Error("Expected error for invalid score_base, but got success")
		}
	})
}

// =============================================================================
// 3. weight parameter: string, list, int/float32/float64, nonexistent property
// =============================================================================

func TestDegreeWeight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, cleanup := setupDegreeTestGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("weight_int", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'w_int'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("int weight (10, 20, 5, 15):")
		logDegreeRows(t, resp)
	})

	t.Run("weight_float32", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'w_f32'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("float32 weight (1.5, 3.2, 0.9, 2.0) — BUG: values truncated to int:")
		logDegreeRows(t, resp)
	})

	t.Run("weight_float64", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'w_f64'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("float64 weight (2.7, 4.8, 0.3, 1.1) — BUG: values truncated to int:")
		logDegreeRows(t, resp)
	})

	t.Run("weight_list_multi", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: ['w_int', 'w_f64']}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("multi-weight ['w_int', 'w_f64']:")
		logDegreeRows(t, resp)
	})

	t.Run("weight_nonexistent_property", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'nonexistent'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Logf("Nonexistent weight property error: %v", err)
		} else {
			t.Log("Nonexistent weight property result (should be 0 or error):")
			logDegreeRows(t, resp)
		}
	})

	t.Run("weight_with_direction_in", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'w_int', direction: 'in'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("int weight + direction=in:")
		logDegreeRows(t, resp)
	})

	t.Run("weight_with_normalized", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'w_int', normalized: true}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("int weight + normalized=true:")
		logDegreeRows(t, resp)
	})
}

// =============================================================================
// 4. ids parameter: subset of nodes
// =============================================================================

func TestDegreeIds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, cleanup := setupDegreeTestGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// Get node IDs first
	resp, err := noAuthClient.Gql(ctx, `MATCH (n:Person) RETURN id(n) AS nid ORDER BY n.name`, qc)
	if err != nil {
		t.Fatalf("Failed to get node IDs: %v", err)
	}
	var nodeIDs []string
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		nodeIDs = append(nodeIDs, fmt.Sprintf("%v", nid))
	}
	t.Logf("Node IDs: %v", nodeIDs)

	t.Run("single_node", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.degree({ids: ['%s']}) YIELD nodeId, degree, inDegree, outDegree`, nodeIDs[0])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		logDegreeRows(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row for single node, got %d", resp.RowCount)
		}
	})

	t.Run("two_nodes", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.degree({ids: ['%s', '%s']}) YIELD nodeId, degree`, nodeIDs[0], nodeIDs[1])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		logDegreeRows(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows, got %d", resp.RowCount)
		}
	})

	t.Run("empty_ids_list", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({ids: []}) YIELD nodeId, degree`, qc)
		if err != nil {
			t.Logf("Empty ids list error: %v", err)
		} else {
			t.Logf("Empty ids list: %d rows (should be all nodes or 0)", resp.RowCount)
			logDegreeRows(t, resp)
		}
	})

	t.Run("nonexistent_id", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({ids: ['n:999999']}) YIELD nodeId, degree`, qc)
		if err != nil {
			t.Logf("Nonexistent ID error: %v", err)
		} else {
			t.Logf("Nonexistent ID: %d rows", resp.RowCount)
			logDegreeRows(t, resp)
		}
	})
}

// =============================================================================
// 5. limit + order parameters
// =============================================================================

func TestDegreeLimitOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, cleanup := setupDegreeTestGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("limit_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({limit: 1}) YIELD nodeId, degree`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		logDegreeRows(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row with limit=1, got %d", resp.RowCount)
		}
	})

	t.Run("limit_2", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({limit: 2}) YIELD nodeId, degree`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		logDegreeRows(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows with limit=2, got %d", resp.RowCount)
		}
	})

	t.Run("limit_minus1_all", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({limit: -1}) YIELD nodeId, degree`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		logDegreeRows(t, resp)
		if resp.RowCount != 3 {
			t.Errorf("Expected 3 rows with limit=-1, got %d", resp.RowCount)
		}
	})

	t.Run("order_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({order: 'desc'}) YIELD nodeId, degree, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=desc:")
		logDegreeRows(t, resp)
		// Verify descending order
		var prev float64 = 999999
		for _, row := range resp.Rows {
			sc, _ := row.Get(2)
			score := sc.(float64)
			if score > prev {
				t.Errorf("Not in descending order: %v > %v", score, prev)
			}
			prev = score
		}
	})

	t.Run("order_asc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({order: 'asc'}) YIELD nodeId, degree, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=asc:")
		logDegreeRows(t, resp)
		// Verify ascending order
		var prev float64 = -1
		for _, row := range resp.Rows {
			sc, _ := row.Get(2)
			score := sc.(float64)
			if score < prev {
				t.Errorf("Not in ascending order: %v < %v", score, prev)
			}
			prev = score
		}
	})

	t.Run("order_desc_limit_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({order: 'desc', limit: 1}) YIELD nodeId, degree, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=desc, limit=1 (top node):")
		logDegreeRows(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row, got %d", resp.RowCount)
		}
	})

	t.Run("limit_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({limit: 0}) YIELD nodeId, degree`, qc)
		if err != nil {
			t.Logf("limit=0 error: %v", err)
		} else {
			t.Logf("limit=0: %d rows", resp.RowCount)
			logDegreeRows(t, resp)
		}
	})
}

// =============================================================================
// 6. Run modes: stream, stats, write
// =============================================================================

func TestDegreeRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, cleanup := setupDegreeTestGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree.stream({direction: 'both', order: 'desc'}) YIELD nodeId, degree, score`, qc)
		if err != nil {
			t.Fatalf("stream mode failed: %v", err)
		}
		t.Log("stream mode (direction=both, order=desc):")
		logDegreeRows(t, resp)
	})

	t.Run("stream_mode_with_weight", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree.stream({weight: 'w_int'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("stream mode with weight failed: %v", err)
		}
		t.Log("stream mode with weight=w_int:")
		logDegreeRows(t, resp)
	})

	t.Run("stats_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Fatalf("stats mode failed: %v", err)
		}
		t.Log("stats mode:")
		logDegreeRows(t, resp)
	})

	t.Run("stats_mode_with_direction", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree.stats({direction: 'out'}) YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Fatalf("stats mode with direction failed: %v", err)
		}
		t.Log("stats mode (direction=out):")
		logDegreeRows(t, resp)
	})

	t.Run("write_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree.write({direction: 'both'}, {db: {property: 'deg_score'}}) YIELD task_id, nodesWritten, computeTimeMs, writeTimeMs`, qc)
		if err != nil {
			t.Logf("write mode error (may not be supported): %v", err)
			return
		}
		t.Log("write mode:")
		logDegreeRows(t, resp)

		// Verify written property
		time.Sleep(500 * time.Millisecond)
		verifyResp, err := noAuthClient.Gql(ctx, `MATCH (n:Person) RETURN n.name, n.deg_score ORDER BY n.name`, qc)
		if err != nil {
			t.Logf("Verify written property failed: %v", err)
			return
		}
		t.Log("Written property values:")
		logDegreeRows(t, verifyResp)
	})

	t.Run("write_mode_multi_column", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree.write({direction: 'both'}, {db: {property: {score: 'deg_s', inDegree: 'deg_in'}}}) YIELD task_id, nodesWritten`, qc)
		if err != nil {
			t.Logf("write multi-column error (may not be supported): %v", err)
			return
		}
		t.Log("write mode multi-column:")
		logDegreeRows(t, resp)

		time.Sleep(500 * time.Millisecond)
		verifyResp, err := noAuthClient.Gql(ctx, `MATCH (n:Person) RETURN n.name, n.deg_s, n.deg_in ORDER BY n.name`, qc)
		if err != nil {
			t.Logf("Verify failed: %v", err)
			return
		}
		t.Log("Written multi-column values:")
		logDegreeRows(t, verifyResp)
	})
}

// =============================================================================
// 7. Combined parameters
// =============================================================================

func TestDegreeCombinedParams(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, cleanup := setupDegreeTestGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("direction_out_weight_int_limit_2_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({direction: 'out', weight: 'w_int', limit: 2, order: 'desc'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("direction=out, weight=w_int, limit=2, order=desc:")
		logDegreeRows(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows, got %d", resp.RowCount)
		}
	})

	t.Run("direction_in_normalized_weight_int", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({direction: 'in', normalized: true, weight: 'w_int'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("direction=in, normalized=true, weight=w_int:")
		logDegreeRows(t, resp)
	})

	t.Run("weight_float64_normalized_order_asc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'w_f64', normalized: true, order: 'asc'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("weight=w_f64, normalized=true, order=asc (BUG: float truncated):")
		logDegreeRows(t, resp)
	})
}
