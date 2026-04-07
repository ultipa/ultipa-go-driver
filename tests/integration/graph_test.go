//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestCreateAndDropGraph(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_graph_" + time.Now().Format("20060102150405")

	// Create graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for integration tests")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}

	t.Logf("Created graph: %s", graphName)

	// Get graph info
	info, err := testClient.GetGraphInfo(ctx, graphName)
	if err != nil {
		t.Fatalf("GetGraphInfo failed: %v", err)
	}

	if info.Name != graphName {
		t.Errorf("expected graph name %s, got %s", graphName, info.Name)
	}

	if info.GraphType != gqldb.GraphTypeOpen {
		t.Errorf("expected graph type OPEN, got %v", info.GraphType)
	}

	// List graphs
	graphs, err := testClient.ListGraphs(ctx)
	if err != nil {
		t.Fatalf("ListGraphs failed: %v", err)
	}

	found := false
	for _, g := range graphs {
		if g.Name == graphName {
			found = true
			break
		}
	}

	if !found {
		t.Error("created graph not found in list")
	}

	// Drop graph
	err = testClient.DropGraph(ctx, graphName, false)
	if err != nil {
		t.Fatalf("DropGraph failed: %v", err)
	}

	t.Logf("Dropped graph: %s", graphName)
}

func TestListGraphs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphs, err := testClient.ListGraphs(ctx)
	if err != nil {
		t.Fatalf("ListGraphs failed: %v", err)
	}

	t.Logf("Found %d graphs", len(graphs))
	for _, g := range graphs {
		t.Logf("  - %s (type: %v)", g.Name, g.GraphType)
	}
}
