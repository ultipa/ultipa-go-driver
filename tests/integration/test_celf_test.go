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

func TestCELFAlgoInfo(t *testing.T) {
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
		if name == "algo.celf" {
			t.Logf("\n=== %s ===", name)
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
}

// setupCELFGraph creates a social-network-like graph for CELF testing.
// Hub-and-spoke: A is connected to B,C,D,E,F; B-C connected; D-E connected; F isolated leaf.
// A should be the top seed (highest influence).
func setupCELFGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}

	graphName := fmt.Sprintf("test_celf_%d", time.Now().UnixMilli())
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
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "A"}}, // hub
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "C"}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "D"}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "E"}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "F"}}, // leaf
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "G"}}, // isolated
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "H"}}, // chain H-I-J
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "I"}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "J"}},
	}
	nr, err := noAuthClient.InsertNodes(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}

	ids := nr.NodeIDs
	edges := []*gqldb.EdgeData{
		// Hub A -> B,C,D,E,F
		{Label: "FOLLOWS", FromNodeID: ids[0], ToNodeID: ids[1]},
		{Label: "FOLLOWS", FromNodeID: ids[0], ToNodeID: ids[2]},
		{Label: "FOLLOWS", FromNodeID: ids[0], ToNodeID: ids[3]},
		{Label: "FOLLOWS", FromNodeID: ids[0], ToNodeID: ids[4]},
		{Label: "FOLLOWS", FromNodeID: ids[0], ToNodeID: ids[5]},
		// B-C cluster
		{Label: "FOLLOWS", FromNodeID: ids[1], ToNodeID: ids[2]},
		{Label: "FOLLOWS", FromNodeID: ids[2], ToNodeID: ids[1]},
		// D-E cluster
		{Label: "FOLLOWS", FromNodeID: ids[3], ToNodeID: ids[4]},
		{Label: "FOLLOWS", FromNodeID: ids[4], ToNodeID: ids[3]},
		// Chain: H->I->J
		{Label: "FOLLOWS", FromNodeID: ids[7], ToNodeID: ids[8]},
		{Label: "FOLLOWS", FromNodeID: ids[8], ToNodeID: ids[9]},
		// G is isolated (no edges)
	}
	_, err = noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	if err != nil {
		t.Fatalf("InsertEdges failed: %v", err)
	}
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Graph: %s, 10 nodes, A(hub)=%s G(isolated)=%s", graphName, ids[0], ids[6])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func celfLog(t *testing.T, resp *gqldb.Response) {
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
// 1. Basic run — default params
// =============================================================================

func TestCELFBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupCELFGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.celf() YIELD nodeId, spread, rank`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	celfLog(t, resp)

	// A (hub) should be rank 1 with highest spread
	if resp.RowCount > 0 {
		nid, _ := resp.Rows[0].Get(0)
		rank, _ := resp.Rows[0].Get(2)
		t.Logf("Top seed: nodeId=%v, rank=%v (expected A=%s)", nid, rank, ids[0])
	}

	// All spreads should be >= 1 (at least the node itself)
	for _, row := range resp.Rows {
		sp, _ := row.Get(1)
		spread := sp.(float64)
		if spread < 1.0 {
			nid, _ := row.Get(0)
			t.Errorf("Spread should be >= 1.0, got %v for node %v", spread, nid)
		}
	}
}

// =============================================================================
// 2. seedSetSize parameter
// =============================================================================

func TestCELFSeedSetSize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupCELFGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("seedSetSize_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 1}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		celfLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 seed, got %d", resp.RowCount)
		}
	})

	t.Run("seedSetSize_3", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 3}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		celfLog(t, resp)
		if resp.RowCount != 3 {
			t.Errorf("Expected 3 seeds, got %d", resp.RowCount)
		}
		// Ranks should be 1,2,3
		for i, row := range resp.Rows {
			r, _ := row.Get(2)
			expectedRank := int64(i + 1)
			if rank, ok := r.(int64); ok && rank != expectedRank {
				t.Errorf("Row %d: expected rank %d, got %d", i, expectedRank, rank)
			}
		}
	})

	t.Run("seedSetSize_exceeds_nodes", func(t *testing.T) {
		// Graph has 10 nodes, request 20 seeds
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 20}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Logf("seedSetSize > nodeCount error: %v", err)
			return
		}
		t.Logf("seedSetSize=20 (10 nodes): got %d rows", resp.RowCount)
		celfLog(t, resp)
	})

	t.Run("seedSetSize_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 0}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Logf("seedSetSize=0 error: %v", err)
		} else {
			t.Logf("seedSetSize=0: %d rows (should be error or 0)", resp.RowCount)
			celfLog(t, resp)
		}
	})

	t.Run("seedSetSize_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: -1}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Logf("seedSetSize=-1 error: %v", err)
		} else {
			t.Logf("seedSetSize=-1: %d rows", resp.RowCount)
			celfLog(t, resp)
		}
	})
}

// =============================================================================
// 3. monteCarloRuns parameter
// =============================================================================

func TestCELFMonteCarloRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupCELFGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("mc_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 3, monteCarloRuns: 1}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("monteCarloRuns=1 (low accuracy):")
		celfLog(t, resp)
	})

	t.Run("mc_1000", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 3, monteCarloRuns: 1000}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("monteCarloRuns=1000 (high accuracy):")
		celfLog(t, resp)
	})

	t.Run("mc_0", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.celf({monteCarloRuns: 0}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Logf("monteCarloRuns=0 error: %v", err)
		} else {
			t.Log("monteCarloRuns=0 did not error (should it?)")
		}
	})

	t.Run("mc_negative", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.celf({monteCarloRuns: -1}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Logf("monteCarloRuns=-1 error: %v", err)
		} else {
			t.Log("monteCarloRuns=-1 did not error (should it?)")
		}
	})
}

// =============================================================================
// 4. probability parameter
// =============================================================================

func TestCELFProbability(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupCELFGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("prob_0.01_low", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 3, probability: 0.01}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("probability=0.01 (low spread expected):")
		celfLog(t, resp)
	})

	t.Run("prob_0.5_medium", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 3, probability: 0.5}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("probability=0.5 (medium spread):")
		celfLog(t, resp)
	})

	t.Run("prob_1.0_certain", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 3, probability: 1.0}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Logf("probability=1.0 error: %v", err)
			return
		}
		t.Log("probability=1.0 (certain activation, max spread):")
		celfLog(t, resp)
		// With p=1.0, seed A should reach all connected nodes
	})

	t.Run("prob_0", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 3, probability: 0}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Logf("probability=0 error: %v", err)
			return
		}
		t.Log("probability=0 (no activation, spread=1 for each seed):")
		celfLog(t, resp)
		// With p=0, each seed can only influence itself, spread should be 1
		for _, row := range resp.Rows {
			sp, _ := row.Get(1)
			spread := sp.(float64)
			if spread != 1.0 {
				t.Errorf("With probability=0, spread should be 1.0, got %v", spread)
			}
		}
	})

	t.Run("prob_negative", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.celf({probability: -0.1}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Logf("probability=-0.1 error (expected): %v", err)
		} else {
			t.Error("probability=-0.1 should error but didn't")
		}
	})

	t.Run("prob_greater_than_1", func(t *testing.T) {
		_, err := noAuthClient.Gql(ctx, `CALL algo.celf({probability: 1.5}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Logf("probability=1.5 error (expected): %v", err)
		} else {
			t.Error("probability=1.5 should error but didn't")
		}
	})
}

// =============================================================================
// 5. limit + order
// =============================================================================

func TestCELFLimitOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupCELFGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("limit_2", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 5, limit: 2}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		celfLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows, got %d", resp.RowCount)
		}
	})

	t.Run("order_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 5, order: 'desc'}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=desc:")
		celfLog(t, resp)
		var prev float64 = math.MaxFloat64
		for _, row := range resp.Rows {
			sp, _ := row.Get(1)
			spread := sp.(float64)
			if spread > prev {
				t.Errorf("Not descending: %v > %v", spread, prev)
			}
			prev = spread
		}
	})

	t.Run("order_asc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 5, order: 'asc'}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("order=asc:")
		celfLog(t, resp)
	})
}

// =============================================================================
// 6. Run modes: stream, stats
// =============================================================================

func TestCELFRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupCELFGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf.stream({seedSetSize: 3}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream mode:")
		celfLog(t, resp)
	})

	t.Run("stats_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf.stats() YIELD nodeCount, minSpread, maxSpread, avgSpread`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats mode:")
		celfLog(t, resp)
	})

	t.Run("write_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf.write({seedSetSize: 3}, {db: {property: 'celf_spread'}}) YIELD task_id`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		celfLog(t, resp)

		time.Sleep(1 * time.Second)
		vResp, err := noAuthClient.Gql(ctx, `MATCH (n:Person) RETURN n.name, n.celf_spread ORDER BY n.name`, qc)
		if err != nil {
			t.Logf("Verify failed: %v", err)
			return
		}
		t.Log("Written spreads:")
		celfLog(t, vResp)
	})
}

// =============================================================================
// 7. Correctness: spread monotonicity
// =============================================================================

func TestCELFSpreadMonotonicity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupCELFGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// With more seeds, total spread should not decrease (submodularity)
	spreads := make(map[int]float64)
	for _, k := range []int{1, 3, 5} {
		q := fmt.Sprintf(`CALL algo.celf({seedSetSize: %d, monteCarloRuns: 500, probability: 0.5}) YIELD nodeId, spread, rank`, k)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("seedSetSize=%d failed: %v", k, err)
		}
		// Total spread is the spread of the last seed (cumulative)
		if resp.RowCount > 0 {
			lastRow := resp.Rows[resp.RowCount-1]
			sp, _ := lastRow.Get(1)
			spreads[k] = sp.(float64)
			t.Logf("seedSetSize=%d: last spread=%v", k, sp)
		}
	}

	// spread(k=1) <= spread(k=3) <= spread(k=5)
	if spreads[1] > spreads[3] {
		t.Errorf("Monotonicity violated: spread(k=1)=%v > spread(k=3)=%v", spreads[1], spreads[3])
	}
	if spreads[3] > spreads[5] {
		t.Errorf("Monotonicity violated: spread(k=3)=%v > spread(k=5)=%v", spreads[3], spreads[5])
	}
}

// =============================================================================
// 8. Probability impact: higher p -> higher spread
// =============================================================================

func TestCELFProbabilityImpact(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupCELFGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	spreads := make(map[float64]float64)
	for _, p := range []float64{0.01, 0.1, 0.5, 0.9} {
		q := fmt.Sprintf(`CALL algo.celf({seedSetSize: 1, monteCarloRuns: 500, probability: %v}) YIELD nodeId, spread, rank`, p)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("probability=%v failed: %v", p, err)
		}
		if resp.RowCount > 0 {
			sp, _ := resp.Rows[0].Get(1)
			spreads[p] = sp.(float64)
			t.Logf("probability=%v: spread=%v", p, sp)
		}
	}

	// Higher probability should generally lead to higher spread
	if spreads[0.01] > spreads[0.9] {
		t.Errorf("Expected spread(p=0.01) <= spread(p=0.9), got %v > %v", spreads[0.01], spreads[0.9])
	}
}

// =============================================================================
// 9. Combined parameters
// =============================================================================

func TestCELFCombined(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupCELFGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("all_params", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.celf({seedSetSize: 3, monteCarloRuns: 200, probability: 0.3, limit: 2, order: 'desc'}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("seedSetSize=3, mc=200, p=0.3, limit=2, order=desc:")
		celfLog(t, resp)
		if resp.RowCount != 2 {
			t.Errorf("Expected 2 rows (limit=2), got %d", resp.RowCount)
		}
	})
}

// =============================================================================
// 10. On miniCircle (larger graph)
// =============================================================================

func TestCELFMiniCircle(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	qc := &gqldb.QueryConfig{GraphName: "miniCircle"}

	t.Run("top5_seeds", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.celf({seedSetSize: 5, monteCarloRuns: 50, probability: 0.1}) YIELD nodeId, spread, rank`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("miniCircle top 5 seeds:")
		celfLog(t, resp)
	})

	t.Run("stats", func(t *testing.T) {
		resp, err := testClient.Gql(ctx, `CALL algo.celf.stats({seedSetSize: 5}) YIELD nodeCount, minSpread, maxSpread, avgSpread`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("miniCircle CELF stats:")
		celfLog(t, resp)
	})
}
