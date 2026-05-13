//go:build unit

package unit

import (
	"strings"
	"testing"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// Tests for Row's container protocol (driver #4 — Python-parity
// additions: row.GetByName, row.Has, row.Len, ColumnNames auto-injected
// by Response).

func mkRow(t *testing.T, values ...interface{}) *gqldb.Row {
	t.Helper()
	tvs := make([]*gqldb.TypedValue, len(values))
	for i, v := range values {
		tv, err := gqldb.NewTypedValue(v)
		if err != nil {
			t.Fatalf("NewTypedValue(%v) failed: %v", v, err)
		}
		tvs[i] = tv
	}
	return gqldb.NewRow(tvs)
}

func TestRow_GetByName_ReturnsDecodedValue(t *testing.T) {
	resp := gqldb.NewResponse(
		[]string{"name", "age", "active"},
		[]*gqldb.Row{mkRow(t, "Alice", int64(30), true)},
		1, false, nil, 0,
	)
	row := resp.Rows[0]

	got, err := row.GetByName("name")
	if err != nil {
		t.Fatalf("GetByName(name) error: %v", err)
	}
	if got != "Alice" {
		t.Errorf("GetByName(name) = %v, want Alice", got)
	}

	got, err = row.GetByName("age")
	if err != nil {
		t.Fatalf("GetByName(age) error: %v", err)
	}
	if got != int64(30) {
		t.Errorf("GetByName(age) = %v, want 30", got)
	}

	got, err = row.GetByName("active")
	if err != nil {
		t.Fatalf("GetByName(active) error: %v", err)
	}
	if got != true {
		t.Errorf("GetByName(active) = %v, want true", got)
	}
}

func TestRow_GetByName_ErrorsWhenColumnNamesNotPopulated(t *testing.T) {
	row := mkRow(t, "Alice")
	_, err := row.GetByName("name")
	if err == nil {
		t.Fatal("expected error when ColumnNames is nil")
	}
	if !strings.Contains(err.Error(), "not populated") {
		t.Errorf("error %q does not mention 'not populated'", err.Error())
	}
}

func TestRow_GetByName_ErrorsOnMissingColumn(t *testing.T) {
	resp := gqldb.NewResponse([]string{"name"}, []*gqldb.Row{mkRow(t, "Alice")}, 1, false, nil, 0)
	_, err := resp.Rows[0].GetByName("nope")
	if err == nil {
		t.Fatal("expected error for missing column")
	}
	if !strings.Contains(err.Error(), "column not found") {
		t.Errorf("error %q does not mention 'column not found'", err.Error())
	}
}

func TestRow_Has_ReportsColumnNameMembership(t *testing.T) {
	resp := gqldb.NewResponse([]string{"x", "y"}, []*gqldb.Row{mkRow(t, int64(1), int64(2))}, 1, false, nil, 0)
	r := resp.Rows[0]
	if !r.Has("x") {
		t.Error("Has(x) = false, want true")
	}
	if !r.Has("y") {
		t.Error("Has(y) = false, want true")
	}
	if r.Has("nope") {
		t.Error("Has(nope) = true, want false")
	}
	// Standalone row also reports false (not panic).
	if mkRow(t, int64(1)).Has("x") {
		t.Error("standalone Row.Has(x) = true, want false")
	}
}

func TestRow_Len_ReturnsColumnCount(t *testing.T) {
	r := mkRow(t, "a", int64(7), false)
	if got := r.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
}

func TestResponse_PropagatesColumns_IntoEveryRow(t *testing.T) {
	resp := gqldb.NewResponse(
		[]string{"a", "b"},
		[]*gqldb.Row{mkRow(t, int64(1), int64(2)), mkRow(t, int64(3), int64(4))},
		2, false, nil, 0,
	)
	for i, r := range resp.Rows {
		if got := r.ColumnNames; len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("row %d ColumnNames = %v, want [a b]", i, got)
		}
	}
}

func TestResponse_PropagateColumnNames_IsIdempotent(t *testing.T) {
	resp := gqldb.NewResponse([]string{"a"}, []*gqldb.Row{mkRow(t, int64(1))}, 1, false, nil, 0)
	// Call again — must not panic and must keep the names intact.
	resp.PropagateColumnNames()
	if resp.Rows[0].ColumnNames[0] != "a" {
		t.Errorf("ColumnNames lost after second propagation")
	}
}

// ---------- #11 Row.String() ----------

func TestRow_String_WithColumnNames(t *testing.T) {
	resp := gqldb.NewResponse(
		[]string{"id", "name", "age"},
		[]*gqldb.Row{mkRow(t, "alice", "Alice", int64(30))},
		1, false, nil, 0,
	)
	got := resp.Rows[0].String()
	want := "Row(id='alice', name='Alice', age=30)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestRow_String_WithoutColumnNames(t *testing.T) {
	r := mkRow(t, "alice", "Alice", int64(30))
	got := r.String()
	want := "Row('alice', 'Alice', 30)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// ---------- #12 Row.Equal ----------

func TestRow_Equal_SameValuesAndColumnNames(t *testing.T) {
	a := gqldb.NewResponse([]string{"id", "age"}, []*gqldb.Row{mkRow(t, "alice", int64(30))}, 1, false, nil, 0)
	b := gqldb.NewResponse([]string{"id", "age"}, []*gqldb.Row{mkRow(t, "alice", int64(30))}, 1, false, nil, 0)
	if !a.Rows[0].Equal(b.Rows[0]) {
		t.Error("equal rows reported not equal")
	}
}

func TestRow_Equal_ColumnNamesDiffer(t *testing.T) {
	a := gqldb.NewResponse([]string{"id", "age"}, []*gqldb.Row{mkRow(t, "alice", int64(30))}, 1, false, nil, 0)
	b := gqldb.NewResponse([]string{"name", "age"}, []*gqldb.Row{mkRow(t, "alice", int64(30))}, 1, false, nil, 0)
	if a.Rows[0].Equal(b.Rows[0]) {
		t.Error("rows with different column names should not be equal")
	}
}

func TestRow_Equal_ValueDiffers(t *testing.T) {
	a := gqldb.NewResponse([]string{"id", "age"}, []*gqldb.Row{mkRow(t, "alice", int64(30))}, 1, false, nil, 0)
	b := gqldb.NewResponse([]string{"id", "age"}, []*gqldb.Row{mkRow(t, "alice", int64(31))}, 1, false, nil, 0)
	if a.Rows[0].Equal(b.Rows[0]) {
		t.Error("rows with different values should not be equal")
	}
}

func TestRow_Equal_StandaloneRows(t *testing.T) {
	if !mkRow(t, "alice", int64(30)).Equal(mkRow(t, "alice", int64(30))) {
		t.Error("equal standalone rows reported not equal")
	}
	if mkRow(t, "alice", int64(30)).Equal(mkRow(t, "alice", int64(31))) {
		t.Error("unequal standalone rows reported equal")
	}
}
