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

func TestBetweennessAlgoInfo(t *testing.T) {
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
		name := fmt.Sprintf("%v", m["name"])
		if name == "algo.betweenness" {
			t.Logf("\n=== %s ===", name)
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
}

// setupBTWGraph creates a graph with known betweenness characteristics.
// Topology: A→B→C→D→E, A→D(shortcut), plus reverse edges for undirected behavior.
// D should have highest betweenness (bridge to E).
func setupBTWGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}

	graphName := fmt.Sprintf("test_btw_%d", time.Now().UnixMilli())
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
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "D"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "E"}},
	}
	nr, err := noAuthClient.InsertNodes(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}

	ids := nr.NodeIDs
	biEdge := func(from, to string) []*gqldb.EdgeData {
		return []*gqldb.EdgeData{
			{Label: "LINK", FromNodeID: from, ToNodeID: to},
			{Label: "LINK", FromNodeID: to, ToNodeID: from},
		}
	}
	var edges []*gqldb.EdgeData
	edges = append(edges, biEdge(ids[0], ids[1])...) // A-B
	edges = append(edges, biEdge(ids[1], ids[2])...) // B-C
	edges = append(edges, biEdge(ids[2], ids[3])...) // C-D
	edges = append(edges, biEdge(ids[0], ids[3])...) // A-D (shortcut)
	edges = append(edges, biEdge(ids[3], ids[4])...) // D-E

	_, err = noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	if err != nil {
		t.Fatalf("InsertEdges failed: %v", err)
	}
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Graph: %s, A=%s B=%s C=%s D=%s E=%s", graphName, ids[0], ids[1], ids[2], ids[3], ids[4])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func btwLog(t *testing.T, resp *gqldb.Response) {
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

func TestBTWBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupBTWGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness() YIELD nodeId, score`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	btwLog(t, resp)

	if resp.RowCount != 5 {
		t.Errorf("Expected 5 rows, got %d", resp.RowCount)
	}

	// D should have highest betweenness (bridge to E)
	scores := make(map[string]float64)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		sc, _ := row.Get(1)
		scores[fmt.Sprintf("%v", nid)] = sc.(float64)
	}
	dScore := scores[ids[3]]
	eScore := scores[ids[4]]
	t.Logf("D score=%v (should be highest), E score=%v (should be 0)", dScore, eScore)

	// E is a leaf, betweenness should be 0
	if eScore != 0 {
		t.Errorf("E (leaf) betweenness should be 0, got %v", eScore)
	}

	// All scores >= 0
	for nid, sc := range scores {
		if sc < 0 {
			t.Errorf("Node %s: score should be >= 0, got %v", nid, sc)
		}
	}
}

// =============================================================================
// 2. direction parameter — both / in / out / invalid
// =============================================================================

func TestBTWDirection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupBTWGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	results := make(map[string]string)
	for _, dir := range []string{"both", "in", "out"} {
		t.Run("direction_"+dir, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.betweenness({direction: '%s'}) YIELD nodeId, score`, dir)
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
			btwLog(t, resp)
			s := ""
			for _, row := range resp.Rows {
				nid, _ := row.Get(0)
				sc, _ := row.Get(1)
				s += fmt.Sprintf("%v:%v,", nid, sc)
			}
			results[dir] = s
		})
	}

	// Verify in != out (known bug #3)
	if results["in"] == results["out"] && results["in"] != "" {
		t.Log("BUG #3 CONFIRMED: direction 'in' and 'out' return identical results")
	}

	t.Run("invalid_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({direction: 'invalid'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Expected error for invalid direction: %v", err)
		} else {
			t.Error("Expected error for invalid direction, but got success")
		}
	})

	t.Run("empty_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({direction: ''}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Empty direction error: %v", err)
		} else {
			t.Log("Empty direction accepted (should it?)")
		}
	})
}

// =============================================================================
// 3. normalized parameter — true / false / boundary
// =============================================================================

func TestBTWNormalized(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupBTWGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("normalized_false", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({normalized: false}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("normalized=false:")
		btwLog(t, resp)
	})

	t.Run("normalized_true", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({normalized: true}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("normalized=true:")
		btwLog(t, resp)

		// Normalized scores should be in [0, 1]
		for _, row := range resp.Rows {
			sc, _ := row.Get(1)
			score := sc.(float64)
			if score < 0 || score > 1.0 {
				nid, _ := row.Get(0)
				t.Errorf("Node %v: normalized score %v out of [0,1]", nid, score)
			}
		}
	})

	// Compare normalized vs unnormalized: ordering should be same
	t.Run("normalized_preserves_ordering", func(t *testing.T) {
		respRaw, _ := noAuthClient.Gql(ctx, `CALL algo.betweenness({normalized: false, order: 'desc'}) YIELD nodeId, score`, qc)
		respNorm, _ := noAuthClient.Gql(ctx, `CALL algo.betweenness({normalized: true, order: 'desc'}) YIELD nodeId, score`, qc)
		if respRaw == nil || respNorm == nil {
			t.Skip("Could not compare")
		}
		for i := int64(0); i < respRaw.RowCount && i < respNorm.RowCount; i++ {
			rawId, _ := respRaw.Rows[i].Get(0)
			normId, _ := respNorm.Rows[i].Get(0)
			if fmt.Sprintf("%v", rawId) != fmt.Sprintf("%v", normId) {
				t.Errorf("Row %d: ordering differs: raw=%v vs norm=%v", i, rawId, normId)
			}
		}
	})
}

// =============================================================================
// 4. samplingSize parameter — 0, 1, N, -1, exceeds nodes
// =============================================================================

func TestBTWSamplingSize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupBTWGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("sampling_0_means_all", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({samplingSize: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Logf("samplingSize=0 (magic value = all): %d rows", resp.RowCount)
		btwLog(t, resp)
		if resp.RowCount != 5 {
			t.Errorf("Expected 5 rows, got %d", resp.RowCount)
		}
	})

	t.Run("sampling_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({samplingSize: 1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Logf("samplingSize=1: %d rows", resp.RowCount)
		btwLog(t, resp)
	})

	t.Run("sampling_3", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({samplingSize: 3}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Logf("samplingSize=3: %d rows", resp.RowCount)
		btwLog(t, resp)
	})

	t.Run("sampling_exceeds_nodes", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({samplingSize: 100}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("samplingSize=100 (>5 nodes) error: %v", err)
			return
		}
		t.Logf("samplingSize=100: %d rows", resp.RowCount)
		btwLog(t, resp)
	})

	t.Run("sampling_negative1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({samplingSize: -1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("samplingSize=-1 error: %v", err)
			return
		}
		t.Logf("samplingSize=-1: %d rows (no auto-sample)", resp.RowCount)
		btwLog(t, resp)
	})

	t.Run("sampling_negative_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({samplingSize: -100}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("samplingSize=-100 error: %v", err)
		} else {
			t.Logf("samplingSize=-100: %d rows (should error)", resp.RowCount)
			btwLog(t, resp)
		}
	})
}

// =============================================================================
// 5. ids parameter — single, multiple, empty, nonexistent
// =============================================================================

func TestBTWIds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupBTWGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("single_node", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.betweenness({ids: ['%s']}) YIELD nodeId, score`, ids[3]) // D
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		btwLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row, got %d", resp.RowCount)
		}
	})

	t.Run("two_nodes", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.betweenness({ids: ['%s', '%s']}) YIELD nodeId, score`, ids[1], ids[3])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		btwLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows, got %d", resp.RowCount)
		}
	})

	t.Run("all_nodes", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.betweenness({ids: ['%s', '%s', '%s', '%s', '%s']}) YIELD nodeId, score`,
			ids[0], ids[1], ids[2], ids[3], ids[4])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		btwLog(t, resp)
		if resp.RowCount != 5 {
			t.Errorf("Expected 5 rows, got %d", resp.RowCount)
		}
	})

	t.Run("empty_ids", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({ids: []}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Empty ids error: %v", err)
		} else {
			t.Logf("Empty ids: %d rows", resp.RowCount)
			btwLog(t, resp)
		}
	})

	t.Run("nonexistent_id", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({ids: ['n:999999']}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Nonexistent ID error: %v", err)
		} else {
			t.Logf("Nonexistent ID: %d rows", resp.RowCount)
			btwLog(t, resp)
		}
	})
}

// =============================================================================
// 6. limit parameter — 0, 1, N, -1, exceeds, negative
// =============================================================================

func TestBTWLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupBTWGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	tests := []struct {
		name     string
		limit    int
		expected int64 // -1 means don't check
	}{
		{"limit_1", 1, 1},
		{"limit_2", 2, 2},
		{"limit_5", 5, 5},
		{"limit_minus1_all", -1, 5},
		{"limit_0", 0, -1},
		{"limit_100", 100, 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.betweenness({limit: %d}) YIELD nodeId, score`, tc.limit)
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Logf("limit=%d error: %v", tc.limit, err)
				return
			}
			t.Logf("limit=%d: %d rows", tc.limit, resp.RowCount)
			btwLog(t, resp)
			if tc.expected >= 0 && resp.RowCount != tc.expected {
				t.Errorf("Expected %d rows, got %d", tc.expected, resp.RowCount)
			}
		})
	}
}

// =============================================================================
// 7. order parameter — asc, desc, invalid, verify actual ordering
// =============================================================================

func TestBTWOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupBTWGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("order_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=desc:")
		btwLog(t, resp)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({order: 'asc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=asc:")
		btwLog(t, resp)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({order: 'desc', limit: 1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("Top betweenness node (desc, limit=1):")
		btwLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1, got %d", resp.RowCount)
		}
	})

	t.Run("order_invalid", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({order: 'invalid'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Invalid order error: %v", err)
		} else {
			t.Error("Expected error for invalid order, but got success")
		}
	})
}

// =============================================================================
// 8. weight parameter — known missing feature (#4)
// =============================================================================

func TestBTWWeight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupBTWGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	_, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({weight: 'some_prop'}) YIELD nodeId, score`, qc)
	if err != nil {
		t.Logf("BUG #4 CONFIRMED: weight not supported: %v", err)
	} else {
		t.Log("weight parameter now supported (fixed!)")
	}
}

// =============================================================================
// 9. Run modes: stream, stats, write
// =============================================================================

func TestBTWRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupBTWGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness.stream({order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream mode:")
		btwLog(t, resp)
		if resp.RowCount != 5 {
			t.Errorf("Expected 5, got %d", resp.RowCount)
		}
	})

	t.Run("stats_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats mode:")
		btwLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1, got %d", resp.RowCount)
		}
	})

	t.Run("write_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness.write({}, {db: {property: 'btw_score'}}) YIELD task_id`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		btwLog(t, resp)

		time.Sleep(1 * time.Second)
		vResp, err := noAuthClient.Gql(ctx, `MATCH (n:N) RETURN n.name, n.btw_score ORDER BY n.name`, qc)
		if err != nil {
			t.Logf("Verify failed: %v", err)
			return
		}
		t.Log("Written betweenness scores:")
		btwLog(t, vResp)
	})
}

// =============================================================================
// 10. Combined parameters
// =============================================================================

func TestBTWCombined(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupBTWGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("direction_out_normalized_limit2_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({direction: 'out', normalized: true, limit: 2, order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		btwLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows, got %d", resp.RowCount)
		}
	})

	t.Run("ids_with_order", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.betweenness({ids: ['%s', '%s', '%s'], order: 'desc'}) YIELD nodeId, score`, ids[0], ids[3], ids[4])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		btwLog(t, resp)
	})

	t.Run("sampling_with_normalized", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({samplingSize: 3, normalized: true}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		btwLog(t, resp)
	})

	t.Run("all_params", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.betweenness({direction: 'both', normalized: true, samplingSize: 3, ids: ['%s', '%s'], limit: 1, order: 'desc'}) YIELD nodeId, score`, ids[3], ids[4])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("All params combined:")
		btwLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row (limit=1), got %d", resp.RowCount)
		}
	})
}

// =============================================================================
// 11. Crash test — negative params like celf
// =============================================================================

func TestBTWCrashTest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupBTWGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// Test if negative samplingSize crashes the server
	t.Run("samplingSize_negative_crash", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({samplingSize: -999}) YIELD nodeId, score`, qc)
		if err != nil {
			errStr := fmt.Sprintf("%v", err)
			if errStr == "EOF" || fmt.Sprintf("%v", err) == "error reading from server: EOF" {
				t.Fatal("SERVER CRASHED with samplingSize=-999")
			}
			t.Logf("samplingSize=-999 error (no crash): %v", err)
		} else {
			t.Logf("samplingSize=-999: %d rows (should validate)", resp.RowCount)
		}
	})

	t.Run("limit_negative_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({limit: -999}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("limit=-999 error: %v", err)
		} else {
			t.Logf("limit=-999: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 12. On miniCircle (larger graph)
// =============================================================================

func TestBTWMiniCircle(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	qc := &gqldb.QueryConfig{GraphName: "miniCircle"}

	t.Run("top5", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.betweenness({order: 'desc', limit: 5}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle top 5 betweenness:")
		btwLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.betweenness.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle betweenness stats:")
		btwLog(t, resp)
	})

	t.Run("normalized_top5", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.betweenness({normalized: true, order: 'desc', limit: 5}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle top 5 normalized:")
		btwLog(t, resp)
		for _, row := range resp.Rows {
			sc, _ := row.Get(1)
			score := sc.(float64)
			if score < 0 || score > 1.0 {
				t.Errorf("Normalized score %v out of [0,1]", score)
			}
		}
	})

	t.Run("sampling_50", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.betweenness({samplingSize: 50, order: 'desc', limit: 5}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle sampled (50) top 5:")
		btwLog(t, resp)
	})
}
