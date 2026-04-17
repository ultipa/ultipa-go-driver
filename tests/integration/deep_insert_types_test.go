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

func TestDeepInsertTypes(t *testing.T) {
	if testClient == nil {
		t.Skip("No auth client available")
	}

	ctx := context.Background()
	graphName := fmt.Sprintf("test_deep_ins_%d", time.Now().UnixMilli())
	cfg := &gqldb.QueryConfig{GraphName: graphName}

	// Setup
	testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "")
	time.Sleep(500 * time.Millisecond)
	defer func() {
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, graphName, true)
	}()

	clean := func() {
		testClient.Gql(ctx, "MATCH (n) DETACH DELETE n", cfg)
	}

	insertAndVerify := func(t *testing.T, label, props string) {
		t.Helper()
		_, err := testClient.Gql(ctx, fmt.Sprintf("INSERT (:%s {%s})", label, props), cfg)
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
		resp, err := testClient.Gql(ctx, fmt.Sprintf("MATCH (n:%s) RETURN count(n) AS cnt", label), cfg)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if resp.RowCount < 1 {
			t.Fatal("No rows returned")
		}
	}

	// Numeric types
	t.Run("Int32Positive", func(t *testing.T) { clean(); insertAndVerify(t, "IntTest", "_id: 'i1', val: 42") })
	t.Run("Int32Zero", func(t *testing.T) { clean(); insertAndVerify(t, "IntTest", "_id: 'i2', val: 0") })
	t.Run("Int32Negative", func(t *testing.T) { clean(); insertAndVerify(t, "IntTest", "_id: 'i3', val: -100") })
	t.Run("Int32Max", func(t *testing.T) { clean(); insertAndVerify(t, "IntTest", "_id: 'i4', val: 2147483647") })
	t.Run("Int64Large", func(t *testing.T) { clean(); insertAndVerify(t, "IntTest", "_id: 'i5', val: 9999999999999") })
	t.Run("FloatPositive", func(t *testing.T) { clean(); insertAndVerify(t, "FloatTest", "_id: 'f1', val: 3.14") })
	t.Run("FloatNegative", func(t *testing.T) { clean(); insertAndVerify(t, "FloatTest", "_id: 'f2', val: -1.5") })
	t.Run("DoublePrecision", func(t *testing.T) { clean(); insertAndVerify(t, "FloatTest", "_id: 'f3', val: 3.141592653589793") })

	// Boolean
	t.Run("BoolTrue", func(t *testing.T) { clean(); insertAndVerify(t, "BoolTest", "_id: 'b1', val: true") })
	t.Run("BoolFalse", func(t *testing.T) { clean(); insertAndVerify(t, "BoolTest", "_id: 'b2', val: false") })

	// String
	t.Run("StringNormal", func(t *testing.T) { clean(); insertAndVerify(t, "StrTest", "_id: 's1', val: 'hello'") })
	t.Run("StringEmpty", func(t *testing.T) { clean(); insertAndVerify(t, "StrTest", "_id: 's2', val: ''") })
	t.Run("StringQuote", func(t *testing.T) { clean(); insertAndVerify(t, "StrTest", `_id: 's3', val: 'it\'s a test'`) })
	t.Run("StringUnicode", func(t *testing.T) { clean(); insertAndVerify(t, "StrTest", "_id: 's4', val: '你好世界'") })
	t.Run("StringEmoji", func(t *testing.T) { clean(); insertAndVerify(t, "StrTest", "_id: 's5', val: '🚀🎉'") })
	t.Run("StringLong", func(t *testing.T) {
		clean()
		s := strings.Repeat("x", 1000)
		insertAndVerify(t, "StrTest", fmt.Sprintf("_id: 's6', val: '%s'", s))
	})

	// NULL
	t.Run("NullValue", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:NullTest {_id: 'n1', name: 'test', val: NULL})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:NullTest) WHERE id(n) = 'n1' RETURN n.val AS val", cfg)
		if resp.RowCount != 1 {
			t.Fatalf("Expected 1 row, got %d", resp.RowCount)
		}
	})

	// List/Map
	t.Run("ListInt", func(t *testing.T) { clean(); insertAndVerify(t, "ListTest", "_id: 'l1', val: [1, 2, 3]") })
	t.Run("ListStr", func(t *testing.T) { clean(); insertAndVerify(t, "ListTest", "_id: 'l2', val: ['a', 'b']") })
	t.Run("ListEmpty", func(t *testing.T) { clean(); insertAndVerify(t, "ListTest", "_id: 'l3', val: []") })
	t.Run("ListNested", func(t *testing.T) { clean(); insertAndVerify(t, "ListTest", "_id: 'l4', val: [[1,2],[3,4]]") })
	t.Run("MapSimple", func(t *testing.T) { clean(); insertAndVerify(t, "MapTest", "_id: 'm1', val: {key: 'v', num: 42}") })

	// INSERT OVERWRITE
	t.Run("OverwriteString", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:OW {_id: 'ow1', val: 'old'})", cfg)
		testClient.Gql(ctx, "INSERT OVERWRITE (:OW {_id: 'ow1', val: 'new'})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:OW) WHERE id(n) = 'ow1' RETURN n.val AS val", cfg)
		val, _ := resp.GetByName(resp.Rows[0], "val")
		if fmt.Sprintf("%v", val) != "new" {
			t.Fatalf("Expected 'new', got '%v'", val)
		}
	})

	t.Run("OverwriteNoDuplicate", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:OW {_id: 'ow2', val: 1})", cfg)
		testClient.Gql(ctx, "INSERT OVERWRITE (:OW {_id: 'ow2', val: 2})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:OW) WHERE id(n) = 'ow2' RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "1" {
			t.Fatalf("Expected 1, got %v", cnt)
		}
	})

	t.Run("DuplicateIdFails", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Dup {_id: 'dup1', name: 'first'})", cfg)
		_, err := testClient.Gql(ctx, "INSERT (:Dup {_id: 'dup1', name: 'second'})", cfg)
		if err == nil {
			t.Fatal("Expected error for duplicate _id")
		}
	})

	// Edge inserts
	t.Run("EdgeIntProp", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:P {_id: 'ea'}), (:P {_id: 'eb'})", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='ea'), (b WHERE id(b)='eb') INSERT (a)-[:R {val: 42}]->(b)", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH ()-[e:R]->() RETURN e.val", cfg)
		if resp.RowCount != 1 {
			t.Fatalf("Expected 1 edge, got %d", resp.RowCount)
		}
	})

	t.Run("EdgeSelfLoop", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:P {_id: 'self1'})", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='self1') INSERT (a)-[:SELF]->(a)", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (a)-[e:SELF]->(b) WHERE id(a)='self1' RETURN e", cfg)
		if resp.RowCount != 1 {
			t.Fatalf("Expected 1 self-loop, got %d", resp.RowCount)
		}
	})

	// Batch
	t.Run("Batch50", func(t *testing.T) {
		clean()
		parts := make([]string, 50)
		for i := 0; i < 50; i++ {
			parts[i] = fmt.Sprintf("(:Batch {_id: 'b%d', idx: %d})", i, i)
		}
		testClient.Gql(ctx, "INSERT "+strings.Join(parts, ", "), cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:Batch) RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "50" {
			t.Fatalf("Expected 50, got %v", cnt)
		}
	})
}
