//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"
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
