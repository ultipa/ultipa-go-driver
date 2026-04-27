//go:build integration

package integration

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestDeepDDLLifecycle(t *testing.T) {
	if testClient == nil {
		t.Skip("No auth client available")
	}

	ctx := context.Background()
	prefix := fmt.Sprintf("test_ddl_%d", time.Now().UnixMilli())

	defer func() {
		testClient.UseGraph(ctx, "default")
		for _, suffix := range []string{"_open", "_closed", "_renamed", "_trunc", "_lbl", "_elbl", "_ren", "_prop", "_idx", "_eidx", "_ft", "_cst", "_uni"} {
			testClient.DropGraph(ctx, prefix+suffix, true)
		}
	}()

	t.Run("GraphCreateOpenDrop", func(t *testing.T) {
		g := prefix + "_open"
		testClient.CreateGraph(ctx, g, gqldb.GraphTypeOpen, "")
		time.Sleep(500 * time.Millisecond)
		resp, err := testClient.Gql(ctx, "SHOW GRAPHS", nil)
		if err != nil {
			t.Fatalf("SHOW GRAPHS failed: %v", err)
		}
		found := false
		for _, row := range resp.Rows {
			name, _ := resp.GetByName(row, "graph_name")
			if fmt.Sprintf("%v", name) == g {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("Graph should exist")
		}
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g, true)
	})

	t.Run("GraphCreateClosedDrop", func(t *testing.T) {
		g := prefix + "_closed"
		testClient.Gql(ctx, "CREATE GRAPH "+g+" {}", nil)
		time.Sleep(500 * time.Millisecond)
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g, true)
	})

	t.Run("GraphIfNotExists", func(t *testing.T) {
		g := prefix + "_open"
		testClient.CreateGraph(ctx, g, gqldb.GraphTypeOpen, "")
		time.Sleep(500 * time.Millisecond)
		testClient.Gql(ctx, "CREATE GRAPH IF NOT EXISTS "+g, nil)
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g, true)
	})

	t.Run("GraphDuplicateError", func(t *testing.T) {
		g := prefix + "_open"
		testClient.CreateGraph(ctx, g, gqldb.GraphTypeOpen, "")
		time.Sleep(500 * time.Millisecond)
		_, err := testClient.Gql(ctx, "CREATE GRAPH "+g, nil)
		if err == nil {
			t.Fatal("Expected error for duplicate graph")
		}
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g, true)
	})

	t.Run("GraphRename", func(t *testing.T) {
		g1 := prefix + "_open"
		g2 := prefix + "_renamed"
		testClient.DropGraph(ctx, g2, true)
		testClient.CreateGraph(ctx, g1, gqldb.GraphTypeOpen, "")
		time.Sleep(500 * time.Millisecond)
		testClient.Gql(ctx, "ALTER GRAPH "+g1+" RENAME TO "+g2, nil)
		resp, _ := testClient.Gql(ctx, "SHOW GRAPHS", nil)
		foundNew, foundOld := false, false
		for _, row := range resp.Rows {
			name, _ := resp.GetByName(row, "graph_name")
			s := fmt.Sprintf("%v", name)
			if s == g2 {
				foundNew = true
			}
			if s == g1 {
				foundOld = true
			}
		}
		if !foundNew {
			t.Fatal("Expected new graph name")
		}
		if foundOld {
			t.Fatal("Old graph name should not exist")
		}
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g2, true)
	})

	t.Run("GraphDropNonexistent", func(t *testing.T) {
		_, err := testClient.Gql(ctx, "DROP GRAPH nonexistent_xyz_999", nil)
		if err == nil {
			t.Fatal("Expected error")
		}
	})

	t.Run("LabelCreateNode", func(t *testing.T) {
		g := prefix + "_lbl"
		testClient.DropGraph(ctx, g, true)
		testClient.Gql(ctx, "CREATE GRAPH "+g+" {}", nil)
		time.Sleep(500 * time.Millisecond)
		cfg := &gqldb.QueryConfig{GraphName: g}
		testClient.Gql(ctx, "ALTER GRAPH "+g+" ADD NODE { Person ({name STRING, age INT64}) }", cfg)
		testClient.Gql(ctx, "INSERT (:Person {name: 'test', age: 1})", cfg)
		resp, _ := testClient.Gql(ctx, "SHOW NODE LABELS", cfg)
		if resp.RowCount < 1 {
			t.Fatal("Expected at least 1 label")
		}
		testClient.Gql(ctx, "MATCH (n:Person) DELETE n", cfg)
		testClient.Gql(ctx, "ALTER GRAPH "+g+" DROP NODE Person", cfg)
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g, true)
	})

	t.Run("LabelRename", func(t *testing.T) {
		g := prefix + "_ren"
		testClient.DropGraph(ctx, g, true)
		testClient.CreateGraph(ctx, g, gqldb.GraphTypeOpen, "")
		time.Sleep(500 * time.Millisecond)
		cfg := &gqldb.QueryConfig{GraphName: g}
		oldLbl := fmt.Sprintf("OldLbl%d", rand.Intn(900)+100)
		newLbl := fmt.Sprintf("NewLbl%d", rand.Intn(900)+100)
		testClient.Gql(ctx, "INSERT (:"+oldLbl+" {name: 'test'})", cfg)
		testClient.Gql(ctx, "ALTER NODE "+oldLbl+" RENAME TO "+newLbl, cfg)
		resp, err := testClient.Gql(ctx, "SHOW NODE LABELS", cfg)
		if err != nil || resp == nil {
			t.Fatalf("SHOW NODE LABELS failed (resp=%v err=%v)", resp, err)
		}
		foundNew := false
		if len(resp.Columns) > 0 {
			col0 := resp.Columns[0]
			for _, row := range resp.Rows {
				v, _ := resp.GetByName(row, col0)
				if strings.Contains(fmt.Sprintf("%v", v), newLbl) {
					foundNew = true
					break
				}
			}
		}
		if !foundNew {
			t.Fatalf("%s should exist after rename", newLbl)
		}
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g, true)
	})

	t.Run("IndexLifecycle", func(t *testing.T) {
		g := prefix + "_idx"
		testClient.DropGraph(ctx, g, true)
		testClient.CreateGraph(ctx, g, gqldb.GraphTypeOpen, "")
		time.Sleep(500 * time.Millisecond)
		cfg := &gqldb.QueryConfig{GraphName: g}
		testClient.Gql(ctx, "INSERT (:IdxTest {name: 'test', age: 30})", cfg)
		testClient.Gql(ctx, "CREATE INDEX idx_name ON NODE IdxTest (name)", cfg)
		time.Sleep(1000 * time.Millisecond)
		resp, _ := testClient.Gql(ctx, "SHOW NODE INDEX", cfg)
		if resp.RowCount < 1 {
			t.Fatal("Expected at least 1 index")
		}
		testClient.Gql(ctx, "DROP NODE INDEX idx_name", cfg)
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g, true)
	})

	t.Run("IndexDuplicateError", func(t *testing.T) {
		g := prefix + "_idx"
		testClient.DropGraph(ctx, g, true)
		testClient.CreateGraph(ctx, g, gqldb.GraphTypeOpen, "")
		time.Sleep(500 * time.Millisecond)
		cfg := &gqldb.QueryConfig{GraphName: g}
		testClient.Gql(ctx, "INSERT (:IdxTest {name: 'test'})", cfg)
		testClient.Gql(ctx, "CREATE INDEX idx_dup ON NODE IdxTest (name)", cfg)
		time.Sleep(1000 * time.Millisecond)
		_, err := testClient.Gql(ctx, "CREATE INDEX idx_dup ON NODE IdxTest (name)", cfg)
		if err == nil {
			t.Fatal("Expected error for duplicate index")
		}
		testClient.Gql(ctx, "DROP NODE INDEX idx_dup", cfg)
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g, true)
	})

	t.Run("FulltextLifecycle", func(t *testing.T) {
		g := prefix + "_ft"
		testClient.DropGraph(ctx, g, true)
		testClient.CreateGraph(ctx, g, gqldb.GraphTypeOpen, "")
		time.Sleep(500 * time.Millisecond)
		cfg := &gqldb.QueryConfig{GraphName: g}
		testClient.Gql(ctx, "INSERT (:FtTest {title: 'hello world'})", cfg)
		testClient.Gql(ctx, "CREATE FULLTEXT ft_title ON NODE FtTest (title)", cfg)
		time.Sleep(2000 * time.Millisecond)
		resp, _ := testClient.Gql(ctx, "SHOW NODE FULLTEXT", cfg)
		if resp.RowCount < 1 {
			t.Fatal("Expected at least 1 fulltext")
		}
		testClient.Gql(ctx, "DROP NODE FULLTEXT ft_title", cfg)
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g, true)
	})

	t.Run("ConstraintNotNull", func(t *testing.T) {
		g := prefix + "_cst"
		testClient.DropGraph(ctx, g, true)
		testClient.Gql(ctx, "CREATE GRAPH "+g+" {}", nil)
		time.Sleep(500 * time.Millisecond)
		cfg := &gqldb.QueryConfig{GraphName: g}
		testClient.Gql(ctx, "ALTER GRAPH "+g+" ADD NODE { CstTest ({name STRING, age INT64}) }", cfg)
		testClient.Gql(ctx, "ALTER NODE CstTest ADD CONSTRAINT NOT NULL ON name", cfg)
		testClient.Gql(ctx, "INSERT (:CstTest {name: 'valid', age: 30})", cfg)
		_, err := testClient.Gql(ctx, "INSERT (:CstTest {age: 25})", cfg)
		if err == nil {
			t.Fatal("Expected error for NOT NULL violation")
		}
		testClient.Gql(ctx, "ALTER NODE CstTest DROP CONSTRAINT NOT NULL ON name", cfg)
		testClient.Gql(ctx, "INSERT (:CstTest {age: 20})", cfg)
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g, true)
	})

	t.Run("ConstraintUnique", func(t *testing.T) {
		g := prefix + "_uni"
		testClient.DropGraph(ctx, g, true)
		testClient.Gql(ctx, "CREATE GRAPH "+g+" {}", nil)
		time.Sleep(500 * time.Millisecond)
		cfg := &gqldb.QueryConfig{GraphName: g}
		testClient.Gql(ctx, "ALTER GRAPH "+g+" ADD NODE { UniTest ({email STRING}) }", cfg)
		testClient.Gql(ctx, "ALTER NODE UniTest ADD CONSTRAINT UNIQUE ON email", cfg)
		testClient.Gql(ctx, "INSERT (:UniTest {email: 'a@test.com'})", cfg)
		_, err := testClient.Gql(ctx, "INSERT (:UniTest {email: 'a@test.com'})", cfg)
		if err == nil {
			t.Fatal("Expected error for UNIQUE violation")
		}
		testClient.Gql(ctx, "ALTER NODE UniTest DROP CONSTRAINT UNIQUE ON email", cfg)
		testClient.Gql(ctx, "INSERT (:UniTest {email: 'a@test.com'})", cfg)
		testClient.UseGraph(ctx, "default")
		testClient.DropGraph(ctx, g, true)
	})
}
