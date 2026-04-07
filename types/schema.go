package types

// Schema represents node/edge schema metadata.
type Schema struct {
	Name       string
	Properties []*PropertyDef
}

// PropertyDef represents a property definition in a schema.
type PropertyDef struct {
	Name string
	Type PropertyType
}

// Table represents a generic result table.
type Table struct {
	Name    string
	Headers []*Header
	Rows    [][]interface{}
}

// Header represents a column header in a table.
type Header struct {
	Name string
	Type PropertyType
}

// Attr represents a scalar or list attribute result.
type Attr struct {
	Name   string
	Type   PropertyType
	Values []interface{}
}
