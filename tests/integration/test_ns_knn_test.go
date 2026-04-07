//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNodeSimilarityKNN(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// show algos for nodesimilarity and knn
	resp, _ := testClient.Gql(ctx, `show algos`, nil)
	for _, row := range resp.Rows {
		val, _ := row.Get(0)
		m, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		name := fmt.Sprintf("%v", m["name"])
		if name == "algo.nodesimilarity" || name == "algo.knn" || name == "algo.filteredknn" || name == "algo.filterednodesimilarity" {
			fmt.Printf("\n=== %s ===\n", name)
			fmt.Printf("  description: %v\n", m["description"])
			fmt.Printf("  parameters: %v\n", m["parameters"])
			fmt.Printf("  returns: %v\n", m["returns"])
			fmt.Printf("  examples: %v\n", m["examples"])
		}
	}
}
