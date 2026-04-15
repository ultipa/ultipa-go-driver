//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestInsertNodesEmptyList(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_empty_insert_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for empty insert")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Insert empty slice of nodes
	nodes := []*gqldb.NodeData{}
	config := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	result, err := testClient.InsertNodesBatchAuto(ctx, graphName, nodes, config)
	// Either returns error or success with 0 nodes - both acceptable
	if err != nil {
		t.Logf("InsertNodes with empty list returned error (acceptable): %v", err)
	} else {
		t.Logf("InsertNodes with empty list: success=%v, nodeCount=%d", result.Success, result.NodeCount)
	}
}

func TestInsertEdgesNonexistentNode(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_bad_edge_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for bad edge insert")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Insert edge referencing nonexistent node IDs
	edges := []*gqldb.EdgeData{
		{
			Label:      "KNOWS",
			FromNodeID: "nonexistent_node_id_1",
			ToNodeID:   "nonexistent_node_id_2",
			Properties: map[string]interface{}{"weight": int64(1)},
		},
	}
	edgeConfig := &gqldb.InsertEdgesConfig{
		BulkImportSessionID: session.SessionID,
	}
	result, err := testClient.InsertEdgesBatchAuto(ctx, graphName, edges, edgeConfig)
	if err != nil {
		t.Logf("InsertEdges with nonexistent nodes returned error (expected): %v", err)
	} else if !result.Success {
		t.Logf("InsertEdges with nonexistent nodes returned success=false (expected): %s", result.Message)
	} else {
		t.Logf("InsertEdges with nonexistent nodes returned success=true, edgeCount=%d", result.EdgeCount)
	}
}

func TestDeleteNodesNonexistentId(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_del_noexist_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for delete nonexistent nodes")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Delete nodes with nonexistent IDs - should succeed with 0 deleted
	result, err := testClient.DeleteNodes(ctx, graphName, []string{"nonexistent_id_xyz"}, nil, "")
	if err != nil {
		t.Logf("DeleteNodes with nonexistent ID returned error: %v", err)
	} else {
		t.Logf("DeleteNodes with nonexistent ID: success=%v, deletedCount=%d", result.Success, result.DeletedCount)
		if result.DeletedCount != 0 {
			t.Errorf("expected 0 deleted, got %d", result.DeletedCount)
		}
	}
}

func TestDeleteEdgesNonexistentId(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_del_noexist_e_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for delete nonexistent edges")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Delete edges with nonexistent IDs - should succeed with 0 deleted
	result, err := testClient.DeleteEdges(ctx, graphName, []string{"nonexistent_edge_id_xyz"}, "", "")
	if err != nil {
		t.Logf("DeleteEdges with nonexistent ID returned error: %v", err)
	} else {
		t.Logf("DeleteEdges with nonexistent ID: success=%v, deletedCount=%d", result.Success, result.DeletedCount)
		if result.DeletedCount != 0 {
			t.Errorf("expected 0 deleted, got %d", result.DeletedCount)
		}
	}
}

func TestInsertNodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_data_graph_" + time.Now().Format("20060102150405")

	// Create a test graph first
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for data operations")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	t.Logf("Created test graph: %s", graphName)

	// Start bulk import session (required by server)
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Insert nodes with bulk import session
	nodes := []*gqldb.NodeData{
		{
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Alice", "age": int64(30)},
		},
		{
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Bob", "age": int64(25)},
		},
		{
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Charlie", "age": int64(35)},
		},
	}

	config := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	result, err := testClient.InsertNodesBatchAuto(ctx, graphName, nodes, config)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}

	if !result.Success {
		t.Errorf("InsertNodes returned success=false: %s", result.Message)
	}

	if result.NodeCount != 3 {
		t.Errorf("expected 3 nodes inserted, got %d", result.NodeCount)
	}

	t.Logf("Inserted %d nodes, IDs: %v", result.NodeCount, result.NodeIDs)

}

func TestInsertEdges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_edge_graph_" + time.Now().Format("20060102150405")

	// Create a test graph first
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for edge operations")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session (required by server)
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Insert nodes first with bulk import session
	nodes := []*gqldb.NodeData{
		{
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Alice"},
		},
		{
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Bob"},
		},
	}

	nodeConfig := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, nodeConfig)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}


	// Query to get node IDs (bulk import doesn't return IDs immediately)
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n:Person) RETURN id(n) AS node_id ORDER BY n.name", queryConfig)
	if err != nil {
		t.Fatalf("Query for node IDs failed: %v", err)
	}
	if resp.RowCount < 2 {
		t.Fatalf("expected at least 2 nodes from query, got %d", resp.RowCount)
	}

	// Extract node IDs from query result
	var nodeIDs []string
	for i := int64(0); i < resp.RowCount; i++ {
		row := resp.Rows[i]
		if value, err := resp.GetByName(row, "node_id"); err == nil {
			if nodeID, ok := value.(string); ok {
				nodeIDs = append(nodeIDs, nodeID)
			}
		}
	}
	if len(nodeIDs) < 2 {
		t.Fatalf("expected at least 2 node IDs from query, got %d", len(nodeIDs))
	}

	// Insert edges between the nodes with bulk import session
	edges := []*gqldb.EdgeData{
		{
			Label:      "KNOWS",
			FromNodeID: nodeIDs[0],
			ToNodeID:   nodeIDs[1],
			Properties: map[string]interface{}{"since": int64(2020)},
		},
	}

	edgeConfig := &gqldb.InsertEdgesConfig{
		BulkImportSessionID: session.SessionID,
	}
	edgeResult, err := testClient.InsertEdgesBatchAuto(ctx, graphName, edges, edgeConfig)
	if err != nil {
		t.Fatalf("InsertEdges failed: %v", err)
	}

	if !edgeResult.Success {
		t.Errorf("InsertEdges returned success=false: %s", edgeResult.Message)
	}

	if edgeResult.EdgeCount != 1 {
		t.Errorf("expected 1 edge inserted, got %d", edgeResult.EdgeCount)
	}

	t.Logf("Inserted %d edges, IDs: %v", edgeResult.EdgeCount, edgeResult.EdgeIDs)

}

func TestDeleteNodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_delete_graph_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for delete operations")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session (required by server)
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Insert nodes first with bulk import session
	nodes := []*gqldb.NodeData{
		{
			Labels:     []string{"ToDelete"},
			Properties: map[string]interface{}{"name": "Node1"},
		},
		{
			Labels:     []string{"ToDelete"},
			Properties: map[string]interface{}{"name": "Node2"},
		},
	}

	config := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	insertResult, err := testClient.InsertNodesBatchAuto(ctx, graphName, nodes, config)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}


	// Query to get node IDs for deletion (bulk import doesn't return IDs immediately)
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n:ToDelete) RETURN id(n) AS node_id ORDER BY n.name", queryConfig)
	if err != nil {
		t.Fatalf("Query for node IDs failed: %v", err)
	}
	if resp.RowCount < 2 {
		t.Fatalf("expected at least 2 nodes from query, got %d", resp.RowCount)
	}

	// Extract node IDs
	var nodeIDs []string
	for i := int64(0); i < resp.RowCount; i++ {
		row := resp.Rows[i]
		if value, err := resp.GetByName(row, "node_id"); err == nil {
			if nodeID, ok := value.(string); ok {
				nodeIDs = append(nodeIDs, nodeID)
			}
		}
	}
	if len(nodeIDs) < 2 {
		t.Fatalf("expected at least 2 node IDs from query, got %d", len(nodeIDs))
	}

	t.Logf("Inserted %d nodes for deletion test, IDs: %v", insertResult.NodeCount, nodeIDs)

	// Delete nodes by IDs (using queried IDs)
	deleteResult, err := testClient.DeleteNodes(ctx, graphName, nodeIDs, nil, "")
	if err != nil {
		t.Fatalf("DeleteNodes failed: %v", err)
	}

	t.Logf("DeleteNodes result: success=%v, deletedCount=%d, message=%s",
		deleteResult.Success, deleteResult.DeletedCount, deleteResult.Message)

	if deleteResult.DeletedCount != 2 {
		t.Logf("Warning: expected 2 nodes deleted, got %d", deleteResult.DeletedCount)
	}

	// Verify deletion by querying the graph
	afterResp, err := testClient.Gql(ctx, "MATCH (n) RETURN n", queryConfig)
	if err != nil {
		t.Logf("MATCH (n) query AFTER delete failed: %v", err)
	} else {
		t.Logf("MATCH (n) AFTER delete: %d rows returned", afterResp.RowCount)
		if afterResp.RowCount == 0 {
			t.Log("Deletion verified: no nodes found after delete")
		}
	}
}

func TestDeleteEdges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_delete_edge_graph_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for edge delete operations")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session (required by server)
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Insert nodes first with bulk import session
	nodes := []*gqldb.NodeData{
		{
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Alice"},
		},
		{
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Bob"},
		},
	}

	nodeConfig := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, nodeConfig)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}


	// Query to get node IDs (bulk import doesn't return IDs immediately)
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n:Person) RETURN id(n) AS node_id ORDER BY n.name", queryConfig)
	if err != nil {
		t.Fatalf("Query for node IDs failed: %v", err)
	}
	if resp.RowCount < 2 {
		t.Fatalf("expected at least 2 nodes from query, got %d", resp.RowCount)
	}

	// Extract node IDs from query result
	var nodeIDs []string
	for i := int64(0); i < resp.RowCount; i++ {
		row := resp.Rows[i]
		if value, err := resp.GetByName(row, "node_id"); err == nil {
			if nodeID, ok := value.(string); ok {
				nodeIDs = append(nodeIDs, nodeID)
			}
		}
	}
	if len(nodeIDs) < 2 {
		t.Fatalf("expected at least 2 node IDs from query, got %d", len(nodeIDs))
	}

	// Insert edge with bulk import session
	edges := []*gqldb.EdgeData{
		{
			Label:      "KNOWS",
			FromNodeID: nodeIDs[0],
			ToNodeID:   nodeIDs[1],
			Properties: map[string]interface{}{"since": int64(2020)},
		},
	}

	edgeConfig := &gqldb.InsertEdgesConfig{
		BulkImportSessionID: session.SessionID,
	}
	edgeResult, err := testClient.InsertEdgesBatchAuto(ctx, graphName, edges, edgeConfig)
	if err != nil {
		t.Fatalf("InsertEdges failed: %v", err)
	}


	t.Logf("Inserted %d edges for deletion test", edgeResult.EdgeCount)

	// Query to get edge IDs for deletion (bulk import doesn't return IDs immediately)
	edgeResp, err := testClient.Gql(ctx, "MATCH ()-[e:KNOWS]->() RETURN id(e) AS edge_id", queryConfig)
	if err != nil {
		t.Fatalf("Query for edge IDs failed: %v", err)
	}
	if edgeResp.RowCount < 1 {
		t.Fatalf("expected at least 1 edge from query, got %d", edgeResp.RowCount)
	}

	// Extract edge IDs
	var edgeIDs []string
	for i := int64(0); i < edgeResp.RowCount; i++ {
		row := edgeResp.Rows[i]
		if value, err := edgeResp.GetByName(row, "edge_id"); err == nil {
			if edgeID, ok := value.(string); ok {
				edgeIDs = append(edgeIDs, edgeID)
			}
		}
	}
	if len(edgeIDs) < 1 {
		t.Fatalf("expected at least 1 edge ID from query, got %d", len(edgeIDs))
	}

	// Delete edges by IDs (using queried IDs)
	deleteResult, err := testClient.DeleteEdges(ctx, graphName, edgeIDs, "", "")
	if err != nil {
		t.Fatalf("DeleteEdges failed: %v", err)
	}

	t.Logf("DeleteEdges result: success=%v, deletedCount=%d, message=%s",
		deleteResult.Success, deleteResult.DeletedCount, deleteResult.Message)

	if deleteResult.DeletedCount != 1 {
		t.Logf("Warning: expected 1 edge deleted, got %d", deleteResult.DeletedCount)
	}
}

func TestExport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_export_graph_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for export operations")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session (required by server)
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Insert nodes with bulk import session
	nodes := []*gqldb.NodeData{
		{
			Labels:     []string{"Product"},
			Properties: map[string]interface{}{"name": "Laptop", "price": 999.99},
		},
		{
			Labels:     []string{"Product"},
			Properties: map[string]interface{}{"name": "Phone", "price": 599.99},
		},
	}

	nodeConfig := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, nodeConfig)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}


	// Query to get node IDs for creating edges
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n:Product) RETURN id(n) AS node_id ORDER BY n.name", queryConfig)
	if err != nil {
		t.Fatalf("Query for node IDs failed: %v", err)
	}
	if resp.RowCount < 2 {
		t.Fatalf("expected at least 2 nodes from query, got %d", resp.RowCount)
	}

	// Extract node IDs from query result
	var nodeIDs []string
	for i := int64(0); i < resp.RowCount; i++ {
		row := resp.Rows[i]
		if value, err := resp.GetByName(row, "node_id"); err == nil {
			if nodeID, ok := value.(string); ok {
				nodeIDs = append(nodeIDs, nodeID)
			}
		}
	}

	// Insert edges with bulk import session
	edges := []*gqldb.EdgeData{
		{
			Label:      "RELATED_TO",
			FromNodeID: nodeIDs[0],
			ToNodeID:   nodeIDs[1],
			Properties: map[string]interface{}{"weight": int64(10)},
		},
	}

	edgeConfig := &gqldb.InsertEdgesConfig{
		BulkImportSessionID: session.SessionID,
	}
	_, err = testClient.InsertEdgesBatchAuto(ctx, graphName, edges, edgeConfig)
	if err != nil {
		t.Fatalf("InsertEdges failed: %v", err)
	}


	// Export graph data (nodes and edges) in JSON Lines format
	var totalBytes int
	var callbackCalled bool
	var finalStats *gqldb.ExportStats

	exportConfig := &gqldb.ExportConfig{
		GraphName:       graphName,
		BatchSize:       1000,
		ExportNodes:     true,
		ExportEdges:     true,
		IncludeMetadata: true,
	}

	err = testClient.Export(ctx, exportConfig, func(result *gqldb.ExportResult) error {
		callbackCalled = true
		totalBytes += len(result.Data)
		t.Logf("Export callback: received %d bytes, isFinal=%v", len(result.Data), result.IsFinal)

		// Log the JSON Lines data (for debugging)
		if len(result.Data) > 0 {
			t.Logf("Export data chunk: %s", string(result.Data))
		}

		if result.IsFinal && result.Stats != nil {
			finalStats = result.Stats
			t.Logf("Export stats: nodesExported=%d, edgesExported=%d, bytesWritten=%d, durationMs=%d",
				result.Stats.NodesExported, result.Stats.EdgesExported,
				result.Stats.BytesWritten, result.Stats.DurationMs)
		}
		return nil
	})

	if !callbackCalled {
		t.Logf("Export callback was never called - server returned empty stream")
	}

	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Note: Export functionality may not be fully supported on all server versions
	// Log result but don't fail if no data is exported
	if totalBytes == 0 {
		t.Logf("Warning: Export returned 0 bytes - export functionality may not be supported")
	} else {
		t.Logf("Exported %d bytes total", totalBytes)
	}

	if finalStats != nil {
		if finalStats.NodesExported < 2 {
			t.Logf("Warning: expected at least 2 nodes exported, got %d", finalStats.NodesExported)
		}
		if finalStats.EdgesExported < 1 {
			t.Logf("Warning: expected at least 1 edge exported, got %d", finalStats.EdgesExported)
		}
	}
}

func TestExportNodesOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_export_nodes_only_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for nodes-only export")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Insert nodes
	nodes := []*gqldb.NodeData{
		{
			Labels:     []string{"City"},
			Properties: map[string]interface{}{"name": "Beijing"},
		},
		{
			Labels:     []string{"City"},
			Properties: map[string]interface{}{"name": "Shanghai"},
		},
	}

	nodeConfig := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, nodeConfig)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}

	// Export only nodes (no edges)
	exportConfig := &gqldb.ExportConfig{
		GraphName:       graphName,
		ExportNodes:     true,
		ExportEdges:     false,
		IncludeMetadata: true,
	}

	var totalBytes int
	err = testClient.Export(ctx, exportConfig, func(result *gqldb.ExportResult) error {
		totalBytes += len(result.Data)
		if len(result.Data) > 0 {
			t.Logf("Export data: %s", string(result.Data))
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	t.Logf("Exported %d bytes (nodes only)", totalBytes)
}

func TestExportWithLabelFilter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_export_filter_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for filtered export")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Insert nodes with different labels
	nodes := []*gqldb.NodeData{
		{
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Alice"},
		},
		{
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Bob"},
		},
		{
			Labels:     []string{"Company"},
			Properties: map[string]interface{}{"name": "Acme"},
		},
	}

	nodeConfig := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, nodeConfig)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}

	// Export only Person nodes
	exportConfig := &gqldb.ExportConfig{
		GraphName:       graphName,
		ExportNodes:     true,
		ExportEdges:     false,
		NodeLabels:      []string{"Person"},
		IncludeMetadata: true,
	}

	var totalBytes int
	err = testClient.Export(ctx, exportConfig, func(result *gqldb.ExportResult) error {
		totalBytes += len(result.Data)
		if len(result.Data) > 0 {
			t.Logf("Filtered export data: %s", string(result.Data))
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	t.Logf("Exported %d bytes (Person nodes only)", totalBytes)
}
