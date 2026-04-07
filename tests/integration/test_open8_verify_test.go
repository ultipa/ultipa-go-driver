//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// setupOpen8Graph creates a test graph for open8 verification.
// Topology: A→B(int=10,f64=1.5), B→C(int=20,f64=3.2), A→C(int=5,f64=0.9), C→A(int=15,f64=2.0)
func setupOpen8Graph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}

	graphName := fmt.Sprintf("test_open8_%d", time.Now().UnixMilli())
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
	}
	nr, _ := noAuthClient.InsertNodes(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})

	edges := []*gqldb.EdgeData{
		{Label: "E", FromNodeID: nr.NodeIDs[0], ToNodeID: nr.NodeIDs[1], Properties: map[string]interface{}{"w_int": int64(10), "w_f64": float64(1.5)}},
		{Label: "E", FromNodeID: nr.NodeIDs[1], ToNodeID: nr.NodeIDs[2], Properties: map[string]interface{}{"w_int": int64(20), "w_f64": float64(3.2)}},
		{Label: "E", FromNodeID: nr.NodeIDs[0], ToNodeID: nr.NodeIDs[2], Properties: map[string]interface{}{"w_int": int64(5), "w_f64": float64(0.9)}},
		{Label: "E", FromNodeID: nr.NodeIDs[2], ToNodeID: nr.NodeIDs[0], Properties: map[string]interface{}{"w_int": int64(15), "w_f64": float64(2.0)}},
	}
	noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, nr.NodeIDs, cleanup
}

func o8log(t *testing.T, resp *gqldb.Response) {
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
// #1: seed=0 magic value
// =============================================================================

func TestOpen8_1_SeedZeroMagicValue(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen8Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// Run LPA with seed=0 twice — results should differ (random) if seed=0 is magic
	results := make([]string, 2)
	for i := 0; i < 2; i++ {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.lpa({seed: 0}) YIELD nodeId, communityId`, qc)
		if err != nil {
			t.Fatalf("Run %d failed: %v", i, err)
		}
		s := ""
		for _, row := range resp.Rows {
			nid, _ := row.Get(0)
			cid, _ := row.Get(1)
			s += fmt.Sprintf("%v:%v,", nid, cid)
		}
		results[i] = s
		t.Logf("seed=0 run %d: %s", i, s)
	}

	// Run with seed=42 twice — results should be identical (fixed)
	fixedResults := make([]string, 2)
	for i := 0; i < 2; i++ {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.lpa({seed: 42}) YIELD nodeId, communityId`, qc)
		if err != nil {
			t.Fatalf("Run %d failed: %v", i, err)
		}
		s := ""
		for _, row := range resp.Rows {
			nid, _ := row.Get(0)
			cid, _ := row.Get(1)
			s += fmt.Sprintf("%v:%v,", nid, cid)
		}
		fixedResults[i] = s
		t.Logf("seed=42 run %d: %s", i, s)
	}

	if results[0] != results[1] {
		t.Logf("BUG CONFIRMED: seed=0 produces different results (magic value = random)")
	} else {
		t.Logf("seed=0 produced same results (may need more runs to confirm)")
	}

	if fixedResults[0] != fixedResults[1] {
		t.Errorf("seed=42 should produce identical results, but got different")
	} else {
		t.Logf("seed=42 correctly produces identical results")
	}
}

// =============================================================================
// #2: degree float/double weight truncated to int
// =============================================================================

func TestOpen8_2_DegreeFloatWeightTruncated(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen8Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// Edges: A→B(1.5), B→C(3.2), A→C(0.9), C→A(2.0)
	// Expected weighted degree (both): A=1.5+0.9+2.0=4.4, B=1.5+3.2=4.7, C=3.2+0.9+2.0=6.1
	resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'w_f64'}) YIELD nodeId, degree, score, weightScores`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Log("float64 weight (1.5, 3.2, 0.9, 2.0) — expected fractional scores:")
	o8log(t, resp)

	for _, row := range resp.Rows {
		sc, _ := row.Get(2)
		score := sc.(float64)
		if score == float64(int64(score)) {
			t.Logf("BUG CONFIRMED: score=%v is integer (truncated)", score)
		}
	}
}

// =============================================================================
// #3: degree.write nodesWritten is nil
// =============================================================================

func TestOpen8_3_DegreeWriteNodesWrittenNil(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen8Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.degree.write({}, {db: {property: 'deg_v'}}) YIELD task_id, nodesWritten, computeTimeMs, writeTimeMs`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	o8log(t, resp)

	if resp.RowCount > 0 {
		nw, _ := resp.Rows[0].Get(1)
		if nw == nil {
			t.Log("BUG CONFIRMED: nodesWritten is nil")
		} else {
			t.Logf("nodesWritten=%v (fixed or not nil)", nw)
		}
	}
}

// =============================================================================
// #4: degree weightScores doesn't follow direction
// =============================================================================

func TestOpen8_4_DegreeWeightScoresDirection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen8Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// Edges: A→B(10), B→C(20), A→C(5), C→A(15)
	// direction=in: A gets C→A(15), B gets A→B(10), C gets B→C(20)+A→C(5)=25
	t.Run("direction_in", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({direction: 'in', weight: 'w_int'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		o8log(t, resp)
		for _, row := range resp.Rows {
			nid, _ := row.Get(0)
			deg, _ := row.Get(1)
			ws, _ := row.Get(3)
			wsMap := ws.(map[string]interface{})
			wsVal := wsMap["w_int"]
			if fmt.Sprintf("%v", wsVal) != fmt.Sprintf("%v", deg) {
				t.Logf("BUG CONFIRMED: node=%v degree=%v but weightScores={w_int:%v} (not matching direction)", nid, deg, wsVal)
			} else {
				t.Logf("node=%v degree=%v weightScores={w_int:%v} (matching)", nid, deg, wsVal)
			}
		}
	})
}

// =============================================================================
// #5: betweenness direction doesn't work
// =============================================================================

func TestOpen8_5_BetweennessDirectionNotWorking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen8Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	results := make(map[string]string)
	for _, dir := range []string{"both", "in", "out"} {
		q := fmt.Sprintf(`CALL algo.betweenness({direction: '%s'}) YIELD nodeId, score`, dir)
		resp, err := noAuthClient.Gql(ctx, q, qc)
		if err != nil {
			t.Fatalf("%s failed: %v", dir, err)
		}
		s := ""
		for _, row := range resp.Rows {
			nid, _ := row.Get(0)
			sc, _ := row.Get(1)
			s += fmt.Sprintf("%v:%v,", nid, sc)
		}
		results[dir] = s
		t.Logf("direction=%s: %s", dir, s)
	}

	if results["both"] == results["in"] && results["in"] == results["out"] {
		t.Log("BUG CONFIRMED: all three directions return identical results")
	} else {
		t.Log("Direction appears to work (results differ)")
	}
}

// =============================================================================
// #6: betweenness.write nodesWritten is nil
// =============================================================================

func TestOpen8_6_BetweennessWriteNodesWrittenNil(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen8Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness.write({}, {db: {property: 'btw_v'}}) YIELD task_id, nodesWritten`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	o8log(t, resp)

	if resp.RowCount > 0 {
		nw, _ := resp.Rows[0].Get(1)
		if nw == nil {
			t.Log("BUG CONFIRMED: nodesWritten is nil")
		} else {
			t.Logf("nodesWritten=%v", nw)
		}
	}
}

// =============================================================================
// #7: betweenness missing weight parameter
// =============================================================================

func TestOpen8_7_BetweennessMissingWeight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen8Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	_, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({weight: 'w_int'}) YIELD nodeId, score`, qc)
	if err != nil {
		t.Logf("BUG CONFIRMED: weight parameter not supported: %v", err)
	} else {
		t.Log("weight parameter now supported (fixed)")
	}
}

// =============================================================================
// #8: betweenness samplingSize=0 magic value
// =============================================================================

func TestOpen8_8_BetweennessSamplingSizeZero(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen8Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// samplingSize=0 should mean "sample 0 nodes" but actually means "all nodes"
	t.Run("samplingSize_0_means_all", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({samplingSize: 0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if resp.RowCount == 3 {
			t.Log("BUG CONFIRMED: samplingSize=0 returns all nodes (magic value)")
		}
		o8log(t, resp)
	})

	// samplingSize=1 should sample 1 node as source
	t.Run("samplingSize_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({samplingSize: 1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Log("samplingSize=1:")
		o8log(t, resp)
	})

	// samplingSize=-1 — what happens? (should be auto-sample if supported)
	t.Run("samplingSize_negative1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.betweenness({samplingSize: -1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("samplingSize=-1 error: %v", err)
		} else {
			t.Logf("samplingSize=-1 returned %d rows (no auto-sample support)", resp.RowCount)
			o8log(t, resp)
		}
	})
}
