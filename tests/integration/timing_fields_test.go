//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// Verify the 3 server-side timing fields land on the public Response.
//
// Added with proto3-additive GqlResponse fields time_cost_ns / disk_cost_ns /
// compute_cost_ns. Old servers omit them — 0 means "not reported", not
// "took zero time", so the assertions accept zero AND non-negative.
func TestResponseTimingFields(t *testing.T) {
	if testClient == nil {
		t.Skip("testClient unavailable")
	}
	ctx := context.Background()
	cfg := &types.QueryConfig{GraphName: "miniCircle"}

	cases := []struct {
		label string
		gql   string
	}{
		{"trivial RETURN", "RETURN 1 + 1 AS x"},
		{"MATCH node count", "MATCH (n) RETURN count(n) AS c"},
		{"MATCH edge count", "MATCH ()-[r]->() RETURN count(r) AS c"},
	}

	for _, tc := range cases {
		r, err := testClient.Gql(ctx, tc.gql, cfg)
		if err != nil {
			t.Errorf("[%s] %v", tc.label, err)
			continue
		}
		// All three must be non-negative
		if r.TimeCostNs < 0 || r.DiskCostNs < 0 || r.ComputeCostNs < 0 {
			t.Errorf("[%s] negative timing: time=%d disk=%d compute=%d",
				tc.label, r.TimeCostNs, r.DiskCostNs, r.ComputeCostNs)
		}
		// disk + compute should never exceed total time (they're sub-buckets)
		if r.TimeCostNs > 0 && r.DiskCostNs+r.ComputeCostNs > r.TimeCostNs {
			t.Errorf("[%s] sub-bucket overshoot: disk(%d) + compute(%d) > time(%d)",
				tc.label, r.DiskCostNs, r.ComputeCostNs, r.TimeCostNs)
		}
		t.Logf("[%s] time=%d  disk=%d  compute=%d",
			tc.label, r.TimeCostNs, r.DiskCostNs, r.ComputeCostNs)
	}
}
