//go:build integration

// Driver convenience method `Client.CreateGraphWithEdgeId(...)`.
//
// Covers the new `edgeId` parameter on the Go SDK. The driver creates
// the graph via the existing gRPC path (preserving description), then —
// when `edgeId` is not EdgeIdUnset — issues a follow-up
// `ALTER GRAPH <name> SET EDGE_ID ENABLED|DISABLED` to lock in the
// requested state.
//
// Matrix:
//  1. OPEN   + EdgeIdEnabled
//  2. OPEN   + EdgeIdDisabled
//  3. CLOSED + EdgeIdEnabled  (also exercises gRPC-creates-CLOSED path)
//  4. CLOSED + EdgeIdDisabled
//  5. EdgeIdUnset (default) — no follow-up ALTER, 1-RPC path unchanged
//  6. ENABLED end-to-end — `e._id` projection returns a non-empty edge id
//  7. DISABLED end-to-end — `e._id` projection errors with EDGE_ID hint
//
// Port of Python test_create_graph_edge_id.py.
package integration

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// uniqueGraphName returns a short, per-subtest random graph name.
func uniqueGraphName() string {
	return fmt.Sprintf("cg_eid_%d", 10000+rand.Intn(90000))
}

// edgeIdStatus reads `SHOW EDGE_ID STATUS` against `graph` and returns
// the upper-cased value of the `status` column.
func edgeIdStatus(t *testing.T, ctx context.Context, graph string) string {
	t.Helper()
	if err := testClient.UseGraph(ctx, graph); err != nil {
		t.Fatalf("UseGraph(%s) failed: %v", graph, err)
	}
	r, err := testClient.Gql(ctx, "SHOW EDGE_ID STATUS", nil)
	if err != nil {
		t.Fatalf("SHOW EDGE_ID STATUS failed for %s: %v", graph, err)
	}
	if len(r.Rows) == 0 {
		t.Fatalf("SHOW EDGE_ID STATUS returned no rows for %s", graph)
	}
	v, err := r.Rows[0].GetByName("status")
	if err != nil {
		// Fallback to positional lookup if the column name differs.
		v, err = r.Rows[0].Get(0)
		if err != nil {
			t.Fatalf("could not read status column from SHOW EDGE_ID STATUS: %v", err)
		}
	}
	s, ok := v.(string)
	if !ok {
		s = fmt.Sprintf("%v", v)
	}
	return strings.ToUpper(s)
}

// cleanupGraph drops the graph after switching to a safe default. The
// server rejects dropping the "currently active graph".
func cleanupGraph(ctx context.Context, graph string) {
	_ = testClient.UseGraph(ctx, "miniCircle")
	_ = testClient.DropGraph(ctx, graph, true)
}

// TestCreateGraphEdgeId mirrors the 7-case Python parametrization.
func TestCreateGraphEdgeId(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}

	type matrixCase struct {
		name      string
		graphType gqldb.GraphType
		mode      gqldb.EdgeIdMode
		expected  string
	}
	matrix := []matrixCase{
		{"Open-Enabled", gqldb.GraphTypeOpen, gqldb.EdgeIdEnabled, "ENABLED"},
		{"Open-Disabled", gqldb.GraphTypeOpen, gqldb.EdgeIdDisabled, "DISABLED"},
		{"Closed-Enabled", gqldb.GraphTypeClosed, gqldb.EdgeIdEnabled, "ENABLED"},
		{"Closed-Disabled", gqldb.GraphTypeClosed, gqldb.EdgeIdDisabled, "DISABLED"},
	}

	// Cases 1-4: matrix of (graphType, mode) -> expected SHOW STATUS.
	for _, tc := range matrix {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			graph := uniqueGraphName()
			t.Cleanup(func() { cleanupGraph(ctx, graph) })

			if err := testClient.CreateGraphWithEdgeId(ctx, graph, tc.graphType, "test desc", tc.mode); err != nil {
				t.Fatalf("CreateGraphWithEdgeId(%s, %v, %v) failed: %v",
					graph, tc.graphType, tc.mode, err)
			}
			got := edgeIdStatus(t, ctx, graph)
			if got != tc.expected {
				t.Errorf("expected EDGE_ID=%s for %v+%v; got %q",
					tc.expected, tc.graphType, tc.mode, got)
			}
		})
	}

	// Case 5: EdgeIdUnset (default) — no follow-up ALTER, status still
	// returns a valid value.
	t.Run("DefaultEdgeIdUnsetSkipsAlter", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		graph := uniqueGraphName()
		t.Cleanup(func() { cleanupGraph(ctx, graph) })

		// Use legacy CreateGraph entrypoint, which delegates to
		// CreateGraphWithEdgeId(..., EdgeIdUnset).
		if err := testClient.CreateGraph(ctx, graph, gqldb.GraphTypeOpen, "no edge_id arg"); err != nil {
			t.Fatalf("CreateGraph failed: %v", err)
		}
		status := edgeIdStatus(t, ctx, graph)
		if status != "ENABLED" && status != "DISABLED" {
			t.Errorf("unexpected EDGE_ID status on fresh graph: %q", status)
		}
	})

	// Case 6: ENABLED end-to-end — `e._id` projection returns a non-empty id.
	t.Run("ExplicitEnabledThenQueryEIdReturnsValue", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		graph := uniqueGraphName()
		t.Cleanup(func() { cleanupGraph(ctx, graph) })

		if err := testClient.CreateGraphWithEdgeId(ctx, graph, gqldb.GraphTypeOpen, "", gqldb.EdgeIdEnabled); err != nil {
			t.Fatalf("CreateGraphWithEdgeId(ENABLED) failed: %v", err)
		}
		if err := testClient.UseGraph(ctx, graph); err != nil {
			t.Fatalf("UseGraph failed: %v", err)
		}
		if _, err := testClient.Gql(ctx, "INSERT (:U {_id:'a'}), (:U {_id:'b'})", nil); err != nil {
			t.Fatalf("insert nodes failed: %v", err)
		}
		if _, err := testClient.Gql(ctx,
			"MATCH (a:U {_id:'a'}), (b:U {_id:'b'}) INSERT (a)-[:K]->(b)", nil); err != nil {
			t.Fatalf("insert edge failed: %v", err)
		}
		r, err := testClient.Gql(ctx,
			"MATCH ()-[e:K]->() RETURN e._id LIMIT 1",
			&types.QueryConfig{GraphName: graph})
		if err != nil {
			t.Fatalf("MATCH RETURN e._id failed when EDGE_ID=ENABLED: %v", err)
		}
		if len(r.Rows) == 0 {
			t.Fatalf("expected at least one row, got none")
		}
		eid, _ := r.Rows[0].Get(0)
		if s, ok := eid.(string); !ok || s == "" {
			t.Errorf("e._id should be non-empty when create_graph used EdgeIdEnabled; got %v", eid)
		}
	})

	// Case 7: DISABLED end-to-end — `e._id` projection must error with
	// "EDGE_ID" / "not available" / "not enabled".
	t.Run("ExplicitDisabledThenQueryEIdErrors", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		graph := uniqueGraphName()
		t.Cleanup(func() { cleanupGraph(ctx, graph) })

		if err := testClient.CreateGraphWithEdgeId(ctx, graph, gqldb.GraphTypeOpen, "", gqldb.EdgeIdDisabled); err != nil {
			t.Fatalf("CreateGraphWithEdgeId(DISABLED) failed: %v", err)
		}
		if err := testClient.UseGraph(ctx, graph); err != nil {
			t.Fatalf("UseGraph failed: %v", err)
		}
		if _, err := testClient.Gql(ctx, "INSERT (:U {_id:'a'}), (:U {_id:'b'})", nil); err != nil {
			t.Fatalf("insert nodes failed: %v", err)
		}
		if _, err := testClient.Gql(ctx,
			"MATCH (a:U {_id:'a'}), (b:U {_id:'b'}) INSERT (a)-[:K]->(b)", nil); err != nil {
			t.Fatalf("insert edge failed: %v", err)
		}
		_, err := testClient.Gql(ctx,
			"MATCH ()-[e:K]->() RETURN e._id LIMIT 1",
			&types.QueryConfig{GraphName: graph})
		if err == nil {
			t.Fatalf("RETURN e._id must error when EDGE_ID=DISABLED, but succeeded")
		}
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "edge_id") &&
			!strings.Contains(msg, "not available") &&
			!strings.Contains(msg, "not enabled") {
			t.Errorf("unexpected error message: %q", err.Error())
		}
	})
}
