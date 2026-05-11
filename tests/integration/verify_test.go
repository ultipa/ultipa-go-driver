//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestVerifyConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Re-login to ensure session is valid
	session, err := testClient.Login(ctx, testUsername, testPassword)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	t.Logf("Login successful - Session ID: %d, Server Version: %s, Roles: %v",
		session.ID, session.ServerVersion, session.Roles)

	// Test list graphs
	graphs, err := testClient.ListGraphs(ctx)
	if err != nil {
		t.Fatalf("ListGraphs failed: %v", err)
	}
	t.Logf("Found %d graphs", len(graphs))

	// Test create graph
	graphName := "test_verify_" + time.Now().Format("20060102150405")
	err = testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Verification test graph")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	t.Logf("Created graph: %s", graphName)

	// Test drop graph
	err = testClient.DropGraph(ctx, graphName, false)
	if err != nil {
		t.Fatalf("DropGraph failed: %v", err)
	}
	t.Logf("Dropped graph: %s", graphName)

	t.Log("All verification tests passed!")
}

// TestNoAuthServer tests operations on the server without authentication
func TestNoAuthServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect without authentication
	err := noAuthClient.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	t.Log("Connected to no-auth server")

	// Test list graphs
	graphs, err := noAuthClient.ListGraphs(ctx)
	if err != nil {
		t.Fatalf("ListGraphs failed: %v", err)
	}
	t.Logf("Found %d graphs on no-auth server", len(graphs))

	// Test create graph
	graphName := "test_noauth_" + time.Now().Format("20060102150405")
	err = noAuthClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "No-auth test graph")
	if err != nil {
		t.Fatalf("CreateGraph failed on no-auth server: %v", err)
	}
	t.Logf("Created graph on no-auth server: %s", graphName)

	// Test drop graph
	err = noAuthClient.DropGraph(ctx, graphName, false)
	if err != nil {
		t.Fatalf("DropGraph failed on no-auth server: %v", err)
	}
	t.Logf("Dropped graph on no-auth server: %s", graphName)

	t.Log("All no-auth server tests passed!")
}

// TestDebugSessionMetadata prints the session ID being sent
func TestDebugSessionMetadata(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Login
	session, err := testClient.Login(ctx, testUsername, testPassword)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	t.Logf("=== Session Info ===")
	t.Logf("Session ID: %d", session.ID)
	t.Logf("Session ID (string): \"%d\"", session.ID)
	t.Logf("Server Version: %s", session.ServerVersion)
	t.Logf("Roles: %v", session.Roles)

	// Verify session is stored
	currentSession := testClient.GetSession()
	if currentSession != nil {
		t.Logf("Stored Session ID: %d", currentSession.ID)
	} else {
		t.Log("WARNING: No session stored!")
	}

	t.Logf("IsLoggedIn: %v", testClient.IsLoggedIn())

	// The metadata being sent would be: "session-id" = "18" (or whatever the ID is)
	t.Logf("Metadata header: session-id = \"%d\"", session.ID)
}

// TestGqlCreateGraphSimple tests CREATE GRAPH via GQL statement
func TestGqlCreateGraphSimple(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Login
	session, err := testClient.Login(ctx, testUsername, testPassword)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	t.Logf("Login successful - Session ID: %d", session.ID)

	// First, list graphs to find an existing one
	graphs, err := testClient.ListGraphs(ctx)
	if err != nil {
		t.Fatalf("ListGraphs failed: %v", err)
	}
	t.Logf("Found %d graphs", len(graphs))
	for _, g := range graphs {
		t.Logf("  - %s", g.Name)
	}

	// Use an existing graph first
	if len(graphs) > 0 {
		err = testClient.UseGraph(ctx, graphs[0].Name)
		if err != nil {
			t.Logf("UseGraph failed: %v", err)
		} else {
			t.Logf("UseGraph successful: %s", graphs[0].Name)
		}
	}

	// Test CREATE GRAPH via GQL with graph context
	config := &gqldb.QueryConfig{GraphName: "default"}
	resp, err := testClient.Gql(ctx, "CREATE GRAPH test", config)
	if err != nil {
		t.Logf("GQL CREATE GRAPH test failed: %v", err)
	} else {
		t.Logf("GQL CREATE GRAPH test result: rows=%d, warnings=%v", resp.RowCount, resp.Warnings)
	}

	// Try DROP GRAPH test
	resp, err = testClient.Gql(ctx, "DROP GRAPH test", config)
	if err != nil {
		t.Logf("GQL DROP GRAPH test failed: %v", err)
	} else {
		t.Logf("GQL DROP GRAPH test result: rows=%d, warnings=%v", resp.RowCount, resp.Warnings)
	}
}

// TestPingWithSession tests ping with session to ensure session is working
func TestPingWithSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Re-login to ensure session is valid
	session, err := testClient.Login(ctx, testUsername, testPassword)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	t.Logf("Login successful - Session ID: %d", session.ID)

	// Test ping (requires session)
	latency, err := testClient.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	t.Logf("Ping successful, latency: %d ns", latency)

	// Test list graphs (should work)
	graphs, err := testClient.ListGraphs(ctx)
	if err != nil {
		t.Fatalf("ListGraphs failed: %v", err)
	}
	t.Logf("ListGraphs successful, found %d graphs", len(graphs))

	// Get graph info for existing graph
	if len(graphs) > 0 {
		info, err := testClient.GetGraphInfo(ctx, graphs[0].Name)
		if err != nil {
			t.Logf("GetGraphInfo failed: %v", err)
		} else {
			t.Logf("GetGraphInfo successful for %s: nodes=%d, edges=%d", info.Name, info.NodeCount, info.EdgeCount)
		}
	}
}
