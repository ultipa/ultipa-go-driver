package test

import (
	"testing"

	"github.com/ultipa/ultipa-go-driver/v5/sdk/api"
)

func TestCheckGraphName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr bool
	}{
		{"Valid name", "Graph1", false},
		{"Invalid name - starts with digit", "1Graph", true},
		{"Invalid name - contains special character", "Graph@", true},
		{"Empty name", "", true},
		{"Valid name with underscore", "Graph_Name1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := api.CheckGraphName(tt.input)
			if (err != nil) != tt.expectedErr {
				t.Errorf("CheckGraphName(%s) error = %v, expectedErr %v", tt.input, err, tt.expectedErr)
			}
		})
	}
}

func TestReplaceSchemaPropertyNameIfNeeded(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectedErr bool
	}{
		{"Valid name", "Property1", "Property1", false},
		{"Invalid name - contains special character", "Property@", "`Property@`", false},
		{"Empty name", "", "", true},
		{"Valid name with underscore", "Property_Name1", "Property_Name1", false},
		{"Valid name with digits only", "12345", "12345", false},
		{"Valid name with chinese", "账户", "`账户`", false},
		{"Valid name with \"", `"""`, "`\"\"\"`", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := api.CheckReplaceSchemaPropertyName(tt.input)
			if (err != nil) != tt.expectedErr {
				t.Errorf("CheckReplaceSchemaPropertyName(%s) error = %v, expectedErr %v", tt.input, err, tt.expectedErr)
			}
			if result != tt.expected {
				t.Errorf("CheckReplaceSchemaPropertyName(%s) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}
