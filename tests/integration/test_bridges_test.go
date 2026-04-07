//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestBridgesAlgoInfo(t *testing.T) {
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
		if name == "algo.bridges" || name == "algo.bridge" {
			found = true
			t.Logf("\n=== %s ===", name)
			t.Logf("  description: %v", m["description"])
			t.Logf("  parameters: %v", m["parameters"])
			t.Logf("  returns: %v", m["returns"])
			t.Logf("  examples: %v", m["examples"])
		}
	}
	if !found {
		t.Fatal("bridges algorithm not found in show algos")
	}
}

// setupBridgesGraph creates a graph with known bridges.
// Topology: Triangle A-B-C-A + bridge C-D + Triangle D-E-F-D + pendant F-G
// Bridges: C-D (between two triangles), F-G (pendant edge)
func setupBridgesGraph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}

	graphName := fmt.Sprintf("test_br_%d", time.Now().UnixMilli())
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
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "F"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "G"}},
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
	// Triangle 1: A-B-C-A
	edges = append(edges, biEdge(ids[0], ids[1])...)
	edges = append(edges, biEdge(ids[1], ids[2])...)
	edges = append(edges, biEdge(ids[2], ids[0])...)
	// Bridge: C-D
	edges = append(edges, biEdge(ids[2], ids[3])...)
	// Triangle 2: D-E-F-D
	edges = append(edges, biEdge(ids[3], ids[4])...)
	edges = append(edges, biEdge(ids[4], ids[5])...)
	edges = append(edges, biEdge(ids[5], ids[3])...)
	// Pendant: F-G
	edges = append(edges, biEdge(ids[5], ids[6])...)

	_, err = noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	if err != nil {
		t.Fatalf("InsertEdges failed: %v", err)
	}
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	t.Logf("Graph: %s, A=%s B=%s C=%s D=%s E=%s F=%s G=%s", graphName, ids[0], ids[1], ids[2], ids[3], ids[4], ids[5], ids[6])

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

func brLog(t *testing.T, resp *gqldb.Response) {
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
// 1. Basic run — find bridges
// =============================================================================

func TestBridgesBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupBridgesGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.bridges() YIELD fromNodeId, toNodeId`, qc)
	if err != nil {
		// Try alternative YIELD columns
		resp, err = noAuthClient.Gql(ctx, `CALL algo.bridges() YIELD edgeId`, qc)
		if err != nil {
			// Try bare
			resp, err = noAuthClient.Gql(ctx, `CALL algo.bridges() YIELD nodeId`, qc)
			if err != nil {
				t.Fatalf("Failed all YIELD variants: %v", err)
			}
		}
	}
	brLog(t, resp)

	// Bridges should be: C-D and F-G
	t.Logf("Expected bridges: C(%s)-D(%s) and F(%s)-G(%s)", ids[2], ids[3], ids[5], ids[6])
}

// =============================================================================
// 2. Triangle graph — no bridges
// =============================================================================

func TestBridgesNone(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := fmt.Sprintf("test_br_none_%d", time.Now().UnixMilli())
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
	noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	qc := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := noAuthClient.Gql(ctx, `CALL algo.bridges() YIELD fromNodeId, toNodeId`, qc)
	if err != nil {
		resp, err = noAuthClient.Gql(ctx, `CALL algo.bridges() YIELD edgeId`, qc)
		if err != nil {
			resp, err = noAuthClient.Gql(ctx, `CALL algo.bridges() YIELD nodeId`, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
		}
	}
	t.Logf("Triangle (no bridges): %d rows", resp.RowCount)
	brLog(t, resp)
}

// =============================================================================
// 3. Linear graph — all edges are bridges
// =============================================================================

func TestBridgesLinear(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := fmt.Sprintf("test_br_lin_%d", time.Now().UnixMilli())
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
	// Linear: A-B-C-D (3 bridges)
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
	edges = append(edges, biEdge(ids[0], ids[1])...)
	edges = append(edges, biEdge(ids[1], ids[2])...)
	edges = append(edges, biEdge(ids[2], ids[3])...)
	noAuthClient.InsertEdges(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	qc := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := noAuthClient.Gql(ctx, `CALL algo.bridges() YIELD fromNodeId, toNodeId`, qc)
	if err != nil {
		resp, err = noAuthClient.Gql(ctx, `CALL algo.bridges() YIELD edgeId`, qc)
		if err != nil {
			resp, err = noAuthClient.Gql(ctx, `CALL algo.bridges() YIELD nodeId`, qc)
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
		}
	}
	t.Logf("Linear A-B-C-D (all edges are bridges, expect 3): %d rows", resp.RowCount)
	brLog(t, resp)
}

// =============================================================================
// 4. Run modes: stream, stats
// =============================================================================

func TestBridgesRunModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupBridgesGraph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("stream_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.bridges.stream() YIELD fromNodeId, toNodeId`, qc)
		if err != nil {
			resp, err = noAuthClient.Gql(ctx, `CALL algo.bridges.stream() YIELD edgeId`, qc)
			if err != nil {
				t.Logf("stream error: %v", err)
				return
			}
		}
		t.Log("stream mode:")
		brLog(t, resp)
	})

	t.Run("stats_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.bridges.stats() YIELD nodeCount, bridgeCount`, qc)
		if err != nil {
			// Try without bridgeCount
			resp, err = noAuthClient.Gql(ctx, `CALL algo.bridges.stats() YIELD nodeCount`, qc)
			if err != nil {
				t.Logf("stats error: %v", err)
				return
			}
		}
		t.Log("stats mode:")
		brLog(t, resp)
	})

	t.Run("write_mode", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.bridges.write({}, {db: {property: 'is_bridge'}}) YIELD task_id`, qc)
		if err != nil {
			t.Logf("write error: %v", err)
			return
		}
		brLog(t, resp)
	})
}

// =============================================================================
// 5. Verify on miniCircle
// =============================================================================

func TestBridgesMiniCircle(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	qc := &gqldb.QueryConfig{GraphName: "miniCircle"}

	// Stats first
	resp, err := testClient.Gql(ctx, `CALL algo.bridges.stats() YIELD nodeCount, bridgeCount`, qc)
	if err != nil {
		resp, err = testClient.Gql(ctx, `CALL algo.bridges.stats() YIELD nodeCount`, qc)
		if err != nil {
			t.Logf("stats error: %v", err)
			return
		}
	}
	t.Log("miniCircle bridges stats:")
	brLog(t, resp)
}
