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

// TestCreateGraphClosed tests creating a graph with GraphType CLOSED.
// server bug open12 #3: CreateGraph with CLOSED type is expected to fail due to a known server bug.
func TestCreateGraphClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_graph_closed_" + time.Now().Format("20060102150405")
	defer dropTestGraph(graphName)

	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeClosed, "Test closed graph")
	if err == nil {
		t.Errorf("expected error creating CLOSED graph (server bug open12 #3), but got nil")
	} else {
		t.Logf("Got expected error for CLOSED graph: %v", err)
	}
}

func TestCreateGraphOntology(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_graph_onto_" + time.Now().Format("20060102150405")
	defer dropTestGraph(graphName)

	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOntology, "Test ontology graph")
	if err != nil {
		t.Fatalf("CreateGraph with ONTOLOGY type failed: %v", err)
	}

	info, err := testClient.GetGraphInfo(ctx, graphName)
	if err != nil {
		t.Fatalf("GetGraphInfo failed: %v", err)
	}

	if info.GraphType != gqldb.GraphTypeOntology {
		t.Errorf("expected graph type ONTOLOGY, got %v", info.GraphType)
	}

	// Cross-check the raw SHOW GRAPHS graph_mode column (the value GetGraphInfo
	// parses into GraphType). Proves the engine reports ONTOLOGY at the source,
	// independent of the enum mapping — catches a silent OPEN downgrade or a
	// GraphTypeFromMode mis-parse.
	resp, err := testClient.Gql(ctx, "SHOW GRAPHS", &gqldb.QueryConfig{})
	if err != nil {
		t.Fatalf("SHOW GRAPHS failed: %v", err)
	}
	var mode string
	found := false
	for _, row := range resp.Rows {
		nameV, _ := resp.GetByName(row, "graph_name")
		if name, ok := nameV.(string); ok && name == graphName {
			modeV, _ := resp.GetByName(row, "graph_mode")
			mode, _ = modeV.(string)
			found = true
			break
		}
	}
	if !found {
		t.Errorf("graph %q not found in SHOW GRAPHS", graphName)
	} else if mode != "ONTOLOGY" {
		t.Errorf("SHOW GRAPHS graph_mode = %q, expected ONTOLOGY", mode)
	}

	t.Logf("Created ontology graph: %s (graph_mode=%s)", graphName, mode)
}

func TestCreateGraphEmptyName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := testClient.CreateGraph(ctx, "", gqldb.GraphTypeOpen, "Graph with empty name")
	if err == nil {
		t.Error("expected error creating graph with empty name, but got nil")
	} else {
		t.Logf("Got expected error for empty name: %v", err)
	}
}

func TestCreateGraphDuplicateName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_graph_dup_" + time.Now().Format("20060102150405")
	defer dropTestGraph(graphName)

	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "First graph")
	if err != nil {
		t.Fatalf("CreateGraph (first) failed: %v", err)
	}

	err = testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Duplicate graph")
	if err == nil {
		t.Error("expected error creating graph with duplicate name, but got nil")
	} else {
		t.Logf("Got expected error for duplicate name: %v", err)
	}
}

func TestDropGraphNonExistent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := testClient.DropGraph(ctx, "test_graph_nonexistent_xyz", false)
	if err == nil {
		t.Error("expected error dropping non-existent graph, but got nil")
	} else {
		t.Logf("Got expected error for non-existent graph: %v", err)
	}
}

func TestDropGraphIfExistsNonExistent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := testClient.DropGraph(ctx, "test_graph_nonexistent_xyz", true)
	if err != nil {
		t.Errorf("DropGraph with if_exists=true on non-existent graph should succeed, got: %v", err)
	} else {
		t.Log("DropGraph with if_exists=true on non-existent graph succeeded as expected")
	}
}

func TestDropGraphEmptyName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := testClient.DropGraph(ctx, "", false)
	if err == nil {
		t.Error("expected error dropping graph with empty name, but got nil")
	} else {
		t.Logf("Got expected error for empty name: %v", err)
	}
}

func TestDropGraphDefault(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := testClient.DropGraph(ctx, "default", false)
	if err == nil {
		t.Error("expected error dropping 'default' graph, but got nil")
	} else {
		t.Logf("Got expected error for dropping default graph: %v", err)
	}
}

func TestUseGraphNonExistent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := testClient.UseGraph(ctx, "test_graph_nonexistent_xyz")
	if err == nil {
		t.Error("expected error using non-existent graph, but got nil")
	} else {
		t.Logf("Got expected error for non-existent graph: %v", err)
	}

	// Restore to a valid graph so subsequent tests are not affected
	_ = testClient.UseGraph(ctx, "miniCircle")
}

func TestUseGraphEmptyName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := testClient.UseGraph(ctx, "")
	if err == nil {
		t.Error("expected error using graph with empty name, but got nil")
	} else {
		t.Logf("Got expected error for empty name: %v", err)
	}
}

func TestGetGraphInfoNonExistent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.GetGraphInfo(ctx, "test_graph_nonexistent_xyz")
	if err == nil {
		t.Error("expected error getting info for non-existent graph, but got nil")
	} else {
		t.Logf("Got expected error for non-existent graph: %v", err)
	}
}

func TestGetGraphInfoEmptyName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.GetGraphInfo(ctx, "")
	if err == nil {
		t.Error("expected error getting info for graph with empty name, but got nil")
	} else {
		t.Logf("Got expected error for empty name: %v", err)
	}
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
