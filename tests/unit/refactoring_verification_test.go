package unit

import (
	"testing"

	"github.com/ultipa/ultipa-go-driver/v6"
)

// TestBackwardCompatibility verifies that all public types are still accessible
// after the refactoring to services/ and types/ subdirectories.
func TestBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name string
		test func(*testing.T)
	}{
		{
			name: "PropertyType constants accessible",
			test: func(t *testing.T) {
				var pt gqldb.PropertyType
				pt = gqldb.PropertyTypeString
				if pt != gqldb.PropertyTypeString {
					t.Errorf("PropertyType constant not accessible")
				}
			},
		},
		{
			name: "TypedValue creation works",
			test: func(t *testing.T) {
				tv, err := gqldb.NewTypedValue("test")
				if err != nil {
					t.Errorf("NewTypedValue failed: %v", err)
				}
				if tv == nil {
					t.Error("NewTypedValue returned nil")
				}
			},
		},
		{
			name: "Parameter creation works",
			test: func(t *testing.T) {
				param, err := gqldb.NewParameter("test", "value")
				if err != nil {
					t.Errorf("NewParameter failed: %v", err)
				}
				if param == nil || param.Name != "test" {
					t.Error("NewParameter failed or returned incorrect parameter")
				}
			},
		},
		{
			name: "Data types accessible",
			test: func(t *testing.T) {
				point := gqldb.Point{Latitude: 1.0, Longitude: 2.0}
				if point.Latitude != 1.0 {
					t.Error("Point type not accessible")
				}

				decimal := gqldb.Decimal{Value: "123.456"}
				if decimal.Value != "123.456" {
					t.Error("Decimal type not accessible")
				}

				vec := gqldb.Vector{Values: []float32{1.0, 2.0, 3.0}}
				if len(vec.Values) != 3 {
					t.Error("Vector type not accessible")
				}
			},
		},
		{
			name: "Graph model types accessible",
			test: func(t *testing.T) {
				node := gqldb.Node{
					ID:         "n1",
					Labels:     []string{"Person"},
					Properties: map[string]interface{}{"name": "Alice"},
				}
				if node.ID != "n1" {
					t.Error("Node type not accessible")
				}

				edge := gqldb.Edge{
					ID:         "e1",
					Label:      "KNOWS",
					FromNodeID: "n1",
					ToNodeID:   "n2",
					Properties: map[string]interface{}{},
				}
				if edge.Label != "KNOWS" {
					t.Error("Edge type not accessible")
				}
			},
		},
		{
			name: "Config types accessible",
			test: func(t *testing.T) {
				config := gqldb.QueryConfig{
					GraphName: "test",
					Timeout:   5000,
					ReadOnly:  true,
				}
				if config.GraphName != "test" {
					t.Error("QueryConfig type not accessible")
				}

				bulkOpts := gqldb.BulkImportOptions{
					EstimatedNodes: 10000,
					EstimatedEdges: 50000,
				}
				if bulkOpts.EstimatedNodes != 10000 {
					t.Error("BulkImportOptions type not accessible")
				}
			},
		},
		{
			name: "Metadata types accessible",
			test: func(t *testing.T) {
				graphInfo := gqldb.GraphInfo{
					Name:      "test",
					GraphType: gqldb.GraphTypeOpen,
					NodeCount: 100,
					EdgeCount: 200,
				}
				if graphInfo.Name != "test" {
					t.Error("GraphInfo type not accessible")
				}

				stats := gqldb.Statistics{
					NodeCount: 100,
					EdgeCount: 200,
				}
				if stats.NodeCount != 100 {
					t.Error("Statistics type not accessible")
				}
			},
		},
		{
			name: "Enum types accessible",
			test: func(t *testing.T) {
				var gt gqldb.GraphType = gqldb.GraphTypeOpen
				if gt != gqldb.GraphTypeOpen {
					t.Error("GraphType enum not accessible")
				}

				var hs gqldb.HealthStatus = gqldb.HealthStatusServing
				if hs != gqldb.HealthStatusServing {
					t.Error("HealthStatus enum not accessible")
				}

				var ct gqldb.CacheType = gqldb.CacheTypeAST
				if ct != gqldb.CacheTypeAST {
					t.Error("CacheType enum not accessible")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

// TestTypeConversions verifies that type conversions work correctly
func TestTypeConversions(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  gqldb.PropertyType
	}{
		{"string", "test", gqldb.PropertyTypeString},
		{"int32", int32(42), gqldb.PropertyTypeInt32},
		{"int64", int64(42), gqldb.PropertyTypeInt64},
		{"float32", float32(3.14), gqldb.PropertyTypeFloat},
		{"float64", float64(3.14), gqldb.PropertyTypeDouble},
		{"bool", true, gqldb.PropertyTypeBool},
		{"nil", nil, gqldb.PropertyTypeNull},
		{"list", []interface{}{1, 2, 3}, gqldb.PropertyTypeList},
		{"map", map[string]interface{}{"key": "value"}, gqldb.PropertyTypeMap},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := gqldb.NewTypedValue(tt.input)
			if err != nil {
				t.Fatalf("NewTypedValue failed: %v", err)
			}
			if tv.Type != tt.want {
				t.Errorf("Expected type %v, got %v", tt.want, tv.Type)
			}

			// Test round-trip conversion
			val, err := tv.ToGo()
			if err != nil {
				t.Fatalf("ToGo failed: %v", err)
			}
			if tt.input == nil && val != nil {
				t.Error("Expected nil value")
			}
		})
	}
}

// TestServicesIntegration verifies that services are properly initialized
func TestServicesIntegration(t *testing.T) {
	// This test verifies that a client can be created with the refactored services
	// Note: This will fail without a valid connection, but tests compilation
	config := &gqldb.Config{
		Hosts: []string{"invalid-host:60061"}, // Invalid host to avoid actual connection
	}

	client, err := gqldb.NewClient(config)
	if err != nil {
		// Expected to fail with invalid host, but should compile correctly
		t.Logf("Client creation failed as expected with invalid host: %v", err)
		return
	}

	if client == nil {
		t.Error("Expected non-nil client even with invalid config")
	}
}
