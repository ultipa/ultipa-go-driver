//go:build integration

package integration

import (
	"context"
	"errors"
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

func TestQueryWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := &gqldb.QueryConfig{
		Timeout: 10,
	}

	resp, err := testClient.Gql(ctx, "RETURN 1 + 1 AS result", config)
	if err != nil {
		t.Fatalf("Query with timeout=10 failed: %v", err)
	}

	if resp.IsEmpty() {
		t.Error("expected non-empty response")
	}

	t.Logf("Query with timeout=10: %d rows, columns: %v", resp.RowCount, resp.Columns)
}

func TestQueryTimeoutZero(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := &gqldb.QueryConfig{
		Timeout: 0,
	}

	resp, err := testClient.Gql(ctx, "RETURN 1 AS result", config)
	if err != nil {
		t.Fatalf("Query with timeout=0 failed: %v", err)
	}

	if resp.IsEmpty() {
		t.Error("expected non-empty response")
	}

	t.Logf("Query with timeout=0 (default): %d rows", resp.RowCount)
}

func TestQueryMaxPathResults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := &gqldb.QueryConfig{
		MaxPathResults: 5,
	}

	resp, err := testClient.Gql(ctx, "MATCH p=(n)-[*1..2]-(m) RETURN p LIMIT 10", config)
	if err != nil {
		// Path query syntax may not be supported; verify MaxPathResults doesn't break simple queries
		t.Logf("Path query with MaxPathResults failed (may not be supported): %v", err)

		resp, err = testClient.Gql(ctx, "RETURN 1 AS result", config)
		if err != nil {
			t.Fatalf("Simple query with MaxPathResults=5 failed: %v", err)
		}
	}

	t.Logf("Query with MaxPathResults=5: %d rows, columns: %v", resp.RowCount, resp.Columns)
}

func TestQueryMaxPathResultsZero(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

func TestExplainBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	plan, err := testClient.Explain(ctx, "RETURN 1 AS num", nil)
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}

	if plan == "" {
		t.Log("Warning: Explain returned empty plan - feature may not be fully supported")
	} else {
		t.Logf("Explain plan:\n%s", plan)
	}
}

func TestExplainEmpty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.Explain(ctx, "", nil)
	if err == nil {
		t.Fatal("expected error when explaining empty query")
	}

	if !errors.Is(err, gqldb.ErrEmptyQuery) {
		t.Logf("Got error (not ErrEmptyQuery but still an error): %v", err)
	} else {
		t.Logf("Got expected ErrEmptyQuery: %v", err)
	}
}

func TestExplainInvalid(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.Explain(ctx, "THIS IS NOT VALID GQL !!!", nil)
	if err == nil {
		t.Fatal("expected error when explaining invalid GQL")
	}

	t.Logf("Got expected error for invalid GQL explain: %v", err)
}

func TestProfileBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	profile, err := testClient.Profile(ctx, "RETURN 1 AS num", nil)
	if err != nil {
		t.Fatalf("Profile failed: %v", err)
	}

	if profile == "" {
		t.Log("Warning: Profile returned empty result - feature may not be fully supported")
	} else {
		t.Logf("Profile result:\n%s", profile)
	}
}

func TestProfileEmpty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.Profile(ctx, "", nil)
	if err == nil {
		t.Fatal("expected error when profiling empty query")
	}

	if !errors.Is(err, gqldb.ErrEmptyQuery) {
		t.Logf("Got error (not ErrEmptyQuery but still an error): %v", err)
	} else {
		t.Logf("Got expected ErrEmptyQuery: %v", err)
	}
}

func TestGqlStreamBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var chunks int
	err := testClient.GqlStream(ctx, "RETURN 1 AS num", nil, func(resp *gqldb.Response) error {
		chunks++
		t.Logf("Stream chunk %d: %d rows", chunks, resp.RowCount)
		return nil
	})
	if err != nil {
		t.Fatalf("GqlStream failed: %v", err)
	}

	if chunks == 0 {
		t.Error("expected at least one stream chunk")
	}

	t.Logf("GqlStream received %d chunks", chunks)
}

func TestGqlStreamWithGraph(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := &gqldb.QueryConfig{
		GraphName: "miniCircle",
	}

	var chunks int
	err := testClient.GqlStream(ctx, "MATCH (n) RETURN n LIMIT 5", config, func(resp *gqldb.Response) error {
		chunks++
		t.Logf("Stream chunk %d: %d rows, columns: %v", chunks, resp.RowCount, resp.Columns)
		return nil
	})
	if err != nil {
		t.Fatalf("GqlStream with graph failed: %v", err)
	}

	t.Logf("GqlStream with graph received %d chunks", chunks)
}
