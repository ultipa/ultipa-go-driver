//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

var testClient *gqldb.Client
var noAuthClient *gqldb.Client // Client for server without authentication

// Server addresses
const (
	authHost   = "192.168.1.100:60061" // Server with authentication
	noAuthHost = "192.168.1.100:60062" // Server without authentication
)

func init() {
	// Bypass local proxy for gRPC connections — must be set before any gRPC dial
	os.Setenv("NO_PROXY", "*")
	os.Setenv("no_proxy", "*")
	os.Setenv("HTTP_PROXY", "")
	os.Setenv("HTTPS_PROXY", "")
	os.Setenv("http_proxy", "")
	os.Setenv("https_proxy", "")
}

func TestMain(m *testing.M) {

	// Setup
	host := os.Getenv("GQLDB_HOST")
	if host == "" {
		host = authHost
	}

	username := os.Getenv("GQLDB_USERNAME")
	if username == "" {
		username = "admin"
	}

	password := os.Getenv("GQLDB_PASSWORD")
	if password == "" {
		password = "root11"
	}

	// Try to create auth client (optional - may not be available)
	config := gqldb.NewConfigBuilder().
		Hosts(host).
		Username(username).
		Password(password).
		DefaultGraph("miniCircle").
		Timeout(30 * time.Second).
		Build()

	var err error
	testClient, err = gqldb.NewClient(config)
	if err != nil {
		println("Warning: Failed to create auth client:", err.Error())
		testClient = nil
	}

	// Create client for no-auth server
	noAuthHostEnv := os.Getenv("GQLDB_NOAUTH_HOST")
	if noAuthHostEnv == "" {
		noAuthHostEnv = noAuthHost
	}
	noAuthConfig := gqldb.NewConfigBuilder().
		Hosts(noAuthHostEnv).
		DefaultGraph("miniCircle").
		Timeout(300 * time.Second). // Longer timeout for bulk imports
		MaxRecvSize(128 * 1024 * 1024). // 128MB for large responses
		Build()

	noAuthClient, err = gqldb.NewClient(noAuthConfig)
	if err != nil {
		println("Warning: Failed to create no-auth client:", err.Error())
		noAuthClient = nil
	}

	// Login if auth client is available
	if testClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = testClient.Login(ctx, username, password)
		cancel()
		if err != nil {
			println("Warning: Failed to login:", err.Error())
			testClient.Close()
			testClient = nil
		}
	}

	// Clean up leftover test graphs before running tests
	cleanupTestGraphs()

	// Run tests
	code := m.Run()

	// Clean up leftover test graphs after running tests
	cleanupTestGraphs()

	// Teardown
	if testClient != nil {
		testClient.Close()
	}
	if noAuthClient != nil {
		noAuthClient.Close()
	}

	os.Exit(code)
}

// cleanupTestGraphs drops any leftover test graphs (names starting with "test_")
// to prevent hitting the 3-graph database limit.
func cleanupTestGraphs() {
	if testClient == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphs, err := testClient.ListGraphs(ctx)
	if err != nil {
		return
	}
	for _, g := range graphs {
		if strings.HasPrefix(g.Name, "test_") {
			println("Cleaning up leftover test graph:", g.Name)
			_ = testClient.UseGraph(ctx, "miniCircle")
			_ = testClient.DropGraph(ctx, g.Name, true)
		}
	}
}

// dropTestGraph switches away from the test graph and drops it.
// Server rejects dropping the "currently active graph", so we switch first.
func dropTestGraph(graphName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = testClient.UseGraph(ctx, "miniCircle")
	_ = testClient.DropGraph(ctx, graphName, true)
}
