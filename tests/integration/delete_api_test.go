//go:build integration

// Integration test for the new GQL-emitter delete API on Client:
//   DeleteNodesByIDs / DeleteNodesByCondition /
//   DeleteEdgesByIDs / DeleteEdgesByCondition.
//
// Mirrors java/src/test/java/com/gqldb/integration/DeleteIT.java's
// 13-point matrix.
package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
	"github.com/ultipa/ultipa-go-driver/v6/types"
)

func setupDeleteGraph(t *testing.T, ctx context.Context) string {
	t.Helper()
	g := fmt.Sprintf("test_delete_api_%d", time.Now().UnixNano())
	if err := testClient.CreateGraph(ctx, g, gqldb.GraphTypeOpen, "delete API test"); err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	if _, err := testClient.Gql(ctx, fmt.Sprintf("ALTER GRAPH %s SET EDGE_ID ENABLED", g), nil); err != nil {
		t.Fatalf("ALTER GRAPH SET EDGE_ID failed: %v", err)
	}
	return g
}

func cleanupDeleteGraph(t *testing.T, ctx context.Context, g string) {
	t.Helper()
	_ = testClient.DropGraph(ctx, g, true)
}

func dc(g string) *gqldb.DeleteConfig {
	c := gqldb.NewDeleteConfig()
	c.GraphName = g
	return c
}

func seedNodes(t *testing.T, ctx context.Context, g string, ids ...string) {
	t.Helper()
	nodes := make([]gqldb.NodeData, 0, len(ids))
	for _, id := range ids {
		nodes = append(nodes, gqldb.NodeData{
			ID: id, Labels: []string{"P"},
			Properties: map[string]interface{}{"age": int64(30)},
		})
	}
	if _, err := testClient.InsertNodesGql(ctx, nodes,
		&gqldb.InsertConfig{QueryConfig: types.QueryConfig{GraphName: g}}); err != nil {
		t.Fatalf("seed nodes failed: %v", err)
	}
}

func seedEdge(t *testing.T, ctx context.Context, g, eid, from, to, label string, w int64) {
	t.Helper()
	if _, err := testClient.InsertEdgesGql(ctx, []types.EdgeData{
		{ID: eid, Label: label, FromNodeID: from, ToNodeID: to,
			Properties: map[string]interface{}{"w": w}},
	}, &gqldb.InsertConfig{QueryConfig: types.QueryConfig{GraphName: g}}); err != nil {
		t.Fatalf("seed edge failed: %v", err)
	}
}

// (1) DeleteNodesByIDs control
func TestDeleteAPI_NodeByIDs(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	seedNodes(t, ctx, g, "n1", "n2", "n3")
	r, err := testClient.DeleteNodesByIDs(ctx, []string{"n1", "n2"}, dc(g))
	if err != nil {
		t.Fatalf("DeleteNodesByIDs failed: %v", err)
	}
	if r.RowsAffected != 2 {
		t.Errorf("expected 2 rowsAffected, got %d", r.RowsAffected)
	}
	ar, err := r.Alias("n")
	if err != nil {
		t.Fatalf("Alias(n) failed: %v", err)
	}
	nodes, _, err := ar.AsNodes()
	if err != nil {
		t.Fatalf("AsNodes failed: %v", err)
	}
	ids := map[string]bool{}
	for _, n := range nodes {
		ids[n.ID] = true
	}
	if !ids["n1"] || !ids["n2"] {
		t.Errorf("expected n1,n2; got %v", ids)
	}
}

// (2) DeleteNodesByCondition labels + where
func TestDeleteAPI_NodeByCondition(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	if _, err := testClient.InsertNodesGql(ctx, []gqldb.NodeData{
		{ID: "p1", Labels: []string{"Person"}, Properties: map[string]interface{}{"age": int64(70)}},
		{ID: "p2", Labels: []string{"Person"}, Properties: map[string]interface{}{"age": int64(30)}},
		{ID: "a1", Labels: []string{"Admin"}, Properties: map[string]interface{}{"age": int64(65)}},
		{ID: "a2", Labels: []string{"Admin"}, Properties: map[string]interface{}{"age": int64(25)}},
	}, &gqldb.InsertConfig{QueryConfig: types.QueryConfig{GraphName: g}}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	r, err := testClient.DeleteNodesByCondition(ctx,
		[]string{"Person", "Admin"}, "n.age > 60", 0, dc(g))
	if err != nil {
		t.Fatalf("DeleteNodesByCondition failed: %v", err)
	}
	if r.RowsAffected != 2 {
		t.Errorf("expected 2; got %d", r.RowsAffected)
	}
}

// (3) returnDeleted=false
func TestDeleteAPI_NodeReturnDeletedFalse(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	seedNodes(t, ctx, g, "q1", "q2", "q3")
	cfg := dc(g)
	cfg.ReturnDeleted = false
	r, err := testClient.DeleteNodesByIDs(ctx, []string{"q1", "q2", "q3"}, cfg)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if r.RowCount != 0 {
		t.Errorf("expected rowCount=0; got %d", r.RowCount)
	}
	if r.RowsAffected != 3 {
		t.Errorf("expected rowsAffected=3; got %d", r.RowsAffected)
	}
}

// (4) allowDeleteAll safety latch
func TestDeleteAPI_AllowDeleteAllSafetyLatch(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	_, err := testClient.DeleteNodesByCondition(ctx, nil, "", 0, dc(g))
	if err == nil {
		t.Fatalf("expected error from empty labels+where; got nil")
	}
	if !strings.Contains(err.Error(), "AllowDeleteAll") {
		t.Errorf("expected AllowDeleteAll in error message; got %q", err.Error())
	}
}

// (5) DeleteEdgesByIDs reshape
func TestDeleteAPI_EdgeByIDsReshape(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	seedNodes(t, ctx, g, "u", "v")
	seedEdge(t, ctx, g, "e1", "u", "v", "Knows", 1)
	seedEdge(t, ctx, g, "e2", "u", "v", "Knows", 2)

	r, err := testClient.DeleteEdgesByIDs(ctx, []string{"e1", "e2"}, dc(g))
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if r.RowsAffected != 2 {
		t.Errorf("expected 2 rowsAffected; got %d", r.RowsAffected)
	}
	if len(r.Columns) != 1 || r.Columns[0] != "e" {
		t.Errorf("expected columns=[e]; got %v", r.Columns)
	}
	ar, err := r.Alias("e")
	if err != nil {
		t.Fatalf("Alias(e) failed: %v", err)
	}
	edges, _, err := ar.AsEdges()
	if err != nil {
		t.Fatalf("AsEdges failed: %v", err)
	}
	if len(edges) != 2 {
		t.Errorf("expected 2 edges; got %d", len(edges))
	}
	for _, e := range edges {
		if e.FromNodeID != "u" || e.ToNodeID != "v" || e.Label != "Knows" {
			t.Errorf("bad edge: %+v", e)
		}
	}
}

// (6) DeleteEdgesByCondition reshape
func TestDeleteAPI_EdgeByConditionReshape(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	seedNodes(t, ctx, g, "x", "y")
	seedEdge(t, ctx, g, "k1", "x", "y", "Knows", 10)
	seedEdge(t, ctx, g, "k2", "x", "y", "Knows", 99)

	r, err := testClient.DeleteEdgesByCondition(ctx, "Knows", "e.w >= 50", 0, dc(g))
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if r.RowsAffected != 1 {
		t.Errorf("expected 1 rowsAffected; got %d", r.RowsAffected)
	}
	ar, _ := r.Alias("e")
	edges, _, _ := ar.AsEdges()
	if len(edges) != 1 || edges[0].ID != "k2" {
		t.Errorf("expected single edge k2; got %v", edges)
	}
}

// (7) edge returnDeleted=false
func TestDeleteAPI_EdgeReturnDeletedFalse(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	seedNodes(t, ctx, g, "s", "t")
	seedEdge(t, ctx, g, "ef1", "s", "t", "Knows", 1)
	seedEdge(t, ctx, g, "ef2", "s", "t", "Knows", 2)

	cfg := dc(g)
	cfg.ReturnDeleted = false
	r, err := testClient.DeleteEdgesByIDs(ctx, []string{"ef1", "ef2"}, cfg)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if r.RowCount != 0 {
		t.Errorf("expected rowCount=0; got %d", r.RowCount)
	}
	if r.RowsAffected != 2 {
		t.Errorf("expected rowsAffected=2; got %d", r.RowsAffected)
	}
}

// (8) empty short-circuit
func TestDeleteAPI_EmptyShortCircuit(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	a, _ := testClient.DeleteNodesByIDs(ctx, []string{}, dc(g))
	b, _ := testClient.DeleteEdgesByIDs(ctx, []string{}, dc(g))
	if a.RowCount != 0 || a.RowsAffected != 0 || b.RowCount != 0 || b.RowsAffected != 0 {
		t.Errorf("expected all zero; got node=%+v edge=%+v", a, b)
	}
}

// (9) unknown id — rowsAffected
func TestDeleteAPI_UnknownID(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	r, err := testClient.DeleteNodesByIDs(ctx, []string{"never-inserted-xyz"}, dc(g))
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	// Server quirk: rowCount=1 phantom; rowsAffected=0 is truth.
	if r.RowsAffected != 0 {
		t.Errorf("expected rowsAffected=0; got %d", r.RowsAffected)
	}
}

// (10) EDGE_ID disabled graph — deletion by id must error and guide the
// user to enable EDGE_ID (server-team decision: edge delete is keyed on
// e._id and does NOT fall back to the discouraged internal_id(e)).
func TestDeleteAPI_EdgeIDDisabledGraph(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := fmt.Sprintf("test_no_edge_id_%d", time.Now().UnixNano())
	if err := testClient.CreateGraph(ctx, g, gqldb.GraphTypeOpen, "no edge_id"); err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer testClient.DropGraph(ctx, g, true)

	// New graphs default to EDGE_ID ENABLED; force DISABLED for this test.
	if _, err := testClient.Gql(ctx, fmt.Sprintf("ALTER GRAPH %s SET EDGE_ID DISABLED", g), nil); err != nil {
		t.Fatalf("ALTER GRAPH SET EDGE_ID DISABLED failed: %v", err)
	}

	_, err := testClient.InsertNodesGql(ctx, []gqldb.NodeData{
		{ID: "a", Labels: []string{"P"}}, {ID: "b", Labels: []string{"P"}},
	}, &gqldb.InsertConfig{QueryConfig: types.QueryConfig{GraphName: g}})
	if err != nil {
		t.Fatalf("insert nodes: %v", err)
	}
	_, err = testClient.InsertEdgesGql(ctx, []types.EdgeData{
		{Label: "Knows", FromNodeID: "a", ToNodeID: "b", Properties: map[string]interface{}{"w": int64(1)}},
		{Label: "Knows", FromNodeID: "a", ToNodeID: "b", Properties: map[string]interface{}{"w": int64(2)}},
	}, &gqldb.InsertConfig{QueryConfig: types.QueryConfig{GraphName: g}})
	if err != nil {
		t.Fatalf("insert edges: %v", err)
	}

	// Deletion by id on a disabled graph must error, guiding to enable EDGE_ID.
	cfg := gqldb.NewDeleteConfig()
	cfg.GraphName = g
	_, err = testClient.DeleteEdgesByIDs(ctx, []string{"e:1", "e:2"}, cfg)
	if err == nil {
		t.Fatalf("DeleteEdgesByIDs must error on an EDGE_ID-disabled graph")
	}
	if !strings.Contains(err.Error(), "EDGE_ID") && !strings.Contains(err.Error(), "edge _id") {
		t.Errorf("error must guide toward enabling EDGE_ID; got: %v", err)
	}
}

// (11) special-char ids
func TestDeleteAPI_SpecialCharIDs(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	ids := []string{"id'with'quote", `id\with\backslash`, "id-🚀-emoji"}
	nodes := make([]gqldb.NodeData, 0, len(ids))
	for _, id := range ids {
		nodes = append(nodes, gqldb.NodeData{ID: id, Labels: []string{"P"}})
	}
	if _, err := testClient.InsertNodesGql(ctx, nodes,
		&gqldb.InsertConfig{QueryConfig: types.QueryConfig{GraphName: g}}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	r, err := testClient.DeleteNodesByIDs(ctx, ids, dc(g))
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if r.RowsAffected != 3 {
		t.Errorf("expected 3 rowsAffected; got %d", r.RowsAffected)
	}
}

// (12) self-loop
func TestDeleteAPI_SelfLoopEdge(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	seedNodes(t, ctx, g, "loopnode")
	seedEdge(t, ctx, g, "self1", "loopnode", "loopnode", "Self", 42)

	r, err := testClient.DeleteEdgesByIDs(ctx, []string{"self1"}, dc(g))
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if r.RowsAffected != 1 {
		t.Errorf("expected 1; got %d", r.RowsAffected)
	}
	ar, _ := r.Alias("e")
	edges, _, _ := ar.AsEdges()
	if len(edges) != 1 || edges[0].FromNodeID != "loopnode" || edges[0].ToNodeID != "loopnode" {
		t.Errorf("bad self-loop edge: %+v", edges)
	}
}

// (13) default ReturnDeleted=true
func TestDeleteAPI_ReturnDeletedTrueDefault(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	seedNodes(t, ctx, g, "dn1")
	cfg := dc(g)
	if !cfg.ReturnDeleted {
		t.Fatalf("NewDeleteConfig default ReturnDeleted should be true")
	}
	r, err := testClient.DeleteNodesByIDs(ctx, []string{"dn1"}, cfg)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if r.RowCount != 1 || r.RowsAffected != 1 {
		t.Errorf("expected 1/1; got rowCount=%d rowsAffected=%d", r.RowCount, r.RowsAffected)
	}
}

// (14) limit on by-condition: caps node delete count
func TestDeleteAPI_NodeByConditionLimit(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	seedNodes(t, ctx, g, "l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10")
	// limit=3: should delete only 3 of 10, returning 3 rows
	r, err := testClient.DeleteNodesByCondition(ctx, []string{"P"}, "", 3, dc(g))
	if err != nil {
		t.Fatalf("DeleteNodesByCondition with limit failed: %v", err)
	}
	if r.RowsAffected != 3 {
		t.Errorf("expected rowsAffected=3 (capped); got %d", r.RowsAffected)
	}
	// 7 should remain
	cntR, _ := testClient.Gql(ctx, "MATCH (n:P) RETURN count(n)",
		&types.QueryConfig{GraphName: g})
	v, _ := cntR.Rows[0].Values[0].ToGo()
	cnt, _ := v.(int64)
	if cnt != 7 {
		t.Errorf("expected 7 remaining; got %d", cnt)
	}
}

// (15) limit on by-condition: caps edge delete count
func TestDeleteAPI_EdgeByConditionLimit(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := setupDeleteGraph(t, ctx)
	defer cleanupDeleteGraph(t, ctx, g)

	seedNodes(t, ctx, g, "u", "v")
	for i := 1; i <= 5; i++ {
		seedEdge(t, ctx, g, fmt.Sprintf("e%d", i), "u", "v", "Knows", int64(i))
	}
	r, err := testClient.DeleteEdgesByCondition(ctx, "Knows", "", 2, dc(g))
	if err != nil {
		t.Fatalf("DeleteEdgesByCondition with limit failed: %v", err)
	}
	if r.RowsAffected != 2 {
		t.Errorf("expected rowsAffected=2; got %d", r.RowsAffected)
	}
}

// (16) DeleteConfig.GraphName routes per-call (overrides session graph)
func TestDeleteAPI_PerCallGraphOverridesSession(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts := time.Now().UnixNano()
	graphA := fmt.Sprintf("delete_route_a_%d", ts)
	graphB := fmt.Sprintf("delete_route_b_%d", ts)
	if err := testClient.CreateGraph(ctx, graphA, gqldb.GraphTypeOpen, "session graph"); err != nil {
		t.Fatalf("CreateGraph A: %v", err)
	}
	defer testClient.DropGraph(ctx, graphA, true)
	if err := testClient.CreateGraph(ctx, graphB, gqldb.GraphTypeOpen, "per-call graph"); err != nil {
		t.Fatalf("CreateGraph B: %v", err)
	}
	defer testClient.DropGraph(ctx, graphB, true)

	// Session on graphA, but per-call cfg should route to graphB.
	if err := testClient.UseGraph(ctx, graphA); err != nil {
		t.Fatalf("UseGraph A: %v", err)
	}

	if _, err := testClient.InsertNodesGql(ctx,
		[]gqldb.NodeData{{ID: "x", Labels: []string{"P"}}},
		&gqldb.InsertConfig{QueryConfig: types.QueryConfig{GraphName: graphA}}); err != nil {
		t.Fatalf("insert into A: %v", err)
	}
	if _, err := testClient.InsertNodesGql(ctx,
		[]gqldb.NodeData{{ID: "x", Labels: []string{"P"}}},
		&gqldb.InsertConfig{QueryConfig: types.QueryConfig{GraphName: graphB}}); err != nil {
		t.Fatalf("insert into B: %v", err)
	}

	cfgB := gqldb.NewDeleteConfig()
	cfgB.GraphName = graphB
	if _, err := testClient.DeleteNodesByIDs(ctx, []string{"x"}, cfgB); err != nil {
		t.Fatalf("delete B: %v", err)
	}

	ra, err := testClient.Gql(ctx, "MATCH (n) WHERE id(n)='x' RETURN n",
		&types.QueryConfig{GraphName: graphA})
	if err != nil {
		t.Fatalf("query A: %v", err)
	}
	if ra.RowCount != 1 {
		t.Errorf("graphA's x must remain; rowCount=%d", ra.RowCount)
	}

	rb, err := testClient.Gql(ctx, "MATCH (n) WHERE id(n)='x' RETURN n",
		&types.QueryConfig{GraphName: graphB})
	if err != nil {
		t.Fatalf("query B: %v", err)
	}
	if rb.RowCount != 0 {
		t.Errorf("graphB's x must be deleted; rowCount=%d", rb.RowCount)
	}
}
