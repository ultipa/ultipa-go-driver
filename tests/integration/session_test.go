//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLogin(t *testing.T) {
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

	session, err := testClient.Login(ctx, username, password)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if session.ID == 0 {
		t.Error("expected non-zero session ID")
	}

	if session.ServerVersion == "" {
		t.Error("expected non-empty server version")
	}

	t.Logf("Logged in with session ID: %d, server version: %s", session.ID, session.ServerVersion)
}

func TestPing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	latency, err := testClient.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if latency < 0 {
		t.Error("expected non-negative latency")
	}

	t.Logf("Ping latency: %d ns", latency)
}

func TestLogout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := testClient.Logout(ctx)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	if testClient.IsLoggedIn() {
		t.Error("expected not to be logged in after logout")
	}
}

// =============================================================================
// Tests for server without authentication (192.168.1.100:60062)
// =============================================================================

func TestLoginWrongPassword(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := testClient.Login(ctx, "admin", "wrong_password_xyz")
	if err == nil {
		t.Fatal("expected error when logging in with wrong password")
	}

	t.Logf("Got expected error for wrong password: %v", err)
}

func TestLoginEmptyUsername(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := testClient.Login(ctx, "", "root11")
	if err == nil {
		t.Fatal("expected error when logging in with empty username")
	}

	t.Logf("Got expected error for empty username: %v", err)
}

func TestLogoutTwice(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}

	username := os.Getenv("GQLDB_USERNAME")
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("GQLDB_PASSWORD")
	if password == "" {
		password = "root11"
	}

	// Ensure logged in first
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := testClient.Login(ctx, username, password)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// First logout should succeed
	err = testClient.Logout(ctx)
	if err != nil {
		t.Fatalf("First logout failed: %v", err)
	}

	// Second logout should be handled gracefully (no panic)
	err = testClient.Logout(ctx)
	// Whether it returns an error or nil, it should not panic
	t.Logf("Second logout result: err=%v", err)

	// Re-login for subsequent tests
	_, err = testClient.Login(ctx, username, password)
	if err != nil {
		t.Fatalf("Re-login after double logout failed: %v", err)
	}
}

// =============================================================================
// Tests for server without authentication (192.168.1.100:60062)
// =============================================================================

func TestNoAuth_Ping(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Ping should work without login on no-auth server
	latency, err := noAuthClient.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed on no-auth server: %v", err)
	}

	if latency < 0 {
		t.Error("expected non-negative latency")
	}

	t.Logf("No-auth server ping latency: %d ns", latency)
}

func TestNoAuth_LoginReturnsError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Login should return "authentication is not enabled" error
	_, err := noAuthClient.Login(ctx, "admin", "root11")
	if err == nil {
		t.Fatal("expected error when logging in to no-auth server")
	}

	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "authentication is not enabled") {
		t.Errorf("expected 'authentication is not enabled' error, got: %v", err)
	}

	t.Logf("Got expected error: %v", err)
}

func TestNoAuth_QueryWithoutLogin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Query should work without login on no-auth server
	resp, err := noAuthClient.Gql(ctx, "MATCH (n) RETURN n LIMIT 1", nil)
	if err != nil {
		// If graph not found, that's ok - the point is we can execute without auth
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			t.Logf("Graph not found (expected on clean server): %v", err)
			return
		}
		t.Fatalf("Query failed on no-auth server: %v", err)
	}

	t.Logf("Query returned %d rows without login", resp.RowCount)
}
