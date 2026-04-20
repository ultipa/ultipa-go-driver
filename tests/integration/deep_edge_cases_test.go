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

	t.Run("LabelDot", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:`my.label` {name: 'test'})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:`my.label`) RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "1" {
			t.Fatalf("Expected 1, got %v", cnt)
		}
	})

	t.Run("EdgeLabelHyphen", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:N {_id: 'sl1'}), (:N {_id: 'sl2'})", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='sl1'), (b WHERE id(b)='sl2') INSERT (a)-[:`has-relation`]->(b)", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH ()-[e:`has-relation`]->() RETURN count(e) AS cnt", cfg)
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
			t.Fatalf("Expected 'value', got %v", val)
		}
	})

	t.Run("LongString10k", func(t *testing.T) {
		clean()
		s := strings.Repeat("x", 10000)
		testClient.Gql(ctx, fmt.Sprintf("INSERT (:LongStr {_id: 'ls1', val: '%s'})", s), cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:LongStr) WHERE id(n) = 'ls1' RETURN n.val AS val", cfg)
		val, _ := resp.GetByName(resp.Rows[0], "val")
		if len(fmt.Sprintf("%v", val)) != 10000 {
			t.Fatalf("Expected length 10000, got %d", len(fmt.Sprintf("%v", val)))
		}
	})

	t.Run("ManyProperties", func(t *testing.T) {
		clean()
		var props strings.Builder
		props.WriteString("_id: 'mp1'")
		for i := 0; i < 20; i++ {
			props.WriteString(fmt.Sprintf(", p%d: %d", i, i))
		}
		testClient.Gql(ctx, "INSERT (:ManyProps {"+props.String()+"})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:ManyProps) WHERE id(n) = 'mp1' RETURN n.p0, n.p19", cfg)
		if resp.RowCount != 1 {
			t.Fatalf("Expected 1, got %d", resp.RowCount)
		}
	})

	t.Run("Batch200Nodes", func(t *testing.T) {
		clean()
		var sb strings.Builder
		sb.WriteString("INSERT ")
		for i := 0; i < 200; i++ {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("(:Batch200 {_id: 'b200_%d', idx: %d})", i, i))
		}
		testClient.Gql(ctx, sb.String(), cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:Batch200) RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "200" {
			t.Fatalf("Expected 200, got %v", cnt)
		}
	})

	t.Run("SkipBeyond", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:SkipTest {val: 1}), (:SkipTest {val: 2})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:SkipTest) RETURN n.val SKIP 100", cfg)
		if resp.RowCount != 0 {
			t.Fatalf("Expected 0, got %d", resp.RowCount)
		}
	})

	t.Run("SyntaxError", func(t *testing.T) {
		_, err := testClient.Gql(ctx, "INVALID GQL STATEMENT", cfg)
		if err == nil {
			t.Fatal("Expected error")
		}
	})

	t.Run("NonexistentLabel", func(t *testing.T) {
		resp, _ := testClient.Gql(ctx, "MATCH (n:NonExistentLabel999) RETURN count(n) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "0" {
			t.Fatalf("Expected 0, got %v", cnt)
		}
	})

	t.Run("NonexistentProperty", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:ErrTest {name: 'test'})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:ErrTest) RETURN n.nonexistent_prop AS val", cfg)
		if resp.RowCount != 1 {
			t.Fatalf("Expected 1, got %d", resp.RowCount)
		}
	})

	t.Run("EmptyGql", func(t *testing.T) {
		_, err := testClient.Gql(ctx, "", cfg)
		if err == nil {
			t.Fatal("Expected error")
		}
	})

	t.Run("UseNonexistentGraph", func(t *testing.T) {
		_, err := testClient.Gql(ctx, "USE GRAPH nonexistent_xyz_999", cfg)
		if err == nil {
			t.Fatal("Expected error")
		}
	})

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

	t.Run("BidirectionalEdge", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:N {_id: 'bi1', name: 'A'}), (:N {_id: 'bi2', name: 'B'})", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='bi1'), (b WHERE id(b)='bi2') INSERT (a)-[:LINK]->(b)", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='bi2'), (b WHERE id(b)='bi1') INSERT (a)-[:LINK]->(b)", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (a:N)-[e:LINK]->(b:N) RETURN count(e) AS cnt", cfg)
		cnt, _ := resp.GetByName(resp.Rows[0], "cnt")
		if fmt.Sprintf("%v", cnt) != "2" {
			t.Fatalf("Expected 2, got %v", cnt)
		}
	})

	t.Run("CaseExpression", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Score {_id: 'sc1', val: 95}), (:Score {_id: 'sc2', val: 70}), (:Score {_id: 'sc3', val: 40})", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (n:Score) RETURN n.val AS score, CASE WHEN n.val >= 90 THEN 'A' WHEN n.val >= 60 THEN 'B' ELSE 'C' END AS grade ORDER BY score DESC", cfg)
		if resp.RowCount != 3 {
			t.Fatalf("Expected 3, got %d", resp.RowCount)
		}
	})

	t.Run("ExistsSubquery", func(t *testing.T) {
		clean()
		testClient.Gql(ctx, "INSERT (:Author {_id: 'a1', name: 'Writer'}), (:Author {_id: 'a2', name: 'NoBooks'}), (:Book {_id: 'b1', title: 'Book1'})", cfg)
		testClient.Gql(ctx, "MATCH (a WHERE id(a)='a1'), (b WHERE id(b)='b1') INSERT (a)-[:WROTE]->(b)", cfg)
		resp, _ := testClient.Gql(ctx, "MATCH (a:Author) WHERE EXISTS { MATCH (a)-[:WROTE]->(:Book) } RETURN a.name AS name", cfg)
		if resp.RowCount != 1 {
			t.Fatalf("Expected 1, got %d", resp.RowCount)
		}
		val, _ := resp.GetByName(resp.Rows[0], "name")
		if fmt.Sprintf("%v", val) != "Writer" {
			t.Fatalf("Expected Writer, got %v", val)
		}
	})
}
