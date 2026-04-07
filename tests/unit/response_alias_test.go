//go:build unit

package unit

import (
	"testing"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// TestResponse_Alias_Success tests successful alias lookup.
func TestResponse_Alias_Success(t *testing.T) {
	columns := []string{"n1", "e", "n2"}
	rows := []*gqldb.Row{}

	resp := gqldb.NewResponse(columns, rows, 0, false, nil, 0)

	for _, colName := range columns {
		ar, err := resp.Alias(colName)
		if err != nil {
			t.Errorf("Alias(%s) failed: %v", colName, err)
			continue
		}

		if ar == nil {
			t.Errorf("Alias(%s) returned nil", colName)
		}
	}
}

// TestResponse_Alias_NotFound tests error when alias not found.
func TestResponse_Alias_NotFound(t *testing.T) {
	columns := []string{"n1", "e", "n2"}
	rows := []*gqldb.Row{}

	resp := gqldb.NewResponse(columns, rows, 0, false, nil, 0)

	ar, err := resp.Alias("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent alias, got nil")
	}
	if ar != nil {
		t.Error("Expected nil AliasResult for non-existent alias")
	}
}

// TestResponse_Get_Success tests successful get by index.
func TestResponse_Get_Success(t *testing.T) {
	columns := []string{"n1", "e", "n2"}
	rows := []*gqldb.Row{}

	resp := gqldb.NewResponse(columns, rows, 0, false, nil, 0)

	for i := range columns {
		ar, err := resp.Get(i)
		if err != nil {
			t.Errorf("Get(%d) failed: %v", i, err)
			continue
		}

		if ar == nil {
			t.Errorf("Get(%d) returned nil", i)
		}
	}
}

// TestResponse_Get_OutOfRange tests error when index out of range.
func TestResponse_Get_OutOfRange(t *testing.T) {
	columns := []string{"n1", "e", "n2"}
	rows := []*gqldb.Row{}

	resp := gqldb.NewResponse(columns, rows, 0, false, nil, 0)

	ar, err := resp.Get(-1)
	if err == nil {
		t.Error("Expected error for negative index, got nil")
	}
	if ar != nil {
		t.Error("Expected nil AliasResult for negative index")
	}

	ar, err = resp.Get(999)
	if err == nil {
		t.Error("Expected error for out-of-range index, got nil")
	}
	if ar != nil {
		t.Error("Expected nil AliasResult for out-of-range index")
	}
}

// TestAliasResult_AsNodes_WrongType tests type mismatch error.
func TestAliasResult_AsNodes_WrongType(t *testing.T) {
	columns := []string{"value"}

	tv1, _ := types.NewTypedValue("hello")
	rows := []*gqldb.Row{
		gqldb.NewRow([]*types.TypedValue{tv1}),
	}

	resp := gqldb.NewResponse(columns, rows, 1, false, nil, 0)

	ar, err := resp.Alias("value")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}

	_, _, err = ar.AsNodes()
	if err == nil {
		t.Error("Expected error when calling AsNodes on string column, got nil")
	}

	if err != nil && !stringContains(err.Error(), "type mismatch") {
		t.Errorf("Expected 'type mismatch' error, got: %v", err)
	}
}

// TestAliasResult_AsEdges_TypeMismatch tests type mismatch error.
func TestAliasResult_AsEdges_TypeMismatch(t *testing.T) {
	columns := []string{"value"}

	tv1, _ := types.NewTypedValue(int64(42))
	rows := []*gqldb.Row{
		gqldb.NewRow([]*types.TypedValue{tv1}),
	}

	resp := gqldb.NewResponse(columns, rows, 1, false, nil, 0)

	ar, err := resp.Alias("value")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}

	_, _, err = ar.AsEdges()
	if err == nil {
		t.Error("Expected error when calling AsEdges on int column, got nil")
	}

	if err != nil && !stringContains(err.Error(), "type mismatch") {
		t.Errorf("Expected 'type mismatch' error, got: %v", err)
	}
}

// TestAliasResult_AsPaths_TypeMismatch tests type mismatch error.
func TestAliasResult_AsPaths_TypeMismatch(t *testing.T) {
	columns := []string{"value"}

	tv1, _ := types.NewTypedValue(int64(42))
	rows := []*gqldb.Row{
		gqldb.NewRow([]*types.TypedValue{tv1}),
	}

	resp := gqldb.NewResponse(columns, rows, 1, false, nil, 0)

	ar, err := resp.Alias("value")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}

	_, err = ar.AsPaths()
	if err == nil {
		t.Error("Expected error when calling AsPaths on int column, got nil")
	}

	if err != nil && !stringContains(err.Error(), "type mismatch") {
		t.Errorf("Expected 'type mismatch' error, got: %v", err)
	}
}

// TestAliasResult_AsAttr tests extracting scalar values.
func TestAliasResult_AsAttr(t *testing.T) {
	columns := []string{"name", "age"}

	tv1, _ := types.NewTypedValue("Alice")
	tv2, _ := types.NewTypedValue(int64(30))
	rows := []*gqldb.Row{
		gqldb.NewRow([]*types.TypedValue{tv1, tv2}),
	}

	resp := gqldb.NewResponse(columns, rows, 1, false, nil, 0)

	ar, err := resp.Alias("name")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}

	attr, err := ar.AsAttr()
	if err != nil {
		t.Fatalf("AsAttr failed: %v", err)
	}

	if attr.Name != "name" {
		t.Errorf("Expected attr name 'name', got '%s'", attr.Name)
	}

	if len(attr.Values) != 1 {
		t.Errorf("Expected 1 value, got %d", len(attr.Values))
	}
}

// Helper function to check if string contains substring
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
