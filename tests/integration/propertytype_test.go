//go:build integration

package integration

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestInsertNodesWithAllPropertyTypes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := "test_propertytype_" + time.Now().Format("20060102150405")

	// Create test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for PropertyType tests")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	t.Logf("Created test graph: %s", graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Test all supported PropertyTypes
	now := time.Now()
	nodes := []*gqldb.NodeData{
		{
			Labels: []string{"TestAllTypes"},
			Properties: map[string]interface{}{
				// Basic numeric types
				"prop_int32":   int32(42),
				"prop_uint32":  uint32(100),
				"prop_int64":   int64(9223372036854775807),
				"prop_uint64":  uint64(18446744073709551615),
				"prop_float32": float32(3.14),
				"prop_float64": float64(3.141592653589793),

				// String types
				"prop_string":  "Hello, World!",
				"prop_unicode": "你好世界 🌍",

				// Boolean and null
				"prop_bool_true":  true,
				"prop_bool_false": false,

				// Time types
				"prop_timestamp": now,

				// Binary data
				"prop_blob": []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},

				// Complex types
				"prop_list":        []interface{}{"a", "b", "c"},
				"prop_list_mixed":  []interface{}{int64(1), "two", float64(3.0)},
				"prop_map":         map[string]interface{}{"key1": "value1", "key2": int64(2)},
				"prop_nested_list": []interface{}{[]interface{}{int64(1), int64(2)}, []interface{}{int64(3), int64(4)}},
			},
		},
		{
			Labels: []string{"TestNumericEdgeCases"},
			Properties: map[string]interface{}{
				// Edge cases for numeric types
				"int32_min":  int32(-2147483648),
				"int32_max":  int32(2147483647),
				"int64_min":  int64(-9223372036854775808),
				"int64_max":  int64(9223372036854775807),
				"float_zero": float64(0.0),
				"float_neg":  float64(-123.456),
			},
		},
		{
			Labels: []string{"TestStringEdgeCases"},
			Properties: map[string]interface{}{
				"empty_string":  "",
				"special_chars": "!@#$%^&*()_+-=[]{}|;':\",./<>?",
				"newlines":      "line1\nline2\rline3",
				"long_string":   string(make([]byte, 1000)), // 1000 bytes of zeros
				"emoji_string":  "😀🎉🚀💻🌟",
				"japanese":      "日本語テスト",
				"arabic":        "اختبار عربي",
			},
		},
	}

	config := &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID}
	result, err := testClient.InsertNodesBatchAuto(ctx, graphName, nodes, config)
	if err != nil {
		t.Fatalf("InsertNodes with all PropertyTypes failed: %v", err)
	}


	if !result.Success {
		t.Errorf("InsertNodes returned success=false: %s", result.Message)
	}

	if result.NodeCount != 3 {
		t.Errorf("expected 3 nodes inserted, got %d", result.NodeCount)
	}

	t.Logf("Successfully inserted %d nodes with all PropertyTypes, IDs: %v", result.NodeCount, result.NodeIDs)

	// Verify by querying
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n:TestAllTypes) RETURN n", queryConfig)
	if err != nil {
		t.Logf("Query verification failed: %v", err)
	} else {
		t.Logf("Query returned %d rows", resp.RowCount)
	}
}

func TestInsertEdgesWithAllPropertyTypes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := "test_edge_propertytype_" + time.Now().Format("20060102150405")

	// Create test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for edge PropertyType tests")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	// Insert nodes first
	nodes := []*gqldb.NodeData{
		{Labels: []string{"Source"}, Properties: map[string]interface{}{"name": "SourceNode"}},
		{Labels: []string{"Target"}, Properties: map[string]interface{}{"name": "TargetNode"}},
	}

	nodeConfig := &gqldb.InsertNodesConfig{BulkImportSessionID: session.SessionID}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, nodeConfig)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}


	// Query to get node IDs (bulk import doesn't return IDs immediately)
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "MATCH (n) RETURN id(n) AS node_id ORDER BY n.name", queryConfig)
	if err != nil {
		t.Fatalf("Query for node IDs failed: %v", err)
	}
	if resp.RowCount < 2 {
		t.Fatalf("expected at least 2 nodes from query, got %d", resp.RowCount)
	}

	// Extract node IDs
	var nodeIDs []string
	for i := int64(0); i < resp.RowCount; i++ {
		row := resp.Rows[i]
		if value, err := resp.GetByName(row, "node_id"); err == nil {
			if nodeID, ok := value.(string); ok {
				nodeIDs = append(nodeIDs, nodeID)
			}
		}
	}
	if len(nodeIDs) < 2 {
		t.Fatalf("expected at least 2 node IDs from query, got %d", len(nodeIDs))
	}

	now := time.Now()

	// Insert edges with all property types
	edges := []*gqldb.EdgeData{
		{
			Label:      "HAS_ALL_TYPES",
			FromNodeID: nodeIDs[0],
			ToNodeID:   nodeIDs[1],
			Properties: map[string]interface{}{
				// Numeric types
				"edge_int32":   int32(100),
				"edge_int64":   int64(200),
				"edge_float64": float64(3.14159),

				// String types
				"edge_string":  "edge property",
				"edge_unicode": "边缘属性 🔗",

				// Boolean
				"edge_bool": true,

				// Time
				"edge_timestamp": now,

				// Binary
				"edge_blob": []byte{0xDE, 0xAD, 0xBE, 0xEF},

				// Complex types
				"edge_list": []interface{}{"x", "y", "z"},
				"edge_map":  map[string]interface{}{"weight": float64(1.5), "type": "strong"},
			},
		},
		{
			Label:      "NUMERIC_EDGE",
			FromNodeID: nodeIDs[0],
			ToNodeID:   nodeIDs[1],
			Properties: map[string]interface{}{
				"weight":   float64(0.75),
				"count":    int64(42),
				"distance": float64(1234.5678),
			},
		},
	}

	edgeConfig := &gqldb.InsertEdgesConfig{BulkImportSessionID: session.SessionID}
	edgeResult, err := testClient.InsertEdgesBatchAuto(ctx, graphName, edges, edgeConfig)
	if err != nil {
		t.Fatalf("InsertEdges with all PropertyTypes failed: %v", err)
	}


	if !edgeResult.Success {
		t.Errorf("InsertEdges returned success=false: %s", edgeResult.Message)
	}

	if edgeResult.EdgeCount != 2 {
		t.Errorf("expected 2 edges inserted, got %d", edgeResult.EdgeCount)
	}

	t.Logf("Successfully inserted %d edges with all PropertyTypes, IDs: %v", edgeResult.EdgeCount, edgeResult.EdgeIDs)
}

// ============================================================================
// Roundtrip Tests - Verify encode/decode correctness for all property types
// ============================================================================

// setupRoundtripTest creates a test graph and returns cleanup function
func setupRoundtripTest(t *testing.T, ctx context.Context, prefix string) (string, func()) {
	graphName := prefix + "_" + time.Now().Format("20060102150405")
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Roundtrip test graph")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	return graphName, func() {
		dropTestGraph(graphName)
	}
}

// insertAndQueryProperty inserts a node with a single property and queries it back
// If sessionID is empty, creates a new bulk import session
func insertAndQueryProperty(t *testing.T, ctx context.Context, graphName, propName string, propValue interface{}, sessionID string) interface{} {
	var err error
	var ownSession bool

	// Ensure session graph context points to the test graph
	_ = testClient.UseGraph(ctx, graphName)

	// Create session if not provided
	if sessionID == "" {
		session, err := testClient.StartBulkImport(ctx, graphName, nil)
		if err != nil {
			t.Fatalf("StartBulkImport failed: %v", err)
		}
		sessionID = session.SessionID
		ownSession = true
		defer func() {
			_, _ = testClient.EndBulkImport(ctx, sessionID)
		}()
	}

	// Insert node with the property
	nodes := []*gqldb.NodeData{
		{
			Labels:     []string{"RoundtripTest"},
			Properties: map[string]interface{}{propName: propValue},
		},
	}

	config := &gqldb.InsertNodesConfig{BulkImportSessionID: sessionID}
	result, err := testClient.InsertNodesBatchAuto(ctx, graphName, nodes, config)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("InsertNodes returned success=false")
	}


	// Query to get node ID (bulk import may not return IDs immediately, retry a few times)
	queryConfig := &gqldb.QueryConfig{GraphName: graphName}
	queryForID := "MATCH (n:RoundtripTest) WHERE n." + propName + " IS NOT NULL RETURN id(n) AS node_id ORDER BY id(n) DESC LIMIT 1"
	var respID *gqldb.Response
	for attempt := 0; attempt < 3; attempt++ {
		respID, err = testClient.Gql(ctx, queryForID, queryConfig)
		if err != nil {
			t.Fatalf("Query for node ID failed: %v", err)
		}
		if respID.RowCount > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if respID.RowCount == 0 {
		t.Fatalf("Query for node ID returned no rows after retries")
	}

	nodeIDValue, err := respID.GetByName(respID.Rows[0], "node_id")
	if err != nil {
		t.Fatalf("GetByName for node_id failed: %v", err)
	}
	nodeID, ok := nodeIDValue.(string)
	if !ok {
		t.Fatalf("node_id is not string, got %T", nodeIDValue)
	}

	// Query back the property (retry for eventual consistency)
	query := "MATCH (n:RoundtripTest) WHERE id(n) = '" + nodeID + "' RETURN n." + propName + " AS value"
	var resp *gqldb.Response
	for attempt := 0; attempt < 3; attempt++ {
		resp, err = testClient.Gql(ctx, query, queryConfig)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if resp.RowCount > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if resp.RowCount == 0 {
		t.Fatalf("Query returned no rows after retries")
	}

	value, err := resp.GetByName(resp.Rows[0], "value")
	if err != nil {
		t.Fatalf("GetByName failed: %v", err)
	}

	// Clean up own session if we created it
	_ = ownSession

	return value
}

// assertFloat64Equal compares two float64 values with tolerance
func assertFloat64Equal(t *testing.T, name string, expected, actual float64, tolerance float64) {
	if math.Abs(expected-actual) > tolerance {
		t.Errorf("%s: expected %v, got %v (tolerance: %v)", name, expected, actual, tolerance)
	}
}

// TestNumericTypesRoundtrip tests encode/decode for all numeric types
func TestNumericTypesRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	graphName, cleanup := setupRoundtripTest(t, ctx, "test_numeric_roundtrip")
	defer cleanup()

	// Start shared bulk import session for all subtests
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	t.Run("Int32", func(t *testing.T) {
		testCases := []struct {
			name  string
			value int32
		}{
			{"zero", 0},
			{"positive", 42},
			{"negative", -100},
			{"max", 2147483647},
			{"min", -2147483648},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "int32_"+tc.name, tc.value, session.SessionID)
				// Result may be int64 due to JSON/gRPC conversion
				var actual int32
				switch v := result.(type) {
				case int32:
					actual = v
				case int64:
					actual = int32(v)
				default:
					t.Fatalf("unexpected type %T for int32", result)
				}
				if actual != tc.value {
					t.Errorf("expected %d, got %d", tc.value, actual)
				}
			})
		}
	})

	t.Run("Int64", func(t *testing.T) {
		testCases := []struct {
			name  string
			value int64
		}{
			{"zero", 0},
			{"positive", 9223372036854775807},
			{"negative", -9223372036854775808},
			{"medium", 1234567890123},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "int64_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(int64)
				if !ok {
					t.Fatalf("expected int64, got %T", result)
				}
				if actual != tc.value {
					t.Errorf("expected %d, got %d", tc.value, actual)
				}
			})
		}
	})

	t.Run("UInt32", func(t *testing.T) {
		testCases := []struct {
			name  string
			value uint32
		}{
			{"zero", 0},
			{"positive", 100},
			{"large", 3000000000},
			{"max", 4294967295},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "uint32_"+tc.name, tc.value, session.SessionID)
				var actual uint32
				switch v := result.(type) {
				case uint32:
					actual = v
				case int64:
					actual = uint32(v)
				case uint64:
					actual = uint32(v)
				default:
					t.Fatalf("unexpected type %T for uint32", result)
				}
				if actual != tc.value {
					t.Errorf("expected %d, got %d", tc.value, actual)
				}
			})
		}
	})

	t.Run("UInt64", func(t *testing.T) {
		testCases := []struct {
			name  string
			value uint64
		}{
			{"zero", 0},
			{"positive", 100},
			{"beyond_int64", 9223372036854775808},
			{"max", 18446744073709551615},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "uint64_"+tc.name, tc.value, session.SessionID)
				var actual uint64
				switch v := result.(type) {
				case uint64:
					actual = v
				case int64:
					actual = uint64(v)
				default:
					t.Fatalf("unexpected type %T for uint64", result)
				}
				if actual != tc.value {
					t.Errorf("expected %d, got %d", tc.value, actual)
				}
			})
		}
	})

	t.Run("Float32", func(t *testing.T) {
		testCases := []struct {
			name  string
			value float32
		}{
			{"zero", 0.0},
			{"positive", 3.14},
			{"negative", -2.71},
			{"small", 1e-30},
			{"large", 1e30},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "float32_"+tc.name, tc.value, session.SessionID)
				var actual float64
				switch v := result.(type) {
				case float32:
					actual = float64(v)
				case float64:
					actual = v
				default:
					t.Fatalf("unexpected type %T for float32", result)
				}
				assertFloat64Equal(t, tc.name, float64(tc.value), actual, 1e-5)
			})
		}
	})

	t.Run("Float64", func(t *testing.T) {
		testCases := []struct {
			name  string
			value float64
		}{
			{"zero", 0.0},
			{"pi", 3.141592653589793},
			{"negative", -2.718281828459045},
			{"small", 1e-300},
			{"large", 1e300},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "float64_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(float64)
				if !ok {
					t.Fatalf("expected float64, got %T", result)
				}
				assertFloat64Equal(t, tc.name, tc.value, actual, 1e-10)
			})
		}
	})
}

// TestStringTypesRoundtrip tests encode/decode for string types
func TestStringTypesRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	graphName, cleanup := setupRoundtripTest(t, ctx, "test_string_roundtrip")
	defer cleanup()

	// Start shared bulk import session for all subtests
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	t.Run("String", func(t *testing.T) {
		testCases := []struct {
			name  string
			value string
		}{
			{"empty", ""},
			{"ascii", "Hello, World!"},
			{"unicode", "你好世界 🌍"},
			{"emoji", "😀🎉🚀💻🌟"},
			{"special", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
			{"newlines", "line1\nline2\rline3"},
			{"japanese", "日本語テスト"},
			{"arabic", "اختبار عربي"},
			{"korean", "한국어 테스트"},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "str_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(string)
				if !ok {
					t.Fatalf("expected string, got %T", result)
				}
				if actual != tc.value {
					t.Errorf("expected %q, got %q", tc.value, actual)
				}
			})
		}
	})

	t.Run("Blob", func(t *testing.T) {
		testCases := []struct {
			name  string
			value []byte
		}{
			{"empty", []byte{}},
			{"binary", []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}},
			{"deadbeef", []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "blob_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.([]byte)
				if !ok {
					t.Fatalf("expected []byte, got %T", result)
				}
				if !reflect.DeepEqual(actual, tc.value) {
					t.Errorf("expected %v, got %v", tc.value, actual)
				}
			})
		}
	})
}

// TestTemporalTypesRoundtrip tests encode/decode for temporal types
func TestTemporalTypesRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	graphName, cleanup := setupRoundtripTest(t, ctx, "test_temporal_roundtrip")
	defer cleanup()

	// Start shared bulk import session for all subtests
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	t.Run("Timestamp", func(t *testing.T) {
		testCases := []struct {
			name  string
			value time.Time
		}{
			{"now", time.Now().UTC().Truncate(time.Millisecond)},
			{"epoch", time.Unix(0, 0).UTC()},
			{"future", time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "ts_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(time.Time)
				if !ok {
					t.Fatalf("expected time.Time, got %T", result)
				}
				// Compare with millisecond precision
				if !actual.Truncate(time.Millisecond).Equal(tc.value.Truncate(time.Millisecond)) {
					t.Errorf("expected %v, got %v", tc.value, actual)
				}
			})
		}
	})

	t.Run("LocalDateTime", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.LocalDateTime
		}{
			{"standard", gqldb.LocalDateTime{Time: time.Date(2024, 6, 15, 14, 30, 45, 123456789, time.UTC)}},
			{"midnight", gqldb.LocalDateTime{Time: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)}},
			{"endOfDay", gqldb.LocalDateTime{Time: time.Date(2024, 6, 15, 23, 59, 59, 999999999, time.UTC)}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "ldt_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.LocalDateTime)
				if !ok {
					t.Fatalf("expected LocalDateTime, got %T", result)
				}
				if !actual.Time.Equal(tc.value.Time) {
					t.Errorf("expected %v, got %v", tc.value.Time, actual.Time)
				}
			})
		}
	})

	t.Run("ZonedDateTime", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.ZonedDateTime
		}{
			{"utc", gqldb.ZonedDateTime{Time: time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC), OffsetMinutes: 0}},
			{"est", gqldb.ZonedDateTime{Time: time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC), OffsetMinutes: -300}},
			{"ist", gqldb.ZonedDateTime{Time: time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC), OffsetMinutes: 330}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "zdt_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.ZonedDateTime)
				if !ok {
					t.Fatalf("expected ZonedDateTime, got %T", result)
				}
				if actual.OffsetMinutes != tc.value.OffsetMinutes {
					t.Errorf("OffsetMinutes mismatch: expected %d, got %d", tc.value.OffsetMinutes, actual.OffsetMinutes)
				}
				// Server stores local time + offset; compare local time components (hour, min, sec)
				expectedLocal := tc.value.Time
				actualLocal := actual.Time
				if expectedLocal.Hour() != actualLocal.Hour() || expectedLocal.Minute() != actualLocal.Minute() || expectedLocal.Second() != actualLocal.Second() {
					t.Errorf("local time mismatch: expected %02d:%02d:%02d, got %02d:%02d:%02d",
						expectedLocal.Hour(), expectedLocal.Minute(), expectedLocal.Second(),
						actualLocal.Hour(), actualLocal.Minute(), actualLocal.Second())
				}
			})
		}
	})

	t.Run("LocalTime", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.LocalTime
		}{
			{"midnight", gqldb.LocalTime{Hour: 0, Minute: 0, Second: 0, Nanosecond: 0}},
			{"noon", gqldb.LocalTime{Hour: 12, Minute: 0, Second: 0, Nanosecond: 0}},
			{"endOfDay", gqldb.LocalTime{Hour: 23, Minute: 59, Second: 59, Nanosecond: 999999999}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "lt_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.LocalTime)
				if !ok {
					t.Fatalf("expected LocalTime, got %T", result)
				}
				if actual.Hour != tc.value.Hour || actual.Minute != tc.value.Minute || actual.Second != tc.value.Second || actual.Nanosecond != tc.value.Nanosecond {
					t.Errorf("expected %+v, got %+v", tc.value, actual)
				}
			})
		}
	})

	t.Run("ZonedTime", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.ZonedTime
		}{
			{"noon_utc", gqldb.ZonedTime{Hour: 12, Minute: 0, Second: 0, Nanosecond: 0, OffsetMinutes: 0}},
			{"noon_est", gqldb.ZonedTime{Hour: 12, Minute: 0, Second: 0, Nanosecond: 0, OffsetMinutes: -300}},
			{"noon_ist", gqldb.ZonedTime{Hour: 12, Minute: 0, Second: 0, Nanosecond: 0, OffsetMinutes: 330}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "zt_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.ZonedTime)
				if !ok {
					t.Fatalf("expected ZonedTime, got %T", result)
				}
				if actual.Hour != tc.value.Hour || actual.Minute != tc.value.Minute || actual.Second != tc.value.Second || actual.Nanosecond != tc.value.Nanosecond || actual.OffsetMinutes != tc.value.OffsetMinutes {
					t.Errorf("expected %+v, got %+v", tc.value, actual)
				}
			})
		}
	})

	t.Run("YearToMonth", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.YearToMonth
		}{
			{"zero", gqldb.YearToMonth{Months: 0}},
			{"oneYear", gqldb.YearToMonth{Months: 12}},
			{"oneAndHalf", gqldb.YearToMonth{Months: 18}},
			{"negative", gqldb.YearToMonth{Months: -6}},
			{"tenYears", gqldb.YearToMonth{Months: 120}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "ytm_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.YearToMonth)
				if !ok {
					t.Fatalf("expected YearToMonth, got %T", result)
				}
				if actual.Months != tc.value.Months {
					t.Errorf("expected %d months, got %d months", tc.value.Months, actual.Months)
				}
			})
		}
	})

	t.Run("DayToSecond", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.DayToSecond
		}{
			{"zero", gqldb.DayToSecond{Seconds: 0, Nanoseconds: 0}},
			{"oneDay", gqldb.DayToSecond{Seconds: 86400, Nanoseconds: 0}},
			{"oneHour", gqldb.DayToSecond{Seconds: 3600, Nanoseconds: 0}},
			{"mixed", gqldb.DayToSecond{Seconds: 93784, Nanoseconds: 0}}, // 1 day + 2 hours + 3 min + 4 sec
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "dts_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.DayToSecond)
				if !ok {
					t.Fatalf("expected DayToSecond, got %T", result)
				}
				if actual.Seconds != tc.value.Seconds || actual.Nanoseconds != tc.value.Nanoseconds {
					t.Errorf("expected %+v, got %+v", tc.value, actual)
				}
			})
		}
	})
}

// TestSpatialTypesRoundtrip tests encode/decode for spatial types
func TestSpatialTypesRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	graphName, cleanup := setupRoundtripTest(t, ctx, "test_spatial_roundtrip")
	defer cleanup()

	// Start shared bulk import session for all subtests
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	t.Run("Point", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.Point
		}{
			{"nyc", gqldb.Point{Latitude: 40.7128, Longitude: -74.0060}},
			{"origin", gqldb.Point{Latitude: 0.0, Longitude: 0.0}},
			{"tokyo", gqldb.Point{Latitude: 35.6762, Longitude: 139.6503}},
			{"maxLat", gqldb.Point{Latitude: 90.0, Longitude: 180.0}},
			{"minLat", gqldb.Point{Latitude: -90.0, Longitude: -180.0}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "point_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.Point)
				if !ok {
					t.Fatalf("expected Point, got %T", result)
				}
				assertFloat64Equal(t, "latitude", tc.value.Latitude, actual.Latitude, 1e-10)
				assertFloat64Equal(t, "longitude", tc.value.Longitude, actual.Longitude, 1e-10)
			})
		}
	})

	t.Run("Point3D", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.Point3D
		}{
			{"origin", gqldb.Point3D{X: 0.0, Y: 0.0, Z: 0.0}},
			{"unit", gqldb.Point3D{X: 1.0, Y: 2.0, Z: 3.0}},
			{"negative", gqldb.Point3D{X: -1.5, Y: -2.5, Z: -3.5}},
			{"large", gqldb.Point3D{X: 1e10, Y: 1e10, Z: 1e10}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "point3d_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.Point3D)
				if !ok {
					t.Fatalf("expected Point3D, got %T", result)
				}
				assertFloat64Equal(t, "X", tc.value.X, actual.X, 1e-10)
				assertFloat64Equal(t, "Y", tc.value.Y, actual.Y, 1e-10)
				assertFloat64Equal(t, "Z", tc.value.Z, actual.Z, 1e-10)
			})
		}
	})
}

// TestSpecialTypesRoundtrip tests encode/decode for special types
func TestSpecialTypesRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	graphName, cleanup := setupRoundtripTest(t, ctx, "test_special_roundtrip")
	defer cleanup()

	// Start shared bulk import session for all subtests
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	t.Run("Bool", func(t *testing.T) {
		testCases := []struct {
			name  string
			value bool
		}{
			{"true", true},
			{"false", false},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "bool_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(bool)
				if !ok {
					t.Fatalf("expected bool, got %T", result)
				}
				if actual != tc.value {
					t.Errorf("expected %v, got %v", tc.value, actual)
				}
			})
		}
	})

	t.Run("Decimal", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.Decimal
		}{
			{"standard", gqldb.Decimal{Value: "123.456"}},
			{"highPrecision", gqldb.Decimal{Value: "123.456789012345678901234567890"}},
			{"negative", gqldb.Decimal{Value: "-987.654321"}},
			{"integer", gqldb.Decimal{Value: "12345"}},
			{"large", gqldb.Decimal{Value: "99999999999999999999.99999999999999999999"}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "decimal_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.Decimal)
				if !ok {
					t.Fatalf("expected Decimal, got %T", result)
				}
				if actual.Value != tc.value.Value {
					t.Errorf("expected %s, got %s", tc.value.Value, actual.Value)
				}
			})
		}
	})

	t.Run("Vector", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.Vector
		}{
			{"small", gqldb.Vector{Values: []float32{1.0, 2.0, 3.0, 4.0}}},
			{"normalized", gqldb.Vector{Values: []float32{0.5, 0.5, 0.5, 0.5}}},
			{"negative", gqldb.Vector{Values: []float32{-1.0, -2.0, 0.0, 1.0}}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "vector_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.Vector)
				if !ok {
					t.Fatalf("expected Vector, got %T", result)
				}
				if len(actual.Values) != len(tc.value.Values) {
					t.Errorf("expected %d values, got %d", len(tc.value.Values), len(actual.Values))
				}
				for i := range tc.value.Values {
					if math.Abs(float64(actual.Values[i]-tc.value.Values[i])) > 1e-5 {
						t.Errorf("value[%d]: expected %f, got %f", i, tc.value.Values[i], actual.Values[i])
					}
				}
			})
		}
	})

	t.Run("Set", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.Set
		}{
			{"strings", gqldb.Set{"a", "b", "c"}},
			{"integers", gqldb.Set{int64(1), int64(2), int64(3)}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "set_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.Set)
				if !ok {
					// May return as []interface{}
					if arr, ok := result.([]interface{}); ok {
						actual = gqldb.Set(arr)
					} else {
						t.Fatalf("expected Set, got %T", result)
					}
				}
				if len(actual) != len(tc.value) {
					t.Errorf("expected %d elements, got %d", len(tc.value), len(actual))
				}
			})
		}
	})

	t.Run("Record", func(t *testing.T) {
		testCases := []struct {
			name  string
			value gqldb.Record
		}{
			{"simple", gqldb.Record{"key1": "value1", "key2": int64(42)}},
			{"nested", gqldb.Record{"outer": map[string]interface{}{"inner": "value"}}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "record_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(gqldb.Record)
				if !ok {
					// May return as map[string]interface{}
					if m, ok := result.(map[string]interface{}); ok {
						actual = gqldb.Record(m)
					} else {
						t.Fatalf("expected Record, got %T", result)
					}
				}
				if len(actual) != len(tc.value) {
					t.Errorf("expected %d keys, got %d", len(tc.value), len(actual))
				}
			})
		}
	})
}

// TestCollectionTypesRoundtrip tests encode/decode for collection types
func TestCollectionTypesRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	graphName, cleanup := setupRoundtripTest(t, ctx, "test_collection_roundtrip")
	defer cleanup()

	// Start shared bulk import session for all subtests
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	defer func() {
		_, _ = testClient.EndBulkImport(ctx, session.SessionID)
	}()

	t.Run("List", func(t *testing.T) {
		testCases := []struct {
			name  string
			value []interface{}
		}{
			{"empty", []interface{}{}},
			{"strings", []interface{}{"a", "b", "c"}},
			{"integers", []interface{}{int64(1), int64(2), int64(3)}},
			{"mixed", []interface{}{int64(1), "two", float64(3.0), true}},
			{"nested", []interface{}{[]interface{}{int64(1), int64(2)}, []interface{}{int64(3), int64(4)}}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "list_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.([]interface{})
				if !ok {
					t.Fatalf("expected []interface{}, got %T", result)
				}
				if len(actual) != len(tc.value) {
					t.Errorf("expected %d elements, got %d", len(tc.value), len(actual))
				}
			})
		}
	})

	t.Run("Map", func(t *testing.T) {
		testCases := []struct {
			name  string
			value map[string]interface{}
		}{
			{"empty", map[string]interface{}{}},
			{"simple", map[string]interface{}{"key1": "value1", "key2": int64(42)}},
			{"nested", map[string]interface{}{"outer": map[string]interface{}{"inner": "value"}}},
			{"mixed", map[string]interface{}{"str": "text", "num": int64(123), "bool": true, "list": []interface{}{int64(1), int64(2), int64(3)}}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := insertAndQueryProperty(t, ctx, graphName, "map_"+tc.name, tc.value, session.SessionID)
				actual, ok := result.(map[string]interface{})
				if !ok {
					t.Fatalf("expected map[string]interface{}, got %T", result)
				}
				if len(actual) != len(tc.value) {
					t.Errorf("expected %d keys, got %d", len(tc.value), len(actual))
				}
			})
		}
	})
}
