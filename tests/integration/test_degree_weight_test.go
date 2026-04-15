//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestDegreeAlgoInfo(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := testClient.Gql(ctx, `show algos`, nil)
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
		if name == "algo.degree" || name == "algo.weighted_degree" || name == "algo.wdegree" {
			fmt.Printf("\n=== %s ===\n", name)
			fmt.Printf("  description: %v\n", m["description"])
			fmt.Printf("  parameters: %v\n", m["parameters"])
			fmt.Printf("  returns: %v\n", m["returns"])
			fmt.Printf("  examples: %v\n", m["examples"])
		}
	}
}

func TestDegreeWithFloatWeight(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := fmt.Sprintf("test_degree_float_%d", time.Now().UnixMilli())

	// Create test graph
	_, err := noAuthClient.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	if err != nil {
		t.Fatalf("Create graph failed: %v", err)
	}
	defer func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		_, _ = noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}()

	_ = noAuthClient.UseGraph(ctx, graphName)

	// Start bulk import
	session, err := noAuthClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	if !session.Success {
		t.Fatalf("StartBulkImport not successful: %s", session.Message)
	}

	// Insert nodes
	nodes := []*gqldb.NodeData{
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "A"}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "C"}},
	}
	nodeResult, err := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	})
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}
	t.Logf("Inserted %d nodes, IDs: %v", nodeResult.NodeCount, nodeResult.NodeIDs)

	if len(nodeResult.NodeIDs) < 3 {
		t.Fatalf("Expected 3 node IDs, got %d", len(nodeResult.NodeIDs))
	}

	// Insert edges with float32 and float64 weight properties
	edges := []*gqldb.EdgeData{
		{Label: "KNOWS", FromNodeID: nodeResult.NodeIDs[0], ToNodeID: nodeResult.NodeIDs[1], Properties: map[string]interface{}{"weight_f32": float32(0.8), "weight_f64": float64(1.23456789)}},
		{Label: "KNOWS", FromNodeID: nodeResult.NodeIDs[1], ToNodeID: nodeResult.NodeIDs[2], Properties: map[string]interface{}{"weight_f32": float32(2.5), "weight_f64": float64(3.45678901)}},
		{Label: "KNOWS", FromNodeID: nodeResult.NodeIDs[0], ToNodeID: nodeResult.NodeIDs[2], Properties: map[string]interface{}{"weight_f32": float32(1.1), "weight_f64": float64(0.99999999)}},
	}
	_, err = noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{
		BulkImportSessionID: session.SessionID,
	})
	if err != nil {
		t.Fatalf("InsertEdges failed: %v", err)
	}
	t.Log("Inserted 3 edges with float32/float64 weight properties")

	// End bulk import
	_, err = noAuthClient.EndBulkImport(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("EndBulkImport failed: %v", err)
	}

	// Wait for data to be queryable
	time.Sleep(1 * time.Second)

	qc := &gqldb.QueryConfig{GraphName: graphName}

	// First show degree algo info to see exact syntax
	t.Run("show_degree_algo", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `show algos`, qc)
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
			if name == "algo.degree" || name == "algo.weighted_degree" || name == "algo.wdegree" {
				found = true
				t.Logf("\n=== %s ===", name)
				t.Logf("  description: %v", m["description"])
				t.Logf("  parameters: %v", m["parameters"])
				t.Logf("  returns: %v", m["returns"])
				t.Logf("  examples: %v", m["examples"])
			}
		}
		if !found {
			t.Log("No degree algorithm found in show algos")
		}
	})

	// Test: degree without weight (baseline)
	t.Run("degree_no_weight", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree() YIELD node, degree`, qc)
		if err != nil {
			t.Fatalf("degree without weight failed: %v", err)
		}
		t.Logf("degree (no weight): %d rows", resp.RowCount)
		for _, row := range resp.Rows {
			node, _ := row.Get(0)
			degree, _ := row.Get(1)
			t.Logf("  node=%v, degree=%v", node, degree)
		}
	})

	// Test: degree with float32 weight property
	t.Run("degree_float32_weight", func(t *testing.T) {
		// Try different parameter names
		queries := []string{
			`CALL algo.degree({weightProperty: 'weight_f32'}) YIELD node, degree`,
			`CALL algo.degree({weight: 'weight_f32'}) YIELD node, degree`,
		}
		for _, q := range queries {
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Logf("Query failed: %s\n  Error: %v", q, err)
				continue
			}
			t.Logf("degree (float32 weight) query: %s", q)
			t.Logf("  rows: %d", resp.RowCount)
			for _, row := range resp.Rows {
				node, _ := row.Get(0)
				degree, _ := row.Get(1)
				t.Logf("  node=%v, degree=%v (type: %T)", node, degree, degree)
			}
			return
		}
		t.Error("All degree with float32 weight queries failed")
	})

	// Test: degree with float64 weight property
	t.Run("degree_float64_weight", func(t *testing.T) {
		queries := []string{
			`CALL algo.degree({weightProperty: 'weight_f64'}) YIELD node, degree`,
			`CALL algo.degree({weight: 'weight_f64'}) YIELD node, degree`,
		}
		for _, q := range queries {
			resp, err := noAuthClient.Gql(ctx, q, qc)
			if err != nil {
				t.Logf("Query failed: %s\n  Error: %v", q, err)
				continue
			}
			t.Logf("degree (float64 weight) query: %s", q)
			t.Logf("  rows: %d", resp.RowCount)
			for _, row := range resp.Rows {
				node, _ := row.Get(0)
				degree, _ := row.Get(1)
				t.Logf("  node=%v, degree=%v (type: %T)", node, degree, degree)
			}
			return
		}
		t.Error("All degree with float64 weight queries failed")
	})
}

func TestDegreeWeightedFullYield(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := fmt.Sprintf("test_deg_fy_%d", time.Now().UnixMilli())
	_, err := noAuthClient.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	if err != nil {
		t.Fatalf("Create graph failed: %v", err)
	}
	defer func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}()
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
	nr, _ := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})

	edges := []*gqldb.EdgeData{
		{Label: "E", FromNodeID: nr.NodeIDs[0], ToNodeID: nr.NodeIDs[1], Properties: map[string]interface{}{"w": float64(1.5)}},
		{Label: "E", FromNodeID: nr.NodeIDs[1], ToNodeID: nr.NodeIDs[2], Properties: map[string]interface{}{"w": float64(2.3)}},
		{Label: "E", FromNodeID: nr.NodeIDs[0], ToNodeID: nr.NodeIDs[2], Properties: map[string]interface{}{"w": float64(0.7)}},
	}
	noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	qc := &gqldb.QueryConfig{GraphName: graphName}

	// Full YIELD with score and weightScores
	t.Run("full_yield_with_weight", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'w'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Logf("Rows: %d, Columns: %v", resp.RowCount, resp.Columns)
		for _, row := range resp.Rows {
			nodeId, _ := row.Get(0)
			degree, _ := row.Get(1)
			score, _ := row.Get(2)
			ws, _ := row.Get(3)
			t.Logf("  nodeId=%v, degree=%v(%T), score=%v(%T), weightScores=%v(%T)", nodeId, degree, degree, score, score, ws, ws)
		}
	})

	// Degree without weight for comparison
	t.Run("full_yield_no_weight", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree() YIELD nodeId, degree, score, inDegree, outDegree`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Logf("Rows: %d", resp.RowCount)
		for _, row := range resp.Rows {
			nodeId, _ := row.Get(0)
			degree, _ := row.Get(1)
			score, _ := row.Get(2)
			inD, _ := row.Get(3)
			outD, _ := row.Get(4)
			t.Logf("  nodeId=%v, degree=%v, score=%v(%T), inDegree=%v, outDegree=%v", nodeId, degree, score, score, inD, outD)
		}
	})
}

func TestDegreeWeightParamTypes(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := fmt.Sprintf("test_deg_pt_%d", time.Now().UnixMilli())
	_, err := noAuthClient.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	if err != nil {
		t.Fatalf("Create graph failed: %v", err)
	}
	defer func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}()
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
	nr, _ := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})

	// Edges with int, float32, float64 weight properties
	edges := []*gqldb.EdgeData{
		{Label: "E", FromNodeID: nr.NodeIDs[0], ToNodeID: nr.NodeIDs[1], Properties: map[string]interface{}{
			"w_int": int64(10), "w_f32": float32(1.5), "w_f64": float64(2.7),
		}},
		{Label: "E", FromNodeID: nr.NodeIDs[1], ToNodeID: nr.NodeIDs[2], Properties: map[string]interface{}{
			"w_int": int64(20), "w_f32": float32(3.2), "w_f64": float64(4.8),
		}},
		{Label: "E", FromNodeID: nr.NodeIDs[0], ToNodeID: nr.NodeIDs[2], Properties: map[string]interface{}{
			"w_int": int64(5), "w_f32": float32(0.9), "w_f64": float64(0.3),
		}},
	}
	noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	qc := &gqldb.QueryConfig{GraphName: graphName}

	// Test 1: weight as single string (int property)
	t.Run("weight_string_int", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'w_int'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Logf("weight='w_int' (int64 values: 10, 20, 5)")
		for _, row := range resp.Rows {
			nid, _ := row.Get(0); deg, _ := row.Get(1); sc, _ := row.Get(2); ws, _ := row.Get(3)
			t.Logf("  nodeId=%v, degree=%v(%T), score=%v(%T), weightScores=%v", nid, deg, deg, sc, sc, ws)
		}
	})

	// Test 2: weight as single string (float32 property)
	t.Run("weight_string_float32", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'w_f32'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Logf("weight='w_f32' (float32 values: 1.5, 3.2, 0.9)")
		for _, row := range resp.Rows {
			nid, _ := row.Get(0); deg, _ := row.Get(1); sc, _ := row.Get(2); ws, _ := row.Get(3)
			t.Logf("  nodeId=%v, degree=%v(%T), score=%v(%T), weightScores=%v", nid, deg, deg, sc, sc, ws)
		}
	})

	// Test 3: weight as single string (float64 property)
	t.Run("weight_string_float64", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: 'w_f64'}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Logf("weight='w_f64' (float64 values: 2.7, 4.8, 0.3)")
		for _, row := range resp.Rows {
			nid, _ := row.Get(0); deg, _ := row.Get(1); sc, _ := row.Get(2); ws, _ := row.Get(3)
			t.Logf("  nodeId=%v, degree=%v(%T), score=%v(%T), weightScores=%v", nid, deg, deg, sc, sc, ws)
		}
	})

	// Test 4: weight as list of strings (multi-weight)
	t.Run("weight_list_int_float", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: ['w_int', 'w_f32', 'w_f64']}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Logf("weight=['w_int', 'w_f32', 'w_f64'] (multi-weight)")
		for _, row := range resp.Rows {
			nid, _ := row.Get(0); deg, _ := row.Get(1); sc, _ := row.Get(2); ws, _ := row.Get(3)
			t.Logf("  nodeId=%v, degree=%v(%T), score=%v(%T), weightScores=%v", nid, deg, deg, sc, sc, ws)
		}
	})

	// Test 5: weight as list with only float properties
	t.Run("weight_list_floats_only", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.degree({weight: ['w_f32', 'w_f64']}) YIELD nodeId, degree, score, weightScores`, qc)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		t.Logf("weight=['w_f32', 'w_f64'] (float-only multi-weight)")
		for _, row := range resp.Rows {
			nid, _ := row.Get(0); deg, _ := row.Get(1); sc, _ := row.Get(2); ws, _ := row.Get(3)
			t.Logf("  nodeId=%v, degree=%v(%T), score=%v(%T), weightScores=%v", nid, deg, deg, sc, sc, ws)
		}
	})
}
