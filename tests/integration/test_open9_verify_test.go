//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// Shared setup for open9 verification: star+chain graph
func setupOpen9Graph(t *testing.T, ctx context.Context) (string, []string, func()) {
	t.Helper()
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	graphName := fmt.Sprintf("test_o9_%d", time.Now().UnixMilli())
	noAuthClient.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	_ = noAuthClient.UseGraph(ctx, graphName)

	session, _ := noAuthClient.StartBulkImport(ctx, graphName, nil)
	nodes := []*gqldb.NodeData{
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "Hub"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "D"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "E"}},
	}
	nr, _ := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	ids := nr.NodeIDs

	biEdge := func(from, to string) []*gqldb.EdgeData {
		return []*gqldb.EdgeData{
			{Label: "LINK", FromNodeID: from, ToNodeID: to},
			{Label: "LINK", FromNodeID: to, ToNodeID: from},
		}
	}
	var edges []*gqldb.EdgeData
	edges = append(edges, biEdge(ids[0], ids[1])...)
	edges = append(edges, biEdge(ids[0], ids[2])...)
	edges = append(edges, biEdge(ids[0], ids[3])...)
	edges = append(edges, biEdge(ids[0], ids[4])...)
	edges = append(edges, biEdge(ids[4], ids[5])...)

	noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	cleanup := func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}
	return graphName, ids, cleanup
}

// =============================================================================
// #2: Systemic direction in/out identical — verify across all affected algorithms
// =============================================================================

func TestOpen9_2_DirectionSystemic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen9Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	algos := []struct {
		name string
		query string
	}{
		{"eigenvector", `CALL algo.eigenvector({direction: '%s'}) YIELD nodeId, score`},
		{"graphcentrality", `CALL algo.graphcentrality({direction: '%s'}) YIELD nodeId, centrality`},
		{"harmonic", `CALL algo.harmonic({direction: '%s'}) YIELD nodeId, score`},
		{"closeness", `CALL algo.closeness({direction: '%s'}) YIELD nodeId, score`},
		{"katz", `CALL algo.katz({direction: '%s'}) YIELD nodeId, score`},
	}

	for _, algo := range algos {
		t.Run(algo.name, func(t *testing.T) {
			results := make(map[string]string)
			for _, dir := range []string{"in", "out"} {
				q := fmt.Sprintf(algo.query, dir)
				resp, err := noAuthClient.Gql(ctx, q, qc)
				if err != nil {
					t.Logf("%s direction=%s error: %v", algo.name, dir, err)
					return
				}
				s := ""
				for _, row := range resp.Rows {
					nid, _ := row.Get(0)
					sc, _ := row.Get(1)
					s += fmt.Sprintf("%v:%.6f,", nid, sc.(float64))
				}
				results[dir] = s
			}
			if results["in"] == results["out"] && results["in"] != "" {
				t.Logf("BUG #2 CONFIRMED for %s: in and out return identical results", algo.name)
			} else if results["in"] != results["out"] {
				t.Logf("FIXED for %s: in and out return DIFFERENT results", algo.name)
			}
		})
	}
}

// =============================================================================
// #3: eigenvector iterations/tolerance accept negatives
// =============================================================================

func TestOpen9_3_EigenvectorNegativeParams(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen9Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("iterations_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({iterations: -1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("iterations=-1 correctly rejected: %v", err)
		} else {
			t.Logf("BUG #3 CONFIRMED: iterations=-1 accepted, returned %d rows", resp.RowCount)
		}
	})

	t.Run("tolerance_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.eigenvector({tolerance: -0.1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("tolerance=-0.1 correctly rejected: %v", err)
		} else {
			t.Logf("BUG #3 CONFIRMED: tolerance=-0.1 accepted, returned %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// #5: graphcentrality ids has no effect
// =============================================================================

func TestOpen9_5_GraphcentralityIds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupOpen9Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	q := fmt.Sprintf(`CALL algo.graphcentrality({ids: ['%s']}) YIELD nodeId, centrality`, ids[0])
	resp, err := noAuthClient.Gql(ctx, q, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	if resp.RowCount == 1 {
		t.Log("FIXED: ids parameter works, returned 1 row")
	} else {
		t.Logf("BUG #5 CONFIRMED: ids=['%s'] but returned %d rows (expected 1)", ids[0], resp.RowCount)
	}
}

// =============================================================================
// #6: graphcentrality disconnected graph wrong eccentricity/centerCount
// =============================================================================

func TestOpen9_6_GraphcentralityDisconnected(t *testing.T) {
	if noAuthClient == nil {
		t.Skip("No-auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := fmt.Sprintf("test_o9_disc_%d", time.Now().UnixMilli())
	noAuthClient.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	defer func() {
		_ = noAuthClient.UseGraph(ctx, "default")
		noAuthClient.Gql(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)
	}()
	_ = noAuthClient.UseGraph(ctx, graphName)

	session, _ := noAuthClient.StartBulkImport(ctx, graphName, nil)
	nodes := []*gqldb.NodeData{
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "A"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "B"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "C"}},
		{Labels: []string{"N"}, Properties: map[string]interface{}{"name": "D"}},
	}
	nr, _ := noAuthClient.InsertNodesBatchAuto(ctx, graphName, nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID})
	ids := nr.NodeIDs
	biEdge := func(from, to string) []*gqldb.EdgeData {
		return []*gqldb.EdgeData{
			{Label: "L", FromNodeID: from, ToNodeID: to},
			{Label: "L", FromNodeID: to, ToNodeID: from},
		}
	}
	var edges []*gqldb.EdgeData
	edges = append(edges, biEdge(ids[0], ids[1])...)
	edges = append(edges, biEdge(ids[2], ids[3])...)
	noAuthClient.InsertEdgesBatchAuto(ctx, graphName, edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID})
	noAuthClient.EndBulkImport(ctx, session.SessionID)
	time.Sleep(1 * time.Second)

	qc := &gqldb.QueryConfig{GraphName: graphName}

	// Check stats
	stats, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality.stats() YIELD nodeCount, radius, diameter, centerCount`, qc)
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if stats.RowCount > 0 {
		nc, _ := stats.Rows[0].Get(0)
		rad, _ := stats.Rows[0].Get(1)
		dia, _ := stats.Rows[0].Get(2)
		cc, _ := stats.Rows[0].Get(3)
		t.Logf("nodeCount=%v, radius=%v, diameter=%v, centerCount=%v", nc, rad, dia, cc)

		// Expected: within components, eccentricity=1, so radius=1, diameter=1, centerCount=4
		ccVal := int64(0)
		switch v := cc.(type) {
		case int64:
			ccVal = v
		case float64:
			ccVal = int64(v)
		}
		if ccVal == 0 {
			t.Log("BUG #6 CONFIRMED: centerCount=0 for disconnected graph (should be 4)")
		} else if ccVal == 4 {
			t.Log("FIXED: centerCount=4 correct")
		} else {
			t.Logf("centerCount=%d (unexpected)", ccVal)
		}
	}

	// Check individual node eccentricities
	resp, err := noAuthClient.Gql(ctx, `CALL algo.graphcentrality() YIELD nodeId, eccentricity, centrality, isCenter`, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	for _, row := range resp.Rows {
		nid, _ := row.Get(0)
		ecc, _ := row.Get(1)
		cen, _ := row.Get(2)
		ic, _ := row.Get(3)
		t.Logf("  nodeId=%v, eccentricity=%v, centrality=%v, isCenter=%v", nid, ecc, cen, ic)
	}
}

// =============================================================================
// #8: katz ids has no effect
// =============================================================================

func TestOpen9_8_KatzIds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, ids, cleanup := setupOpen9Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	q := fmt.Sprintf(`CALL algo.katz({ids: ['%s']}) YIELD nodeId, score`, ids[0])
	resp, err := noAuthClient.Gql(ctx, q, qc)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	if resp.RowCount == 1 {
		t.Log("FIXED: katz ids works, returned 1 row")
	} else {
		t.Logf("BUG #8 CONFIRMED: ids=['%s'] but returned %d rows (expected 1)", ids[0], resp.RowCount)
	}
}

// =============================================================================
// #9: katz beta/tolerance accept negatives
// =============================================================================

func TestOpen9_9_KatzNegativeParams(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen9Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	t.Run("beta_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({beta: -1.0}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("beta=-1.0 correctly rejected: %v", err)
		} else {
			t.Logf("BUG #9 CONFIRMED: beta=-1.0 accepted, returned %d rows", resp.RowCount)
		}
	})

	t.Run("tolerance_negative", func(t *testing.T) {
		resp, err := noAuthClient.Gql(ctx, `CALL algo.katz({tolerance: -0.1}) YIELD nodeId, score`, qc)
		if err != nil {
			t.Logf("tolerance=-0.1 correctly rejected: %v", err)
		} else {
			t.Logf("BUG #9 CONFIRMED: tolerance=-0.1 accepted, returned %d rows", resp.RowCount)
		}
	})
}

// =============================================================================
// #12: pagerank tolerance accepts negatives
// =============================================================================

func TestOpen9_12_PagerankTolerance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen9Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := noAuthClient.Gql(ctx, `CALL algo.pagerank({tolerance: -0.1}) YIELD nodeId, score`, qc)
	if err != nil {
		t.Logf("tolerance=-0.1 correctly rejected: %v", err)
	} else {
		t.Logf("BUG #12 CONFIRMED: tolerance=-0.1 accepted, returned %d rows", resp.RowCount)
	}
}

// =============================================================================
// #1/#7/#10/#11: Missing weight — verify still missing
// =============================================================================

func TestOpen9_MissingWeight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen9Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	algos := []struct {
		issue int
		name  string
		query string
	}{
		{1, "eigenvector", `CALL algo.eigenvector({weight: 'w'}) YIELD nodeId, score`},
		{7, "harmonic", `CALL algo.harmonic({weight: 'w'}) YIELD nodeId, score`},
		{10, "katz", `CALL algo.katz({weight: 'w'}) YIELD nodeId, score`},
		{11, "pagerank", `CALL algo.pagerank({weight: 'w'}) YIELD nodeId, score`},
	}

	for _, algo := range algos {
		t.Run(fmt.Sprintf("issue_%d_%s", algo.issue, algo.name), func(t *testing.T) {
			_, err := noAuthClient.Gql(ctx, algo.query, qc)
			if err != nil {
				if strings.Contains(fmt.Sprintf("%v", err), "unknown parameter") {
					t.Logf("#%d CONFIRMED: %s weight still not supported", algo.issue, algo.name)
				} else {
					t.Logf("#%d %s weight error: %v", algo.issue, algo.name, err)
				}
			} else {
				t.Logf("#%d FIXED: %s weight now supported!", algo.issue, algo.name)
			}
		})
	}
}

// =============================================================================
// #4: eigenvector uses 'iterations' naming
// =============================================================================

func TestOpen9_4_EigenvectorNaming(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	graphName, _, cleanup := setupOpen9Graph(t, ctx)
	defer cleanup()
	qc := &gqldb.QueryConfig{GraphName: graphName}

	// iterations works
	_, err1 := noAuthClient.Gql(ctx, `CALL algo.eigenvector({iterations: 10}) YIELD nodeId, score`, qc)
	// maxIterations should not
	_, err2 := noAuthClient.Gql(ctx, `CALL algo.eigenvector({maxIterations: 10}) YIELD nodeId, score`, qc)

	if err1 == nil && err2 != nil {
		t.Log("#4 CONFIRMED: 'iterations' works but 'maxIterations' does not (naming inconsistency)")
	} else if err1 == nil && err2 == nil {
		t.Log("#4 FIXED: both 'iterations' and 'maxIterations' accepted")
	}
}
