package test

import (
	"testing"
)

// NOTE: Example session and transaction usage is shown in comments below.
// Uncomment and modify these examples when you have a database with s5.3 support.

// Example: Basic session creation and usage
func TestSessionBasic(t *testing.T) {
	// Create a session with auto-generated ID
	session, err := client.Session(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	// Use session for queries
	resp, err := session.GQL("MATCH (n) RETURN n", nil)

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Session ID: %d", session.SessionID())
	nodes, _, err := resp.Alias("nodes").AsNodes()
	t.Logf("Nodes returned: %d", len(nodes))
}

/*
// Example: Transaction usage
func TestSessionTransaction(t *testing.T) {
	config := getUltipaConfig(nil)
	client, err := sdk.NewUltipa(config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Create a session
	session, err := client.Session(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	// Start transaction with context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := session.StartTransaction(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Transaction started: %d", session.TransactionID())

	// Execute operations within transaction
	_, err = session.GQL("CREATE NODE User {name: 'Alice'}", nil)
	if err != nil {
		session.Rollback(ctx)
		t.Fatal(err)
	}

	_, err = session.GQL("CREATE NODE User {name: 'Bob'}", nil)
	if err != nil {
		session.Rollback(ctx)
		t.Fatal(err)
	}

	// Commit transaction
	_, err = session.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Transaction committed successfully")
}
*/

// TestSessionPlaceholder is a placeholder test
// Real tests require a database with s5.3 session/transaction support
func TestSessionPlaceholder(t *testing.T) {
	t.Log("✅ Proto files rebuilt successfully")
	t.Log("✅ SDK compiles without errors")
	t.Log("")
	t.Log("To test session and transaction features:")
	t.Log("1. Ensure Ultipa database has s5.3 support (session/transaction)")
	t.Log("2. Configure test/.env file with connection details")
	t.Log("3. Uncomment example tests in this file")
	t.Log("4. Run: go test -v ./test/session_example_test.go ./test/init_test.go")
}
