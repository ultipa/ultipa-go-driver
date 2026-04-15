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

func TestClosenessAlgoInfo(t *testing.T) {
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
		if fmt.Sprintf("%v", m["name"]) == "algo.closeness" {
			t.Logf("\n=== algo.closeness ===")
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
}

// setupClosenessGraph creates a graph with known closeness characteristics.
// Star graph: Center→A, Center→B, Center→C, Center→D + chain D→E→F
// Center should have highest closeness (closest to all).
// F should have lowest closeness (farthest from others).
func setupClosenessGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	graphName := fmt.Sprintf("test_cl_%d", time.Now().UnixMilli())
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
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "Center"}}, // 0
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}},      // 1
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}},      // 2
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}},      // 3
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "D"}},      // 4
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "E"}},      // 5
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "F"}},      // 6
	}
	nr, _ := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	ids := nr.NodeIDs

	biEdge := func(from, to string) []*gqldb.EdgeData {
		return []*gqldb.EdgeData{
			{Label: "LINK", FromNodeID: from, ToNodeID: to},
			{Label: "LINK", FromNodeID: to, ToNodeID: from},
		}
	}
	var edges []*gqldb.EdgeData
	edges = append(edges, biEdge(ids[0], ids[1])...) // Center-A
	edges = append(edges, biEdge(ids[0], ids[2])...) // Center-B
	edges = append(edges, biEdge(ids[0], ids[3])...) // Center-C
	edges = append(edges, biEdge(ids[0], ids[4])...) // Center-D
	edges = append(edges, biEdge(ids[4], ids[5])...) // D-E
	edges = append(edges, biEdge(ids[5], ids[6])...) // E-F

	noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Graph: %s, Center=%s A=%s B=%s C=%s D=%s E=%s F=%s", graphName, ids[0], ids[1], ids[2], ids[3], ids[4], ids[5], ids[6])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func clLog(t *testing.T, resp *gqldb.Response) {
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

func TestClosenessBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupClosenessGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness() YIELD nodeId, score, rank`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	clLog(t, resp)

	if resp.RowCount != 7 {
		t.Errorf("Expected 7 rows, got %d", resp.RowCount)
	}

	scores := make(map[string]float64)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		sc, _ := row.Get(1)
		scores[fmt.Sprintf("%v", nid)] = sc.(float64)
	}

	centerScore := scores[ids[0]]
	fScore := scores[ids[6]]
	t.Logf("Center score=%v (should be highest), F score=%v (should be lowest)", centerScore, fScore)

	// Center should have highest closeness
	for nid, sc := range scores {
		if sc > centerScore && nid != ids[0] {
			t.Errorf("Node %s (score=%v) > Center (score=%v)", nid, sc, centerScore)
		}
	}

	// All scores should be > 0 and <= 1
	for nid, sc := range scores {
		if sc <= 0 {
			t.Errorf("Node %s: score should be > 0, got %v", nid, sc)
		}
		if sc > 1.0 {
			t.Errorf("Node %s: closeness score should be <= 1.0, got %v", nid, sc)
		}
	}

	// Rank should be consistent with score ordering
	ranks := make(map[string]int64)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		r, _ := row.Get(2)
		ranks[fmt.Sprintf("%v", nid)] = r.(int64)
	}
	if ranks[ids[0]] > ranks[ids[6]] {
		t.Errorf("Center rank=%d should be <= F rank=%d", ranks[ids[0]], ranks[ids[6]])
	}
}

// =============================================================================
// 2. direction parameter — both / in / out / invalid / empty
// =============================================================================

func TestClosenessDirection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupClosenessGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	results := make(map[string]string)
	for _, dir := range []string{"both", "in", "out"} {
		t.Run("direction_"+dir, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.closeness({direction: '%s'}) YIELD nodeId, score`, dir)
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
			clLog(t, resp)
			s := ""
			for _, row := range resp.Rows {
				nid, _ := row.Get(0)
				sc, _ := row.Get(1)
				s += fmt.Sprintf("%v:%v,", nid, sc)
			}
			results[dir] = s
		})
	}

	// in and out should differ for directed consideration
	if results["in"] == results["out"] && results["in"] != "" {
		t.Log("NOTE: direction 'in' and 'out' return identical results (same issue as betweenness #3?)")
	}
	// both should potentially differ from in/out
	if results["both"] == results["in"] && results["both"] != "" {
		t.Log("NOTE: 'both' and 'in' return identical results")
	}

	t.Run("invalid_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.closeness({direction: 'invalid'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Expected error: %v", err)
		} else {
			t.Error("Expected error for invalid direction")
		}
	})

	t.Run("empty_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.closeness({direction: ''}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Empty direction error: %v", err)
		} else {
			t.Log("Empty direction accepted")
		}
	})
}

// =============================================================================
// 3. ids parameter — single, multi, empty, nonexistent
// =============================================================================

func TestClosenessIds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupClosenessGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("single_node", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.closeness({ids: ['%s']}) YIELD nodeId, score`, ids[0])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		clLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row, got %d", resp.RowCount)
		}
	})

	t.Run("three_nodes", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.closeness({ids: ['%s', '%s', '%s']}) YIELD nodeId, score`, ids[0], ids[4], ids[6])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		clLog(t, resp)
		if resp.RowCount != 3 {
			t.Errorf("Expected 3 rows, got %d", resp.RowCount)
		}
	})

	t.Run("empty_ids", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness({ids: []}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Empty ids error: %v", err)
		} else {
			t.Logf("Empty ids: %d rows", resp.RowCount)
		}
	})

	t.Run("nonexistent_id", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness({ids: ['n:999999']}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Nonexistent ID error: %v", err)
		} else {
			t.Logf("Nonexistent ID: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 4. limit parameter — 0, 1, N, -1, exceeds nodes
// =============================================================================

func TestClosenessLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupClosenessGraph(t, ctx)
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
			q := fmt.Sprintf(`CALL algo.closeness({limit: %d}) YIELD nodeId, score`, tc.limit)
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Logf("limit=%d error: %v", tc.limit, err)
				return
			}
			t.Logf("limit=%d: %d rows", tc.limit, resp.RowCount)
			if resp.RowCount != tc.expected {
				t.Errorf("Expected %d rows, got %d", tc.expected, resp.RowCount)
			}
		})
	}

	// Negative boundary
	t.Run("limit_negative_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness({limit: -999}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("limit=-999 error: %v", err)
		} else {
			t.Logf("limit=-999: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 5. order parameter — asc, desc, invalid, verify ordering
// =============================================================================

func TestClosenessOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupClosenessGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("order_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness({order: 'desc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		clLog(t, resp)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness({order: 'asc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		clLog(t, resp)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness({order: 'desc', limit: 1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		clLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1, got %d", resp.RowCount)
		}
	})

	t.Run("order_invalid", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.closeness({order: 'invalid'}) YIELD nodeId, score`, qc)
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

func TestClosenessRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupClosenessGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness.stream({order: 'desc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream mode:")
		clLog(t, resp)
	})

	t.Run("stats_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats mode:")
		clLog(t, resp)

		if resp.RowCount > 0 {
			nc, _ := resp.Rows[0].Get(0)
			if nc.(int64) != 7 {
				t.Errorf("Expected nodeCount=7, got %v", nc)
			}
			minSc, _ := resp.Rows[0].Get(1)
			maxSc, _ := resp.Rows[0].Get(2)
			if minSc.(float64) > maxSc.(float64) {
				t.Errorf("minScore %v > maxScore %v", minSc, maxSc)
			}
		}
	})

	t.Run("write_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness.write({}, {db: {property: 'cl_score'}}) YIELD task_id`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		clLog(t, resp)

		time.Sleep(1 * time.Second)
		vResp, err := noAuthClient.Gql(ctx, `MATCH (n:N) RETURN n.name, n.cl_score ORDER BY n.name`, qc)
		if err != nil {
			t.Logf("Verify failed: %v", err)
			return
		}
		t.Log("Written closeness scores:")
		clLog(t, vResp)
	})
}

// =============================================================================
// 7. Combined parameters
// =============================================================================

func TestClosenessCombined(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupClosenessGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("direction_out_limit2_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness({direction: 'out', limit: 2, order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		clLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows, got %d", resp.RowCount)
		}
	})

	t.Run("ids_with_order", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.closeness({ids: ['%s', '%s', '%s'], order: 'desc'}) YIELD nodeId, score, rank`, ids[0], ids[4], ids[6])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("ids=[Center,D,F], order=desc:")
		clLog(t, resp)
	})

	t.Run("all_params", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.closeness({direction: 'both', ids: ['%s', '%s', '%s', '%s'], limit: 2, order: 'desc'}) YIELD nodeId, score, rank`, ids[0], ids[1], ids[4], ids[6])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("all params combined:")
		clLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows (limit=2), got %d", resp.RowCount)
		}
	})
}

// =============================================================================
// 8. Crash test — negative/extreme params
// =============================================================================

func TestClosenessCrashTest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupClosenessGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("limit_negative_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness({limit: -999}) YIELD nodeId, score`, qc)
		if err != nil {
			errStr := fmt.Sprintf("%v", err)
			if strings.Contains(errStr, "EOF") {
				t.Fatal("SERVER CRASHED with limit=-999")
			}
			t.Logf("limit=-999 error (no crash): %v", err)
		} else {
			t.Logf("limit=-999: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 9. Correctness: closeness formula verification
// =============================================================================

func TestClosenessFormula(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupClosenessGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.closeness({order: 'desc'}) YIELD nodeId, score`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// For star+chain: Center is distance 1 to A,B,C,D; 2 to E; 3 to F
	// closeness(Center) = (7-1) / (1+1+1+1+2+3) = 6/9 = 0.6667
	// F is distance 3 to Center,A,B,C; 2 to D; 1 to E
	// closeness(F) = 6 / (3+3+3+3+2+1) = 6/15 = 0.4

	scores := make(map[string]float64)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		sc, _ := row.Get(1)
		scores[fmt.Sprintf("%v", nid)] = sc.(float64)
	}

	expectedCenter := 6.0 / 9.0 // 0.6667
	expectedF := 6.0 / 15.0     // 0.4

	centerScore := scores[ids[0]]
	fScore := scores[ids[6]]

	t.Logf("Center: expected=%.4f, actual=%.4f", expectedCenter, centerScore)
	t.Logf("F: expected=%.4f, actual=%.4f", expectedF, fScore)

	if math.Abs(centerScore-expectedCenter) > 0.01 {
		t.Errorf("Center closeness: expected ~%.4f, got %.4f", expectedCenter, centerScore)
	}
	if math.Abs(fScore-expectedF) > 0.01 {
		t.Errorf("F closeness: expected ~%.4f, got %.4f", expectedF, fScore)
	}
}

// =============================================================================
// 10. Design review: missing weight parameter?
// =============================================================================

func TestClosenessWeight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupClosenessGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// closeness has no weight param per show algos; verify behavior
	_, err := noAuthClient.Gql(ctx, `CALL algo.closeness({weight: 'some_prop'}) YIELD nodeId, score`, qc)
	if err != nil {
		t.Logf("weight not supported (as expected per show algos): %v", err)
	} else {
		t.Log("weight parameter accepted (not in show algos spec)")
	}
}

// =============================================================================
// 11. On miniCircle
// =============================================================================

func TestClosenessMiniCircle(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	qc := &gqldb.QueryConfig{GraphName: "miniCircle"}

	t.Run("top5", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.closeness({order: 'desc', limit: 5}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle top 5 closeness:")
		clLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.closeness.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle closeness stats:")
		clLog(t, resp)
	})

	t.Run("direction_comparison", func(t *testing.T) {
		for _, dir := range []string{"both", "in", "out"} {
			q := fmt.Sprintf(`CALL algo.closeness({direction: '%s', order: 'desc', limit: 3}) YIELD nodeId, score`, dir)
			resp, err := testClient.Gql(ctx, q, qc)
			if err != nil {
				t.Logf("direction=%s error: %v", dir, err)
				continue
			}
			t.Logf("miniCircle direction=%s top 3:", dir)
			clLog(t, resp)
		}
	})
}
