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

func TestHarmonicAlgoInfo(t *testing.T) {
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
		if fmt.Sprintf("%v", m["name"]) == "algo.harmonic" {
			t.Logf("\n=== algo.harmonic ===")
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
}

// setupHarmonicGraph: Star+chain — Center-A, Center-B, Center-C, Center-D, D-E, E-F
// Same as closeness test graph for comparison.
func setupHarmonicGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	graphName := fmt.Sprintf("test_hm_%d", time.Now().UnixMilli())
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
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "Center"}},
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

	t.Logf("Graph: %s, Center=%s A=%s B=%s C=%s D=%s E=%s F=%s", graphName, ids[0], ids[1], ids[2], ids[3], ids[4], ids[5], ids[6])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func hmLog(t *testing.T, resp *gqldb.Response) {
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
// 1. Basic run + correctness (formula verification)
// =============================================================================

func TestHarmonicBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupHarmonicGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic() YIELD nodeId, score, rank`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	hmLog(t, resp)

	if resp.RowCount != 7 {
		t.Errorf("Expected 7 rows, got %d", resp.RowCount)
	}

	scores := make(map[string]float64)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		sc, _ := row.Get(1)
		scores[fmt.Sprintf("%v", nid)] = sc.(float64)
	}

	// Harmonic centrality = sum(1/d(u,v)) for all v != u
	// Center: distances 1,1,1,1,2,3 → harmonic = 1/1+1/1+1/1+1/1+1/2+1/3 = 4.833
	// F: distances 3,3,3,3,2,1 → harmonic = 1/3+1/3+1/3+1/3+1/2+1/1 = 2.833
	expectedCenter := 1.0 + 1.0 + 1.0 + 1.0 + 0.5 + 1.0/3.0 // 4.833
	expectedF := 1.0/3.0 + 1.0/3.0 + 1.0/3.0 + 1.0/3.0 + 0.5 + 1.0 // 2.833

	centerScore := scores[ids[0]]
	fScore := scores[ids[6]]
	t.Logf("Center: expected=%.4f, actual=%.4f", expectedCenter, centerScore)
	t.Logf("F: expected=%.4f, actual=%.4f", expectedF, fScore)

	if math.Abs(centerScore-expectedCenter) > 0.05 {
		t.Errorf("Center harmonic: expected ~%.4f, got %.4f", expectedCenter, centerScore)
	}
	if math.Abs(fScore-expectedF) > 0.05 {
		t.Errorf("F harmonic: expected ~%.4f, got %.4f", expectedF, fScore)
	}

	// Center should have highest score
	for nid, sc := range scores {
		if sc > centerScore && nid != ids[0] {
			t.Errorf("Node %s (score=%v) > Center (score=%v)", nid, sc, centerScore)
		}
	}

	// All scores should be > 0
	for nid, sc := range scores {
		if sc <= 0 {
			t.Errorf("Node %s: score should be > 0, got %v", nid, sc)
		}
	}
}

// =============================================================================
// 2. direction parameter
// =============================================================================

func TestHarmonicDirection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHarmonicGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	results := make(map[string]string)
	for _, dir := range []string{"both", "in", "out"} {
		t.Run("direction_"+dir, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.harmonic({direction: '%s'}) YIELD nodeId, score`, dir)
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
			hmLog(t, resp)
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
	}

	t.Run("invalid_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({direction: 'invalid'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Expected error: %v", err)
		} else {
			t.Error("Expected error for invalid direction")
		}
	})

	t.Run("empty_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({direction: ''}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Empty direction error: %v", err)
		} else {
			t.Log("Empty direction accepted")
		}
	})
}

// =============================================================================
// 3. ids parameter
// =============================================================================

func TestHarmonicIds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupHarmonicGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("single_node", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.harmonic({ids: ['%s']}) YIELD nodeId, score`, ids[0])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		hmLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row, got %d (ids not working?)", resp.RowCount)
		}
	})

	t.Run("three_nodes", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.harmonic({ids: ['%s', '%s', '%s']}) YIELD nodeId, score`, ids[0], ids[4], ids[6])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		hmLog(t, resp)
		if resp.RowCount != 3 {
			t.Errorf("Expected 3 rows, got %d", resp.RowCount)
		}
	})

	t.Run("empty_ids", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({ids: []}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Empty ids error: %v", err)
		} else {
			t.Logf("Empty ids: %d rows", resp.RowCount)
		}
	})

	t.Run("nonexistent_id", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({ids: ['n:999999']}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("Nonexistent ID error: %v", err)
		} else {
			t.Logf("Nonexistent ID: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 4. limit parameter
// =============================================================================

func TestHarmonicLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHarmonicGraph(t, ctx)
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
			q := fmt.Sprintf(`CALL algo.harmonic({limit: %d}) YIELD nodeId, score`, tc.limit)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({limit: -999}) YIELD nodeId, score`, qc)
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
// 5. order parameter
// =============================================================================

func TestHarmonicOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHarmonicGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("order_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({order: 'desc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		hmLog(t, resp)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({order: 'asc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		hmLog(t, resp)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({order: 'desc', limit: 1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		hmLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1, got %d", resp.RowCount)
		}
	})

	t.Run("order_invalid", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({order: 'invalid'}) YIELD nodeId, score`, qc)
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

func TestHarmonicRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHarmonicGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic.stream({order: 'desc'}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream mode:")
		hmLog(t, resp)
	})

	t.Run("stats_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats mode:")
		hmLog(t, resp)
	})

	t.Run("write_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic.write({}, {db: {property: 'hm_score'}}) YIELD task_id`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		hmLog(t, resp)

		time.Sleep(1 * time.Second)
		vResp, err := noAuthClient.Gql(ctx, `MATCH (n:N) RETURN n.name, n.hm_score ORDER BY n.name`, qc)
		if err != nil {
			t.Logf("Verify failed: %v", err)
			return
		}
		t.Log("Written harmonic scores:")
		hmLog(t, vResp)
	})
}

// =============================================================================
// 7. Combined parameters
// =============================================================================

func TestHarmonicCombined(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupHarmonicGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("direction_out_limit2_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({direction: 'out', limit: 2, order: 'desc'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		hmLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2, got %d", resp.RowCount)
		}
	})

	t.Run("ids_order", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.harmonic({ids: ['%s', '%s', '%s'], order: 'desc'}) YIELD nodeId, score, rank`, ids[0], ids[4], ids[6])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		hmLog(t, resp)
	})

	t.Run("all_params", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.harmonic({direction: 'both', ids: ['%s', '%s', '%s', '%s'], limit: 2, order: 'desc'}) YIELD nodeId, score`, ids[0], ids[1], ids[4], ids[6])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("all params:")
		hmLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 (limit=2), got %d", resp.RowCount)
		}
	})
}

// =============================================================================
// 8. Crash test
// =============================================================================

func TestHarmonicCrashTest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHarmonicGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("limit_negative_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({limit: -999}) YIELD nodeId, score`, qc)
		if err != nil {
			if strings.Contains(fmt.Sprintf("%v", err), "EOF") {
				t.Fatal("SERVER CRASHED")
			}
			t.Logf("error (no crash): %v", err)
		} else {
			t.Logf("limit=-999: %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// 9. Disconnected graph — harmonic centrality's key advantage
// =============================================================================

func TestHarmonicDisconnected(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := fmt.Sprintf("test_hm_disc_%d", time.Now().UnixMilli())
	_, err := noAuthClient.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	if err != nil {
		t.Fatalf("Create graph failed: %v", err)
	}
	defer func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}()
	_ = noAuthClient.UseGraph(ctx, graphName)

	session, _ := noAuthClient.StartBulkImport(ctx, graphName, nil)
	// Component 1: A-B-C (triangle), Component 2: D-E (pair), F isolated
	nodes := []*gqldb.NodeData{
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "D"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "E"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "F"}}, // isolated
	}
	nr, _ := noAuthClient.InsertNodes(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	ids := nr.NodeIDs
	biEdge := func(from, to string) []*gqldb.EdgeData {
		return []*gqldb.EdgeData{
			{Label: "L", FromNodeID: from, ToNodeID: to},
			{Label: "L", FromNodeID: to, ToNodeID: from},
		}
	}
	var edges []*gqldb.EdgeData
	edges = append(edges, biEdge(ids[0], ids[1])...)
	edges = append(edges, biEdge(ids[1], ids[2])...)
	edges = append(edges, biEdge(ids[2], ids[0])...)
	edges = append(edges, biEdge(ids[3], ids[4])...)
	// F has no edges
	noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({order: 'desc'}) YIELD nodeId, score, rank`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Log("Disconnected graph (triangle A-B-C, pair D-E, isolated F):")
	hmLog(t, resp)

	// Harmonic centrality handles disconnected graphs:
	// A: 1/d(A,B)+1/d(A,C)+1/d(A,D=∞)+1/d(A,E=∞)+1/d(A,F=∞) = 1+1+0+0+0 = 2
	// D: 1/d(D,A=∞)+1/d(D,B=∞)+1/d(D,C=∞)+1/d(D,E)+1/d(D,F=∞) = 0+0+0+1+0 = 1
	// F: all distances infinite → harmonic = 0

	scores := make(map[string]float64)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		sc, _ := row.Get(1)
		scores[fmt.Sprintf("%v", nid)] = sc.(float64)
	}

	fScore := scores[ids[5]]
	t.Logf("F (isolated): score=%v (should be 0)", fScore)
	if fScore != 0 {
		t.Errorf("Isolated node F should have harmonic=0, got %v", fScore)
	}

	// Triangle nodes should have higher scores than pair nodes
	aScore := scores[ids[0]]
	dScore := scores[ids[3]]
	t.Logf("A (triangle): score=%v, D (pair): score=%v", aScore, dScore)
	if aScore <= dScore {
		t.Errorf("Triangle node A (%v) should have higher harmonic than pair node D (%v)", aScore, dScore)
	}

	// Stats
	stats, err := noAuthClient.Gql(ctx, `CALL algo.harmonic.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
	if err != nil {
		t.Logf("stats error: %v", err)
	} else {
		t.Log("Disconnected graph stats:")
		hmLog(t, stats)
	}
}

// =============================================================================
// 10. Design: missing weight/normalized?
// =============================================================================

func TestHarmonicDesign(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupHarmonicGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("weight_not_supported", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({weight: 'some_prop'}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("weight not supported: %v", err)
		} else {
			t.Log("weight accepted")
		}
	})

	t.Run("normalized_not_supported", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.harmonic({normalized: true}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("normalized not supported: %v", err)
		} else {
			t.Log("normalized accepted")
		}
	})
}

// =============================================================================
// 11. MiniCircle
// =============================================================================

func TestHarmonicMiniCircle(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	qc := &gqldb.QueryConfig{GraphName: "miniCircle"}

	t.Run("top5", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.harmonic({order: 'desc', limit: 5}) YIELD nodeId, score, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle top 5 harmonic:")
		hmLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.harmonic.stats() YIELD nodeCount, minScore, maxScore, avgScore`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle harmonic stats:")
		hmLog(t, resp)
	})

	t.Run("direction_comparison", func(t *testing.T) {
		for _, dir := range []string{"both", "in", "out"} {
			q := fmt.Sprintf(`CALL algo.harmonic({direction: '%s', order: 'desc', limit: 3}) YIELD nodeId, score`, dir)
			resp, err := testClient.Gql(ctx, q, qc)
			if err != nil {
				t.Logf("direction=%s error: %v", dir, err)
				continue
			}
			t.Logf("miniCircle direction=%s top 3:", dir)
			hmLog(t, resp)
		}
	})
}
