//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestArticulationPointsAlgoInfo(t *testing.T) {
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
		if name == "algo.articulationpoints" || name == "algo.articulation_points" || name == "algo.ap" {
			found = true
			t.Logf("\n=== %s ===", name)
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
	if !found {
		t.Fatal("articulationpoints algorithm not found in show algos")
	}
}

// setupAPGraph creates a graph with known articulation points.
// Topology:
//
//	A — B — C
//	    |
//	    D — E
//
// B is an articulation point (removing B disconnects A from C,D,E)
// D is an articulation point (removing D disconnects E from the rest)
// Edges are bidirectional.
func setupAPGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}

	graphName := fmt.Sprintf("test_ap_%d", time.Now().UnixMilli())
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
	nr, err := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}

	ids := nr.NodeIDs
	// Bidirectional edges: A-B, B-C, B-D, D-E
	edges := []*gqldb.EdgeData{
		{Label: "LINK", FromNodeID: ids[0], ToNodeID: ids[1]},
		{Label: "LINK", FromNodeID: ids[1], ToNodeID: ids[0]},
		{Label: "LINK", FromNodeID: ids[1], ToNodeID: ids[2]},
		{Label: "LINK", FromNodeID: ids[2], ToNodeID: ids[1]},
		{Label: "LINK", FromNodeID: ids[1], ToNodeID: ids[3]},
		{Label: "LINK", FromNodeID: ids[3], ToNodeID: ids[1]},
		{Label: "LINK", FromNodeID: ids[3], ToNodeID: ids[4]},
		{Label: "LINK", FromNodeID: ids[4], ToNodeID: ids[3]},
	}
	_, err = noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
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

func apLog(t *testing.T, resp *gqldb.Response) {
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
// 1. Basic run — find articulation points
// =============================================================================

func TestArticulationPointsBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupAPGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.articulationpoints() YIELD nodeId`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	apLog(t, resp)

	// B and D should be articulation points
	foundB, foundD := false, false
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		if fmt.Sprintf("%v", nid) == ids[1] {
			foundB = true
		}
		if fmt.Sprintf("%v", nid) == ids[3] {
			foundD = true
		}
	}
	if !foundB {
		t.Errorf("B (%s) should be an articulation point", ids[1])
	}
	if !foundD {
		t.Errorf("D (%s) should be an articulation point", ids[3])
	}
	t.Logf("B found: %v, D found: %v", foundB, foundD)
}

// =============================================================================
// 2. Graph with no articulation points (complete graph / cycle)
// =============================================================================

func TestArticulationPointsNone(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := fmt.Sprintf("test_ap_none_%d", time.Now().UnixMilli())
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
	nodes := []*gqldb.NodeData{
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "X"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "Y"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "Z"}},
	}
	nr, _ := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	// Triangle: X-Y, Y-Z, Z-X (no articulation points)
	ids := nr.NodeIDs
	edges := []*gqldb.EdgeData{
		{Label: "L", FromNodeID: ids[0], ToNodeID: ids[1]},
		{Label: "L", FromNodeID: ids[1], ToNodeID: ids[0]},
		{Label: "L", FromNodeID: ids[1], ToNodeID: ids[2]},
		{Label: "L", FromNodeID: ids[2], ToNodeID: ids[1]},
		{Label: "L", FromNodeID: ids[2], ToNodeID: ids[0]},
		{Label: "L", FromNodeID: ids[0], ToNodeID: ids[2]},
	}
	noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	qc := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := noAuthClient.Gql(ctx, `CALL algo.articulationpoints() YIELD nodeId`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	t.Logf("Triangle graph (no articulation points): %d results", resp.RowCount)
	apLog(t, resp)
	if resp.RowCount != 0 {
		t.Errorf("Triangle should have 0 articulation points, got %d", resp.RowCount)
	}
}

// =============================================================================
// 3. All parameters from show algos
// =============================================================================

func TestArticulationPointsParams(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupAPGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("limit_1", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articulationpoints({limit: 1}) YIELD nodeId`, qc)
		if err != nil {
			t.Logf("limit error: %v", err)
			return
		}
		apLog(t, resp)
		if resp.RowCount != 1 {
			t.Errorf("Expected 1 row, got %d", resp.RowCount)
		}
	})

	t.Run("order_asc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articulationpoints({order: 'asc'}) YIELD nodeId`, qc)
		if err != nil {
			t.Logf("order error: %v", err)
			return
		}
		apLog(t, resp)
	})

	t.Run("order_desc", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articulationpoints({order: 'desc'}) YIELD nodeId`, qc)
		if err != nil {
			t.Logf("order error: %v", err)
			return
		}
		apLog(t, resp)
	})
}

// =============================================================================
// 4. Run modes: stream, stats, write
// =============================================================================

func TestArticulationPointsRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupAPGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articulationpoints.stream() YIELD nodeId`, qc)
		if err != nil {
			t.Logf("stream error: %v", err)
			return
		}
		t.Log("stream mode:")
		apLog(t, resp)
	})

	t.Run("stats_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articulationpoints.stats() YIELD nodeCount`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
		t.Log("stats mode:")
		apLog(t, resp)
	})

	t.Run("write_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.articulationpoints.write({}, {db: {property: 'is_ap'}}) YIELD task_id`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		apLog(t, resp)

		time.Sleep(500 * time.Millisecond)
		vResp, err := noAuthClient.Gql(ctx, `MATCH (n:N) RETURN n.name, n.is_ap ORDER BY n.name`, qc)
		if err != nil {
			t.Logf("Verify failed: %v", err)
			return
		}
		t.Log("Written is_ap values:")
		apLog(t, vResp)
	})
}

// =============================================================================
// 5. Complex topology: multiple articulation points
// =============================================================================

func TestArticulationPointsComplex(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := fmt.Sprintf("test_ap_cx_%d", time.Now().UnixMilli())
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
	// Barbell graph: two triangles connected by a bridge
	// Triangle 1: A-B-C-A, Triangle 2: D-E-F-D, Bridge: C-D
	// C and D are articulation points
	nodes := []*gqldb.NodeData{
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "D"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "E"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "F"}},
	}
	nr, _ := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	ids := nr.NodeIDs

	biEdge := func(from, to string) []*gqldb.EdgeData {
		return []*gqldb.EdgeData{
			{Label: "L", FromNodeID: from, ToNodeID: to},
			{Label: "L", FromNodeID: to, ToNodeID: from},
		}
	}
	var edges []*gqldb.EdgeData
	// Triangle 1: A-B, B-C, C-A
	edges = append(edges, biEdge(ids[0], ids[1])...)
	edges = append(edges, biEdge(ids[1], ids[2])...)
	edges = append(edges, biEdge(ids[2], ids[0])...)
	// Bridge: C-D
	edges = append(edges, biEdge(ids[2], ids[3])...)
	// Triangle 2: D-E, E-F, F-D
	edges = append(edges, biEdge(ids[3], ids[4])...)
	edges = append(edges, biEdge(ids[4], ids[5])...)
	edges = append(edges, biEdge(ids[5], ids[3])...)

	noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Barbell: A=%s B=%s C=%s D=%s E=%s F=%s", ids[0], ids[1], ids[2], ids[3], ids[4], ids[5])

	qc := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := noAuthClient.Gql(ctx, `CALL algo.articulationpoints() YIELD nodeId`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	apLog(t, resp)

	// C and D should be articulation points
	foundC, foundD := false, false
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		nidStr := fmt.Sprintf("%v", nid)
		if nidStr == ids[2] {
			foundC = true
		}
		if nidStr == ids[3] {
			foundD = true
		}
	}
	if !foundC {
		t.Errorf("C (%s) should be an articulation point (bridge node)", ids[2])
	}
	if !foundD {
		t.Errorf("D (%s) should be an articulation point (bridge node)", ids[3])
	}
	if resp.RowCount != 2 {
		t.Errorf("Barbell graph should have exactly 2 articulation points, got %d", resp.RowCount)
	}
}
