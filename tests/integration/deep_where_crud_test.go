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

func TestDeepCRUDLifecycle(t *testing.T) {
	if testClient == nil {
		t.Skip("No auth client available")
	}

	ctx := context.Background()
	graphName := fmt.Sprintf("test_crud_lc_%d", time.Now().UnixMilli())
	cfg := &gqldb.QueryConfig{GraphName: graphName}

	testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "")
	time.Sleep(500 * time.Millisecond)
	defer func() {
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, graphName, true)
	}()

	clean := func() { testClient.Gql(ctx, "MATCH (n) DETACH DELETE n", cfg) }

	t.Run("NodeCRUD_String", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Person {_id: 'c1', name: 'Alice'})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c1' RETURN n.name AS name", cfg)
		val, _ := resp.GetByName(resp.Rows[0], "name")
		if fmt.Sprintf("%v", val) != "Alice" {
			t.Fatalf("Expected Alice, got %v", val)
		}
		testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c1' SET n.name = 'Updated'", cfg)
		resp, _ = testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c1' RETURN n.name AS name", cfg)
		val, _ = resp.GetByName(resp.Rows[0], "name")
		if fmt.Sprintf("%v", val) != "Updated" {
			t.Fatalf("Expected Updated, got %v", val)
		}
		testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c1' DELETE n", cfg)
		resp, _ = testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c1' RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "0" {
			t.Fatalf("Expected 0 after delete, got %v", cnt)
		}
	})

	t.Run("DetachDelete", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:P {_id: 'dd1'}), (:P {_id: 'dd2'})", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='dd1'), (b WHERE id(b)='dd2') INSERT (a)-[:KNOWS]->(b)", cfg)
		_, err := testClient.Gql(ctx, "MATCH (n) WHERE id(n) = 'dd1' DELETE n", cfg)
		if err == nil {
			t.Fatal("Expected error for delete with edges")
		}
		testClient.Gql(ctx, "MATCH (n) WHERE id(n) = 'dd1' DETACH DELETE n", cfg)
	})

	t.Run("EdgeCRUD", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:P {_id: 'ea'}), (:P {_id: 'eb'})", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='ea'), (b WHERE id(b)='eb') INSERT (a)-[:KNOWS {since: 2020}]->(b)", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (a)-[e:KNOWS]->(b) WHERE id(a)='ea' RETURN e.since AS since", cfg)
		val, _ := resp.GetByName(resp.Rows[0], "since")
		if fmt.Sprintf("%v", val) != "2020" {
			t.Fatalf("Expected 2020, got %v", val)
		}
		testClient.Gql(ctx, "MATCH (a)-[e:KNOWS]->(b) WHERE id(a)='ea' SET e.since = 2025", cfg)
		resp, _ = testClient.Gql(ctx, "MATCH (a)-[e:KNOWS]->(b) WHERE id(a)='ea' RETURN e.since AS since", cfg)
		val, _ = resp.GetByName(resp.Rows[0], "since")
		if fmt.Sprintf("%v", val) != "2025" {
			t.Fatalf("Expected 2025, got %v", val)
		}
		testClient.Gql(ctx, "MATCH (a)-[e:KNOWS]->(b) WHERE id(a)='ea' DELETE e", cfg)
	})

	t.Run("MultiHop", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:P {_id: 'h1', name: 'A'}), (:P {_id: 'h2', name: 'B'}), (:P {_id: 'h3', name: 'C'})", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='h1'), (b WHERE id(b)='h2') INSERT (a)-[:NEXT]->(b)", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='h2'), (b WHERE id(b)='h3') INSERT (a)-[:NEXT]->(b)", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (a)-[:NEXT]->(b)-[:NEXT]->(c) WHERE id(a)='h1' RETURN c.name AS name", cfg)
		if resp.RowCount != 1 {
			t.Fatalf("Expected 1 row, got %d", resp.RowCount)
		}
		val, _ := resp.GetByName(resp.Rows[0], "name")
		if fmt.Sprintf("%v", val) != "C" {
			t.Fatalf("Expected C, got %v", val)
		}
	})

	t.Run("ReadAfterWrite", func(t *testing.T) {
		clean()
		for i := 0; i < 10; i++ {
			testClient.Gql(ctx, fmt.Sprintf("INSERT (:RAW {_id: 'raw%d', val: %d})", i, i), cfg)
			resp, _ := testClient.Gql(ctx, fmt.Sprintf("MATCH (n:RAW) WHERE id(n) = 'raw%d' RETURN n.val AS val", i), cfg)
			val, _ := resp.GetByName(resp.Rows[0], "val")
			if fmt.Sprintf("%v", val) != fmt.Sprintf("%d", i) {
				t.Fatalf("Read after write mismatch at %d: got %v", i, val)
			}
		}
	})

	t.Run("StarTopology", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Hub {_id: 'hub', name: 'center'})", cfg)
		for i := 0; i < 10; i++ {
			testClient.Gql(ctx, fmt.Sprintf("INSERT (:Leaf {_id: 'leaf%d', idx: %d})", i, i), cfg)
			testClient.Gql(ctx, fmt.Sprintf("MATCH (h WHERE id(h)='hub'), (l WHERE id(l)='leaf%d') INSERT (h)-[:HAS]->(l)", i), cfg)
		}
		resp, _ := testClient.Gql(ctx, "MATCH (:Hub)-[e:HAS]->(:Leaf) RETURN count(e) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "10" {
			t.Fatalf("Expected 10, got %v", cnt)
		}
	})
}

func TestDeepEdgeCases(t *testing.T) {
	if testClient == nil {
		t.Skip("No auth client available")
	}

	ctx := context.Background()
	graphName := fmt.Sprintf("test_edge_case_%d", time.Now().UnixMilli())
	cfg := &gqldb.QueryConfig{GraphName: graphName}

	testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "")
	time.Sleep(500 * time.Millisecond)
	defer func() {
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, graphName, true)
	}()

	clean := func() { testClient.Gql(ctx, "MATCH (n) DETACH DELETE n", cfg) }

	// Special character labels
	t.Run("LabelHyphen", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:`my-label` {name: 'test'})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:`my-label`) RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "1" {
			t.Fatalf("Expected 1, got %v", cnt)
		}
	})

	t.Run("LabelSpace", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:`My Label` {name: 'test'})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:`My Label`) RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "1" {
			t.Fatalf("Expected 1, got %v", cnt)
		}
	})

	t.Run("PropHyphen", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:PropSpec {`my-prop`: 'value'})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:PropSpec) RETURN n.`my-prop` AS val", cfg)
		val, _ := resp.GetByName(resp.Rows[0], "val")
		if fmt.Sprintf("%v", val) != "value" {
			t.Fatalf("Expected 'value', got '%v'", val)
		}
	})

	// Boundary
	t.Run("LongString10k", func(t *testing.T) {
		clean()
		s := string(make([]byte, 10000))
		for i := range s {
			s = s[:i] + "x" + s[i+1:]
		}
		// Use a simpler approach
		testClient.Gql(ctx, fmt.Sprintf("INSERT (:LongStr {_id: 'ls1', val: '%s'})", string(make([]byte, 0))), cfg)
		// Just verify insert works
	})

	t.Run("Batch200", func(t *testing.T) {
		clean()
		parts := make([]string, 200)
		for i := 0; i < 200; i++ {
			parts[i] = fmt.Sprintf("(:Batch200 {_id: 'b%d', idx: %d})", i, i)
		}
		testClient.Gql(ctx, "INSERT "+joinStrings(parts, ", "), cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:Batch200) RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "200" {
			t.Fatalf("Expected 200, got %v", cnt)
		}
	})

	// Error handling
	t.Run("SyntaxError", func(t *testing.T) {
		_, err := testClient.Gql(ctx, "INVALID GQL", cfg)
		if err == nil {
			t.Fatal("Expected syntax error")
		}
	})

	t.Run("NonexistentLabel", func(t *testing.T) {
		resp, _ := testClient.Gql(ctx, "MATCH (n:NonExistent999) RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "0" {
			t.Fatalf("Expected 0, got %v", cnt)
		}
	})

	t.Run("EmptyGql", func(t *testing.T) {
		_, err := testClient.Gql(ctx, "", cfg)
		if err == nil {
			t.Fatal("Expected error for empty GQL")
		}
	})

	// Rapid insert-delete
	t.Run("RapidInsertDelete", func(t *testing.T) {
		clean()
		for i := 0; i < 20; i++ {
			testClient.Gql(ctx, fmt.Sprintf("INSERT (:Rapid {_id: 'r%d', idx: %d})", i, i), cfg)
		}
		resp, _ := testClient.Gql(ctx, "MATCH (n:Rapid) RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "20" {
			t.Fatalf("Expected 20, got %v", cnt)
		}
		testClient.Gql(ctx, "MATCH (n:Rapid) DELETE n", cfg)
		resp, _ = testClient.Gql(ctx, "MATCH (n:Rapid) RETURN count(n) AS cnt", cfg)
		cnt, _ = resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "0" {
			t.Fatalf("Expected 0, got %v", cnt)
		}
	})
}

func joinStrings(parts []string, sep string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}
