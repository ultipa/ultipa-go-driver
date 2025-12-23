package types

import (
	"encoding/json"
	"fmt"
)

// Record represents a RECORD type property value in Ultipa
// It stores arbitrary JSON data as a map internally for efficient access
type Record struct {
	data map[string]interface{}
}

// NewRecord creates a new Record from a map
func NewRecord(data map[string]interface{}) *Record {
	return &Record{data: data}
}

// RecordFromJSON creates a Record from a JSON string
func RecordFromJSON(jsonStr string) (*Record, error) {
	if jsonStr == "" {
		return &Record{data: make(map[string]interface{})}, nil
	}

	// First try to unmarshal as an object
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// If it fails, check if it's a JSON array
		var listData []interface{}
		if listErr := json.Unmarshal([]byte(jsonStr), &listData); listErr == nil {
			// It's a valid JSON array, wrap it in an object with "list" key
			data = map[string]interface{}{
				"list": listData,
			}
		} else {
			// Not a valid JSON object or array
			return nil, fmt.Errorf("failed to parse Record from JSON (input: %q): %v", jsonStr, err)
		}
	}
	return &Record{data: data}, nil
}

// ToJSONString converts the Record to a JSON string
func (r *Record) ToJSONString() (string, error) {
	if r == nil || r.data == nil {
		return "{}", nil
	}
	jsonBytes, err := json.Marshal(r.data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Record to JSON: %v", err)
	}
	return string(jsonBytes), nil
}

// String returns the JSON string representation of the Record
func (r *Record) String() string {
	jsonStr, _ := r.ToJSONString()
	return jsonStr
}

// MarshalJSON implements json.Marshaler interface
// This ensures Record is properly serialized when used with json.Marshal()
func (r *Record) MarshalJSON() ([]byte, error) {
	if r == nil || r.data == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(r.data)
}

// ToMap returns the internal map data
func (r *Record) ToMap() map[string]interface{} {
	if r == nil || r.data == nil {
		return nil
	}
	return r.data
}

// Get retrieves a value from the Record by key
func (r *Record) Get(key string) interface{} {
	if r == nil || r.data == nil {
		return nil
	}
	return r.data[key]
}

// Set sets a value in the Record by key
func (r *Record) Set(key string, value interface{}) {
	if r == nil {
		return
	}
	if r.data == nil {
		r.data = make(map[string]interface{})
	}
	r.data[key] = value
}

// Has checks if a key exists in the Record
func (r *Record) Has(key string) bool {
	if r == nil || r.data == nil {
		return false
	}
	_, exists := r.data[key]
	return exists
}

// Keys returns all keys in the Record
func (r *Record) Keys() []string {
	if r == nil || r.data == nil {
		return nil
	}
	keys := make([]string, 0, len(r.data))
	for k := range r.data {
		keys = append(keys, k)
	}
	return keys
}