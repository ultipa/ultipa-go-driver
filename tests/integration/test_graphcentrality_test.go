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

func TestGCAlgoInfo(t *testing.T) {
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
		if fmt.Sprintf("%v", m["name"]) == "algo.graphcentrality" {
			t.Logf("\n=== algo.graphcentrality ===")
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
}

// setupGCGraph creates a graph for graph centrality testing.
// Path: A—B—C—D—E (diameter=4, center=C, radius=2)
// C has eccentricity=2 (min), A and E have eccentricity=4 (max)
func setupGCGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	graphName := fmt.Sprintf("test_gc_%d", time.Now().UnixMilli())
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
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}}, // 0
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}}, // 1
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}}, // 2
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "D"}}, // 3
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "E"}}, // 4
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
	edges = append(edges, biEdge(ids[0], ids[1])...) // A-B
	edges = append(edges, biEdge(ids[1], ids[2])...) // B-C
	edges = append(edges, biEdge(ids[2], ids[3])...) // C-D
	edges = append(edges, biEdge(ids[3], ids[4])...) // D-E

	noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Graph: %s (path A-B-C-D-E), A=%s B=%s C=%s D=%s E=%s", graphName, ids[0], ids[1], ids[2], ids[3], ids[4])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func gcLog(t *testing.T, resp *gqldb.Response) {
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

func TestGCBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupGCGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality() YIELD nodeId, eccentricity, centrality, isCenter`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	gcLog(t, resp)

	if resp.RowCount != 5 {
		t.Errorf("Expected 5 rows, got %d", resp.RowCount)
	}

	// Path A-B-C-D-E:
	// A: max dist to E=4, eccentricity=4, centrality=1/4=0.25
	// B: max dist to E=3, eccentricity=3, centrality=1/3=0.333
	// C: max dist to A or E=2, eccentricity=2, centrality=1/2=0.5, isCenter=1
	// D: max dist to A=3, eccentricity=3, centrality=1/3=0.333
	// E: max dist to A=4, eccentricity=4, centrality=1/4=0.25
	type nodeResult struct {
		eccentricity int64
		centrality   float64
		isCenter     int64
	}
	results := make(map[string]nodeResult)
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		ecc, _ := row.Get(1)
		cen, _ := row.Get(2)
		ic, _ := row.Get(3)
		var eccVal int64
		var cenVal float64
		var icVal int64
		switch v := ecc.(type) {
		case int64:
			eccVal = v
		case float64:
			eccVal = int64(v)
		}
		switch v := cen.(type) {
		case float64:
			cenVal = v
		}
		switch v := ic.(type) {
		case int64:
			icVal = v
		case float64:
			icVal = int64(v)
		}
		results[fmt.Sprintf("%v", nid)] = nodeResult{eccVal, cenVal, icVal}
	}

	// Verify C is center
	cResult := results[ids[2]]
	t.Logf("C: eccentricity=%d, centrality=%v, isCenter=%d", cResult.eccentricity, cResult.centrality, cResult.isCenter)

	if cResult.eccentricity != 2 {
		t.Errorf("C eccentricity: expected 2, got %d", cResult.eccentricity)
	}
	if math.Abs(cResult.centrality-0.5) > 0.01 {
		t.Errorf("C centrality: expected ~0.5, got %v", cResult.centrality)
	}
	if cResult.isCenter != 1 {
		t.Errorf("C isCenter: expected 1, got %d", cResult.isCenter)
	}

	// Verify A and E are endpoints
	aResult := results[ids[0]]
	eResult := results[ids[4]]
	if aResult.eccentricity != 4 {
		t.Errorf("A eccentricity: expected 4, got %d", aResult.eccentricity)
	}
	if eResult.eccentricity != 4 {
		t.Errorf("E eccentricity: expected 4, got %d", eResult.eccentricity)
	}
	if aResult.isCenter != 0 {
		t.Errorf("A isCenter: expected 0, got %d", aResult.isCenter)
	}

	// centrality = 1/eccentricity
	for nid, r := range results {
		expected := 1.0 / float64(r.eccentricity)
		if math.Abs(r.centrality-expected) > 0.01 {
			t.Errorf("Node %s: centrality=%v, expected 1/%d=%v", nid, r.centrality, r.eccentricity, expected)
		}
	}
}

// =============================================================================
// 2. direction parameter
// =============================================================================

func TestGCDirection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupGCGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	results := make(map[string]string)
	for _, dir := range []string{"both", "in", "out"} {
		t.Run("direction_"+dir, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.graphcentrality({direction: '%s'}) YIELD nodeId, eccentricity, centrality`, dir)
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
			gcLog(t, resp)
			s := ""
			for _, row := range resp.Rows {
				nid, _ := row.Get(0)
				ecc, _ := row.Get(1)
				s += fmt.Sprintf("%v:%v,", nid, ecc)
			}
			results[dir] = s
		})
	}

	if results["in"] == results["out"] && results["in"] != "" {
		t.Log("NOTE: direction 'in' and 'out' return identical results")
	}

	t.Run("invalid_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({direction: 'invalid'}) YIELD nodeId, centrality`, qc)
		if err != nil {
			t.Logf("Expected error: %v", err)
		} else {
			t.Error("Expected error for invalid direction")
		}
	})

	t.Run("empty_direction", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({direction: ''}) YIELD nodeId, centrality`, qc)
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

func TestGCIds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupGCGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("single_node", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.graphcentrality({ids: ['%s']}) YIELD nodeId, eccentricity, centrality, isCenter`, ids[2])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		gcLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row, got %d", resp.RowCount)
		}
	})

	t.Run("two_nodes", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.graphcentrality({ids: ['%s', '%s']}) YIELD nodeId, eccentricity, centrality`, ids[0], ids[4])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		gcLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows, got %d", resp.RowCount)
		}
	})

	t.Run("empty_ids", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({ids: []}) YIELD nodeId, centrality`, qc)
		if err != nil {
			t.Logf("Empty ids error: %v", err)
		} else {
			t.Logf("Empty ids: %d rows", resp.RowCount)
		}
	})

	t.Run("nonexistent_id", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({ids: ['n:999999']}) YIELD nodeId, centrality`, qc)
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

func TestGCLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupGCGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	tests := []struct {
		name     string
		limit    int
		expected int64
	}{
		{"limit_1", 1, 1},
		{"limit_3", 3, 3},
		{"limit_5", 5, 5},
		{"limit_minus1_all", -1, 5},
		{"limit_0", 0, 0},
		{"limit_100", 100, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := fmt.Sprintf(`CALL algo.graphcentrality({limit: %d}) YIELD nodeId, centrality`, tc.limit)
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
		resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({limit: -999}) YIELD nodeId, centrality`, qc)
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
// 5. order parameter
// =============================================================================

func TestGCOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupGCGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("order_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({order: 'desc'}) YIELD nodeId, eccentricity, centrality`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		gcLog(t, resp)
		var prev float64 = math.MaxFloat64
		for _, row := range resp.Rows {
			cen, _ := row.Get(2)
			c := cen.(float64)
			if c > prev {
				t.Errorf("Not descending: %v > %v", c, prev)
			}
			prev = c
		}
	})

	t.Run("order_asc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({order: 'asc'}) YIELD nodeId, eccentricity, centrality`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		gcLog(t, resp)
		var prev float64 = -1
		for _, row := range resp.Rows {
			cen, _ := row.Get(2)
			c := cen.(float64)
			if c < prev {
				t.Errorf("Not ascending: %v < %v", c, prev)
			}
			prev = c
		}
	})

	t.Run("order_desc_limit_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({order: 'desc', limit: 1}) YIELD nodeId, centrality, isCenter`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		gcLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1, got %d", resp.RowCount)
		}
	})

	t.Run("order_invalid", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({order: 'invalid'}) YIELD nodeId, centrality`, qc)
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

func TestGCRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupGCGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality.stream({order: 'desc'}) YIELD nodeId, eccentricity, centrality, isCenter`, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream mode:")
		gcLog(t, resp)
	})

	t.Run("stats_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality.stats() YIELD nodeCount, radius, diameter, centerCount`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats mode:")
		gcLog(t, resp)

		// Path A-B-C-D-E: radius=2, diameter=4, centerCount=1 (C)
		if resp.RowCount > 0 {
			nc, _ := resp.Rows[0].Get(0)
			rad, _ := resp.Rows[0].Get(1)
			dia, _ := resp.Rows[0].Get(2)
			cc, _ := resp.Rows[0].Get(3)
			t.Logf("nodeCount=%v, radius=%v (expect 2), diameter=%v (expect 4), centerCount=%v (expect 1)", nc, rad, dia, cc)
		}
	})

	t.Run("write_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality.write({}, {db: {property: 'gc_score'}}) YIELD task_id`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		gcLog(t, resp)

		time.Sleep(1 * time.Second)
		vResp, err := noAuthClient.Gql(ctx, `MATCH (n:N) RETURN n.name, n.gc_score ORDER BY n.name`, qc)
		if err != nil {
			t.Logf("Verify failed: %v", err)
			return
		}
		t.Log("Written centrality scores:")
		gcLog(t, vResp)
	})
}

// =============================================================================
// 7. Combined parameters
// =============================================================================

func TestGCCombined(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupGCGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("direction_out_limit2_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({direction: 'out', limit: 2, order: 'desc'}) YIELD nodeId, eccentricity, centrality`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		gcLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2, got %d", resp.RowCount)
		}
	})

	t.Run("ids_order", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.graphcentrality({ids: ['%s', '%s', '%s'], order: 'desc'}) YIELD nodeId, eccentricity, centrality, isCenter`, ids[0], ids[2], ids[4])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("ids=[A,C,E], order=desc:")
		gcLog(t, resp)
	})

	t.Run("all_params", func(t *testing.T) {
		q := fmt.Sprintf(`CALL algo.graphcentrality({direction: 'both', ids: ['%s', '%s', '%s', '%s'], limit: 2, order: 'desc'}) YIELD nodeId, centrality`, ids[0], ids[1], ids[2], ids[3])
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("all params:")
		gcLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 (limit=2), got %d", resp.RowCount)
		}
	})
}

// =============================================================================
// 8. Crash test
// =============================================================================

func TestGCCrashTest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupGCGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("limit_negative_large", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({limit: -999}) YIELD nodeId, centrality`, qc)
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
// 9. Disconnected graph — centrality=0 for unreachable nodes
// =============================================================================

func TestGCDisconnected(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := fmt.Sprintf("test_gc_disc_%d", time.Now().UnixMilli())
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
	// Two disconnected components: A-B and C-D
	nodes := []*gqldb.NodeData{
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "D"}},
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
	edges = append(edges, biEdge(ids[0], ids[1])...) // A-B
	edges = append(edges, biEdge(ids[2], ids[3])...) // C-D
	noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality() YIELD nodeId, eccentricity, centrality, isCenter`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Log("Disconnected graph (A-B, C-D):")
	gcLog(t, resp)

	// Description says centrality=0 if disconnected — but each component is connected
	// eccentricity within component: A=1, B=1, C=1, D=1 — all are centers

	stats, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality.stats() YIELD nodeCount, radius, diameter, centerCount`, qc)
	if err != nil {
		t.Logf("stats error: %v", err)
	} else {
		t.Log("Disconnected graph stats:")
		gcLog(t, stats)
	}
}

// =============================================================================
// 10. Weight parameter (not in spec)
// =============================================================================

func TestGCWeight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupGCGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	_, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality({weight: 'some_prop'}) YIELD nodeId, centrality`, qc)
	if err != nil {
		t.Logf("weight not supported: %v", err)
	} else {
		t.Log("weight accepted")
	}
}

// =============================================================================
// 11. MiniCircle
// =============================================================================

func TestGCMiniCircle(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	qc := &gqldb.QueryConfig{GraphName: "miniCircle"}

	t.Run("top5", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.graphcentrality({order: 'desc', limit: 5}) YIELD nodeId, eccentricity, centrality, isCenter`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle top 5 graph centrality:")
		gcLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.graphcentrality.stats() YIELD nodeCount, radius, diameter, centerCount`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle graph centrality stats:")
		gcLog(t, resp)
	})
}
