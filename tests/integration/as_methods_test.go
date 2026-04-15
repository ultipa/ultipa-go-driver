//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestAsNodes_WithRealData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test graph
	graphName := fmt.Sprintf("test_as_nodes_%d", time.Now().UnixNano())
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test as methods")
	if err != nil {
		t.Fatalf("Failed to create graph: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		c, cn := context.WithTimeout(context.Background(), 10*time.Second)
		defer cn()
		_, _ = testClient.EndBulkImport(c, session.SessionID)
	}()

	// Insert test nodes
	nodes := []*gqldb.NodeData{
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Alice", "age": int64(30)}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Bob", "age": int64(25)}},
	}
	config := &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, config)
	if err != nil {
		t.Fatalf("Failed to insert nodes: %v", err)
	}


	// Query and extract nodes - use RETURN n to get PropertyType.NODE typed values
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n:Person) RETURN n", queryConfig)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Extract nodes from the "n" column
	ar, err := resp.Alias("n")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}
	extractedNodes, schemas, err := ar.AsNodes()
	if err != nil {
		t.Fatalf("AsNodes failed: %v", err)
	}

	if len(extractedNodes) < 2 {
		t.Errorf("Expected at least 2 nodes, got %d", len(extractedNodes))
	}

	// Verify some data was extracted
	var foundNames []string
	for _, node := range extractedNodes {
		if name, ok := node.Properties["name"].(string); ok {
			foundNames = append(foundNames, name)
		}
	}

	if !contains(foundNames, "Alice") || !contains(foundNames, "Bob") {
		t.Errorf("Expected Alice and Bob, got %v", foundNames)
	}

	// Test printer
	output := gqldb.PrintNodes(extractedNodes, schemas)
	t.Logf("Printed nodes:\n%s", output)

	if !strings.Contains(output, "node(s)") {
		t.Error("Expected node count in output")
	}
}

func TestAsEdges_WithRealData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test graph
	graphName := fmt.Sprintf("test_as_edges_%d", time.Now().UnixNano())
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test as methods")
	if err != nil {
		t.Fatalf("Failed to create graph: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		c, cn := context.WithTimeout(context.Background(), 10*time.Second)
		defer cn()
		_, _ = testClient.EndBulkImport(c, session.SessionID)
	}()

	// Insert test nodes
	nodes := []*gqldb.NodeData{
		{Labels: []string{"EdgeTestPerson"}, Properties: map[string]interface{}{"name": "Dave"}},
		{Labels: []string{"EdgeTestPerson"}, Properties: map[string]interface{}{"name": "Eve"}},
	}
	config := &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, config)
	if err != nil {
		t.Fatalf("Failed to insert nodes: %v", err)
	}


	// Query to get node IDs (bulk import doesn't return IDs immediately)
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n:EdgeTestPerson) RETURN id(n) AS node_id ORDER BY n.name", queryConfig)
	if err != nil {
		t.Fatalf("Query for node IDs failed: %v", err)
	}
	if resp.RowCount < 2 {
		t.Skip("Not enough nodes from query")
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
		t.Skip("Not enough node IDs extracted")
	}

	// Insert edge
	edges := []*gqldb.EdgeData{
		{
			Label:      "KNOWS_TEST",
			FromNodeID: nodeIDs[0],
			ToNodeID:   nodeIDs[1],
			Properties: map[string]interface{}{"since": int64(2024)},
		},
	}
	edgeConfig := &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID}
	_, err = testClient.InsertEdgesBatchAuto(ctx, graphName, edges, edgeConfig)
	if err != nil {
		t.Fatalf("Failed to insert edges: %v", err)
	}


	// Query and extract edges - use RETURN e to get PropertyType.EDGE typed values
	resp, err = testClient.Gql(ctx,
		"MATCH (a)-[e:KNOWS_TEST]->(b) RETURN e",
		queryConfig)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Extract edges from the "e" column
	ar, err := resp.Alias("e")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}
	extractedEdges, schemas, err := ar.AsEdges()
	if err != nil {
		t.Fatalf("AsEdges failed: %v", err)
	}

	if len(extractedEdges) < 1 {
		t.Errorf("Expected at least 1 edge, got %d", len(extractedEdges))
	}

	// Test printer
	output := gqldb.PrintEdges(extractedEdges, schemas)
	t.Logf("Printed edges:\n%s", output)

	if !strings.Contains(output, "edge(s)") {
		t.Error("Expected edge count in output")
	}
}

func TestAsTable_WithRealData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test graph
	graphName := fmt.Sprintf("test_as_table_%d", time.Now().UnixNano())
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test as methods")
	if err != nil {
		t.Fatalf("Failed to create graph: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		c, cn := context.WithTimeout(context.Background(), 10*time.Second)
		defer cn()
		_, _ = testClient.EndBulkImport(c, session.SessionID)
	}()

	// Insert test nodes
	nodes := []*gqldb.NodeData{
		{Labels: []string{"TableTest"}, Properties: map[string]interface{}{"val": int64(1)}},
		{Labels: []string{"TableTest"}, Properties: map[string]interface{}{"val": int64(2)}},
	}
	config := &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, config)
	if err != nil {
		t.Fatalf("Failed to insert nodes: %v", err)
	}


	// Query and get table
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n:TableTest) RETURN n.val as value ORDER BY n.val", queryConfig)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Extract table from the "value" column
	ar, err := resp.Alias("value")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}
	table, err := ar.AsTable()
	if err != nil {
		t.Fatalf("AsTable failed: %v", err)
	}

	if len(table.Headers) < 1 {
		t.Errorf("Expected headers, got %d", len(table.Headers))
	}

	// Test printer
	output := gqldb.PrintTable(table)
	t.Logf("Printed table:\n%s", output)

	if !strings.Contains(output, "row(s)") {
		t.Error("Expected row count in output")
	}
}

func TestAsAttr_WithRealData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test graph
	graphName := fmt.Sprintf("test_as_attr_%d", time.Now().UnixNano())
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test as methods")
	if err != nil {
		t.Fatalf("Failed to create graph: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		c, cn := context.WithTimeout(context.Background(), 10*time.Second)
		defer cn()
		_, _ = testClient.EndBulkImport(c, session.SessionID)
	}()

	// Insert test nodes
	nodes := []*gqldb.NodeData{
		{Labels: []string{"AttrTest"}, Properties: map[string]interface{}{"name": "Alice"}},
		{Labels: []string{"AttrTest"}, Properties: map[string]interface{}{"name": "Bob"}},
		{Labels: []string{"AttrTest"}, Properties: map[string]interface{}{"name": "Charlie"}},
	}
	config := &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, config)
	if err != nil {
		t.Fatalf("Failed to insert nodes: %v", err)
	}


	// Query and extract attribute
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n:AttrTest) RETURN n.name as name", queryConfig)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Extract attribute from the "name" column
	ar, err := resp.Alias("name")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}
	attr, err := ar.AsAttr()
	if err != nil {
		t.Fatalf("AsAttr failed: %v", err)
	}

	if attr.Name != "name" {
		t.Errorf("Expected attr name 'name', got %s", attr.Name)
	}

	if len(attr.Values) < 3 {
		t.Errorf("Expected at least 3 values, got %d", len(attr.Values))
	}

	t.Logf("Extracted names: %v", attr.Values)
}

func TestPrintAny_AutoDetection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test graph
	graphName := fmt.Sprintf("test_print_any_%d", time.Now().UnixNano())
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test as methods")
	if err != nil {
		t.Fatalf("Failed to create graph: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		c, cn := context.WithTimeout(context.Background(), 10*time.Second)
		defer cn()
		_, _ = testClient.EndBulkImport(c, session.SessionID)
	}()

	// Insert test nodes
	nodes := []*gqldb.NodeData{
		{Labels: []string{"PrintAnyTest"}, Properties: map[string]interface{}{"val": int64(1)}},
	}
	config := &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, config)
	if err != nil {
		t.Fatalf("Failed to insert nodes: %v", err)
	}


	// Test auto-detection with node query - use RETURN n to get PropertyType.NODE typed values
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n:PrintAnyTest) RETURN n", queryConfig)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	output := gqldb.PrintAny(resp)
	t.Logf("Auto-detected output (nodes):\n%s", output)

	// Test with empty result
	resp, err = testClient.Gql(ctx, "MATCH (n:NonExistent12345) RETURN n LIMIT 1", queryConfig)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	output = gqldb.PrintAny(resp)
	if !strings.Contains(output, "No data") {
		t.Errorf("Expected 'No data' for empty result, got: %s", output)
	}

	// Test with generic table
	resp, err = testClient.Gql(ctx, "RETURN 1 + 1 as result, 'hello' as greeting", queryConfig)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	output = gqldb.PrintAny(resp)
	t.Logf("Auto-detected output (table):\n%s", output)
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
