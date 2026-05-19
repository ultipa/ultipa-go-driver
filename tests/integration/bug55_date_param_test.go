//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// Regression for #55 — GqldbDate as $-parameter was rejected by the server's
// strict 4-byte DATE converter because the SDK was sending 8 bytes (4-byte
// trailing pad). Verify all 8 temporal/numeric parameter types round-trip.
func TestBug55TemporalParameterBinding(t *testing.T) {
	if testClient == nil {
		t.Skip("testClient unavailable")
	}
	ctx := context.Background()
	graph := "bug55_go_param"
	_, _ = testClient.Gql(ctx, "DROP GRAPH "+graph, nil)
	if _, err := testClient.Gql(ctx, "CREATE GRAPH "+graph, nil); err != nil {
		t.Fatalf("create graph: %v", err)
	}
	defer testClient.Gql(ctx, "DROP GRAPH "+graph, nil)

	probe := func(label string, value interface{}, wantType string) {
		nodeID := "n_" + label
		qc := &types.QueryConfig{
			GraphName:  graph,
			Parameters: map[string]interface{}{"v": value},
		}
		if _, err := testClient.Gql(ctx,
			"INSERT (:T {_id: '"+nodeID+"', v: $v})", qc); err != nil {
			t.Errorf("[%s] INSERT failed: %v", label, err)
			return
		}
		r, err := testClient.Gql(ctx,
			"MATCH (n:T {_id: '"+nodeID+"'}) RETURN typeOf(n.v)",
			&types.QueryConfig{GraphName: graph})
		if err != nil {
			t.Errorf("[%s] READ failed: %v", label, err)
			return
		}
		if r.RowCount == 0 {
			t.Errorf("[%s] READ returned no rows — parameter was dropped", label)
			return
		}
		got, _ := r.Rows[0].Get(0)
		if gotS, _ := got.(string); gotS != wantType {
			t.Errorf("[%s] typeOf = %v, want %s", label, got, wantType)
		} else {
			t.Logf("[%s] OK typeOf=%v", label, got)
		}
	}

	probe("GqldbDate", types.GqldbDate{Year: 2024, Month: 3, Day: 15}, "DATE")
	probe("LocalDateTime", types.LocalDateTime{Time: time.Date(2024, 3, 15, 10, 30, 45, 0, time.UTC)}, "LOCAL_DATETIME")
	probe("LocalTime", types.LocalTime{Hour: 10, Minute: 30, Second: 45, Nanosecond: 0}, "TIME")
	probe("Decimal", types.Decimal{Value: "123.456"}, "DECIMAL")
}
