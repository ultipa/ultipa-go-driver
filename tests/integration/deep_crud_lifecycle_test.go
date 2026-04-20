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

	t.Run("NodeCrudString", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Person {_id: 'c1', name: 'Alice'})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c1' RETURN n.name AS name", cfg)
		val, _ := resp.GetByName(resp.Rows[0], "name")
		if fmt.Sprintf("%v", val) != "Alice" {
			t.Fatalf("Expected Alice, got %v", val)
		}

		testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c1' SET n.name = 'Alice Updated'", cfg)
		resp, _ = testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c1' RETURN n.name AS name", cfg)
		val, _ = resp.GetByName(resp.Rows[0], "name")
		if fmt.Sprintf("%v", val) != "Alice Updated" {
			t.Fatalf("Expected Alice Updated, got %v", val)
		}

		testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c1' DELETE n", cfg)
		resp, _ = testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c1' RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "0" {
			t.Fatalf("Expected 0, got %v", cnt)
		}
	})

	t.Run("NodeCrudInt", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Person {_id: 'c2', age: 25})", cfg)
		testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c2' SET n.age = 30", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c2' RETURN n.age AS age", cfg)
		val, _ := resp.GetByName(resp.Rows[0], "age")
		if fmt.Sprintf("%v", val) != "30" {
			t.Fatalf("Expected 30, got %v", val)
		}
	})

	t.Run("NodeSetMultiple", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Person {_id: 'c3', name: 'Eve', age: 20, score: 70.0})", cfg)
		testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c3' SET n.name = 'Eve V2', n.age = 25, n.score = 90.0", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c3' RETURN n.name, n.age", cfg)
		name, _ := resp.GetByName(resp.Rows[0], "n.name")
		age, _ := resp.GetByName(resp.Rows[0], "n.age")
		if fmt.Sprintf("%v", name) != "Eve V2" {
			t.Fatalf("Expected Eve V2, got %v", name)
		}
		if fmt.Sprintf("%v", age) != "25" {
			t.Fatalf("Expected 25, got %v", age)
		}
	})

	t.Run("NodeSetUnicode", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Person {_id: 'c4', name: 'test'})", cfg)
		testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c4' SET n.name = '你好世界🚀'", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:Person) WHERE id(n) = 'c4' RETURN n.name AS name", cfg)
		val, _ := resp.GetByName(resp.Rows[0], "name")
		s := fmt.Sprintf("%v", val)
		if !strings.Contains(s, "你好") {
			t.Fatalf("Expected unicode in %v", val)
		}
	})

	t.Run("DetachDelete", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:P {_id: 'dd1'}), (:P {_id: 'dd2'})", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='dd1'), (b WHERE id(b)='dd2') INSERT (a)-[:KNOWS]->(b)", cfg)
		_, err := testClient.Gql(ctx, "MATCH (n) WHERE id(n) = 'dd1' DELETE n", cfg)
		if err == nil {
			t.Fatal("Expected error")
		}
		testClient.Gql(ctx, "MATCH (n) WHERE id(n) = 'dd1' DETACH DELETE n", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n) WHERE id(n) = 'dd1' RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "0" {
			t.Fatalf("Expected 0, got %v", cnt)
		}
	})

	t.Run("DeleteWithWhere", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Temp {name: 'keep', tag: 'A'}), (:Temp {name: 'd1', tag: 'B'}), (:Temp {name: 'd2', tag: 'B'})", cfg)
		testClient.Gql(ctx, "MATCH (n:Temp) WHERE n.tag = 'B' DELETE n", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:Temp) RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "1" {
			t.Fatalf("Expected 1, got %v", cnt)
		}
	})

	t.Run("EdgeCrud", func(t *testing.T) {
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
		resp, _ = testClient.Gql(ctx, "MATCH (a)-[e:KNOWS]->(b) WHERE id(a)='ea' RETURN count(e) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "0" {
			t.Fatalf("Expected 0, got %v", cnt)
		}
	})

	t.Run("MultiHopQuery", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:P {_id: 'h1', name: 'A'}), (:P {_id: 'h2', name: 'B'}), (:P {_id: 'h3', name: 'C'})", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='h1'), (b WHERE id(b)='h2') INSERT (a)-[:NEXT]->(b)", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='h2'), (b WHERE id(b)='h3') INSERT (a)-[:NEXT]->(b)", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (a)-[:NEXT]->(b)-[:NEXT]->(c) WHERE id(a)='h1' RETURN c.name AS name", cfg)
		if resp.RowCount != 1 {
			t.Fatalf("Expected 1, got %d", resp.RowCount)
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
				t.Fatalf("Mismatch at %d: got %v", i, val)
			}
		}
	})

	t.Run("UpdateSameNodeMultipleTimes", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Counter {_id: 'um1', counter: 0})", cfg)
		for i := 1; i <= 10; i++ {
			testClient.Gql(ctx, fmt.Sprintf("MATCH (n:Counter) WHERE id(n) = 'um1' SET n.counter = %d", i), cfg)
		}
		resp, _ := testClient.Gql(ctx, "MATCH (n:Counter) WHERE id(n) = 'um1' RETURN n.counter AS val", cfg)
		val, _ := resp.GetByName(resp.Rows[0], "val")
		if fmt.Sprintf("%v", val) != "10" {
			t.Fatalf("Expected 10, got %v", val)
		}
	})

	t.Run("FriendOfFriend", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:User {_id: 'u1', name: 'Alice'}), (:User {_id: 'u2', name: 'Bob'}), (:User {_id: 'u3', name: 'Charlie'}), (:User {_id: 'u4', name: 'Diana'})", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='u1'), (b WHERE id(b)='u2') INSERT (a)-[:FRIEND]->(b)", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='u2'), (b WHERE id(b)='u3') INSERT (a)-[:FRIEND]->(b)", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='u2'), (b WHERE id(b)='u4') INSERT (a)-[:FRIEND]->(b)", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (a:User)-[:FRIEND]->(b)-[:FRIEND]->(c) WHERE id(a) = 'u1' RETURN c.name AS name ORDER BY name", cfg)
		if resp.RowCount < 2 {
			t.Fatalf("Expected >= 2, got %d", resp.RowCount)
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

	t.Run("ReturnExpression", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Calc {_id: 'c1', a: 10, b: 3})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:Calc) WHERE id(n) = 'c1' RETURN n.a + n.b AS sum_val, n.a * n.b AS prod_val", cfg)
		sumVal, _ := resp.GetByName(resp.Rows[0], "sum_val")
		prodVal, _ := resp.GetByName(resp.Rows[0], "prod_val")
		if fmt.Sprintf("%v", sumVal) != "13" {
			t.Fatalf("Expected 13, got %v", sumVal)
		}
		if fmt.Sprintf("%v", prodVal) != "30" {
			t.Fatalf("Expected 30, got %v", prodVal)
		}
	})
}
