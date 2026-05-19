package gqldb

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// Response represents the result of a GQL query.
type Response struct {
	Columns      []string
	Rows         []*Row
	RowCount     int64
	HasMore      bool
	Warnings     []string
	RowsAffected int64
	// CurrentGraph is the session's current graph after this RPC
	// executed, as authoritatively reported by the server. Always
	// populated on success against new servers (covers single/compound
	// USE GRAPH at any position, last-write-wins). Empty when running
	// against a pre-fix server, in which case the driver falls back to
	// its USE GRAPH text-parsing path internally to keep the local
	// session.DefaultGraph cache in sync.
	CurrentGraph string
	// Server-side timing (nanoseconds), read from the engine's
	// ResultSet. Network / client-side time is NOT included.
	//   - TimeCostNs:    total wall-clock parse + plan + execute
	//   - DiskCostNs:    subset spent in storage / LSM layer
	//   - ComputeCostNs: subset spent in the in-memory compute engine
	//                    (k-hop, shortest path, algo.* via topology
	//                    accelerator); 0 when compute is disabled or
	//                    the query path did not invoke the accelerator.
	// Old servers omit these proto3 fields → treat 0 as "not reported",
	// not "took zero time". Streaming queries populate only on the
	// final batch (HasMore=false), matching CurrentGraph / RowsAffected.
	TimeCostNs    int64
	DiskCostNs    int64
	ComputeCostNs int64
}

// Row represents a single row in the query result.
//
// Positional access via Get / GetString / GetInt / etc. is always available.
// Name-based access (GetByName, Has) requires ColumnNames to be populated,
// which Response does automatically (via NewResponse or PropagateColumnNames).
type Row struct {
	Values []*TypedValue
	// ColumnNames mirrors Response.Columns. Populated by Response so the
	// row can be looked up by name without holding a back-reference to
	// the parent Response. May be nil if the Row was constructed standalone.
	ColumnNames []string
}

// NewResponse creates a new Response and propagates Columns into each
// Row's ColumnNames so row.GetByName works.
func NewResponse(columns []string, rows []*Row, rowCount int64, hasMore bool, warnings []string, rowsAffected int64) *Response {
	resp := &Response{
		Columns:      columns,
		Rows:         rows,
		RowCount:     rowCount,
		HasMore:      hasMore,
		Warnings:     warnings,
		RowsAffected: rowsAffected,
	}
	resp.PropagateColumnNames()
	return resp
}

// PropagateColumnNames pushes resp.Columns into each Row's ColumnNames so
// row.GetByName / row.Has work. Idempotent: rows that already have
// ColumnNames set are not overwritten. Call this after constructing a
// Response via struct literal (NewResponse calls it automatically).
func (resp *Response) PropagateColumnNames() {
	if len(resp.Columns) == 0 {
		return
	}
	for _, row := range resp.Rows {
		if row != nil && row.ColumnNames == nil {
			row.ColumnNames = resp.Columns
		}
	}
}

// NewRow creates a new Row from typed values.
func NewRow(values []*TypedValue) *Row {
	return &Row{Values: values}
}

// Get returns the value at the given column index.
func (r *Row) Get(index int) (interface{}, error) {
	if index < 0 || index >= len(r.Values) {
		return nil, fmt.Errorf("column index out of range: %d", index)
	}
	return r.Values[index].ToGo()
}

// GetByName returns the value for the given column name. Requires
// ColumnNames to be populated (Response does this automatically).
func (r *Row) GetByName(name string) (interface{}, error) {
	if r.ColumnNames == nil {
		return nil, fmt.Errorf("row column names not populated; cannot look up %q by name", name)
	}
	for i, col := range r.ColumnNames {
		if col == name {
			return r.Get(i)
		}
	}
	return nil, fmt.Errorf("column not found: %s", name)
}

// Has reports whether the row knows the given column name. Returns false
// if ColumnNames was never populated.
func (r *Row) Has(name string) bool {
	for _, col := range r.ColumnNames {
		if col == name {
			return true
		}
	}
	return false
}

// Len returns the number of columns in the row.
func (r *Row) Len() int {
	return len(r.Values)
}

// String returns a formatted representation of the row.
// When ColumnNames is populated: "Row(name1=val1, name2=val2)".
// Otherwise: "Row(val0, val1)". Mirrors Python's repr(row) layout.
func (r *Row) String() string {
	if r == nil {
		return "Row(<nil>)"
	}
	var sb strings.Builder
	sb.WriteString("Row(")
	for i, tv := range r.Values {
		if i > 0 {
			sb.WriteString(", ")
		}
		if i < len(r.ColumnNames) {
			sb.WriteString(r.ColumnNames[i])
			sb.WriteByte('=')
		}
		v, _ := tv.ToGo()
		if s, ok := v.(string); ok {
			sb.WriteByte('\'')
			sb.WriteString(s)
			sb.WriteByte('\'')
		} else {
			fmt.Fprintf(&sb, "%v", v)
		}
	}
	sb.WriteByte(')')
	return sb.String()
}

// Equal returns true when the receiver and other have the same column
// names (or both are unset) and their decoded values compare equal
// position-by-position. Mirrors Python Row.__eq__ semantics.
func (r *Row) Equal(other *Row) bool {
	if r == nil || other == nil {
		return r == other
	}
	if len(r.ColumnNames) != len(other.ColumnNames) {
		return false
	}
	for i, c := range r.ColumnNames {
		if c != other.ColumnNames[i] {
			return false
		}
	}
	if len(r.Values) != len(other.Values) {
		return false
	}
	for i, tv := range r.Values {
		a, _ := tv.ToGo()
		b, _ := other.Values[i].ToGo()
		if !reflect.DeepEqual(a, b) {
			return false
		}
	}
	return true
}

// GetString returns the value at the given column index as a string.
func (r *Row) GetString(index int) (string, error) {
	val, err := r.Get(index)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	if s, ok := val.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", val), nil
}

// GetInt returns the value at the given column index as an int64.
func (r *Row) GetInt(index int) (int64, error) {
	val, err := r.Get(index)
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, nil
	}
	switch v := val.(type) {
	case int64:
		return v, nil
	case int32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", val)
	}
}

// GetFloat returns the value at the given column index as a float64.
func (r *Row) GetFloat(index int) (float64, error) {
	val, err := r.Get(index)
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, nil
	}
	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", val)
	}
}

// GetBool returns the value at the given column index as a bool.
func (r *Row) GetBool(index int) (bool, error) {
	val, err := r.Get(index)
	if err != nil {
		return false, err
	}
	if val == nil {
		return false, nil
	}
	if b, ok := val.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("cannot convert %T to bool", val)
}

// GetByName returns the value for the given column name.
func (resp *Response) GetByName(row *Row, columnName string) (interface{}, error) {
	for i, col := range resp.Columns {
		if col == columnName {
			return row.Get(i)
		}
	}
	return nil, fmt.Errorf("column not found: %s", columnName)
}

// IsEmpty returns true if the response has no rows.
func (resp *Response) IsEmpty() bool {
	return len(resp.Rows) == 0
}

// First returns the first row or nil if empty.
func (resp *Response) First() *Row {
	if len(resp.Rows) == 0 {
		return nil
	}
	return resp.Rows[0]
}

// Last returns the last row or nil if empty.
func (resp *Response) Last() *Row {
	if len(resp.Rows) == 0 {
		return nil
	}
	return resp.Rows[len(resp.Rows)-1]
}

// ForEach iterates over all rows with a callback.
func (resp *Response) ForEach(fn func(row *Row, index int) error) error {
	for i, row := range resp.Rows {
		if err := fn(row, i); err != nil {
			return err
		}
	}
	return nil
}

// Map transforms each row using a callback.
func (resp *Response) Map(fn func(row *Row) (interface{}, error)) ([]interface{}, error) {
	result := make([]interface{}, len(resp.Rows))
	for i, row := range resp.Rows {
		val, err := fn(row)
		if err != nil {
			return nil, err
		}
		result[i] = val
	}
	return result, nil
}

// ToMaps converts the response to a slice of maps.
func (resp *Response) ToMaps() ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, len(resp.Rows))
	for i, row := range resp.Rows {
		m := make(map[string]interface{})
		for j, col := range resp.Columns {
			var val interface{}
			// Add boundary check to handle mismatched columns/values
			if j < len(row.Values) {
				var err error
				val, err = row.Get(j)
				if err != nil {
					return nil, err
				}
			} else {
				val = nil
			}
			m[col] = val
		}
		result[i] = m
	}
	return result, nil
}

// ToJSON converts the response to JSON.
func (resp *Response) ToJSON() ([]byte, error) {
	maps, err := resp.ToMaps()
	if err != nil {
		return nil, err
	}
	return json.Marshal(maps)
}

// AiReadResult is the consolidated result of an AiRead / AiGql call.
//
// For AiRead, when Success is true and GeneratedGql is populated, Data
// holds the *Response from re-executing the generated GQL. For AiGql,
// Data is always nil (generate-only).
//
// AI-level errors do not surface as Go errors: Success is set to false
// and Error is populated instead. Only transport-level failures on the
// initial CALL return a non-nil error from AiRead/AiGql.
type AiReadResult struct {
	Stages            []types.AiStage // ordered list of stage rows as streamed
	GeneratedGql      string          // synthesized GQL statement (best-effort extract)
	Data              *Response       // re-executed GQL result for AiRead; nil for AiGql
	Success           bool            // false if any stage reported "error" or re-execution failed
	Error             string          // error message (populated when Success is false)
	TotalElapsedMs    int64           // max elapsed_ms across stages (monotonic timer)
	TotalTokensInput  int64           // sum of tokens_input across stages
	TotalTokensOutput int64           // sum of tokens_output across stages
	TotalTokensCached int64           // sum of tokens_cached across stages
}

// InsertNodesResult represents the result of an insert nodes operation.
type InsertNodesResult struct {
	Success   bool
	NodeIDs   []string
	NodeCount int64
	Message   string
}

// InsertEdgesResult represents the result of an insert edges operation.
type InsertEdgesResult struct {
	Success      bool
	EdgeIDs      []string
	EdgeCount    int64
	Message      string
	SkippedCount int64
}

// ExportConfig represents configuration for the Export operation.
type ExportConfig struct {
	GraphName       string   // Required: graph to export
	BatchSize       int32    // Items per context check (default: 1000)
	ExportNodes     bool     // Export nodes (default: true)
	ExportEdges     bool     // Export edges (default: true)
	NodeLabels      []string // Filter nodes by labels (empty = all)
	EdgeLabels      []string // Filter edges by labels (empty = all)
	IncludeMetadata bool     // Include metadata header (default: true)
}

// ExportResult represents a chunk of exported data.
type ExportResult struct {
	Data    []byte       // JSON Lines chunk
	IsFinal bool         // True for final message with stats
	Stats   *ExportStats // Populated in final message
}

// ExportStats contains export statistics.
type ExportStats struct {
	NodesExported int64
	EdgesExported int64
	BytesWritten  int64
	DurationMs    int64
}

// Note: Node, Edge, and Path types are now imported from the types package
// DEPRECATED: The global AsNodes(), AsEdges(), AsPaths(), AsTable(), AsAttr() methods
// have been removed. Use resp.Alias("column").AsNodes() or resp.Get(index).AsNodes() instead.

// buildSchemaFromNode builds a schema from a node's properties.
func buildSchemaFromNode(label string, node *Node) *Schema {
	schema := &Schema{
		Name:       label,
		Properties: make([]*PropertyDef, 0),
	}
	for propName, propVal := range node.Properties {
		propType := inferPropertyType(propVal)
		schema.Properties = append(schema.Properties, &PropertyDef{
			Name: propName,
			Type: propType,
		})
	}
	return schema
}

// buildSchemaFromEdge builds a schema from an edge's properties.
func buildSchemaFromEdge(label string, edge *Edge) *Schema {
	schema := &Schema{
		Name:       label,
		Properties: make([]*PropertyDef, 0),
	}
	for propName, propVal := range edge.Properties {
		propType := inferPropertyType(propVal)
		schema.Properties = append(schema.Properties, &PropertyDef{
			Name: propName,
			Type: propType,
		})
	}
	return schema
}

// inferPropertyType infers the PropertyType from a Go value.
func inferPropertyType(v interface{}) PropertyType {
	if v == nil {
		return PropertyTypeNull
	}
	switch v.(type) {
	case bool:
		return PropertyTypeBool
	case int32:
		return PropertyTypeInt32
	case int64, int:
		return PropertyTypeInt64
	case uint32:
		return PropertyTypeUint32
	case uint64:
		return PropertyTypeUint64
	case float32:
		return PropertyTypeFloat
	case float64:
		return PropertyTypeDouble
	case string:
		return PropertyTypeString
	case []byte:
		return PropertyTypeBlob
	case []interface{}:
		return PropertyTypeList
	case map[string]interface{}:
		return PropertyTypeMap
	default:
		return PropertyTypeString
	}
}

// SingleValue returns the single value from a single-row, single-column response.
func (resp *Response) SingleValue() (interface{}, error) {
	if len(resp.Rows) == 0 {
		return nil, nil
	}
	if len(resp.Rows) > 1 {
		return nil, fmt.Errorf("expected single row, got %d", len(resp.Rows))
	}
	if len(resp.Rows[0].Values) == 0 {
		return nil, nil
	}
	if len(resp.Rows[0].Values) > 1 {
		return nil, fmt.Errorf("expected single column, got %d", len(resp.Rows[0].Values))
	}
	return resp.Rows[0].Get(0)
}

// SingleInt returns the single int64 value from a single-row, single-column response.
func (resp *Response) SingleInt() (int64, error) {
	val, err := resp.SingleValue()
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, nil
	}
	switch v := val.(type) {
	case int64:
		return v, nil
	case int32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", val)
	}
}

// SingleString returns the single string value from a single-row, single-column response.
func (resp *Response) SingleString() (string, error) {
	val, err := resp.SingleValue()
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	if s, ok := val.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", val), nil
}

// AliasResult represents a single column's data from the response.
// It allows chaining operations like resp.Alias("n2").AsNodes() or resp.Get(1).AsNodes().
type AliasResult struct {
	alias     string    // Column name/alias (empty if accessed by index)
	columnIdx int       // Column index in Response.Columns
	response  *Response // Reference to parent response
}

// Alias returns an AliasResult for the specified column name.
// Example: resp.Alias("n2").AsNodes()
func (resp *Response) Alias(alias string) (*AliasResult, error) {
	// Find column index by name
	colIdx := -1
	for i, col := range resp.Columns {
		if col == alias {
			colIdx = i
			break
		}
	}

	if colIdx == -1 {
		return nil, fmt.Errorf("column alias not found: %s", alias)
	}

	return &AliasResult{
		alias:     alias,
		columnIdx: colIdx,
		response:  resp,
	}, nil
}

// Get returns an AliasResult for the column at the specified index.
// Example: resp.Get(1).AsNodes()
func (resp *Response) Get(index int) (*AliasResult, error) {
	// Validate index
	if index < 0 || index >= len(resp.Columns) {
		return nil, fmt.Errorf("column index out of range: %d (total: %d)", index, len(resp.Columns))
	}

	return &AliasResult{
		alias:     resp.Columns[index],
		columnIdx: index,
		response:  resp,
	}, nil
}

// AsNodes extracts nodes from the aliased column with schema information.
// Nodes are identified by PropertyTypeNode type in the TypedValue.
// Returns nodes and a map of schemas keyed by label name.
// Returns an error if the column contains non-null values that are not nodes.
func collectNode(val interface{}, nodes *[]*Node, schemas map[string]*Schema) {
	if node, ok := val.(*Node); ok && node != nil {
		*nodes = append(*nodes, node)
		for _, label := range node.Labels {
			if _, exists := schemas[label]; !exists {
				schemas[label] = buildSchemaFromNode(label, node)
			}
		}
	}
}

func collectEdge(val interface{}, edges *[]*Edge, schemas map[string]*Schema) {
	if edge, ok := val.(*Edge); ok && edge != nil {
		*edges = append(*edges, edge)
		if edge.Label != "" {
			if _, exists := schemas[edge.Label]; !exists {
				schemas[edge.Label] = buildSchemaFromEdge(edge.Label, edge)
			}
		}
	}
}

// AsNodes extracts nodes from the aliased column with schema information.
// Supports both direct NODE columns and LIST of NODE columns
// (e.g., group variables from quantified patterns).
func (ar *AliasResult) AsNodes() ([]*Node, map[string]*Schema, error) {
	nodes := make([]*Node, 0)
	schemas := make(map[string]*Schema)

	for _, row := range ar.response.Rows {
		if ar.columnIdx >= len(row.Values) {
			continue
		}

		tv := row.Values[ar.columnIdx]

		if tv.IsNull {
			continue
		}

		if tv.Type == PropertyTypeNode {
			val, err := tv.ToGo()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to convert node at column %s (index %d): %w",
					ar.alias, ar.columnIdx, err)
			}
			collectNode(val, &nodes, schemas)
		} else if tv.Type == PropertyTypeList {
			// LIST of NODE (group variable from quantified pattern)
			val, err := tv.ToGo()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to convert list at column %s (index %d): %w",
					ar.alias, ar.columnIdx, err)
			}
			if items, ok := val.([]interface{}); ok {
				for _, item := range items {
					collectNode(item, &nodes, schemas)
				}
			}
		} else {
			return nil, nil, fmt.Errorf("type mismatch: column %s (index %d) contains %v, expected NODE or LIST of NODE",
				ar.alias, ar.columnIdx, tv.Type)
		}
	}

	return nodes, schemas, nil
}

// AsEdges extracts edges from the aliased column with schema information.
// Supports both direct EDGE columns and LIST of EDGE columns
// (e.g., group variables from quantified patterns).
func (ar *AliasResult) AsEdges() ([]*Edge, map[string]*Schema, error) {
	edges := make([]*Edge, 0)
	schemas := make(map[string]*Schema)

	for _, row := range ar.response.Rows {
		if ar.columnIdx >= len(row.Values) {
			continue
		}

		tv := row.Values[ar.columnIdx]

		if tv.IsNull {
			continue
		}

		if tv.Type == PropertyTypeEdge {
			val, err := tv.ToGo()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to convert edge at column %s (index %d): %w",
					ar.alias, ar.columnIdx, err)
			}
			collectEdge(val, &edges, schemas)
		} else if tv.Type == PropertyTypeList {
			// LIST of EDGE (group variable from quantified pattern)
			val, err := tv.ToGo()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to convert list at column %s (index %d): %w",
					ar.alias, ar.columnIdx, err)
			}
			if items, ok := val.([]interface{}); ok {
				for _, item := range items {
					collectEdge(item, &edges, schemas)
				}
			}
		} else {
			return nil, nil, fmt.Errorf("type mismatch: column %s (index %d) contains %v, expected EDGE or LIST of EDGE",
				ar.alias, ar.columnIdx, tv.Type)
		}
	}

	return edges, schemas, nil
}

// AsPaths extracts paths from the aliased column.
// Paths are identified by PropertyTypePath type in the TypedValue.
// Returns an error if the column contains non-null values that are not paths.
func (ar *AliasResult) AsPaths() ([]*Path, error) {
	paths := make([]*Path, 0)

	// Iterate rows and extract paths from the specified column
	for _, row := range ar.response.Rows {
		// Boundary check
		if ar.columnIdx >= len(row.Values) {
			continue
		}

		tv := row.Values[ar.columnIdx]

		// Skip if null
		if tv.IsNull {
			continue
		}

		// Check type mismatch
		if tv.Type != PropertyTypePath {
			return nil, fmt.Errorf("type mismatch: column %s (index %d) contains %v, expected PATH",
				ar.alias, ar.columnIdx, tv.Type)
		}

		// Convert to Go type
		val, err := tv.ToGo()
		if err != nil {
			return nil, fmt.Errorf("failed to convert path at column %s (index %d): %w",
				ar.alias, ar.columnIdx, err)
		}

		// TypedValue.ToGo returns Path (not *Path) for PropertyTypePath
		if path, ok := val.(Path); ok {
			paths = append(paths, &path)
		}
	}

	return paths, nil
}

// AsTable returns the aliased column as a generic table structure.
// If the column contains a GqldbTable value (from the table() function),
// it is unwrapped into proper Table headers and rows.
func (ar *AliasResult) AsTable() (*Table, error) {
	// Check if first non-null value is a GqldbTable — if so, unwrap it
	for _, row := range ar.response.Rows {
		if ar.columnIdx < len(row.Values) {
			tv := row.Values[ar.columnIdx]
			if !tv.IsNull && tv.Type == PropertyTypeTable {
				val, err := tv.ToGo()
				if err != nil {
					return nil, fmt.Errorf("failed to convert table at column %s (index %d): %w",
						ar.alias, ar.columnIdx, err)
				}
				if gt, ok := val.(GqldbTable); ok {
					return unwrapGqldbTable(ar.alias, gt), nil
				}
			}
		}
		break // only check first row
	}

	// Fallback: generic single-column table
	table := &Table{
		Name:    ar.alias,
		Headers: make([]*Header, 1),
		Rows:    make([][]interface{}, 0),
	}

	propType := PropertyTypeString
	if len(ar.response.Rows) > 0 && ar.columnIdx < len(ar.response.Rows[0].Values) {
		propType = ar.response.Rows[0].Values[ar.columnIdx].Type
	}
	table.Headers[0] = &Header{
		Name: ar.alias,
		Type: propType,
	}

	for _, row := range ar.response.Rows {
		if ar.columnIdx < len(row.Values) {
			val, err := row.Values[ar.columnIdx].ToGo()
			if err != nil {
				return nil, fmt.Errorf("failed to convert value at column %s (index %d): %w",
					ar.alias, ar.columnIdx, err)
			}
			table.Rows = append(table.Rows, []interface{}{val})
		} else {
			table.Rows = append(table.Rows, []interface{}{nil})
		}
	}

	return table, nil
}

func unwrapGqldbTable(alias string, gt GqldbTable) *Table {
	headers := make([]*Header, len(gt.Columns))
	for i, col := range gt.Columns {
		colType := PropertyTypeString
		if len(gt.Rows) > 0 && i < len(gt.Rows[0]) {
			switch gt.Rows[0][i].(type) {
			case int64, int32, int:
				colType = PropertyTypeInt64
			case uint64, uint32:
				colType = PropertyTypeUint64
			case float64, float32:
				colType = PropertyTypeDouble
			case string:
				colType = PropertyTypeString
			case bool:
				colType = PropertyTypeBool
			}
		}
		headers[i] = &Header{Name: col, Type: colType}
	}
	return &Table{Name: alias, Headers: headers, Rows: gt.Rows}
}

// AsAttr extracts the aliased column as an attribute.
func (ar *AliasResult) AsAttr() (*Attr, error) {
	attr := &Attr{
		Name:   ar.alias,
		Values: make([]interface{}, len(ar.response.Rows)),
	}

	// Determine type from first non-null value
	for _, row := range ar.response.Rows {
		if ar.columnIdx < len(row.Values) && !row.Values[ar.columnIdx].IsNull {
			attr.Type = row.Values[ar.columnIdx].Type
			break
		}
	}

	// Extract values
	for i, row := range ar.response.Rows {
		if ar.columnIdx < len(row.Values) {
			val, err := row.Values[ar.columnIdx].ToGo()
			if err != nil {
				return nil, fmt.Errorf("failed to convert value at column %s (index %d), row %d: %w",
					ar.alias, ar.columnIdx, i, err)
			}
			attr.Values[i] = val
		}
	}

	return attr, nil
}

// AsValues extracts raw Go values from the aliased column.
func (ar *AliasResult) AsValues() ([]interface{}, error) {
	values := make([]interface{}, 0)

	for _, row := range ar.response.Rows {
		if ar.columnIdx < len(row.Values) {
			val, err := row.Values[ar.columnIdx].ToGo()
			if err != nil {
				return nil, fmt.Errorf("failed to convert value at column %s (index %d): %w",
					ar.alias, ar.columnIdx, err)
			}
			values = append(values, val)
		}
	}

	return values, nil
}
