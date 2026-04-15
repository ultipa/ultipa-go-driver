//go:build smoke

// Package smoke provides quick sanity checks to verify basic SDK functionality.
// Run with: go test -tags=smoke ./tests/smoke/...
package smoke

import (
	"context"
	"os"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// Test configuration - can be overridden via environment variables
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

// TestClientCreation verifies that a client can be instantiated successfully.
func TestClientCreation(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(host).
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

	if client == nil {
		t.Fatal("Client is nil")
	}
}

// TestConnectionPing verifies that the client can ping the server.
func TestConnectionPing(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(host).
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

	// Login first if authentication is enabled
	_, loginErr := client.Login(ctx, username, password)
	if loginErr != nil {
		// If auth is not enabled, continue without login
		t.Logf("Login skipped (auth may be disabled): %v", loginErr)
	}

	latency, err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if latency <= 0 {
		t.Errorf("Ping latency should be positive, got: %v", latency)
	}
	t.Logf("Ping latency: %v", latency)
}

// TestAuthentication verifies that login succeeds with valid credentials.
func TestAuthentication(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(host).
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

	sessionInfo, err := client.Login(ctx, username, password)
	if err != nil {
		// Authentication may be disabled on this server
		if containsIgnoreCase(err.Error(), "authentication is not enabled") {
			t.Skip("Authentication is not enabled on this server")
		}
		t.Fatalf("Login failed: %v", err)
	}

	if sessionInfo == nil {
		t.Fatal("Session info is nil")
	}

	t.Logf("Login successful, session created")
}

// TestSimpleQuery verifies that a simple query can be executed.
func TestSimpleQuery(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(host).
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
	_, loginErr := client.Login(ctx, username, password)
	if loginErr != nil {
		t.Logf("Login skipped (auth may be disabled): %v", loginErr)
	}

	// Execute a simple query
	response, err := client.GQL(ctx, "RETURN 1 AS result", nil)
	if err != nil {
		t.Fatalf("Query execution failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	if response.IsEmpty() {
		t.Fatal("Response should not be empty")
	}

	t.Logf("Simple query executed successfully")
}

// TestHealthCheck verifies that the health service responds.
func TestHealthCheck(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts(host).
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

	// Login first
	_, loginErr := client.Login(ctx, username, password)
	if loginErr != nil {
		t.Logf("Login skipped (auth may be disabled): %v", loginErr)
	}

	status, err := client.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	t.Logf("Health status: %v", status)
}

// TestVersionInfo verifies that the SDK version is accessible.
func TestVersionInfo(t *testing.T) {
	version := gqldb.Version()
	if version == "" {
		t.Error("SDK version should not be empty")
	}
	t.Logf("SDK version: %s", version)
}

// Helper function for case-insensitive string contains
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > 0 && len(substr) > 0 &&
				(s[0] == substr[0] || s[0]+32 == substr[0] || s[0] == substr[0]+32) &&
				containsIgnoreCase(s[1:], substr[1:]) ||
			len(s) > 0 && containsIgnoreCase(s[1:], substr))
}
