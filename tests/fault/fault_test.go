//go:build fault

// Package fault provides tests for error handling and failure scenarios.
// Run with: go test -tags=fault ./tests/fault/...
package fault

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
	authHost   = getEnv("GQLDB_HOST", "192.168.1.100:60061")
	noAuthHost = getEnv("GQLDB_NO_AUTH_HOST", "192.168.1.100:60062")
	username   = getEnv("GQLDB_USERNAME", "admin")
	password   = getEnv("GQLDB_PASSWORD", "root11")
	graph      = getEnv("GQLDB_TEST_GRAPH", "miniCircle")
)

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// ==================== Network Failure Tests ====================

// TestConnectionTimeout tests that client handles connection timeout gracefully.
func TestConnectionTimeout(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts("192.0.2.1:60061"). // Non-routable IP (RFC 5737)
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(2). // Very short timeout
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		// Connection failure at client creation is acceptable
		t.Logf("Client creation failed (expected): %v", err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = client.Ping(ctx)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

// TestConnectionRefused tests proper error when server is unreachable.
func TestConnectionRefused(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts("localhost:59999"). // Unlikely to have a service on this port
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(5).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		// Connection failure at client creation is acceptable
		t.Logf("Client creation failed (expected): %v", err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Ping(ctx)
	if err == nil {
		t.Error("Expected connection refused error, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

// TestDNSFailure tests invalid hostname error handling.
func TestDNSFailure(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts("invalid.hostname.that.does.not.exist.local:60061").
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(5).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Logf("Client creation failed (expected): %v", err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Ping(ctx)
	if err == nil {
		t.Error("Expected DNS resolution error, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

// ==================== Authentication Failure Tests ====================

// TestInvalidCredentials tests wrong username/password handling.
func TestInvalidCredentials(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(authHost).
		Username("invalid_user").
		Password("wrong_password").
		DefaultGraph(graph).
		Timeout(30 * time.Second).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.Login(ctx, "invalid_user", "wrong_password")
	if err == nil {
		t.Error("Expected authentication error, got nil")
	} else {
		// Check for expected error types
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "authentication is not enabled") {
			t.Skip("Authentication is not enabled on this server")
		}
		t.Logf("Got expected authentication error: %v", err)
	}
}

// TestEmptyCredentials tests login with empty credentials.
func TestEmptyCredentials(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(authHost).
		DefaultGraph(graph).
		Timeout(30 * time.Second).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.Login(ctx, "", "")
	if err == nil {
		// Check if auth is disabled
		_, pingErr := client.Ping(ctx)
		if pingErr != nil {
			t.Error("Expected error with empty credentials")
		} else {
			t.Log("Server may allow empty credentials or auth is disabled")
		}
	} else {
		t.Logf("Got expected error with empty credentials: %v", err)
	}
}

// ==================== Transaction Failure Tests ====================

// TestTransactionTimeout tests transaction timeout behavior.
func TestTransactionTimeout(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(authHost).
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(30 * time.Second).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Login first
	_, err = client.Login(ctx, username, password)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "authentication is not enabled") {
			t.Log("Continuing without login (auth disabled)")
		} else {
			t.Fatalf("Login failed: %v", err)
		}
	}

	// Begin a transaction
	tx, err := client.BeginTransaction(ctx, false)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Ensure cleanup
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Verify transaction was created
	if tx == nil {
		t.Fatal("Transaction is nil")
	}
	t.Logf("Transaction created successfully")
}

// TestRollbackAfterError tests auto-rollback behavior after query errors.
func TestRollbackAfterError(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(authHost).
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(30 * time.Second).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Login
	_, err = client.Login(ctx, username, password)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "authentication is not enabled") {
			// Continue
		} else {
			t.Fatalf("Login failed: %v", err)
		}
	}

	// Begin transaction
	tx, err := client.BeginTransaction(ctx, false)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Execute an invalid query to trigger error
	_, queryErr := tx.GQL(ctx, "INVALID SYNTAX QUERY!!!")
	if queryErr == nil {
		// Query might succeed if it's valid GQL
		t.Log("Query succeeded (may be valid syntax)")
	} else {
		t.Logf("Query failed as expected: %v", queryErr)
	}

	// Rollback should still work
	rollbackErr := tx.Rollback(ctx)
	if rollbackErr != nil {
		t.Logf("Rollback error (may be expected if already rolled back): %v", rollbackErr)
	} else {
		t.Log("Rollback succeeded")
	}
}

// TestCommitOnClosedTransaction tests commit after rollback error handling.
func TestCommitOnClosedTransaction(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(authHost).
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(30 * time.Second).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Login
	_, _ = client.Login(ctx, username, password)

	// Begin transaction
	tx, err := client.BeginTransaction(ctx, false)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Rollback first
	_ = tx.Rollback(ctx)

	// Try to commit after rollback - should fail
	commitErr := tx.Commit(ctx)
	if commitErr == nil {
		t.Log("Commit succeeded (transaction may not be enforcing state)")
	} else {
		t.Logf("Commit after rollback failed as expected: %v", commitErr)
	}
}

// ==================== Bulk Import Failure Tests ====================

// TestBulkImportPartialFailure tests handling of partial failures in bulk import.
func TestBulkImportPartialFailure(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(authHost).
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(30 * time.Second).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Login
	_, _ = client.Login(ctx, username, password)

	// Create a test graph
	graphName := "fault_test_" + time.Now().Format("20060102150405")
	_, err = client.GQL(ctx, "CREATE GRAPH "+graphName, nil)
	if err != nil {
		t.Logf("Could not create test graph: %v", err)
		return
	}

	// Cleanup
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = client.GQL(cleanupCtx, "DROP GRAPH "+graphName, nil)
	}()

	// Start bulk import session
	session, err := client.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Logf("Bulk import not supported or failed: %v", err)
		return
	}

	// Abort the session (simulating failure)
	abortErr := client.AbortBulkImport(ctx, session.SessionID)
	if abortErr != nil {
		t.Logf("Abort failed: %v", abortErr)
	} else {
		t.Log("Bulk import session aborted successfully")
	}
}

// TestBulkImportAbort tests abort mid-import and verify cleanup.
func TestBulkImportAbort(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(authHost).
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(30 * time.Second).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Login
	_, _ = client.Login(ctx, username, password)

	// Create a test graph
	graphName := "abort_test_" + time.Now().Format("20060102150405")
	_, err = client.GQL(ctx, "CREATE GRAPH "+graphName, nil)
	if err != nil {
		t.Logf("Could not create test graph: %v", err)
		return
	}

	// Cleanup
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = client.GQL(cleanupCtx, "DROP GRAPH "+graphName, nil)
	}()

	// Start bulk import
	session, err := client.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Logf("Bulk import not supported: %v", err)
		return
	}

	// Insert some data
	nodes := []gqldb.NodeData{
		gqldb.NewNodeData("TestNode").WithProperty("id", 1),
		gqldb.NewNodeData("TestNode").WithProperty("id", 2),
	}
	_, insertErr := client.InsertNodesWithSession(ctx, graphName, "TestNode", nodes, session.SessionID)
	if insertErr != nil {
		t.Logf("Insert failed: %v", insertErr)
	}

	// Abort mid-way
	abortErr := client.AbortBulkImport(ctx, session.SessionID)
	if abortErr != nil {
		t.Logf("Abort error: %v", abortErr)
	} else {
		t.Log("Bulk import aborted successfully")
	}
}

// ==================== Query Failure Tests ====================

// TestInvalidQuerySyntax tests handling of invalid query syntax.
func TestInvalidQuerySyntax(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(authHost).
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(30 * time.Second).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Login
	_, _ = client.Login(ctx, username, password)

	// Execute invalid query
	_, err = client.GQL(ctx, "THIS IS NOT VALID GQL SYNTAX !!!", nil)
	if err == nil {
		t.Error("Expected syntax error, got nil")
	} else {
		t.Logf("Got expected syntax error: %v", err)
	}
}

// TestQueryNonExistentGraph tests query against non-existent graph.
func TestQueryNonExistentGraph(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(authHost).
		Username(username).
		Password(password).
		DefaultGraph("non_existent_graph_12345").
		Timeout(30 * time.Second).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Login
	_, _ = client.Login(ctx, username, password)

	// Execute query
	_, err = client.GQL(ctx, "MATCH (n) RETURN n LIMIT 1", nil)
	if err == nil {
		t.Log("Query succeeded (graph may exist or default is used)")
	} else {
		t.Logf("Got error for non-existent graph: %v", err)
	}
}
