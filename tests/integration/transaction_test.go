//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// ensureLoggedIn re-logs in if needed (e.g., after TestLogout runs before this test)
func ensureLoggedIn(t *testing.T) {
	if testClient.IsLoggedIn() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	username := os.Getenv("GQLDB_USERNAME")
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("GQLDB_PASSWORD")
	if password == "" {
		password = "root11"
	}

	_, err := testClient.Login(ctx, username, password)
	if err != nil {
		t.Fatalf("Re-login failed: %v", err)
	}
}

func TestCommitInvalidTxId(t *testing.T) {
	ensureLoggedIn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Commit with an invalid/nonexistent transaction ID
	_, err := testClient.Commit(ctx, 999999999)
	if err == nil {
		t.Fatal("expected error when committing invalid transaction ID")
	}

	t.Logf("Got expected error for invalid commit: %v", err)
}

func TestRollbackInvalidTxId(t *testing.T) {
	ensureLoggedIn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Rollback with an invalid/nonexistent transaction ID
	_, err := testClient.Rollback(ctx, 999999999)
	if err == nil {
		t.Fatal("expected error when rolling back invalid transaction ID")
	}

	t.Logf("Got expected error for invalid rollback: %v", err)
}

func TestBeginNonexistentGraph(t *testing.T) {
	ensureLoggedIn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Begin transaction on a nonexistent graph
	_, err := testClient.BeginTransaction(ctx, "nonexistent_graph_xyz_999", false, 30)
	if err == nil {
		t.Fatal("expected error when beginning transaction on nonexistent graph")
	}

	t.Logf("Got expected error for nonexistent graph transaction: %v", err)
}

func TestTransaction(t *testing.T) {
	ensureLoggedIn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := os.Getenv("GQLDB_TEST_GRAPH")
	if graphName == "" {
		graphName = "default"
	}

	// Begin transaction
	tx, err := testClient.BeginTransaction(ctx, graphName, false, 30)
	if err != nil {
		t.Fatalf("BeginTransaction failed: %v", err)
	}

	t.Logf("Started transaction: %d", tx.ID)

	if !tx.IsActive() {
		t.Error("expected transaction to be active")
	}

	// Rollback
	success, err := testClient.Rollback(ctx, tx.ID)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	if !success {
		t.Error("expected rollback to succeed")
	}

	if tx.IsActive() {
		t.Error("expected transaction to be inactive after rollback")
	}

	t.Log("Transaction rolled back successfully")
}

func TestTransactionCommit(t *testing.T) {
	ensureLoggedIn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := os.Getenv("GQLDB_TEST_GRAPH")
	if graphName == "" {
		graphName = "default"
	}

	// Begin transaction
	tx, err := testClient.BeginTransaction(ctx, graphName, false, 30)
	if err != nil {
		t.Fatalf("BeginTransaction failed: %v", err)
	}

	t.Logf("Started transaction for commit: %d", tx.ID)

	// Commit
	success, err := testClient.Commit(ctx, tx.ID)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if !success {
		t.Error("expected commit to succeed")
	}

	t.Log("Transaction committed successfully")
}

func TestReadOnlyTransaction(t *testing.T) {
	ensureLoggedIn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_ro_tx_" + time.Now().Format("20060102150405")

	// Create a test graph for the read-only transaction
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Read-only transaction test graph")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Begin read-only transaction
	tx, err := testClient.BeginTransaction(ctx, graphName, true, 30)
	if err != nil {
		t.Fatalf("BeginTransaction (read-only) failed: %v", err)
	}

	t.Logf("Started read-only transaction: %d", tx.ID)

	if !tx.ReadOnly {
		t.Error("expected transaction to be read-only")
	}

	// Commit the read-only transaction
	success, err := testClient.Commit(ctx, tx.ID)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if !success {
		t.Error("expected commit to succeed")
	}

	t.Log("Read-only transaction committed successfully")
}

func TestListTransactions(t *testing.T) {
	ensureLoggedIn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_list_tx_" + time.Now().Format("20060102150405")

	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "List transactions test graph")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Begin a transaction so there is at least one active
	tx, err := testClient.BeginTransaction(ctx, graphName, false, 30)
	if err != nil {
		t.Fatalf("BeginTransaction failed: %v", err)
	}

	// List transactions
	transactions, err := testClient.ListTransactions(ctx)
	if err != nil {
		// Clean up before failing
		testClient.Rollback(ctx, tx.ID)
		t.Fatalf("ListTransactions failed: %v", err)
	}

	t.Logf("Found %d active transactions", len(transactions))
	for _, ti := range transactions {
		t.Logf("  - id=%d, graph=%s, read_only=%v", ti.TransactionID, ti.GraphName, ti.ReadOnly)
	}

	// Clean up
	success, err := testClient.Rollback(ctx, tx.ID)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	if !success {
		t.Error("expected rollback to succeed")
	}
}

func TestWithTransactionCommit(t *testing.T) {
	ensureLoggedIn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_with_tx_" + time.Now().Format("20060102150405")

	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "WithTransaction commit test graph")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	var executedTxID uint64

	err = testClient.WithTransaction(ctx, graphName, false, func(txID uint64) error {
		if txID == 0 {
			return errors.New("expected non-zero transaction ID")
		}
		executedTxID = txID
		t.Logf("Executing in transaction %d", txID)
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction failed: %v", err)
	}

	if executedTxID == 0 {
		t.Error("expected transaction function to be executed")
	}

	t.Log("WithTransaction completed successfully (committed)")
}

func TestWithTransactionRollbackOnError(t *testing.T) {
	ensureLoggedIn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_with_tx_rb_" + time.Now().Format("20060102150405")

	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "WithTransaction rollback test graph")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	simulatedErr := errors.New("simulated error")

	err = testClient.WithTransaction(ctx, graphName, false, func(txID uint64) error {
		t.Logf("Executing in transaction %d, will return error", txID)
		return simulatedErr
	})
	if err == nil {
		t.Fatal("expected error from WithTransaction when function returns error")
	}

	t.Logf("WithTransaction rolled back on error as expected: %v", err)
}
