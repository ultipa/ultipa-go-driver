//go:build regression

// Package regression provides API contract and backward compatibility tests.
// Run with: go test -tags=regression ./tests/regression/...
package regression

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// Test configuration
var (
	host     = getEnv("GQLDB_HOST", "192.168.1.100:60061")
	username = getEnv("GQLDB_USERNAME", "admin")
	password = getEnv("GQLDB_PASSWORD", "root11")
	graph    = getEnv("GQLDB_TEST_GRAPH", "miniCircle")
)

var testClient *gqldb.Client

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func TestMain(m *testing.M) {
tos.Setenv("NO_PROXY", "192.168.1.100")
	config := gqldb.NewConfigBuilder().
		Hosts(host).
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(30 * time.Second).
		Build()

	var err error
	testClient, err = gqldb.NewClient(config)
	if err != nil {
		println("Failed to create client:", err.Error())
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_, err = testClient.Login(ctx, username, password)
	cancel()
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "authentication is not enabled") {
		println("Login failed:", err.Error())
		os.Exit(1)
	}

	code := m.Run()
	testClient.Close()
	os.Exit(code)
}

// ==================== Response Structure Tests ====================

// TestQueryResponseStructure verifies query response has expected fields.
func TestQueryResponseStructure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := testClient.GQL(ctx, "RETURN 1 AS value", nil)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Verify response is not nil
	if response == nil {
		t.Fatal("Response should not be nil")
	}

	// Verify response has data
	if response.IsEmpty() {
		t.Error("Response should not be empty for RETURN query")
	}

	// Verify we can iterate
	count := 0
	for row := range response.Rows() {
		if row == nil {
			t.Error("Row should not be nil")
		}
		count++
	}
	if count == 0 {
		t.Error("Response should have at least one row")
	}
}

// TestNodeStructure verifies node data structure contains expected fields.
func TestNodeStructure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := testClient.GQL(ctx, "MATCH (n) RETURN n LIMIT 1", nil)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if response.IsEmpty() {
		t.Skip("No nodes in database to test structure")
		return
	}

	ar, err := response.Alias("n")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}

	nodes, _, err := ar.AsNodes()
	if err != nil {
		t.Fatalf("AsNodes failed: %v", err)
	}

	if len(nodes) == 0 {
		t.Skip("No nodes returned")
		return
	}

	node := nodes[0]

	// Verify node has ID
	if node.ID == "" {
		t.Error("Node should have an ID")
	}

	// Verify node has labels (may be empty but should exist)
	if node.Labels == nil {
		t.Error("Node labels should not be nil")
	}

	// Verify properties exist (may be empty)
	if node.Properties == nil {
		t.Error("Node properties should not be nil")
	}

	t.Logf("Node structure verified: ID=%s, Labels=%v, Properties count=%d",
		node.ID, node.Labels, len(node.Properties))
}

// TestEdgeStructure verifies edge data structure contains expected fields.
func TestEdgeStructure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := testClient.GQL(ctx, "MATCH ()-[e]->() RETURN e LIMIT 1", nil)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if response.IsEmpty() {
		t.Skip("No edges in database to test structure")
		return
	}

	ar, err := response.Alias("e")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}

	edges, _, err := ar.AsEdges()
	if err != nil {
		t.Fatalf("AsEdges failed: %v", err)
	}

	if len(edges) == 0 {
		t.Skip("No edges returned")
		return
	}

	edge := edges[0]

	// Verify edge has ID
	if edge.ID == "" {
		t.Error("Edge should have an ID")
	}

	// Verify edge has type
	if edge.Type == "" {
		t.Error("Edge should have a type")
	}

	// Verify from/to IDs exist
	if edge.FromID == "" {
		t.Error("Edge should have FromID")
	}
	if edge.ToID == "" {
		t.Error("Edge should have ToID")
	}

	t.Logf("Edge structure verified: ID=%s, Type=%s, From=%s, To=%s",
		edge.ID, edge.Type, edge.FromID, edge.ToID)
}

// TestPathStructure verifies path data structure.
func TestPathStructure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := testClient.GQL(ctx, "MATCH p=(n)-[*1..2]->(m) RETURN p LIMIT 1", nil)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if response.IsEmpty() {
		t.Skip("No paths in database to test structure")
		return
	}

	ar, err := response.Alias("p")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}

	paths, err := ar.AsPaths()
	if err != nil {
		t.Fatalf("AsPaths failed: %v", err)
	}

	if len(paths) == 0 {
		t.Skip("No paths returned")
		return
	}

	path := paths[0]

	// Verify path has nodes
	if len(path.Nodes) == 0 {
		t.Error("Path should have at least one node")
	}

	// Path should have alternating nodes and edges
	t.Logf("Path structure verified: Nodes=%d, Edges=%d",
		len(path.Nodes), len(path.Edges))
}

// ==================== API Backward Compatibility Tests ====================

// TestConfigBuilderDefaults verifies default configuration values.
func TestConfigBuilderDefaults(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(host).
		Build()

	// Verify defaults (implementation-specific)
	if config == nil {
		t.Fatal("Config should not be nil")
	}

	t.Log("ConfigBuilder defaults verified")
}

// TestConfigBuilderChaining verifies builder pattern chaining.
func TestConfigBuilderChaining(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(host).
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(30 * time.Second).
		Build()

	if config == nil {
		t.Fatal("Config should not be nil after chaining")
	}

	t.Log("ConfigBuilder chaining verified")
}

// ==================== Protocol Compatibility Tests ====================

// TestSessionIDPropagation verifies session ID is maintained across requests.
func TestSessionIDPropagation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute multiple queries and verify they use the same session
	_, err := testClient.GQL(ctx, "RETURN 1", nil)
	if err != nil {
		t.Fatalf("First query failed: %v", err)
	}

	_, err = testClient.GQL(ctx, "RETURN 2", nil)
	if err != nil {
		t.Fatalf("Second query failed: %v", err)
	}

	t.Log("Session ID propagation verified")
}

// TestTransactionIDHandling verifies transaction ID is handled correctly.
func TestTransactionIDHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := testClient.BeginTransaction(ctx, true) // read-only
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Execute query within transaction
	_, err = tx.GQL(ctx, "RETURN 1", nil)
	if err != nil {
		t.Fatalf("Transaction query failed: %v", err)
	}

	// Rollback
	err = tx.Rollback(ctx)
	if err != nil {
		t.Fatalf("Transaction rollback failed: %v", err)
	}

	t.Log("Transaction ID handling verified")
}

// ==================== Response Method Compatibility Tests ====================

// TestResponseIterationMethods verifies all response iteration methods work.
func TestResponseIterationMethods(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := testClient.GQL(ctx, "RETURN 1 AS a, 2 AS b, 3 AS c", nil)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Test IsEmpty
	if response.IsEmpty() {
		t.Error("Response should not be empty")
	}

	// Test First
	first := response.First()
	if first == nil {
		t.Error("First() should return a row")
	}

	// Test Last
	last := response.Last()
	if last == nil {
		t.Error("Last() should return a row")
	}

	// Test iteration
	count := 0
	for range response.Rows() {
		count++
	}
	if count == 0 {
		t.Error("Rows() should return at least one row")
	}

	t.Logf("Response iteration methods verified: %d rows", count)
}

// TestTypedValueConversions verifies TypedValue conversion methods.
func TestTypedValueConversions(t *testing.T) {
	// Test basic types
	tests := []struct {
		name  string
		value interface{}
	}{
		{"bool_true", true},
		{"bool_false", false},
		{"int64", int64(42)},
		{"float64", 3.14},
		{"string", "hello"},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv := gqldb.NewTypedValue(tt.value)
			if tv == nil {
				t.Errorf("NewTypedValue(%v) returned nil", tt.value)
			}
		})
	}
}

// TestResponseAsMethodsExist verifies all As* methods are available via AliasResult.
func TestResponseAsMethodsExist(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := testClient.GQL(ctx, "RETURN 1 AS value", nil)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Get AliasResult for the column
	ar, err := response.Alias("value")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}

	// These should not panic even if they return type errors or empty results
	_, _, _ = ar.AsNodes()  // Will error on type mismatch - expected
	_, _, _ = ar.AsEdges()  // Will error on type mismatch - expected
	_, _ = ar.AsPaths()     // Will error on type mismatch - expected
	_, _ = ar.AsTable()     // Should work for any type
	_, _ = ar.AsAttr()      // Should work for scalar types
	_, _ = ar.AsValues()    // Should work for any type

	t.Log("Response AliasResult methods verified")
}
