//go:build integration

// Pin the bug-fix where the GQL-emitter convenience path
// `Client.InsertEdgesGql(...)` silently dropped `EdgeData.ID`.
//
// Companion to Java EdgeIdEmitterIT — exercises the same 5-point
// matrix on the Go SDK:
//  1. Graph created with EDGE_ID enabled
//  2. GQL-emitter path with custom edge `_id` (previously broken)
//  3. gRPC bulk-import path with custom edge `_id` (regression guard)
//  4. Node path with custom `_id` (control)
//  5. UPSERT on duplicate edge `_id` updates rather than appending
package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
	"github.com/ultipa/ultipa-go-driver/v6/types"
)

func setupEdgeIdGraph(t *testing.T, ctx context.Context) string {
	t.Helper()
	graph := fmt.Sprintf("test_edge_id_emitter_%d", time.Now().UnixNano())
	if err := testClient.CreateGraph(ctx, graph, gqldb.GraphTypeOpen, "edge _id emitter regression test"); err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	if _, err := testClient.Gql(ctx, fmt.Sprintf("ALTER GRAPH %s SET EDGE_ID ENABLED", graph), nil); err != nil {
		t.Fatalf("SET EDGE_ID failed: %v", err)
	}
	// Seed nodes: alice, bob
	_, err := testClient.InsertNodesGql(ctx, []gqldb.NodeData{
		{ID: "alice", Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Alice"}},
		{ID: "bob", Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Bob"}},
	}, &gqldb.InsertConfig{QueryConfig: types.QueryConfig{GraphName: graph}})
	if err != nil {
		t.Fatalf("seed insertNodes failed: %v", err)
	}
	return graph
}

func dropEdgeIdGraph(t *testing.T, ctx context.Context, graph string) {
	t.Helper()
	_ = testClient.DropGraph(ctx, graph, true)
}

// (4) Control — node path with custom _id round-trips.
func TestEdgeIdEmitter_NodeIdRoundTripControl(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	graph := setupEdgeIdGraph(t, ctx)
	defer dropEdgeIdGraph(t, ctx, graph)

	_, err := testClient.InsertNodesGql(ctx, []gqldb.NodeData{
		{ID: "carol", Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Carol"}},
	}, &gqldb.InsertConfig{QueryConfig: types.QueryConfig{GraphName: graph}})
	if err != nil {
		t.Fatalf("InsertNodesGql failed: %v", err)
	}

	r, err := testClient.Gql(ctx, "MATCH (n:Person {_id:'carol'}) RETURN n._id",
		&types.QueryConfig{GraphName: graph})
	if err != nil {
		t.Fatalf("MATCH failed: %v", err)
	}
	if len(r.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(r.Rows))
	}
	got, _ := r.Rows[0].Values[0].ToGo()
	if got != "carol" {
		t.Errorf("expected 'carol', got %q", got)
	}
}

// (2) The fix — GQL-emitter path with custom edge _id.
func TestEdgeIdEmitter_EdgeIdViaGql(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	graph := setupEdgeIdGraph(t, ctx)
	defer dropEdgeIdGraph(t, ctx, graph)

	_, err := testClient.InsertEdgesGql(ctx, []types.EdgeData{
		{ID: "tx-GQL-1", Label: "Knows", FromNodeID: "alice", ToNodeID: "bob",
			Properties: map[string]interface{}{"since": 2024}},
	}, &gqldb.InsertConfig{QueryConfig: types.QueryConfig{GraphName: graph}})
	if err != nil {
		t.Fatalf("InsertEdgesGql failed: %v", err)
	}

	r, err := testClient.Gql(ctx,
		"MATCH ()-[e:Knows WHERE e._id = 'tx-GQL-1']->() RETURN e._id, e.since",
		&types.QueryConfig{GraphName: graph})
	if err != nil {
		t.Fatalf("MATCH failed: %v", err)
	}
	if len(r.Rows) != 1 {
		t.Fatalf("GQL emitter must propagate EdgeData.ID; got %d rows", len(r.Rows))
	}
	id, _ := r.Rows[0].Values[0].ToGo()
	since, _ := r.Rows[0].Values[1].ToGo()
	if id != "tx-GQL-1" {
		t.Errorf("expected _id='tx-GQL-1', got %q", id)
	}
	if since != 2024 {
		t.Errorf("expected since=2024, got %d", since)
	}
}

// (3) Regression guard — gRPC bulk-import path.
func TestEdgeIdEmitter_EdgeIdViaGrpc(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	graph := setupEdgeIdGraph(t, ctx)
	defer dropEdgeIdGraph(t, ctx, graph)

	session, err := testClient.StartBulkImport(ctx, graph, &gqldb.BulkImportOptions{})
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	_, err = testClient.InsertEdges(ctx, graph, []*gqldb.EdgeData{
		{ID: "tx-GRPC-1", Label: "Knows", FromNodeID: "alice", ToNodeID: "bob",
			Properties: map[string]interface{}{"since": 2025}},
	}, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("InsertEdges (gRPC) failed: %v", err)
	}

	r, err := testClient.Gql(ctx,
		"MATCH ()-[e:Knows WHERE e._id = 'tx-GRPC-1']->() RETURN e._id, e.since",
		&types.QueryConfig{GraphName: graph})
	if err != nil {
		t.Fatalf("MATCH failed: %v", err)
	}
	if len(r.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(r.Rows))
	}
	id, _ := r.Rows[0].Values[0].ToGo()
	since, _ := r.Rows[0].Values[1].ToGo()
	if id != "tx-GRPC-1" || since != 2025 {
		t.Errorf("expected ('tx-GRPC-1', 2025); got (%q, %d)", id, since)
	}
}

// (5) UPSERT on duplicate edge _id updates rather than appending.
func TestEdgeIdEmitter_UpsertOnDuplicateId(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	graph := setupEdgeIdGraph(t, ctx)
	defer dropEdgeIdGraph(t, ctx, graph)

	upsertCfg := &gqldb.InsertConfig{
		QueryConfig: types.QueryConfig{GraphName: graph},
		InsertType:  gqldb.InsertTypeUpsert,
	}

	// First UPSERT — creates.
	if _, err := testClient.InsertEdgesGql(ctx, []types.EdgeData{
		{ID: "tx-UPSERT-1", Label: "Knows", FromNodeID: "alice", ToNodeID: "bob",
			Properties: map[string]interface{}{"since": 2020, "weight": 1}},
	}, upsertCfg); err != nil {
		t.Fatalf("first UPSERT failed: %v", err)
	}

	// Second UPSERT — same _id, different values.
	if _, err := testClient.InsertEdgesGql(ctx, []types.EdgeData{
		{ID: "tx-UPSERT-1", Label: "Knows", FromNodeID: "alice", ToNodeID: "bob",
			Properties: map[string]interface{}{"since": 2030, "weight": 99}},
	}, upsertCfg); err != nil {
		t.Fatalf("second UPSERT failed: %v", err)
	}

	count, err := testClient.Gql(ctx,
		"MATCH ()-[e:Knows WHERE e._id = 'tx-UPSERT-1']->() RETURN count(e)",
		&types.QueryConfig{GraphName: graph})
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	c, _ := count.Rows[0].Values[0].ToGo()
	if c != 1 {
		t.Errorf("UPSERT on duplicate _id must update, not append; count=%d", c)
	}

	props, err := testClient.Gql(ctx,
		"MATCH ()-[e:Knows WHERE e._id = 'tx-UPSERT-1']->() RETURN e.since, e.weight",
		&types.QueryConfig{GraphName: graph})
	if err != nil {
		t.Fatalf("props failed: %v", err)
	}
	since, _ := props.Rows[0].Values[0].ToGo()
	weight, _ := props.Rows[0].Values[1].ToGo()
	if since != 2030 || weight != 99 {
		t.Errorf("expected (2030, 99); got (%d, %d)", since, weight)
	}
}

// Negative control — proves the test would catch a regression.
func TestEdgeIdEmitter_UnknownIdFindsNothing(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	graph := setupEdgeIdGraph(t, ctx)
	defer dropEdgeIdGraph(t, ctx, graph)

	r, err := testClient.Gql(ctx,
		"MATCH ()-[e:Knows WHERE e._id = 'tx-never-inserted']->() RETURN e._id",
		&types.QueryConfig{GraphName: graph})
	if err != nil {
		t.Fatalf("MATCH failed: %v", err)
	}
	if len(r.Rows) != 0 {
		t.Errorf("expected 0 rows for unknown _id; got %d", len(r.Rows))
	}
}
