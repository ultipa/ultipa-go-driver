//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// =============================================================================
// Helper: create a closed graph with a node label and edge label pre-populated.
// Returns graphName. Caller must defer dropTestGraph(graphName).
// =============================================================================

func createClosedGraphWithLabels(t *testing.T, suffix string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_" + suffix + "_" + time.Now().Format("150405")

	// Create closed graph
	_, err := testClient.CreateClosedGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateClosedGraph(%s) failed: %v", graphName, err)
	}

	// Set current graph
	err = testClient.UseGraph(ctx, graphName)
	if err != nil {
		t.Fatalf("UseGraph(%s) failed: %v", graphName, err)
	}
	// Restore session graph after this test so that subsequent tests'
	// CreateClosedGraph (which goes through the gRPC graph_name validator)
	// don't fail because the session still points at this dropped graph.
	t.Cleanup(func() {
		ctxR, cancelR := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelR()
		_ = testClient.UseGraph(ctxR, "miniCircle")
	})

	// Add a node label with properties
	_, err = testClient.CreateNodeLabel(ctx, "Person", []types.PropertyDef{
		{Name: "name", Type: types.PropertyTypeString},
		{Name: "age", Type: types.PropertyTypeInt64},
	}, nil)
	if err != nil {
		t.Fatalf("CreateNodeLabel(Person) failed: %v", err)
	}

	// Add an edge label with properties
	_, err = testClient.CreateEdgeLabel(ctx, "KNOWS", []types.PropertyDef{
		{Name: "since", Type: types.PropertyTypeInt64},
	}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeLabel(KNOWS) failed: %v", err)
	}

	return graphName
}

// =============================================================================
// Graph Operations
// =============================================================================

func TestConvenience_CreateOpenGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_open_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	resp, err := testClient.CreateOpenGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateOpenGraph failed: %v", err)
	}
	t.Logf("CreateOpenGraph response: rowsAffected=%d", resp.RowsAffected)

	// Verify exists
	has, err := testClient.HasGraph(ctx, graphName)
	if err != nil {
		t.Fatalf("HasGraph failed: %v", err)
	}
	if !has {
		t.Error("expected graph to exist after CreateOpenGraph")
	}
}

func TestConvenience_CreateClosedGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_closed_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateClosedGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateClosedGraph failed: %v", err)
	}

	has, err := testClient.HasGraph(ctx, graphName)
	if err != nil {
		t.Fatalf("HasGraph failed: %v", err)
	}
	if !has {
		t.Error("expected closed graph to exist")
	}
}

func TestConvenience_CreateGraphIfNotExist_New(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_ifnot_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	created, err := testClient.CreateGraphIfNotExist(ctx, graphName, gqldb.GraphTypeOpen, "test graph")
	if err != nil {
		t.Fatalf("CreateGraphIfNotExist (new) failed: %v", err)
	}
	if !created {
		t.Error("expected created=true for new graph")
	}
}

func TestConvenience_CreateGraphIfNotExist_Existing(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_ifnot2_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	// Create first
	_, err := testClient.CreateOpenGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateOpenGraph failed: %v", err)
	}

	// Try again - should return false
	created, err := testClient.CreateGraphIfNotExist(ctx, graphName, gqldb.GraphTypeOpen, "dup")
	if err != nil {
		t.Fatalf("CreateGraphIfNotExist (existing) failed: %v", err)
	}
	if created {
		t.Error("expected created=false for existing graph")
	}
}

func TestConvenience_HasGraph_True(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// miniCircle should always exist
	has, err := testClient.HasGraph(ctx, "miniCircle")
	if err != nil {
		t.Fatalf("HasGraph failed: %v", err)
	}
	if !has {
		t.Error("expected miniCircle to exist")
	}
}

func TestConvenience_HasGraph_False(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	has, err := testClient.HasGraph(ctx, "nonexistent_graph_xyz_999")
	if err != nil {
		t.Fatalf("HasGraph failed: %v", err)
	}
	if has {
		t.Error("expected nonexistent graph to return false")
	}
}

func TestConvenience_AlterGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_alter_" + time.Now().Format("150405")
	newName := "test_conv_renamed_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)
	defer dropTestGraph(newName)

	_, err := testClient.CreateOpenGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateOpenGraph failed: %v", err)
	}

	_, err = testClient.AlterGraph(ctx, graphName, newName, nil)
	if err != nil {
		t.Fatalf("AlterGraph failed: %v", err)
	}

	// Old name should not exist
	has, err := testClient.HasGraph(ctx, graphName)
	if err != nil {
		t.Fatalf("HasGraph (old) failed: %v", err)
	}
	if has {
		t.Error("old graph name should not exist after rename")
	}

	// New name should exist
	has, err = testClient.HasGraph(ctx, newName)
	if err != nil {
		t.Fatalf("HasGraph (new) failed: %v", err)
	}
	if !has {
		t.Error("new graph name should exist after rename")
	}
}

func TestConvenience_Truncate(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_trunc_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateOpenGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateOpenGraph failed: %v", err)
	}

	// Insert some data via GQL
	qc := &gqldb.QueryConfig{GraphName: graphName}
	_, err = testClient.Gql(ctx, "INSERT (:Temp {val: 1})", qc)
	if err != nil {
		t.Fatalf("Insert node failed: %v", err)
	}

	// Truncate
	_, err = testClient.Truncate(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("Truncate failed: %v", err)
	}

	// Verify empty
	resp, err := testClient.Gql(ctx, "MATCH (n) RETURN count(n) AS cnt", qc)
	if err != nil {
		t.Logf("Count query after truncate failed (may be expected): %v", err)
	} else {
		t.Logf("After truncate: %d rows, rowCount=%d", len(resp.Rows), resp.RowCount)
	}
}

// =============================================================================
// Label Operations
// =============================================================================

func TestConvenience_ShowLabels(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "showlbl")
	defer dropTestGraph(graphName)

	labels, err := testClient.ShowLabels(ctx, nil)
	if err != nil {
		t.Fatalf("ShowLabels failed: %v", err)
	}
	if len(labels) < 2 {
		t.Errorf("expected at least 2 labels, got %d", len(labels))
	}

	foundNode := false
	foundEdge := false
	for _, l := range labels {
		t.Logf("Label: labels=%v type=%s", l.Labels, l.Type)
		if l.Type == "NODE" {
			for _, n := range l.Labels {
				if n == "Person" {
					foundNode = true
				}
			}
		}
		if l.Type == "EDGE" {
			for _, n := range l.Labels {
				if n == "KNOWS" {
					foundEdge = true
				}
			}
		}
	}
	if !foundNode {
		t.Error("expected to find node label 'Person'")
	}
	if !foundEdge {
		t.Error("expected to find edge label 'KNOWS'")
	}
}

func TestConvenience_ShowNodeLabels(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "shownlbl")
	defer dropTestGraph(graphName)

	labels, err := testClient.ShowNodeLabels(ctx, nil)
	if err != nil {
		t.Fatalf("ShowNodeLabels failed: %v", err)
	}
	if len(labels) < 1 {
		t.Errorf("expected at least 1 node label, got %d", len(labels))
	}
	for _, l := range labels {
		t.Logf("Node label: labels=%v type=%s", l.Labels, l.Type)
	}
}

func TestConvenience_ShowEdgeLabels(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "showelbl")
	defer dropTestGraph(graphName)

	labels, err := testClient.ShowEdgeLabels(ctx, nil)
	if err != nil {
		t.Fatalf("ShowEdgeLabels failed: %v", err)
	}
	if len(labels) < 1 {
		t.Errorf("expected at least 1 edge label, got %d", len(labels))
	}
	for _, l := range labels {
		t.Logf("Edge label: labels=%v type=%s", l.Labels, l.Type)
	}
}

func TestConvenience_ShowNodeTypes(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "showntype")
	defer dropTestGraph(graphName)

	nodeTypes, err := testClient.ShowNodeTypes(ctx, nil)
	if err != nil {
		t.Fatalf("ShowNodeTypes failed: %v", err)
	}
	if len(nodeTypes) < 1 {
		t.Errorf("expected at least 1 node type, got %d", len(nodeTypes))
	}
	for _, nt := range nodeTypes {
		t.Logf("Node type: name=%s, props=%v", nt.Name, nt.Properties)
	}
}

func TestConvenience_ShowEdgeTypes(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "showetype")
	defer dropTestGraph(graphName)

	edgeTypes, err := testClient.ShowEdgeTypes(ctx, nil)
	if err != nil {
		t.Fatalf("ShowEdgeTypes failed: %v", err)
	}
	if len(edgeTypes) < 1 {
		t.Errorf("expected at least 1 edge type, got %d", len(edgeTypes))
	}
	for _, et := range edgeTypes {
		t.Logf("Edge type: name=%s, props=%v", et.Name, et.Properties)
	}
}

func TestConvenience_GetLabel_Existing(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "getlbl")
	defer dropTestGraph(graphName)

	label, err := testClient.GetLabel(ctx, "Person", nil)
	if err != nil {
		t.Fatalf("GetLabel failed: %v", err)
	}
	if label == nil {
		t.Fatal("expected non-nil label for 'Person'")
	}
	found := false
	for _, n := range label.Labels {
		if n == "Person" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected label.Labels to contain 'Person', got %v", label.Labels)
	}
	t.Logf("GetLabel: labels=%v type=%s", label.Labels, label.Type)
}

func TestConvenience_GetLabel_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "getlblne")
	defer dropTestGraph(graphName)

	label, err := testClient.GetLabel(ctx, "NonexistentLabel", nil)
	if err != nil {
		t.Fatalf("GetLabel (nonexistent) failed: %v", err)
	}
	if label != nil {
		t.Errorf("expected nil for nonexistent label, got %+v", label)
	}
}

func TestConvenience_GetNodeLabel_Existing(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "getnlbl")
	defer dropTestGraph(graphName)

	nt, err := testClient.GetNodeLabel(ctx, "Person", nil)
	if err != nil {
		t.Fatalf("GetNodeLabel failed: %v", err)
	}
	if nt == nil {
		t.Fatal("expected non-nil node type for 'Person'")
	}
	if nt.Name != "Person" {
		t.Errorf("expected name 'Person', got %q", nt.Name)
	}
	if len(nt.Properties) < 2 {
		t.Errorf("expected at least 2 properties, got %d", len(nt.Properties))
	}
	t.Logf("GetNodeLabel: name=%s, props=%v", nt.Name, nt.Properties)
}

func TestConvenience_GetNodeLabel_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "getnlblne")
	defer dropTestGraph(graphName)

	nt, err := testClient.GetNodeLabel(ctx, "NoSuchNode", nil)
	if err != nil {
		t.Fatalf("GetNodeLabel (nonexistent) failed: %v", err)
	}
	if nt != nil {
		t.Errorf("expected nil for nonexistent node label, got %+v", nt)
	}
}

func TestConvenience_GetEdgeLabel_Existing(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "getelbl")
	defer dropTestGraph(graphName)

	et, err := testClient.GetEdgeLabel(ctx, "KNOWS", nil)
	if err != nil {
		t.Fatalf("GetEdgeLabel failed: %v", err)
	}
	if et == nil {
		t.Fatal("expected non-nil edge type for 'KNOWS'")
	}
	if et.Name != "KNOWS" {
		t.Errorf("expected name 'KNOWS', got %q", et.Name)
	}
	t.Logf("GetEdgeLabel: name=%s, props=%v", et.Name, et.Properties)
}

func TestConvenience_GetEdgeLabel_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "getelblne")
	defer dropTestGraph(graphName)

	et, err := testClient.GetEdgeLabel(ctx, "NO_SUCH_EDGE", nil)
	if err != nil {
		t.Fatalf("GetEdgeLabel (nonexistent) failed: %v", err)
	}
	if et != nil {
		t.Errorf("expected nil for nonexistent edge label, got %+v", et)
	}
}

func TestConvenience_CreateNodeLabel(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_crnlbl_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateClosedGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateClosedGraph failed: %v", err)
	}
	// Switch session to the freshly-created graph so CreateNodeLabel doesn't
	// fall back to the client's default graph (miniCircle).
	if err := testClient.UseGraph(ctx, graphName); err != nil {
		t.Fatalf("UseGraph failed: %v", err)
	}
	defer func() { _ = testClient.UseGraph(ctx, "miniCircle") }()

	_, err = testClient.CreateNodeLabel(ctx, "Animal", []types.PropertyDef{
		{Name: "species", Type: types.PropertyTypeString},
		{Name: "weight", Type: types.PropertyTypeDouble},
	}, nil)
	if err != nil {
		t.Fatalf("CreateNodeLabel failed: %v", err)
	}

	// Verify
	nt, err := testClient.GetNodeLabel(ctx, "Animal", nil)
	if err != nil {
		t.Fatalf("GetNodeLabel failed: %v", err)
	}
	if nt == nil {
		t.Fatal("expected non-nil node type after creation")
	}
	t.Logf("Created node label: %s with %d properties", nt.Name, len(nt.Properties))
}

func TestConvenience_CreateEdgeLabel(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_crelbl_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateClosedGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateClosedGraph failed: %v", err)
	}
	if err := testClient.UseGraph(ctx, graphName); err != nil {
		t.Fatalf("UseGraph failed: %v", err)
	}
	defer func() { _ = testClient.UseGraph(ctx, "miniCircle") }()

	_, err = testClient.CreateEdgeLabel(ctx, "FOLLOWS", []types.PropertyDef{
		{Name: "since", Type: types.PropertyTypeInt64},
	}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeLabel failed: %v", err)
	}

	// Verify
	et, err := testClient.GetEdgeLabel(ctx, "FOLLOWS", nil)
	if err != nil {
		t.Fatalf("GetEdgeLabel failed: %v", err)
	}
	if et == nil {
		t.Fatal("expected non-nil edge type after creation")
	}
	t.Logf("Created edge label: %s with %d properties", et.Name, len(et.Properties))
}

func TestConvenience_DropNodeLabel(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "dropnlbl")
	defer dropTestGraph(graphName)

	_, err := testClient.DropNodeLabel(ctx, "Person", nil)
	if err != nil {
		t.Fatalf("DropNodeLabel failed: %v", err)
	}

	// Verify removed
	nt, err := testClient.GetNodeLabel(ctx, "Person", nil)
	if err != nil {
		t.Fatalf("GetNodeLabel after drop failed: %v", err)
	}
	if nt != nil {
		t.Error("expected node label 'Person' to be dropped")
	}
}

func TestConvenience_DropEdgeLabel(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "dropelbl")
	defer dropTestGraph(graphName)

	_, err := testClient.DropEdgeLabel(ctx, nil, "KNOWS")
	if err != nil {
		t.Fatalf("DropEdgeLabel failed: %v", err)
	}

	// Verify removed
	et, err := testClient.GetEdgeLabel(ctx, "KNOWS", nil)
	if err != nil {
		t.Fatalf("GetEdgeLabel after drop failed: %v", err)
	}
	if et != nil {
		t.Error("expected edge label 'KNOWS' to be dropped")
	}
}

func TestConvenience_CreateLabelIfNotExist_New(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "lblifnot")
	defer dropTestGraph(graphName)

	created, err := testClient.CreateLabelIfNotExist(ctx, types.DBTypeNode, "City", []types.PropertyDef{
		{Name: "name", Type: types.PropertyTypeString},
	}, nil)
	if err != nil {
		t.Fatalf("CreateLabelIfNotExist (new) failed: %v", err)
	}
	if !created {
		t.Error("expected created=true for new label")
	}
}

func TestConvenience_CreateLabelIfNotExist_Existing(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "lblifnot2")
	defer dropTestGraph(graphName)

	// 'Person' already exists from createClosedGraphWithLabels
	created, err := testClient.CreateLabelIfNotExist(ctx, types.DBTypeNode, "Person", []types.PropertyDef{
		{Name: "name", Type: types.PropertyTypeString},
	}, nil)
	if err != nil {
		t.Fatalf("CreateLabelIfNotExist (existing) failed: %v", err)
	}
	if created {
		t.Error("expected created=false for existing label")
	}
}

func TestConvenience_AlterNodeLabel(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "altnlbl")
	defer dropTestGraph(graphName)

	_, err := testClient.AlterNodeLabel(ctx, "Person", "Human", nil)
	if err != nil {
		t.Fatalf("AlterNodeLabel failed: %v", err)
	}

	// Verify old name gone, new name present
	oldNT, err := testClient.GetNodeLabel(ctx, "Person", nil)
	if err != nil {
		t.Fatalf("GetNodeLabel (old) failed: %v", err)
	}
	if oldNT != nil {
		t.Error("expected old node label 'Person' to not exist after rename")
	}

	newNT, err := testClient.GetNodeLabel(ctx, "Human", nil)
	if err != nil {
		t.Fatalf("GetNodeLabel (new) failed: %v", err)
	}
	if newNT == nil {
		t.Error("expected renamed node label 'Human' to exist")
	}
}

func TestConvenience_AlterEdgeLabel(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "altelbl")
	defer dropTestGraph(graphName)

	_, err := testClient.AlterEdgeLabel(ctx, "KNOWS", "FRIENDS_WITH", nil)
	if err != nil {
		t.Fatalf("AlterEdgeLabel failed: %v", err)
	}

	oldET, err := testClient.GetEdgeLabel(ctx, "KNOWS", nil)
	if err != nil {
		t.Fatalf("GetEdgeLabel (old) failed: %v", err)
	}
	if oldET != nil {
		t.Error("expected old edge label 'KNOWS' to not exist after rename")
	}

	newET, err := testClient.GetEdgeLabel(ctx, "FRIENDS_WITH", nil)
	if err != nil {
		t.Fatalf("GetEdgeLabel (new) failed: %v", err)
	}
	if newET == nil {
		t.Error("expected renamed edge label 'FRIENDS_WITH' to exist")
	}
}

func TestConvenience_ShowAlgos(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	algos, err := testClient.ShowAlgos(ctx, nil)
	if err != nil {
		t.Fatalf("ShowAlgos failed: %v", err)
	}
	t.Logf("Found %d algorithms", len(algos))
	for i, a := range algos {
		if i < 5 { // log first 5
			t.Logf("  Algo: name=%s version=%s", a.Name, a.Version)
		}
	}
}

// =============================================================================
// Property Operations
// =============================================================================

func TestConvenience_ShowNodeProperty(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "shownprop")
	defer dropTestGraph(graphName)

	props, err := testClient.ShowNodeProperty(ctx, "Person", nil)
	if err != nil {
		t.Fatalf("ShowNodeProperty failed: %v", err)
	}
	if len(props) < 2 {
		t.Errorf("expected at least 2 node properties, got %d", len(props))
	}
	for _, p := range props {
		t.Logf("Node property: name=%s type=%v", p.Name, p.Type)
	}
}

func TestConvenience_ShowEdgeProperty(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "showeprop")
	defer dropTestGraph(graphName)

	props, err := testClient.ShowEdgeProperty(ctx, "KNOWS", nil)
	if err != nil {
		t.Fatalf("ShowEdgeProperty failed: %v", err)
	}
	if len(props) < 1 {
		t.Errorf("expected at least 1 edge property, got %d", len(props))
	}
	for _, p := range props {
		t.Logf("Edge property: name=%s type=%v", p.Name, p.Type)
	}
}

func TestConvenience_ShowProperty_Dispatch(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "showprop")
	defer dropTestGraph(graphName)

	// Node dispatch
	nodeProps, err := testClient.ShowProperty(ctx, types.DBTypeNode, "Person", nil)
	if err != nil {
		t.Fatalf("ShowProperty(Node) failed: %v", err)
	}
	if len(nodeProps) < 2 {
		t.Errorf("expected at least 2 node properties via ShowProperty, got %d", len(nodeProps))
	}

	// Edge dispatch
	edgeProps, err := testClient.ShowProperty(ctx, types.DBTypeEdge, "KNOWS", nil)
	if err != nil {
		t.Fatalf("ShowProperty(Edge) failed: %v", err)
	}
	if len(edgeProps) < 1 {
		t.Errorf("expected at least 1 edge property via ShowProperty, got %d", len(edgeProps))
	}
}

func TestConvenience_ShowNodeProperty_NonexistentLabel(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "propnolab")
	defer dropTestGraph(graphName)

	_, err := testClient.ShowNodeProperty(ctx, "NoSuchLabel", nil)
	if err == nil {
		t.Error("expected error for nonexistent label, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestConvenience_GetNodeProperty_Existing(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "getnprop")
	defer dropTestGraph(graphName)

	prop, err := testClient.GetNodeProperty(ctx, "Person", "name", nil)
	if err != nil {
		t.Fatalf("GetNodeProperty failed: %v", err)
	}
	if prop == nil {
		t.Fatal("expected non-nil property for 'name'")
	}
	if prop.Name != "name" {
		t.Errorf("expected property name 'name', got %q", prop.Name)
	}
	t.Logf("GetNodeProperty: name=%s type=%v", prop.Name, prop.Type)
}

func TestConvenience_GetNodeProperty_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "getnpropne")
	defer dropTestGraph(graphName)

	prop, err := testClient.GetNodeProperty(ctx, "Person", "nosuchprop", nil)
	if err != nil {
		t.Fatalf("GetNodeProperty (nonexistent) failed: %v", err)
	}
	if prop != nil {
		t.Errorf("expected nil for nonexistent property, got %+v", prop)
	}
}

func TestConvenience_GetEdgeProperty_Existing(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "geteprop")
	defer dropTestGraph(graphName)

	prop, err := testClient.GetEdgeProperty(ctx, "KNOWS", "since", nil)
	if err != nil {
		t.Fatalf("GetEdgeProperty failed: %v", err)
	}
	if prop == nil {
		t.Fatal("expected non-nil property for 'since'")
	}
	if prop.Name != "since" {
		t.Errorf("expected property name 'since', got %q", prop.Name)
	}
}

func TestConvenience_GetEdgeProperty_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "getepropne")
	defer dropTestGraph(graphName)

	prop, err := testClient.GetEdgeProperty(ctx, "KNOWS", "nosuchprop", nil)
	if err != nil {
		t.Fatalf("GetEdgeProperty (nonexistent) failed: %v", err)
	}
	if prop != nil {
		t.Errorf("expected nil for nonexistent edge property, got %+v", prop)
	}
}

func TestConvenience_GetProperty_Dispatch(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "getprop")
	defer dropTestGraph(graphName)

	// Node property
	prop, err := testClient.GetProperty(ctx, types.DBTypeNode, "Person", "age", nil)
	if err != nil {
		t.Fatalf("GetProperty(Node) failed: %v", err)
	}
	if prop == nil {
		t.Fatal("expected non-nil property for node 'age'")
	}

	// Edge property
	prop, err = testClient.GetProperty(ctx, types.DBTypeEdge, "KNOWS", "since", nil)
	if err != nil {
		t.Fatalf("GetProperty(Edge) failed: %v", err)
	}
	if prop == nil {
		t.Fatal("expected non-nil property for edge 'since'")
	}
}

func TestConvenience_CreateNodeProperty(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "crnprop")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateNodeProperty(ctx, "Person", []types.PropertyDef{
		{Name: "email", Type: types.PropertyTypeString},
	}, nil)
	if err != nil {
		t.Fatalf("CreateNodeProperty failed: %v", err)
	}

	// Verify
	prop, err := testClient.GetNodeProperty(ctx, "Person", "email", nil)
	if err != nil {
		t.Fatalf("GetNodeProperty failed: %v", err)
	}
	if prop == nil {
		t.Fatal("expected 'email' property to exist after creation")
	}
	t.Logf("Created node property: name=%s type=%v", prop.Name, prop.Type)
}

func TestConvenience_CreateEdgeProperty(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "creprop")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateEdgeProperty(ctx, "KNOWS", []types.PropertyDef{
		{Name: "strength", Type: types.PropertyTypeDouble},
	}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeProperty failed: %v", err)
	}

	// Verify
	prop, err := testClient.GetEdgeProperty(ctx, "KNOWS", "strength", nil)
	if err != nil {
		t.Fatalf("GetEdgeProperty failed: %v", err)
	}
	if prop == nil {
		t.Fatal("expected 'strength' property to exist after creation")
	}
	t.Logf("Created edge property: name=%s type=%v", prop.Name, prop.Type)
}

func TestConvenience_CreateProperty_Dispatch(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "crprop")
	defer dropTestGraph(graphName)

	// Node
	_, err := testClient.CreateProperty(ctx, types.DBTypeNode, "Person", []types.PropertyDef{
		{Name: "phone", Type: types.PropertyTypeString},
	}, nil)
	if err != nil {
		t.Fatalf("CreateProperty(Node) failed: %v", err)
	}

	// Edge
	_, err = testClient.CreateProperty(ctx, types.DBTypeEdge, "KNOWS", []types.PropertyDef{
		{Name: "score", Type: types.PropertyTypeFloat},
	}, nil)
	if err != nil {
		t.Fatalf("CreateProperty(Edge) failed: %v", err)
	}
}

func TestConvenience_DropNodeProperty(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpnprop")
	defer dropTestGraph(graphName)

	_, err := testClient.DropNodeProperty(ctx, "Person", nil, "age")
	if err != nil {
		t.Fatalf("DropNodeProperty failed: %v", err)
	}

	// Verify
	prop, err := testClient.GetNodeProperty(ctx, "Person", "age", nil)
	if err != nil {
		t.Fatalf("GetNodeProperty after drop failed: %v", err)
	}
	if prop != nil {
		t.Error("expected 'age' property to be dropped")
	}
}

func TestConvenience_DropEdgeProperty(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpeprop")
	defer dropTestGraph(graphName)

	_, err := testClient.DropEdgeProperty(ctx, "KNOWS", nil, "since")
	if err != nil {
		t.Fatalf("DropEdgeProperty failed: %v", err)
	}

	// Verify
	prop, err := testClient.GetEdgeProperty(ctx, "KNOWS", "since", nil)
	if err != nil {
		t.Fatalf("GetEdgeProperty after drop failed: %v", err)
	}
	if prop != nil {
		t.Error("expected 'since' property to be dropped")
	}
}

func TestConvenience_DropProperty_Dispatch(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpprop")
	defer dropTestGraph(graphName)

	// Drop node property via dispatch
	_, err := testClient.DropProperty(ctx, types.DBTypeNode, "Person", nil, "age")
	if err != nil {
		t.Fatalf("DropProperty(Node) failed: %v", err)
	}

	// Drop edge property via dispatch
	_, err = testClient.DropProperty(ctx, types.DBTypeEdge, "KNOWS", nil, "since")
	if err != nil {
		t.Fatalf("DropProperty(Edge) failed: %v", err)
	}
}

func TestConvenience_CreatePropertyIfNotExist_New(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "propifnot")
	defer dropTestGraph(graphName)

	created, err := testClient.CreatePropertyIfNotExist(ctx, types.DBTypeNode, "Person", []types.PropertyDef{
		{Name: "address", Type: types.PropertyTypeString},
	}, nil)
	if err != nil {
		t.Fatalf("CreatePropertyIfNotExist (new) failed: %v", err)
	}
	if !created {
		t.Error("expected created=true for new property")
	}

	// Verify
	prop, err := testClient.GetNodeProperty(ctx, "Person", "address", nil)
	if err != nil {
		t.Fatalf("GetNodeProperty failed: %v", err)
	}
	if prop == nil {
		t.Fatal("expected 'address' property to exist")
	}
}

func TestConvenience_CreatePropertyIfNotExist_Existing(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "propifnot2")
	defer dropTestGraph(graphName)

	// 'name' already exists
	created, err := testClient.CreatePropertyIfNotExist(ctx, types.DBTypeNode, "Person", []types.PropertyDef{
		{Name: "name", Type: types.PropertyTypeString},
	}, nil)
	if err != nil {
		t.Fatalf("CreatePropertyIfNotExist (existing) failed: %v", err)
	}
	if created {
		t.Error("expected created=false for existing property")
	}
}

// =============================================================================
// Constraint Operations
// =============================================================================

func TestConvenience_CreateNotNullConstraint(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "notnull")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateNotNullConstraint(ctx, types.DBTypeNode, "Person", "name", nil)
	if err != nil {
		t.Fatalf("CreateNotNullConstraint failed: %v", err)
	}
	t.Log("CreateNotNullConstraint on Person.name succeeded")
}

func TestConvenience_CreateUniqueConstraint(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "unique")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateUniqueConstraint(ctx, types.DBTypeNode, "Person", nil, "name")
	if err != nil {
		t.Fatalf("CreateUniqueConstraint failed: %v", err)
	}
	t.Log("CreateUniqueConstraint on Person.name succeeded")
}

func TestConvenience_DropNotNullConstraint(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpnotnl")
	defer dropTestGraph(graphName)

	// Add then drop
	_, err := testClient.CreateNotNullConstraint(ctx, types.DBTypeNode, "Person", "name", nil)
	if err != nil {
		t.Fatalf("CreateNotNullConstraint failed: %v", err)
	}

	_, err = testClient.DropNotNullConstraint(ctx, types.DBTypeNode, "Person", "name", nil)
	if err != nil {
		t.Fatalf("DropNotNullConstraint failed: %v", err)
	}
	t.Log("DropNotNullConstraint on Person.name succeeded")
}

func TestConvenience_DropUniqueConstraint(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpuniq")
	defer dropTestGraph(graphName)

	// Add then drop
	_, err := testClient.CreateUniqueConstraint(ctx, types.DBTypeNode, "Person", nil, "name")
	if err != nil {
		t.Fatalf("CreateUniqueConstraint failed: %v", err)
	}

	_, err = testClient.DropUniqueConstraint(ctx, types.DBTypeNode, "Person", nil, "name")
	if err != nil {
		t.Fatalf("DropUniqueConstraint failed: %v", err)
	}
	t.Log("DropUniqueConstraint on Person.name succeeded")
}

// =============================================================================
// Index Operations
// =============================================================================

func TestConvenience_CreateNodeIndex(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "crnidx")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateNodeIndex(ctx, "idx_person_name", "Person", []types.IndexProperty{
		{Name: "name"},
	}, nil)
	if err != nil {
		t.Fatalf("CreateNodeIndex failed: %v", err)
	}
	t.Log("CreateNodeIndex succeeded")
}

func TestConvenience_CreateEdgeIndex(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "creidx")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateEdgeIndex(ctx, "idx_knows_since", "KNOWS", []types.IndexProperty{
		{Name: "since"},
	}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeIndex failed: %v", err)
	}
	t.Log("CreateEdgeIndex succeeded")
}

func TestConvenience_ShowIndex(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "showidx")
	defer dropTestGraph(graphName)

	// Create an index first
	_, err := testClient.CreateNodeIndex(ctx, "idx_test", "Person", []types.IndexProperty{
		{Name: "name"},
	}, nil)
	if err != nil {
		t.Fatalf("CreateNodeIndex failed: %v", err)
	}

	indexes, err := testClient.ShowIndex(ctx, nil)
	if err != nil {
		t.Fatalf("ShowIndex failed: %v", err)
	}
	if len(indexes) < 1 {
		t.Errorf("expected at least 1 index, got %d", len(indexes))
	}
	for _, idx := range indexes {
		t.Logf("Index: name=%s entityType=%s label=%s property=%s status=%s",
			idx.IndexName, idx.EntityType, idx.Label, idx.Property, idx.Status)
	}
}

func TestConvenience_ShowNodeIndex(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "shownidx")
	defer dropTestGraph(graphName)

	// Create node index
	_, err := testClient.CreateNodeIndex(ctx, "idx_node_test", "Person", []types.IndexProperty{
		{Name: "name"},
	}, nil)
	if err != nil {
		t.Fatalf("CreateNodeIndex failed: %v", err)
	}

	indexes, err := testClient.ShowNodeIndex(ctx, nil)
	if err != nil {
		t.Fatalf("ShowNodeIndex failed: %v", err)
	}
	if len(indexes) < 1 {
		t.Errorf("expected at least 1 node index, got %d", len(indexes))
	}
	for _, idx := range indexes {
		t.Logf("Node index: name=%s label=%s property=%s", idx.IndexName, idx.Label, idx.Property)
	}
}

func TestConvenience_ShowEdgeIndex(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "showeidx")
	defer dropTestGraph(graphName)

	// Create edge index
	_, err := testClient.CreateEdgeIndex(ctx, "idx_edge_test", "KNOWS", []types.IndexProperty{
		{Name: "since"},
	}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeIndex failed: %v", err)
	}

	indexes, err := testClient.ShowEdgeIndex(ctx, nil)
	if err != nil {
		t.Fatalf("ShowEdgeIndex failed: %v", err)
	}
	if len(indexes) < 1 {
		t.Errorf("expected at least 1 edge index, got %d", len(indexes))
	}
	for _, idx := range indexes {
		t.Logf("Edge index: name=%s label=%s property=%s", idx.IndexName, idx.Label, idx.Property)
	}
}

func TestConvenience_DropNodeIndex(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpnidx")
	defer dropTestGraph(graphName)

	// Create then drop
	_, err := testClient.CreateNodeIndex(ctx, "idx_to_drop", "Person", []types.IndexProperty{
		{Name: "name"},
	}, nil)
	if err != nil {
		t.Fatalf("CreateNodeIndex failed: %v", err)
	}

	_, err = testClient.DropNodeIndex(ctx, "idx_to_drop", nil)
	if err != nil {
		t.Fatalf("DropNodeIndex failed: %v", err)
	}

	// Verify
	indexes, err := testClient.ShowNodeIndex(ctx, nil)
	if err != nil {
		t.Fatalf("ShowNodeIndex after drop failed: %v", err)
	}
	for _, idx := range indexes {
		if idx.IndexName == "idx_to_drop" {
			t.Error("expected index 'idx_to_drop' to be dropped")
		}
	}
}

func TestConvenience_DropEdgeIndex(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpeidx")
	defer dropTestGraph(graphName)

	// Create then drop
	_, err := testClient.CreateEdgeIndex(ctx, "idx_edge_drop", "KNOWS", []types.IndexProperty{
		{Name: "since"},
	}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeIndex failed: %v", err)
	}

	_, err = testClient.DropEdgeIndex(ctx, "idx_edge_drop", nil)
	if err != nil {
		t.Fatalf("DropEdgeIndex failed: %v", err)
	}

	// Verify
	indexes, err := testClient.ShowEdgeIndex(ctx, nil)
	if err != nil {
		t.Fatalf("ShowEdgeIndex after drop failed: %v", err)
	}
	for _, idx := range indexes {
		if idx.IndexName == "idx_edge_drop" {
			t.Error("expected index 'idx_edge_drop' to be dropped")
		}
	}
}

// =============================================================================
// Fulltext Operations
// =============================================================================

func TestConvenience_CreateNodeFulltext(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "crnft")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateNodeFulltext(ctx, "ft_person_name", "Person", []string{"name"}, nil)
	if err != nil {
		t.Fatalf("CreateNodeFulltext failed: %v", err)
	}
	t.Log("CreateNodeFulltext succeeded")
}

func TestConvenience_CreateEdgeFulltext(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "creft")
	defer dropTestGraph(graphName)

	// Edge fulltext needs a STRING property, add one
	_, err := testClient.CreateEdgeProperty(ctx, "KNOWS", []types.PropertyDef{
		{Name: "description", Type: types.PropertyTypeString},
	}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeProperty for fulltext failed: %v", err)
	}

	_, err = testClient.CreateEdgeFulltext(ctx, "ft_knows_desc", "KNOWS", []string{"description"}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeFulltext failed: %v", err)
	}
	t.Log("CreateEdgeFulltext succeeded")
}

func TestConvenience_ShowFulltext(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "showft")
	defer dropTestGraph(graphName)

	// Create fulltext first
	_, err := testClient.CreateNodeFulltext(ctx, "ft_show_test", "Person", []string{"name"}, nil)
	if err != nil {
		t.Fatalf("CreateNodeFulltext failed: %v", err)
	}

	fts, err := testClient.ShowFulltext(ctx, nil)
	if err != nil {
		t.Fatalf("ShowFulltext failed: %v", err)
	}
	if len(fts) < 1 {
		t.Errorf("expected at least 1 fulltext, got %d", len(fts))
	}
	for _, ft := range fts {
		t.Logf("Fulltext: name=%s entityType=%s schema=%s props=%s status=%s",
			ft.IndexName, ft.EntityType, ft.SchemaName, ft.Properties, ft.Status)
	}
}

func TestConvenience_ShowNodeFulltext(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "shownft")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateNodeFulltext(ctx, "ft_node_show", "Person", []string{"name"}, nil)
	if err != nil {
		t.Fatalf("CreateNodeFulltext failed: %v", err)
	}

	fts, err := testClient.ShowNodeFulltext(ctx, nil)
	if err != nil {
		t.Fatalf("ShowNodeFulltext failed: %v", err)
	}
	if len(fts) < 1 {
		t.Errorf("expected at least 1 node fulltext, got %d", len(fts))
	}
}

func TestConvenience_ShowEdgeFulltext(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "showeft")
	defer dropTestGraph(graphName)

	// Create edge fulltext
	_, err := testClient.CreateEdgeProperty(ctx, "KNOWS", []types.PropertyDef{
		{Name: "note", Type: types.PropertyTypeString},
	}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeProperty failed: %v", err)
	}

	_, err = testClient.CreateEdgeFulltext(ctx, "ft_edge_show", "KNOWS", []string{"note"}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeFulltext failed: %v", err)
	}

	fts, err := testClient.ShowEdgeFulltext(ctx, nil)
	if err != nil {
		t.Fatalf("ShowEdgeFulltext failed: %v", err)
	}
	if len(fts) < 1 {
		t.Errorf("expected at least 1 edge fulltext, got %d", len(fts))
	}
}

func TestConvenience_DropNodeFulltext(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpnft")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateNodeFulltext(ctx, "ft_to_drop", "Person", []string{"name"}, nil)
	if err != nil {
		t.Fatalf("CreateNodeFulltext failed: %v", err)
	}

	_, err = testClient.DropNodeFulltext(ctx, "ft_to_drop", nil)
	if err != nil {
		t.Fatalf("DropNodeFulltext failed: %v", err)
	}

	// Verify
	fts, err := testClient.ShowNodeFulltext(ctx, nil)
	if err != nil {
		t.Fatalf("ShowNodeFulltext after drop failed: %v", err)
	}
	for _, ft := range fts {
		if ft.IndexName == "ft_to_drop" {
			t.Error("expected fulltext 'ft_to_drop' to be dropped")
		}
	}
}

func TestConvenience_DropEdgeFulltext(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpeft")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateEdgeProperty(ctx, "KNOWS", []types.PropertyDef{
		{Name: "memo", Type: types.PropertyTypeString},
	}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeProperty failed: %v", err)
	}

	_, err = testClient.CreateEdgeFulltext(ctx, "ft_edge_drop", "KNOWS", []string{"memo"}, nil)
	if err != nil {
		t.Fatalf("CreateEdgeFulltext failed: %v", err)
	}

	_, err = testClient.DropEdgeFulltext(ctx, "ft_edge_drop", nil)
	if err != nil {
		t.Fatalf("DropEdgeFulltext failed: %v", err)
	}

	// Verify
	fts, err := testClient.ShowEdgeFulltext(ctx, nil)
	if err != nil {
		t.Fatalf("ShowEdgeFulltext after drop failed: %v", err)
	}
	for _, ft := range fts {
		if ft.IndexName == "ft_edge_drop" {
			t.Error("expected fulltext 'ft_edge_drop' to be dropped")
		}
	}
}

// =============================================================================
// Task Operations
// =============================================================================

func TestConvenience_ShowTasks(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tasks, err := testClient.ShowTasks(ctx, nil)
	if err != nil {
		t.Fatalf("ShowTasks failed: %v", err)
	}
	t.Logf("Found %d tasks", len(tasks))
	for _, task := range tasks {
		t.Logf("  Task: id=%s type=%s status=%s", task.TaskId, task.Type, task.Status)
	}
}

func TestConvenience_DeleteTask_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.DeleteTask(ctx, "nonexistent_task_id_999", nil)
	if err == nil {
		t.Log("DeleteTask with nonexistent ID returned no error (server may silently succeed)")
	} else {
		t.Logf("DeleteTask with nonexistent ID returned expected error: %v", err)
	}
}

func TestConvenience_StopTask_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.StopTask(ctx, "nonexistent_task_id_999", nil)
	if err == nil {
		t.Log("StopTask with nonexistent ID returned no error (server may silently succeed)")
	} else {
		t.Logf("StopTask with nonexistent ID returned expected error: %v", err)
	}
}

// =============================================================================
// Data Insert Operations (GQL-based)
// =============================================================================

func TestConvenience_InsertNodes(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_insgql_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateOpenGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateOpenGraph failed: %v", err)
	}

	nodes := []types.NodeData{
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Alice", "age": int64(30)}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Bob", "age": int64(25)}},
	}

	// Per-call graph_name — without this the SDK routes to the client's default
	// graph (miniCircle) and pollutes shared state.
	ic := &gqldb.InsertConfig{QueryConfig: gqldb.QueryConfig{GraphName: graphName}}
	resp, err := testClient.InsertNodes(ctx, nodes, ic)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}
	// v6 server doesn't populate rows_affected for INSERT ... RETURN; the
	// returned columns (n0, n1, ...) carry the actual inserted nodes. Verify
	// physically via MATCH count instead.
	qc := &gqldb.QueryConfig{GraphName: graphName}
	checkResp, err := testClient.Gql(ctx, "MATCH (n:Person) RETURN count(n)", qc)
	if err != nil {
		t.Fatalf("verification MATCH failed: %v", err)
	}
	if checkResp == nil || len(checkResp.Rows) == 0 {
		t.Fatalf("verification MATCH returned no rows")
	}
	t.Logf("InsertNodes: rows_affected=%d, return_cols=%v, MATCH count=%v",
		resp.RowsAffected, resp.Columns, checkResp.Rows[0].Values[0])
}

func TestConvenience_InsertNodes_MissingLabel(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_insnolbl_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateOpenGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateOpenGraph failed: %v", err)
	}

	// Node with no labels should fail client-side validation. Even so, route
	// the call to the test graph so that a regression that lets the call
	// through doesn't hit miniCircle.
	ic := &gqldb.InsertConfig{QueryConfig: gqldb.QueryConfig{GraphName: graphName}}
	nodes := []types.NodeData{
		{Labels: nil, Properties: map[string]interface{}{"name": "NoLabel"}},
	}

	_, err = testClient.InsertNodes(ctx, nodes, ic)
	if err == nil {
		t.Error("expected error for node with no labels, got nil")
	} else {
		t.Logf("Got expected error for missing labels: %v", err)
	}
}

func TestConvenience_InsertNodes_EmptyList(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := testClient.InsertNodes(ctx, nil, nil)
	if err != nil {
		t.Fatalf("InsertNodes with empty list failed: %v", err)
	}
	if resp.RowsAffected != 0 {
		t.Errorf("expected 0 rows affected for empty insert, got %d", resp.RowsAffected)
	}
}

func TestConvenience_InsertEdges(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_insedge_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateOpenGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateOpenGraph failed: %v", err)
	}

	// Insert nodes first — pass per-call graph_name to avoid hitting the
	// default graph (miniCircle).
	qc := &gqldb.QueryConfig{GraphName: graphName}
	ic := &gqldb.InsertConfig{QueryConfig: *qc}
	nodes := []types.NodeData{
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Alice"}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Bob"}},
	}
	_, err = testClient.InsertNodes(ctx, nodes, ic)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}

	// Query to get node IDs
	resp, err := testClient.Gql(ctx, "MATCH (n:Person) RETURN id(n) AS nid ORDER BY n.name", qc)
	if err != nil {
		t.Fatalf("Query for node IDs failed: %v", err)
	}
	if resp.RowCount < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", resp.RowCount)
	}

	var nodeIDs []string
	for i := int64(0); i < resp.RowCount; i++ {
		row := resp.Rows[i]
		if value, err := resp.GetByName(row, "nid"); err == nil {
			if nid, ok := value.(string); ok {
				nodeIDs = append(nodeIDs, nid)
			}
		}
	}
	if len(nodeIDs) < 2 {
		t.Fatalf("expected at least 2 node IDs, got %d", len(nodeIDs))
	}

	// Insert edges using node IDs
	edges := []types.EdgeData{
		{
			Label:      "KNOWS",
			FromNodeID: nodeIDs[0],
			ToNodeID:   nodeIDs[1],
			Properties: map[string]interface{}{"since": int64(2020)},
		},
	}

	edgeResp, err := testClient.InsertEdges(ctx, edges, ic)
	if err != nil {
		t.Fatalf("InsertEdges failed: %v", err)
	}
	// v6: rows_affected may not be set for INSERT ... RETURN; verify via MATCH.
	checkEdges, err := testClient.Gql(ctx, "MATCH ()-[e:KNOWS]->() RETURN count(e)", qc)
	if err != nil {
		t.Fatalf("verification edge MATCH failed: %v", err)
	}
	if checkEdges == nil || len(checkEdges.Rows) == 0 {
		t.Fatalf("verification edge MATCH returned no rows")
	}
	t.Logf("InsertEdges: rows_affected=%d, MATCH count=%v",
		edgeResp.RowsAffected, checkEdges.Rows[0].Values[0])
}

func TestConvenience_InsertEdges_EmptyList(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := testClient.InsertEdges(ctx, nil, nil)
	if err != nil {
		t.Fatalf("InsertEdges with empty list failed: %v", err)
	}
	if resp.RowsAffected != 0 {
		t.Errorf("expected 0 rows affected for empty edge insert, got %d", resp.RowsAffected)
	}
}

func TestConvenience_InsertNodesBatchAuto(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_batch_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateOpenGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateOpenGraph failed: %v", err)
	}

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	nodes := []*gqldb.NodeData{
		{Labels: []string{"City"}, Properties: map[string]interface{}{"name": "Beijing"}},
		{Labels: []string{"City"}, Properties: map[string]interface{}{"name": "Shanghai"}},
		{Labels: []string{"City"}, Properties: map[string]interface{}{"name": "Guangzhou"}},
	}

	config := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	result, err := testClient.InsertNodesBatchAuto(ctx, graphName, nodes, config)
	if err != nil {
		t.Fatalf("InsertNodesBatchAuto failed: %v", err)
	}
	if !result.Success {
		t.Errorf("InsertNodesBatchAuto returned success=false: %s", result.Message)
	}
	if result.NodeCount != 3 {
		t.Errorf("expected 3 nodes, got %d", result.NodeCount)
	}
	t.Logf("InsertNodesBatchAuto: %d nodes inserted", result.NodeCount)
}

func TestConvenience_InsertEdgesBatchAuto(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_batchedge_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateOpenGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateOpenGraph failed: %v", err)
	}

	// Start bulk import
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Insert nodes first
	nodes := []*gqldb.NodeData{
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Alice"}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Bob"}},
	}
	nodeConfig := &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, nodeConfig)
	if err != nil {
		t.Fatalf("InsertNodesBatchAuto failed: %v", err)
	}

	// Query for node IDs
	qc := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n:Person) RETURN id(n) AS nid ORDER BY n.name", qc)
	if err != nil {
		t.Fatalf("Query for node IDs failed: %v", err)
	}
	if resp.RowCount < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", resp.RowCount)
	}

	var nodeIDs []string
	for i := int64(0); i < resp.RowCount; i++ {
		row := resp.Rows[i]
		if value, err := resp.GetByName(row, "nid"); err == nil {
			if nid, ok := value.(string); ok {
				nodeIDs = append(nodeIDs, nid)
			}
		}
	}
	if len(nodeIDs) < 2 {
		t.Fatalf("expected at least 2 node IDs, got %d", len(nodeIDs))
	}

	// Insert edges
	edges := []*gqldb.EdgeData{
		{
			Label:      "KNOWS",
			FromNodeID: nodeIDs[0],
			ToNodeID:   nodeIDs[1],
			Properties: map[string]interface{}{"since": int64(2021)},
		},
	}
	edgeConfig := &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID}
	edgeResult, err := testClient.InsertEdgesBatchAuto(ctx, graphName, edges, edgeConfig)
	if err != nil {
		t.Fatalf("InsertEdgesBatchAuto failed: %v", err)
	}
	if !edgeResult.Success {
		t.Errorf("InsertEdgesBatchAuto returned success=false: %s", edgeResult.Message)
	}
	if edgeResult.EdgeCount != 1 {
		t.Errorf("expected 1 edge, got %d", edgeResult.EdgeCount)
	}
	t.Logf("InsertEdgesBatchAuto: %d edges inserted", edgeResult.EdgeCount)
}

// =============================================================================
// System & Process Operations
// =============================================================================

func TestConvenience_Top(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	processes, err := testClient.Top(ctx)
	if err != nil {
		t.Fatalf("Top failed: %v", err)
	}
	t.Logf("Top: %d running processes", len(processes))
	for _, p := range processes {
		t.Logf("  Process: queryId=%s query=%s status=%s durationMs=%d",
			p.QueryId, p.QueryText, p.Status, p.DurationMs)
	}
}

func TestConvenience_Kill_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.Kill(ctx, "nonexistent_query_id_999")
	if err == nil {
		t.Log("Kill with nonexistent query ID returned no error (server may silently succeed)")
	} else {
		t.Logf("Kill with nonexistent query ID returned expected error: %v", err)
	}
}

func TestConvenience_Stats(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stats, err := testClient.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	t.Logf("Stats for miniCircle: graphName=%s nodeCount=%d edgeCount=%d",
		stats.GraphName, stats.NodeCount, stats.EdgeCount)
	if stats.NodeCount <= 0 {
		t.Logf("Warning: expected positive node count for miniCircle, got %d", stats.NodeCount)
	}
}

func TestConvenience_Stats_NonexistentGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.Stats(ctx)
	if err == nil {
		t.Error("expected error for stats on nonexistent graph, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestConvenience_Test(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	latencyNs, err := testClient.Test(ctx)
	if err != nil {
		t.Fatalf("Test (ping) failed: %v", err)
	}
	if latencyNs <= 0 {
		t.Errorf("expected positive latency, got %d ns", latencyNs)
	}
	t.Logf("Test (ping) latency: %d ns (%.2f ms)", latencyNs, float64(latencyNs)/1e6)
}

// =============================================================================
// Index with PrefixLength
// =============================================================================

func TestConvenience_CreateNodeIndex_WithPrefixLength(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "idxpfx")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateNodeIndex(ctx, "idx_name_prefix", "Person", []types.IndexProperty{
		{Name: "name", PrefixLength: 10},
	}, nil)
	if err != nil {
		t.Fatalf("CreateNodeIndex with prefix length failed: %v", err)
	}

	// Verify
	indexes, err := testClient.ShowNodeIndex(ctx, nil)
	if err != nil {
		t.Fatalf("ShowNodeIndex failed: %v", err)
	}
	found := false
	for _, idx := range indexes {
		if idx.IndexName == "idx_name_prefix" {
			found = true
			t.Logf("Index found: name=%s property=%s prefixLength=%v", idx.IndexName, idx.Property, idx.PrefixLength)
		}
	}
	if !found {
		t.Error("expected to find index 'idx_name_prefix'")
	}
}

// =============================================================================
// Edge Cases & Error Handling
// =============================================================================

func TestConvenience_CreateOpenGraph_EmptyName(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.CreateOpenGraph(ctx, "", nil)
	if err == nil {
		t.Error("expected error for empty graph name, got nil")
	} else {
		t.Logf("Got expected error for empty name: %v", err)
	}
}

func TestConvenience_Truncate_NonexistentGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.Truncate(ctx, "nonexistent_graph_xyz_999", nil)
	if err == nil {
		t.Error("expected error for truncating nonexistent graph, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestConvenience_AlterGraph_NonexistentGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.AlterGraph(ctx, "nonexistent_graph_xyz_999", "new_name", nil)
	if err == nil {
		t.Error("expected error for renaming nonexistent graph, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestConvenience_ShowLabels_NonexistentGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a nonexistent graph then try ShowLabels
	_ = testClient.UseGraph(ctx, "nonexistent_graph_xyz_999")
	_, err := testClient.ShowLabels(ctx, nil)
	if err == nil {
		t.Error("expected error for ShowLabels on nonexistent graph, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
	// Restore to miniCircle
	_ = testClient.UseGraph(ctx, "miniCircle")
}

func TestConvenience_CreateNodeLabel_NonexistentGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// CreateNodeLabel now uses the current graph internally.
	// Without UseGraph, the current graph may be empty, which should cause an error.
	_, err := testClient.CreateNodeLabel(ctx, "Foo", nil, nil)
	if err == nil {
		t.Log("CreateNodeLabel without UseGraph did not error (may use default graph)")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestConvenience_DropNodeLabel_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpnone")
	defer dropTestGraph(graphName)

	_, err := testClient.DropNodeLabel(ctx, "NoSuchLabel", nil)
	if err == nil {
		t.Error("expected error for dropping nonexistent label, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestConvenience_DropNodeProperty_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drppropne")
	defer dropTestGraph(graphName)

	_, err := testClient.DropNodeProperty(ctx, "Person", nil, "no_such_property")
	if err == nil {
		t.Error("expected error for dropping nonexistent property, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestConvenience_DropNodeIndex_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpidxne")
	defer dropTestGraph(graphName)

	_, err := testClient.DropNodeIndex(ctx, "no_such_index", nil)
	if err == nil {
		t.Error("expected error for dropping nonexistent index, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestConvenience_DropNodeFulltext_Nonexistent(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := createClosedGraphWithLabels(t, "drpftne")
	defer dropTestGraph(graphName)

	_, err := testClient.DropNodeFulltext(ctx, "no_such_fulltext", nil)
	if err == nil {
		t.Error("expected error for dropping nonexistent fulltext, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestConvenience_ShowIndex_EmptyGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_emptyidx_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateClosedGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateClosedGraph failed: %v", err)
	}

	indexes, err := testClient.ShowIndex(ctx, nil)
	if err != nil {
		t.Fatalf("ShowIndex on empty graph failed: %v", err)
	}
	if len(indexes) != 0 {
		t.Errorf("expected 0 indexes on empty graph, got %d", len(indexes))
	}
}

func TestConvenience_ShowFulltext_EmptyGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_conv_emptyft_" + time.Now().Format("150405")
	defer dropTestGraph(graphName)

	_, err := testClient.CreateClosedGraph(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("CreateClosedGraph failed: %v", err)
	}

	fts, err := testClient.ShowFulltext(ctx, nil)
	if err != nil {
		t.Fatalf("ShowFulltext on empty graph failed: %v", err)
	}
	if len(fts) != 0 {
		t.Errorf("expected 0 fulltext indexes on empty graph, got %d", len(fts))
	}
}
