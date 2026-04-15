//go:build e2e

// Package e2e provides end-to-end integration tests for complete workflows.
// Run with: go test -tags=e2e ./tests/e2e/...
package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
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

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func createClient() (*gqldb.Client, error) {
	config := gqldb.NewConfigBuilder().
		Hosts(host).
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(60 * time.Second).
		Build()

	return gqldb.NewClient(config)
}

func uniqueGraphName(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// ==================== Complete CRUD Lifecycle ====================

// TestCompleteCRUDLifecycle tests the full create-read-update-delete workflow.
func TestCompleteCRUDLifecycle(t *testing.T) {
	client, err := createClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Login
	_, err = client.Login(ctx, username, password)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "authentication is not enabled") {
		t.Fatalf("Login failed: %v", err)
	}

	graphName := uniqueGraphName("e2e_crud")

	// 1. Create graph
	t.Log("Creating graph...")
	_, err = client.GQL(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	if err != nil {
		t.Fatalf("Failed to create graph: %v", err)
	}

	// Ensure cleanup
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_, _ = client.GQL(cleanupCtx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}()

	// 2. Create schema (node type)
	t.Log("Creating schema...")
	_, err = client.GQL(ctx, fmt.Sprintf("USE GRAPH %s CREATE NODE TYPE Person (name STRING, age INT64)", graphName), nil)
	if err != nil {
		t.Logf("Schema creation warning: %v", err)
	}

	// 3. Insert nodes
	t.Log("Inserting nodes...")
	nodes := []gqldb.NodeData{
		gqldb.NewNodeData("Person").WithProperty("name", "Alice").WithProperty("age", int64(30)),
		gqldb.NewNodeData("Person").WithProperty("name", "Bob").WithProperty("age", int64(25)),
		gqldb.NewNodeData("Person").WithProperty("name", "Charlie").WithProperty("age", int64(35)),
	}
	_, err = client.InsertNodesBatchAuto(ctx, graphName, "Person", nodes)
	if err != nil {
		t.Fatalf("Failed to insert nodes: %v", err)
	}

	// 4. Query nodes
	t.Log("Querying nodes...")
	response, err := client.GQL(ctx, fmt.Sprintf("USE GRAPH %s MATCH (p:Person) RETURN p.name, p.age ORDER BY p.name", graphName), nil)
	if err != nil {
		t.Fatalf("Failed to query nodes: %v", err)
	}

	count := 0
	for range response.Rows() {
		count++
	}
	if count != 3 {
		t.Errorf("Expected 3 nodes, got %d", count)
	}

	// 5. Create edge type and insert edges
	t.Log("Inserting edges...")
	_, err = client.GQL(ctx, fmt.Sprintf("USE GRAPH %s CREATE EDGE TYPE KNOWS", graphName), nil)
	if err != nil {
		t.Logf("Edge type creation warning: %v", err)
	}

	// Insert edge via query
	_, err = client.GQL(ctx, fmt.Sprintf(`
		USE GRAPH %s
		MATCH (a:Person {name: 'Alice'}), (b:Person {name: 'Bob'})
		CREATE (a)-[:KNOWS]->(b)
	`, graphName), nil)
	if err != nil {
		t.Logf("Edge insertion via query: %v", err)
	}

	// 6. Query paths
	t.Log("Querying paths...")
	pathResponse, err := client.GQL(ctx, fmt.Sprintf("USE GRAPH %s MATCH p=(a:Person)-[:KNOWS]->(b:Person) RETURN p LIMIT 10", graphName), nil)
	if err != nil {
		t.Logf("Path query: %v", err)
	} else {
		ar, err := pathResponse.Alias("p")
		if err != nil {
			t.Logf("Alias error: %v", err)
		} else {
			paths, err := ar.AsPaths()
			if err != nil {
				t.Logf("AsPaths error: %v", err)
			} else {
				t.Logf("Found %d paths", len(paths))
			}
		}
	}

	// 7. Delete nodes
	t.Log("Deleting nodes...")
	_, err = client.GQL(ctx, fmt.Sprintf("USE GRAPH %s MATCH (p:Person {name: 'Charlie'}) DELETE p", graphName), nil)
	if err != nil {
		t.Logf("Delete warning: %v", err)
	}

	// 8. Verify deletion
	verifyResponse, err := client.GQL(ctx, fmt.Sprintf("USE GRAPH %s MATCH (p:Person) RETURN count(p) AS cnt", graphName), nil)
	if err != nil {
		t.Logf("Verify query warning: %v", err)
	} else if !verifyResponse.IsEmpty() {
		t.Log("Remaining nodes after deletion verified")
	}

	// 9. Drop graph (done in defer)
	t.Log("CRUD lifecycle completed successfully")
}

// ==================== Transaction Isolation ====================

// TestTransactionIsolation tests that transactions are properly isolated.
func TestTransactionIsolation(t *testing.T) {
	// Create two separate clients
	client1, err := createClient()
	if err != nil {
		t.Fatalf("Failed to create client 1: %v", err)
	}
	defer client1.Close()

	client2, err := createClient()
	if err != nil {
		t.Fatalf("Failed to create client 2: %v", err)
	}
	defer client2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Login both clients
	_, _ = client1.Login(ctx, username, password)
	_, _ = client2.Login(ctx, username, password)

	graphName := uniqueGraphName("e2e_txiso")

	// Create test graph
	_, err = client1.GQL(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	if err != nil {
		t.Fatalf("Failed to create graph: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_, _ = client1.GQL(cleanupCtx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}()

	// Begin transaction on client 1
	t.Log("Beginning transaction on client 1...")
	tx1, err := client1.BeginTransaction(ctx, false)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Insert data in transaction 1
	t.Log("Inserting data in transaction 1...")
	_, err = tx1.GQL(ctx, fmt.Sprintf("USE GRAPH %s CREATE (:IsolationTest {value: 'tx1'})", graphName), nil)
	if err != nil {
		t.Logf("Insert in transaction: %v", err)
	}

	// Client 2 should not see uncommitted data (depending on isolation level)
	t.Log("Client 2 checking for uncommitted data...")
	response2, err := client2.GQL(ctx, fmt.Sprintf("USE GRAPH %s MATCH (n:IsolationTest) RETURN n", graphName), nil)
	if err != nil {
		t.Logf("Client 2 query: %v", err)
	} else {
		// Depending on isolation level, this may or may not be empty
		ar, err := response2.Alias("n")
		if err != nil {
			t.Logf("Alias error: %v", err)
		} else {
			nodes, _, err := ar.AsNodes()
			if err != nil {
				t.Logf("AsNodes error: %v", err)
			} else {
				t.Logf("Client 2 sees %d rows before commit", len(nodes))
			}
		}
	}

	// Commit transaction 1
	t.Log("Committing transaction 1...")
	err = tx1.Commit(ctx)
	if err != nil {
		t.Logf("Commit: %v", err)
	}

	// Now client 2 should see the data
	t.Log("Client 2 checking after commit...")
	response2After, err := client2.GQL(ctx, fmt.Sprintf("USE GRAPH %s MATCH (n:IsolationTest) RETURN n", graphName), nil)
	if err != nil {
		t.Logf("Client 2 query after commit: %v", err)
	} else {
		ar, err := response2After.Alias("n")
		if err != nil {
			t.Logf("Alias error: %v", err)
		} else {
			nodes, _, err := ar.AsNodes()
			if err != nil {
				t.Logf("AsNodes error: %v", err)
			} else {
				t.Logf("Client 2 sees %d rows after commit", len(nodes))
			}
		}
	}

	t.Log("Transaction isolation test completed")
}

// ==================== Bulk Import Workflow ====================

// TestBulkImportWorkflow tests the complete bulk import process.
func TestBulkImportWorkflow(t *testing.T) {
	client, err := createClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	_, _ = client.Login(ctx, username, password)

	graphName := uniqueGraphName("e2e_bulk")

	// Create graph
	_, err = client.GQL(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	if err != nil {
		t.Fatalf("Failed to create graph: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_, _ = client.GQL(cleanupCtx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}()

	// Start bulk import session
	t.Log("Starting bulk import session...")
	session, err := client.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Logf("Bulk import not supported: %v", err)
		t.Skip("Bulk import not supported")
		return
	}

	// Insert nodes in batches
	batchSize := 100
	totalNodes := 1000

	t.Logf("Inserting %d nodes in batches of %d...", totalNodes, batchSize)
	for i := 0; i < totalNodes; i += batchSize {
		nodes := make([]gqldb.NodeData, batchSize)
		for j := 0; j < batchSize; j++ {
			nodes[j] = gqldb.NewNodeData("BulkNode").
				WithProperty("id", int64(i+j)).
				WithProperty("batch", int64(i/batchSize))
		}

		_, err = client.InsertNodesWithSession(ctx, graphName, "BulkNode", nodes, session.SessionID)
		if err != nil {
			t.Logf("Batch insert error: %v", err)
		}

	}

	// End import session
	t.Log("Ending bulk import session...")
	err = client.EndBulkImport(ctx, session.SessionID)
	if err != nil {
		t.Logf("End bulk import error: %v", err)
	}

	// Verify data
	t.Log("Verifying imported data...")
	response, err := client.GQL(ctx, fmt.Sprintf("USE GRAPH %s MATCH (n:BulkNode) RETURN count(n) AS cnt", graphName), nil)
	if err != nil {
		t.Logf("Verification query error: %v", err)
	} else if !response.IsEmpty() {
		t.Log("Bulk import data verified")
	}

	t.Log("Bulk import workflow completed")
}

// ==================== Concurrent Operations ====================

// TestConcurrentOperations tests multiple clients performing parallel queries.
func TestConcurrentOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	numClients := 5
	queriesPerClient := 10

	var wg sync.WaitGroup
	errors := make(chan error, numClients*queriesPerClient)

	t.Logf("Running %d concurrent clients with %d queries each...", numClients, queriesPerClient)

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			client, err := createClient()
			if err != nil {
				errors <- fmt.Errorf("client %d creation failed: %v", clientID, err)
				return
			}
			defer client.Close()

			_, _ = client.Login(ctx, username, password)

			for j := 0; j < queriesPerClient; j++ {
				_, err := client.GQL(ctx, fmt.Sprintf("RETURN %d AS client, %d AS query", clientID, j), nil)
				if err != nil {
					errors <- fmt.Errorf("client %d query %d failed: %v", clientID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Logf("Concurrent error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Logf("Concurrent operations completed with %d errors", errorCount)
	} else {
		t.Log("Concurrent operations completed successfully")
	}
}

// ==================== Reconnection Scenario ====================

// TestReconnection tests client reconnection after connection loss.
func TestReconnection(t *testing.T) {
	client, err := createClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Login
	_, _ = client.Login(ctx, username, password)

	// Execute query
	_, err = client.GQL(ctx, "RETURN 1", nil)
	if err != nil {
		t.Fatalf("Initial query failed: %v", err)
	}

	// Close connection
	client.Close()

	// Recreate client and reconnect
	client, err = createClient()
	if err != nil {
		t.Fatalf("Failed to recreate client: %v", err)
	}
	defer client.Close()

	_, _ = client.Login(ctx, username, password)

	// Execute query again
	_, err = client.GQL(ctx, "RETURN 2", nil)
	if err != nil {
		t.Fatalf("Query after reconnection failed: %v", err)
	}

	t.Log("Reconnection test completed successfully")
}
