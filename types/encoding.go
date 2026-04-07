package types

import (
	"encoding/binary"
	"encoding/json"
)

// encodeList encodes a list of values into bytes.
// Binary format: [count:2 uint16LE][TypedValueEntry]...
// TypedValueEntry: [type:1][isNull:1][dataLen:4 uint32LE][data]
func encodeList(values []interface{}) (*TypedValue, error) {
	var data []byte

	// Write count (2 bytes)
	countBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(countBytes, uint16(len(values)))
	data = append(data, countBytes...)

	for _, v := range values {
		tv, err := NewTypedValue(v)
		if err != nil {
			return nil, err
		}
		data = append(data, encodeTypedValueEntry(tv)...)
	}

	return &TypedValue{Type: PropertyTypeList, Data: data}, nil
}

// encodeTypedValueEntry encodes a TypedValue to binary format.
// Format: [type:1][isNull:1][dataLen:4 uint32LE][data]
func encodeTypedValueEntry(tv *TypedValue) []byte {
	result := make([]byte, 6+len(tv.Data))
	result[0] = byte(tv.Type)
	if tv.IsNull {
		result[1] = 1
	} else {
		result[1] = 0
	}
	binary.LittleEndian.PutUint32(result[2:6], uint32(len(tv.Data)))
	if len(tv.Data) > 0 {
		copy(result[6:], tv.Data)
	}
	return result
}

// decodeList decodes a list from bytes.
// Supports both JSON format (e.g., "[1,2,3]") and binary format.
// Binary format: [count:2 uint16LE][TypedValueEntry]...
func decodeList(data []byte) ([]interface{}, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// Check if data is JSON format (starts with '[')
	if data[0] == '[' {
		var result []interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result, nil
	}

	// Binary format: [count:2 uint16LE][TypedValueEntry]...
	if len(data) < 2 {
		return nil, nil
	}

	count := int(binary.LittleEndian.Uint16(data[0:2]))
	offset := 2

	result := make([]interface{}, 0, count)

	for i := 0; i < count && offset < len(data); i++ {
		tv, tvLen := decodeTypedValueEntry(data, offset)
		if tvLen == 0 {
			break
		}
		offset += tvLen

		val, err := tv.ToGo()
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}

	return result, nil
}

// encodeMap encodes a map into bytes.
// Binary format: [count:2][keyLen:2][key][TVE]...
func encodeMap(m map[string]interface{}) (*TypedValue, error) {
	var data []byte

	// Write count (2 bytes)
	countBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(countBytes, uint16(len(m)))
	data = append(data, countBytes...)

	for k, v := range m {
		// Write key (length-prefixed with 2 bytes)
		keyBytes := []byte(k)
		keyLenBytes := make([]byte, 2)
		binary.LittleEndian.PutUint16(keyLenBytes, uint16(len(keyBytes)))
		data = append(data, keyLenBytes...)
		data = append(data, keyBytes...)

		// Write value as TypedValueEntry
		tv, err := NewTypedValue(v)
		if err != nil {
			return nil, err
		}
		data = append(data, encodeTypedValueEntry(tv)...)
	}

	return &TypedValue{Type: PropertyTypeMap, Data: data}, nil
}

// decodeMap decodes a map from bytes.
// Binary format: [count:2][keyLen:2][key][TVE]...
func decodeMap(data []byte) (map[string]interface{}, error) {
	if len(data) < 2 {
		return nil, nil
	}

	count := int(binary.LittleEndian.Uint16(data[0:2]))
	offset := 2

	result := make(map[string]interface{})

	for i := 0; i < count && offset < len(data); i++ {
		// Read key
		key, keyLen := decodeString(data, offset)
		if keyLen == 0 {
			break
		}
		offset += keyLen

		// Read value as TypedValueEntry
		tv, tvLen := decodeTypedValueEntry(data, offset)
		if tvLen == 0 {
			break
		}
		offset += tvLen

		val, err := tv.ToGo()
		if err != nil {
			return nil, err
		}
		result[key] = val
	}

	return result, nil
}

// encodeSet encodes a set of values into bytes (same format as list).
func encodeSet(values Set) (*TypedValue, error) {
	var data []byte

	// Write count (2 bytes)
	countBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(countBytes, uint16(len(values)))
	data = append(data, countBytes...)

	for _, v := range values {
		tv, err := NewTypedValue(v)
		if err != nil {
			return nil, err
		}
		data = append(data, encodeTypedValueEntry(tv)...)
	}

	return &TypedValue{Type: PropertyTypeSet, Data: data}, nil
}

// decodeSet decodes a set from bytes (same format as list).
func decodeSet(data []byte) (Set, error) {
	list, err := decodeList(data)
	if err != nil {
		return nil, err
	}
	return Set(list), nil
}
