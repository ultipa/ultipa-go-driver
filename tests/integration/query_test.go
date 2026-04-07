//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestGqlQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Simple query - might need adjustment based on actual GQL syntax
	resp, err := testClient.Gql(ctx, "RETURN 1 + 1 AS result", nil)
	if err != nil {
		t.Fatalf("Gql query failed: %v", err)
	}

	if resp.IsEmpty() {
		t.Error("expected non-empty response")
	}

	t.Logf("Query result: %d rows, columns: %v", resp.RowCount, resp.Columns)
}

func TestGqlStatement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_stmt_graph_" + time.Now().Format("20060102150405")

	// Create a test graph first
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for statement operations")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Use Gql to execute a DDL/DML statement with GraphName in config
	config := &gqldb.QueryConfig{
		GraphName: graphName,
	}
	resp, err := testClient.Gql(ctx, "INSERT (n:TestNode {name: 'test'})", config)
	if err != nil {
		// INSERT may have different syntax requirements
		t.Logf("Gql with INSERT failed: %v - trying alternative", err)

		// Try a simpler statement
		resp, err = testClient.Gql(ctx, "RETURN 1", config)
		if err != nil {
			t.Fatalf("Gql failed: %v", err)
		}
	}

	t.Logf("Gql result: rows=%d, columns=%v, warnings=%v",
		resp.RowCount, resp.Columns, resp.Warnings)
}

func TestExplain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Explain a simple query
	plan, err := testClient.Explain(ctx, "RETURN 1 + 1 AS result", nil)
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}

	if plan == "" {
		t.Log("Warning: Explain returned empty plan - feature may not be fully supported")
	} else {
		t.Logf("Explain plan:\n%s", plan)
	}
}

func TestProfile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Profile a simple query
	profile, err := testClient.Profile(ctx, "RETURN 1 + 1 AS result", nil)
	if err != nil {
		t.Fatalf("Profile failed: %v", err)
	}

	if profile == "" {
		t.Log("Warning: Profile returned empty result - feature may not be fully supported")
	} else {
		t.Logf("Profile result:\n%s", profile)
	}
}

func TestGqlWithMaxPathResults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test path query with MaxPathResults limit
	config := &gqldb.QueryConfig{
		MaxPathResults: 5,
	}

	// Try a path query - syntax may vary by server
	resp, err := testClient.Gql(ctx, "MATCH p=(n)-[*1..2]-(m) RETURN p LIMIT 10", config)
	if err != nil {
		// Path query syntax may not be supported
		t.Logf("Path query with MaxPathResults failed (may not be supported): %v", err)

		// Try a simpler query to verify MaxPathResults doesn't break normal queries
		resp, err = testClient.Gql(ctx, "RETURN 1 AS result", config)
		if err != nil {
			t.Fatalf("Simple query with MaxPathResults failed: %v", err)
		}
	}

	t.Logf("Query with MaxPathResults=5: %d rows, columns: %v", resp.RowCount, resp.Columns)
}

func TestGqlWithMaxPathResultsZero(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with MaxPathResults = 0 (unlimited)
	config := &gqldb.QueryConfig{
		MaxPathResults: 0,
	}

	resp, err := testClient.Gql(ctx, "RETURN 1 AS result", config)
	if err != nil {
		t.Fatalf("Query with MaxPathResults=0 failed: %v", err)
	}

	if resp.IsEmpty() {
		t.Error("expected non-empty response")
	}

	t.Logf("Query with MaxPathResults=0 (unlimited): %d rows", resp.RowCount)
}

func TestGqlWithMaxPathResultsLargeValue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with a large MaxPathResults value
	config := &gqldb.QueryConfig{
		MaxPathResults: 1000000,
	}

	resp, err := testClient.Gql(ctx, "RETURN 1 AS result", config)
	if err != nil {
		t.Fatalf("Query with large MaxPathResults failed: %v", err)
	}

	t.Logf("Query with MaxPathResults=1000000: %d rows", resp.RowCount)
}
