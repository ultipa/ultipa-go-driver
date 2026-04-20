//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestDeepWhereConditions(t *testing.T) {
	if testClient == nil {
		t.Skip("No auth client available")
	}

	ctx := context.Background()
	graphName := fmt.Sprintf("test_deep_where_%d", time.Now().UnixMilli())
	cfg := &gqldb.QueryConfig{GraphName: graphName}

	testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "")
	time.Sleep(500 * time.Millisecond)
	defer func() {
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, graphName, true)
	}()

	// Insert test data
	testClient.Gql(ctx, `INSERT
		(:Person {_id: 'p1', name: 'Alice', age: 30, score: 95.5, active: true, city: 'Beijing'}),
		(:Person {_id: 'p2', name: 'Bob', age: 25, score: 80.0, active: false, city: 'Shanghai'}),
		(:Person {_id: 'p3', name: 'Charlie', age: 35, score: 70.5, active: true, city: 'Beijing'}),
		(:Person {_id: 'p4', name: 'Diana', age: 28, score: 90.0, active: false, city: 'Guangzhou'}),
		(:Person {_id: 'p5', name: 'Eve', age: 30, score: 85.5, active: true, city: 'Shanghai'}),
		(:Person {_id: 'p6', name: '', age: 0, score: 0.0, active: false, city: ''})`, cfg)
	testClient.Gql(ctx, "MATCH (a WHERE id(a)='p1'), (b WHERE id(b)='p2') INSERT (a)-[:KNOWS {since: 2020, weight: 0.9}]->(b)", cfg)
	testClient.Gql(ctx, "MATCH (a WHERE id(a)='p2'), (b WHERE id(b)='p3') INSERT (a)-[:KNOWS {since: 2021, weight: 0.5}]->(b)", cfg)
	testClient.Gql(ctx, "MATCH (a WHERE id(a)='p3'), (b WHERE id(b)='p4') INSERT (a)-[:KNOWS {since: 2022, weight: 0.3}]->(b)", cfg)

	count := func(t *testing.T, q string, expected int) {
		t.Helper()
		resp, err := testClient.Gql(ctx, q, cfg)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != fmt.Sprintf("%d", expected) {
			t.Fatalf("Expected %d, got %v", expected, cnt)
		}
	}

	// Comparison
	t.Run("EqInt", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.age = 30 RETURN count(n) AS cnt", 2) })
	t.Run("NeqInt", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.age <> 30 RETURN count(n) AS cnt", 4) })
	t.Run("Gt", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.age > 30 RETURN count(n) AS cnt", 1) })
	t.Run("Gte", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.age >= 30 RETURN count(n) AS cnt", 3) })
	t.Run("Lt", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.age < 28 RETURN count(n) AS cnt", 2) })
	t.Run("EqEmpty", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.name = '' RETURN count(n) AS cnt", 1) })
	t.Run("NeqEmpty", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.name != '' RETURN count(n) AS cnt", 5) })
	t.Run("EqBoolTrue", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.active = true RETURN count(n) AS cnt", 3) })

	// String operators
	t.Run("Contains", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.name CONTAINS 'li' RETURN count(n) AS cnt", 2) })
	t.Run("StartsWith", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.name STARTS WITH 'A' RETURN count(n) AS cnt", 1) })
	t.Run("EndsWith", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.name ENDS WITH 'e' RETURN count(n) AS cnt", 3) })

	// Boolean logic
	t.Run("And", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.age > 25 AND n.active = true RETURN count(n) AS cnt", 3) })
	t.Run("Or", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.city = 'Beijing' OR n.city = 'Shanghai' RETURN count(n) AS cnt", 4) })
	t.Run("AndOrParens", func(t *testing.T) {
		count(t, "MATCH (n:Person) WHERE n.active = true AND (n.city = 'Beijing' OR n.city = 'Shanghai') RETURN count(n) AS cnt", 3)
	})
	t.Run("TripleAnd", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.age >= 25 AND n.active = true AND n.score > 80 RETURN count(n) AS cnt", 2) })
	t.Run("AndNot", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.city = 'Beijing' AND n.name <> 'Alice' RETURN count(n) AS cnt", 1) })

	// IN operator
	t.Run("InStrList", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.city IN ['Beijing', 'Guangzhou'] RETURN count(n) AS cnt", 3) })
	t.Run("InIntList", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.age IN [25, 30] RETURN count(n) AS cnt", 3) })
	t.Run("InEmpty", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE n.name IN [] RETURN count(n) AS cnt", 0) })
	t.Run("IdFunction", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE id(n) = 'p1' RETURN count(n) AS cnt", 1) })
	t.Run("IdInList", func(t *testing.T) { count(t, "MATCH (n:Person) WHERE id(n) IN ['p1','p2','p3'] RETURN count(n) AS cnt", 3) })

	// Edge WHERE
	t.Run("EdgeSince", func(t *testing.T) { count(t, "MATCH ()-[e:KNOWS]->() WHERE e.since >= 2021 RETURN count(e) AS cnt", 2) })
	t.Run("EdgeWeight", func(t *testing.T) { count(t, "MATCH ()-[e:KNOWS]->() WHERE e.weight > 0.5 RETURN count(e) AS cnt", 1) })

	// Aggregations
	t.Run("Sum", func(t *testing.T) {
		resp, _ := testClient.Gql(ctx, "MATCH (n:Person) RETURN sum(n.age) AS total", cfg)
		total, _ := resp.GetByName(resp.Rows[0], "total")
		if fmt.Sprintf("%v", total) != "148" {
			t.Fatalf("Expected sum=148, got %v", total)
		}
	})
	t.Run("MinMax", func(t *testing.T) {
		resp, _ := testClient.Gql(ctx, "MATCH (n:Person) WHERE n.name != '' RETURN min(n.age) AS mn, max(n.age) AS mx", cfg)
		mn, _ := resp.GetByName(resp.Rows[0], "mn")
		mx, _ := resp.GetByName(resp.Rows[0], "mx")
		if fmt.Sprintf("%v", mn) != "25" || fmt.Sprintf("%v", mx) != "35" {
			t.Fatalf("Expected min=25 max=35, got %v %v", mn, mx)
		}
	})

	// LIMIT/SKIP
	t.Run("Limit", func(t *testing.T) {
		resp, _ := testClient.Gql(ctx, "MATCH (n:Person) RETURN n.name LIMIT 3", cfg)
		if resp.RowCount != 3 {
			t.Fatalf("Expected 3, got %d", resp.RowCount)
		}
	})
}
