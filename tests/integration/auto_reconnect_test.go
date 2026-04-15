//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// newReconnectClient creates a client WITHOUT DefaultGraph to avoid auth error.
func newReconnectClient() (*gqldb.Client, error) {
	config := gqldb.NewConfigBuilder().
		Hosts(authHost).
		Timeout(30 * time.Second).
		Build()
	return gqldb.NewClient(config)
}

func getCredentials() (string, string) {
	username := os.Getenv("GQLDB_USERNAME")
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("GQLDB_PASSWORD")
	if password == "" {
		password = "root11"
	}
	return username, password
}

func TestAutoReconnect_AfterLogout(t *testing.T) {
	client, err := newReconnectClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	username, password := getCredentials()

	_, err = client.Login(ctx, username, password)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	err = client.UseGraph(ctx, "default")
	if err != nil {
		t.Fatalf("UseGraph failed: %v", err)
	}

	// Query should work normally
	resp, err := client.Gql(ctx, "RETURN 1 AS num", nil)
	if err != nil {
		t.Fatalf("First query failed: %v", err)
	}
	if resp.RowCount != 1 {
		t.Errorf("expected 1 row, got %d", resp.RowCount)
	}

	// Invalidate server session
	err = client.Logout(ctx)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// Next call should trigger auto-reconnect
	resp, err = client.Gql(ctx, "RETURN 2 AS num", nil)
	if err != nil {
		t.Fatalf("Query after logout failed (auto-reconnect should have handled it): %v", err)
	}
	if resp.RowCount != 1 {
		t.Errorf("expected 1 row after reconnect, got %d", resp.RowCount)
	}

	t.Log("Auto-reconnect after logout succeeded")
}

func TestAutoReconnect_MultipleCallsAfterReconnect(t *testing.T) {
	client, err := newReconnectClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	username, password := getCredentials()

	_, err = client.Login(ctx, username, password)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	err = client.UseGraph(ctx, "default")
	if err != nil {
		t.Fatalf("UseGraph failed: %v", err)
	}

	// Invalidate session
	err = client.Logout(ctx)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// First call triggers reconnect
	resp, err := client.Gql(ctx, "RETURN 1 AS a", nil)
	if err != nil {
		t.Fatalf("First query after logout failed: %v", err)
	}
	if resp.RowCount != 1 {
		t.Errorf("expected 1 row, got %d", resp.RowCount)
	}

	// Subsequent calls should work without another reconnect
	for i := 0; i < 5; i++ {
		resp, err = client.Gql(ctx, fmt.Sprintf("RETURN %d AS val", i), nil)
		if err != nil {
			t.Fatalf("Query %d after reconnect failed: %v", i, err)
		}
		if resp.RowCount != 1 {
			t.Errorf("query %d: expected 1 row, got %d", i, resp.RowCount)
		}
	}

	t.Log("Multiple calls after auto-reconnect all succeeded")
}

func TestAutoReconnect_RestoresSession(t *testing.T) {
	client, err := newReconnectClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	username, password := getCredentials()

	_, err = client.Login(ctx, username, password)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	err = client.UseGraph(ctx, "default")
	if err != nil {
		t.Fatalf("UseGraph failed: %v", err)
	}

	err = client.Logout(ctx)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	if client.IsLoggedIn() {
		t.Error("expected not logged in after logout")
	}

	// Trigger auto-reconnect
	resp, err := client.Gql(ctx, "RETURN 42 AS answer", nil)
	if err != nil {
		t.Fatalf("Query after logout failed: %v", err)
	}
	if resp.RowCount != 1 {
		t.Errorf("expected 1 row, got %d", resp.RowCount)
	}

	// Session should be restored
	if !client.IsLoggedIn() {
		t.Error("expected to be logged in after auto-reconnect")
	}

	t.Log("Session restored after auto-reconnect")
}

func TestAutoReconnect_RepeatedCycles(t *testing.T) {
	client, err := newReconnectClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	username, password := getCredentials()

	for cycle := 0; cycle < 3; cycle++ {
		_, err = client.Login(ctx, username, password)
		if err != nil {
			t.Fatalf("Cycle %d: login failed: %v", cycle, err)
		}
		err = client.UseGraph(ctx, "default")
		if err != nil {
			t.Fatalf("Cycle %d: UseGraph failed: %v", cycle, err)
		}

		resp, err := client.Gql(ctx, fmt.Sprintf("RETURN %d AS cycle", cycle), nil)
		if err != nil {
			t.Fatalf("Cycle %d: query before logout failed: %v", cycle, err)
		}
		if resp.RowCount != 1 {
			t.Errorf("Cycle %d: expected 1 row, got %d", cycle, resp.RowCount)
		}

		err = client.Logout(ctx)
		if err != nil {
			t.Fatalf("Cycle %d: logout failed: %v", cycle, err)
		}

		// Auto-reconnect on next call
		resp, err = client.Gql(ctx, fmt.Sprintf("RETURN %d AS after_reconnect", cycle+10), nil)
		if err != nil {
			t.Fatalf("Cycle %d: query after logout failed: %v", cycle, err)
		}
		if resp.RowCount != 1 {
			t.Errorf("Cycle %d: expected 1 row after reconnect, got %d", cycle, resp.RowCount)
		}

		t.Logf("Cycle %d: auto-reconnect succeeded", cycle)
	}
}
