package unit

import (
	"math"
	"strings"
	"testing"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// TestNumericBoundaries tests edge cases for numeric types.
func TestNumericBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		wantType string
	}{
		// Int64 boundaries
		{"int64_max", int64(math.MaxInt64), "int64"},
		{"int64_min", int64(math.MinInt64), "int64"},
		{"int64_zero", int64(0), "int64"},

		// Int32 boundaries
		{"int32_max", int32(math.MaxInt32), "int32"},
		{"int32_min", int32(math.MinInt32), "int32"},
		{"int32_zero", int32(0), "int32"},

		// Float64 boundaries
		{"float64_max", math.MaxFloat64, "float64"},
		{"float64_min", -math.MaxFloat64, "float64"},
		{"float64_smallest", math.SmallestNonzeroFloat64, "float64"},
		{"float64_zero", float64(0), "float64"},
		{"float64_negative_zero", math.Copysign(0, -1), "float64"},
		{"float64_positive_infinity", math.Inf(1), "float64"},
		{"float64_negative_infinity", math.Inf(-1), "float64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.value)
			if err != nil {
				t.Errorf("NewTypedValue(%v) returned error: %v", tt.value, err)
				return
			}
			if tv == nil {
				t.Errorf("NewTypedValue(%v) returned nil", tt.value)
				return
			}

			// Verify value can be retrieved
			retrieved, err := tv.ToGo()
			if err != nil {
				t.Errorf("ToGo() returned error: %v", err)
				return
			}
			if retrieved == nil && tt.value != nil {
				t.Errorf("ToGo() returned nil for %v", tt.value)
			}
		})
	}
}

// TestStringBoundaries tests edge cases for string values.
func TestStringBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"empty_string", ""},
		{"single_char", "a"},
		{"whitespace_only", "   "},
		{"newlines", "\n\n\n"},
		{"tabs", "\t\t\t"},
		{"mixed_whitespace", " \t\n\r "},

		// Unicode edge cases
		{"unicode_emoji", "\U0001F600\U0001F601\U0001F602"}, // 😀😁😂
		{"unicode_chinese", "中文测试"},
		{"unicode_arabic", "اختبار"},
		{"unicode_hebrew", "בדיקה"},
		{"unicode_combining", "e\u0301"}, // é using combining character
		{"unicode_zero_width", "a\u200Bb"},
		{"unicode_rtl", "\u202Bhello\u202C"},
		{"unicode_surrogate_pair", "\U0001F4A9"}, // 💩

		// Special characters
		{"null_byte", "hello\x00world"},
		{"control_chars", "\x01\x02\x03"},
		{"quotes", `"quoted"'single'`},
		{"backslashes", `path\\to\\file`},
		{"html_entities", "<script>alert('xss')</script>"},
		{"sql_injection", "'; DROP TABLE users; --"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.value)
			if err != nil {
				t.Errorf("NewTypedValue(%q) returned error: %v", tt.value, err)
				return
			}
			if tv == nil {
				t.Errorf("NewTypedValue(%q) returned nil", tt.value)
				return
			}

			retrieved, err := tv.ToGo()
			if err != nil {
				t.Errorf("ToGo() returned error: %v", err)
				return
			}
			if retrieved != tt.value {
				t.Errorf("ToGo() = %q, want %q", retrieved, tt.value)
			}
		})
	}
}

// TestLargeStrings tests handling of large string values.
func TestLargeStrings(t *testing.T) {
	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			largeString := strings.Repeat("a", sz.size)
			tv, err := gqldb.NewTypedValue(largeString)
			if err != nil {
				t.Errorf("NewTypedValue for %s string returned error: %v", sz.name, err)
				return
			}
			if tv == nil {
				t.Errorf("NewTypedValue for %s string returned nil", sz.name)
				return
			}

			retrieved, err := tv.ToGo()
			if err != nil {
				t.Errorf("ToGo() returned error: %v", err)
				return
			}
			if str, ok := retrieved.(string); ok {
				if len(str) != sz.size {
					t.Errorf("String length = %d, want %d", len(str), sz.size)
				}
			} else {
				t.Errorf("ToGo() returned non-string type: %T", retrieved)
			}
		})
	}
}

// TestCollectionBoundaries tests edge cases for collections.
func TestCollectionBoundaries(t *testing.T) {
	t.Run("empty_list", func(t *testing.T) {
		tv, err := gqldb.NewTypedValue([]interface{}{})
		if err != nil {
			t.Errorf("NewTypedValue for empty list returned error: %v", err)
			return
		}
		if tv == nil {
			t.Error("NewTypedValue for empty list returned nil")
			return
		}
		retrieved, err := tv.ToGo()
		if err != nil {
			t.Errorf("ToGo() returned error: %v", err)
			return
		}
		if list, ok := retrieved.([]interface{}); ok {
			if len(list) != 0 {
				t.Errorf("List length = %d, want 0", len(list))
			}
		}
	})

	t.Run("empty_map", func(t *testing.T) {
		tv, err := gqldb.NewTypedValue(map[string]interface{}{})
		if err != nil {
			t.Errorf("NewTypedValue for empty map returned error: %v", err)
			return
		}
		if tv == nil {
			t.Error("NewTypedValue for empty map returned nil")
			return
		}
		retrieved, err := tv.ToGo()
		if err != nil {
			t.Errorf("ToGo() returned error: %v", err)
			return
		}
		if m, ok := retrieved.(map[string]interface{}); ok {
			if len(m) != 0 {
				t.Errorf("Map length = %d, want 0", len(m))
			}
		}
	})

	t.Run("single_element_list", func(t *testing.T) {
		tv, err := gqldb.NewTypedValue([]interface{}{42})
		if err != nil {
			t.Errorf("NewTypedValue for single-element list returned error: %v", err)
			return
		}
		if tv == nil {
			t.Error("NewTypedValue for single-element list returned nil")
			return
		}
		retrieved, err := tv.ToGo()
		if err != nil {
			t.Errorf("ToGo() returned error: %v", err)
			return
		}
		if list, ok := retrieved.([]interface{}); ok {
			if len(list) != 1 {
				t.Errorf("List length = %d, want 1", len(list))
			}
		}
	})

	t.Run("single_element_map", func(t *testing.T) {
		tv, err := gqldb.NewTypedValue(map[string]interface{}{"key": "value"})
		if err != nil {
			t.Errorf("NewTypedValue for single-element map returned error: %v", err)
			return
		}
		if tv == nil {
			t.Error("NewTypedValue for single-element map returned nil")
			return
		}
		retrieved, err := tv.ToGo()
		if err != nil {
			t.Errorf("ToGo() returned error: %v", err)
			return
		}
		if m, ok := retrieved.(map[string]interface{}); ok {
			if len(m) != 1 {
				t.Errorf("Map length = %d, want 1", len(m))
			}
		}
	})

	t.Run("mixed_type_list", func(t *testing.T) {
		values := []interface{}{
			int64(42),
			"string",
			true,
			3.14,
			nil,
			[]interface{}{1, 2, 3},
			map[string]interface{}{"nested": "map"},
		}
		tv, err := gqldb.NewTypedValue(values)
		if err != nil {
			t.Errorf("NewTypedValue for mixed-type list returned error: %v", err)
			return
		}
		if tv == nil {
			t.Error("NewTypedValue for mixed-type list returned nil")
			return
		}
		retrieved, err := tv.ToGo()
		if err != nil {
			t.Errorf("ToGo() returned error: %v", err)
			return
		}
		if list, ok := retrieved.([]interface{}); ok {
			if len(list) != len(values) {
				t.Errorf("List length = %d, want %d", len(list), len(values))
			}
		}
	})

	t.Run("large_list", func(t *testing.T) {
		size := 10000
		values := make([]interface{}, size)
		for i := 0; i < size; i++ {
			values[i] = int64(i)
		}
		tv, err := gqldb.NewTypedValue(values)
		if err != nil {
			t.Errorf("NewTypedValue for list of %d elements returned error: %v", size, err)
			return
		}
		if tv == nil {
			t.Errorf("NewTypedValue for list of %d elements returned nil", size)
			return
		}
		retrieved, err := tv.ToGo()
		if err != nil {
			t.Errorf("ToGo() returned error: %v", err)
			return
		}
		if list, ok := retrieved.([]interface{}); ok {
			if len(list) != size {
				t.Errorf("List length = %d, want %d", len(list), size)
			}
		}
	})

	t.Run("large_map", func(t *testing.T) {
		size := 10000
		values := make(map[string]interface{}, size)
		for i := 0; i < size; i++ {
			values[strings.Repeat("k", 10)+string(rune(i))] = int64(i)
		}
		tv, err := gqldb.NewTypedValue(values)
		if err != nil {
			t.Errorf("NewTypedValue for map of %d elements returned error: %v", size, err)
			return
		}
		if tv == nil {
			t.Errorf("NewTypedValue for map of %d elements returned nil", size)
			return
		}
	})
}

// TestNestedStructures tests deeply nested collections.
func TestNestedStructures(t *testing.T) {
	t.Run("deeply_nested_list", func(t *testing.T) {
		depth := 10
		var nested interface{} = int64(42)
		for i := 0; i < depth; i++ {
			nested = []interface{}{nested}
		}
		tv, err := gqldb.NewTypedValue(nested)
		if err != nil {
			t.Errorf("NewTypedValue for nested list (depth %d) returned error: %v", depth, err)
			return
		}
		if tv == nil {
			t.Errorf("NewTypedValue for nested list (depth %d) returned nil", depth)
		}
	})

	t.Run("deeply_nested_map", func(t *testing.T) {
		depth := 10
		var nested interface{} = int64(42)
		for i := 0; i < depth; i++ {
			nested = map[string]interface{}{"level": nested}
		}
		tv, err := gqldb.NewTypedValue(nested)
		if err != nil {
			t.Errorf("NewTypedValue for nested map (depth %d) returned error: %v", depth, err)
			return
		}
		if tv == nil {
			t.Errorf("NewTypedValue for nested map (depth %d) returned nil", depth)
		}
	})
}

// TestNullAndNilValues tests null/nil value handling.
func TestNullAndNilValues(t *testing.T) {
	t.Run("nil_value", func(t *testing.T) {
		tv, err := gqldb.NewTypedValue(nil)
		if err != nil {
			t.Errorf("NewTypedValue(nil) returned error: %v", err)
			return
		}
		if tv == nil {
			t.Error("NewTypedValue(nil) returned nil TypedValue")
			return
		}
		if !tv.IsNull {
			t.Error("IsNull should be true for nil value")
		}
	})

	t.Run("list_with_nil", func(t *testing.T) {
		values := []interface{}{1, nil, "three", nil}
		tv, err := gqldb.NewTypedValue(values)
		if err != nil {
			t.Errorf("NewTypedValue for list with nil returned error: %v", err)
			return
		}
		if tv == nil {
			t.Error("NewTypedValue for list with nil returned nil")
			return
		}
		retrieved, err := tv.ToGo()
		if err != nil {
			t.Errorf("ToGo() returned error: %v", err)
			return
		}
		if list, ok := retrieved.([]interface{}); ok {
			if len(list) != len(values) {
				t.Errorf("List length = %d, want %d", len(list), len(values))
			}
		}
	})

	t.Run("map_with_nil_value", func(t *testing.T) {
		values := map[string]interface{}{
			"key1": "value1",
			"key2": nil,
			"key3": "value3",
		}
		tv, err := gqldb.NewTypedValue(values)
		if err != nil {
			t.Errorf("NewTypedValue for map with nil value returned error: %v", err)
			return
		}
		if tv == nil {
			t.Error("NewTypedValue for map with nil value returned nil")
			return
		}
	})
}

// TestBytesBoundaries tests edge cases for byte arrays.
func TestBytesBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
	}{
		{"empty_bytes", []byte{}},
		{"single_byte", []byte{0}},
		{"null_bytes", []byte{0, 0, 0}},
		{"max_bytes", []byte{255, 255, 255}},
		{"binary_data", []byte{0x00, 0x7F, 0x80, 0xFF}},
		{"large_bytes", make([]byte, 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.value)
			if err != nil {
				t.Errorf("NewTypedValue for bytes returned error: %v", err)
				return
			}
			if tv == nil {
				t.Errorf("NewTypedValue for bytes returned nil")
				return
			}
			retrieved, err := tv.ToGo()
			if err != nil {
				t.Errorf("ToGo() returned error: %v", err)
				return
			}
			if bytes, ok := retrieved.([]byte); ok {
				if len(bytes) != len(tt.value) {
					t.Errorf("Bytes length = %d, want %d", len(bytes), len(tt.value))
				}
			}
		})
	}
}

// TestBooleanValues tests boolean value handling.
func TestBooleanValues(t *testing.T) {
	tests := []struct {
		name  string
		value bool
		want  bool
	}{
		{"true", true, true},
		{"false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.value)
			if err != nil {
				t.Errorf("NewTypedValue for bool returned error: %v", err)
				return
			}
			if tv == nil {
				t.Error("NewTypedValue for bool returned nil")
				return
			}
			retrieved, err := tv.ToGo()
			if err != nil {
				t.Errorf("ToGo() returned error: %v", err)
				return
			}
			if b, ok := retrieved.(bool); ok {
				if b != tt.want {
					t.Errorf("ToGo() = %v, want %v", b, tt.want)
				}
			}
		})
	}
}
