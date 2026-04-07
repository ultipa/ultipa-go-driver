package unit

import (
	"testing"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestQueryConfigDefaults(t *testing.T) {
	config := &gqldb.QueryConfig{}

	if config.GraphName != "" {
		t.Errorf("expected empty GraphName, got %v", config.GraphName)
	}

	if config.TransactionID != 0 {
		t.Errorf("expected TransactionID 0, got %v", config.TransactionID)
	}

	if config.Timeout != 0 {
		t.Errorf("expected Timeout 0, got %v", config.Timeout)
	}

	if config.ReadOnly != false {
		t.Errorf("expected ReadOnly false, got %v", config.ReadOnly)
	}

	if config.MaxPathResults != 0 {
		t.Errorf("expected MaxPathResults 0 (unlimited), got %v", config.MaxPathResults)
	}

	if config.Parameters != nil {
		t.Errorf("expected nil Parameters, got %v", config.Parameters)
	}
}

func TestQueryConfigWithValues(t *testing.T) {
	config := &gqldb.QueryConfig{
		GraphName:      "testGraph",
		TransactionID:  12345,
		Timeout:        60,
		ReadOnly:       true,
		MaxPathResults: 100,
		Parameters: map[string]interface{}{
			"name": "test",
		},
	}

	if config.GraphName != "testGraph" {
		t.Errorf("expected GraphName 'testGraph', got %v", config.GraphName)
	}

	if config.TransactionID != 12345 {
		t.Errorf("expected TransactionID 12345, got %v", config.TransactionID)
	}

	if config.Timeout != 60 {
		t.Errorf("expected Timeout 60, got %v", config.Timeout)
	}

	if config.ReadOnly != true {
		t.Errorf("expected ReadOnly true, got %v", config.ReadOnly)
	}

	if config.MaxPathResults != 100 {
		t.Errorf("expected MaxPathResults 100, got %v", config.MaxPathResults)
	}

	if config.Parameters["name"] != "test" {
		t.Errorf("expected Parameters['name'] = 'test', got %v", config.Parameters["name"])
	}
}

func TestQueryConfigMaxPathResultsValues(t *testing.T) {
	tests := []struct {
		name     string
		value    int64
		expected int64
	}{
		{"zero (unlimited)", 0, 0},
		{"small limit", 10, 10},
		{"medium limit", 1000, 1000},
		{"large limit", 1000000, 1000000},
		{"max int64", 9223372036854775807, 9223372036854775807},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &gqldb.QueryConfig{
				MaxPathResults: tt.value,
			}

			if config.MaxPathResults != tt.expected {
				t.Errorf("expected MaxPathResults %v, got %v", tt.expected, config.MaxPathResults)
			}
		})
	}
}
