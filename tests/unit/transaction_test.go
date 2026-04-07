package unit

import (
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestTransactionManager(t *testing.T) {
	tm := gqldb.NewTransactionManager()

	// Initially no active transactions
	if tm.HasActive() {
		t.Error("expected no active transactions initially")
	}

	if tm.Count() != 0 {
		t.Error("expected count 0 initially")
	}

	// Begin transaction
	tx := tm.Begin(100, 1, "testGraph", false, 30*time.Second)

	if !tm.HasActive() {
		t.Error("expected active transaction after Begin")
	}

	if tm.Count() != 1 {
		t.Errorf("expected count 1, got %d", tm.Count())
	}

	if tx.ID != 100 {
		t.Errorf("expected transaction ID 100, got %d", tx.ID)
	}

	if tx.SessionID != 1 {
		t.Errorf("expected session ID 1, got %d", tx.SessionID)
	}

	if tx.GraphName != "testGraph" {
		t.Errorf("expected graph name testGraph, got %s", tx.GraphName)
	}

	if tx.ReadOnly {
		t.Error("expected ReadOnly to be false")
	}

	// Get transaction
	retrieved := tm.Get(100)
	if retrieved == nil {
		t.Fatal("expected to retrieve transaction")
	}

	if retrieved.ID != tx.ID {
		t.Error("retrieved transaction doesn't match")
	}

	// Transaction state
	if !tx.IsActive() {
		t.Error("expected transaction to be active")
	}

	if tx.IsCommitted() {
		t.Error("expected transaction not to be committed")
	}

	if tx.IsRolledBack() {
		t.Error("expected transaction not to be rolled back")
	}

	// Commit
	err := tm.Commit(100)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if tm.HasActive() {
		t.Error("expected no active transactions after Commit")
	}

	if tx.IsActive() {
		t.Error("expected transaction to be inactive after Commit")
	}

	if !tx.IsCommitted() {
		t.Error("expected transaction to be committed")
	}
}

func TestTransactionRollback(t *testing.T) {
	tm := gqldb.NewTransactionManager()

	tx := tm.Begin(200, 1, "testGraph", true, 0)

	if !tx.ReadOnly {
		t.Error("expected ReadOnly to be true")
	}

	err := tm.Rollback(200)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	if tm.HasActive() {
		t.Error("expected no active transactions after Rollback")
	}

	if !tx.IsRolledBack() {
		t.Error("expected transaction to be rolled back")
	}

	if tx.IsActive() {
		t.Error("expected transaction to be inactive after Rollback")
	}
}

func TestTransactionNotFound(t *testing.T) {
	tm := gqldb.NewTransactionManager()

	err := tm.Commit(999)
	if err != gqldb.ErrTransactionNotFound {
		t.Errorf("expected ErrTransactionNotFound, got %v", err)
	}

	err = tm.Rollback(999)
	if err != gqldb.ErrTransactionNotFound {
		t.Errorf("expected ErrTransactionNotFound, got %v", err)
	}
}

func TestTransactionGetActive(t *testing.T) {
	tm := gqldb.NewTransactionManager()

	tm.Begin(1, 100, "graph1", false, 0)
	tm.Begin(2, 100, "graph2", true, 0)
	tm.Begin(3, 200, "graph1", false, 0)

	active := tm.GetActive()
	if len(active) != 3 {
		t.Errorf("expected 3 active transactions, got %d", len(active))
	}

	forSession := tm.GetActiveForSession(100)
	if len(forSession) != 2 {
		t.Errorf("expected 2 transactions for session 100, got %d", len(forSession))
	}
}

func TestTransactionClearAll(t *testing.T) {
	tm := gqldb.NewTransactionManager()

	tm.Begin(1, 100, "graph1", false, 0)
	tm.Begin(2, 100, "graph2", false, 0)

	if tm.Count() != 2 {
		t.Errorf("expected 2 transactions, got %d", tm.Count())
	}

	tm.ClearAll()

	if tm.Count() != 0 {
		t.Errorf("expected 0 transactions after ClearAll, got %d", tm.Count())
	}

	if tm.HasActive() {
		t.Error("expected no active transactions after ClearAll")
	}
}

func TestTransactionExpiry(t *testing.T) {
	tm := gqldb.NewTransactionManager()

	// Transaction with short timeout
	tx := tm.Begin(1, 100, "graph", false, 10*time.Millisecond)

	if tx.IsExpired() {
		t.Error("expected transaction not to be expired immediately")
	}

	time.Sleep(20 * time.Millisecond)

	if !tx.IsExpired() {
		t.Error("expected transaction to be expired after timeout")
	}

	// Transaction with no timeout
	tx2 := tm.Begin(2, 100, "graph", false, 0)
	time.Sleep(10 * time.Millisecond)

	if tx2.IsExpired() {
		t.Error("expected transaction with no timeout never to expire")
	}
}

func TestTransactionAge(t *testing.T) {
	tm := gqldb.NewTransactionManager()

	tx := tm.Begin(1, 100, "graph", false, 0)

	time.Sleep(50 * time.Millisecond)

	age := tx.Age()
	if age < 50*time.Millisecond {
		t.Errorf("expected age >= 50ms, got %v", age)
	}
}
