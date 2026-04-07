package gqldb

import (
	"fmt"
	"strings"

	"github.com/alexeyco/simpletable"
)

// PrintNodes prints nodes in tabular format grouped by schema.
// Following the ultipa-go-sdk pattern.
func PrintNodes(nodes []*Node, schemas map[string]*Schema) string {
	if len(nodes) == 0 {
		return "No nodes found."
	}

	var result strings.Builder
	var lastSchema string
	var table *simpletable.Table

	for _, node := range nodes {
		if node == nil {
			continue
		}

		// Get the first label as schema name
		schemaName := ""
		if len(node.Labels) > 0 {
			schemaName = node.Labels[0]
		}

		// Switch schema if needed
		if schemaName != lastSchema {
			// Print previous table if exists
			if table != nil {
				result.WriteString(table.String())
				result.WriteString("\n")
			}

			// Create new table
			table = simpletable.New()
			table.Header = buildNodeHeader(schemaName, schemas)
			lastSchema = schemaName
		}

		// Add row
		if table != nil {
			row := buildNodeRow(node, schemaName, schemas)
			table.Body.Cells = append(table.Body.Cells, row)
		}
	}

	// Print last table
	if table != nil {
		result.WriteString(table.String())
	}

	result.WriteString(fmt.Sprintf("\n%d node(s)\n", len(nodes)))
	return result.String()
}

// PrintNodesWithoutSchema prints nodes deriving schema from data.
func PrintNodesWithoutSchema(nodes []*Node) string {
	if len(nodes) == 0 {
		return "No nodes found."
	}

	// Build schemas from nodes
	schemas := make(map[string]*Schema)
	for _, node := range nodes {
		if node == nil {
			continue
		}
		for _, label := range node.Labels {
			if _, exists := schemas[label]; !exists {
				schemas[label] = buildSchemaFromNode(label, node)
			}
		}
	}

	return PrintNodes(nodes, schemas)
}

// PrintEdges prints edges in tabular format grouped by schema.
func PrintEdges(edges []*Edge, schemas map[string]*Schema) string {
	if len(edges) == 0 {
		return "No edges found."
	}

	var result strings.Builder
	var lastSchema string
	var table *simpletable.Table

	for _, edge := range edges {
		if edge == nil {
			continue
		}

		schemaName := edge.Label

		// Switch schema if needed
		if schemaName != lastSchema {
			// Print previous table if exists
			if table != nil {
				result.WriteString(table.String())
				result.WriteString("\n")
			}

			// Create new table
			table = simpletable.New()
			table.Header = buildEdgeHeader(schemaName, schemas)
			lastSchema = schemaName
		}

		// Add row
		if table != nil {
			row := buildEdgeRow(edge, schemaName, schemas)
			table.Body.Cells = append(table.Body.Cells, row)
		}
	}

	// Print last table
	if table != nil {
		result.WriteString(table.String())
	}

	result.WriteString(fmt.Sprintf("\n%d edge(s)\n", len(edges)))
	return result.String()
}

// PrintEdgesWithoutSchema prints edges deriving schema from data.
func PrintEdgesWithoutSchema(edges []*Edge) string {
	if len(edges) == 0 {
		return "No edges found."
	}

	// Build schemas from edges
	schemas := make(map[string]*Schema)
	for _, edge := range edges {
		if edge == nil {
			continue
		}
		if edge.Label != "" {
			if _, exists := schemas[edge.Label]; !exists {
				schemas[edge.Label] = buildSchemaFromEdge(edge.Label, edge)
			}
		}
	}

	return PrintEdges(edges, schemas)
}

// PrintPaths prints paths with node-edge sequence notation.
func PrintPaths(paths []*Path) string {
	if len(paths) == 0 {
		return "No paths found."
	}

	table := simpletable.New()
	table.Header = &simpletable.Header{
		Cells: []*simpletable.Cell{
			{Align: simpletable.AlignCenter, Text: "#"},
			{Align: simpletable.AlignCenter, Text: "Path"},
		},
	}

	for i, path := range paths {
		if path == nil {
			continue
		}

		pathStr := formatPath(path)
		table.Body.Cells = append(table.Body.Cells, []*simpletable.Cell{
			{Align: simpletable.AlignCenter, Text: fmt.Sprintf("%d", i)},
			{Align: simpletable.AlignLeft, Text: pathStr},
		})
	}

	return table.String() + fmt.Sprintf("\n%d path(s)\n", len(paths))
}

// PrintTable prints generic table data.
func PrintTable(t *Table) string {
	if t == nil || len(t.Headers) == 0 {
		return "No table data."
	}

	table := simpletable.New()

	// Build header
	headerCells := make([]*simpletable.Cell, len(t.Headers))
	for i, h := range t.Headers {
		headerCells[i] = &simpletable.Cell{
			Align: simpletable.AlignCenter,
			Text:  h.Name,
		}
	}
	table.Header = &simpletable.Header{Cells: headerCells}

	// Build rows
	for _, row := range t.Rows {
		cells := make([]*simpletable.Cell, len(row))
		for i, val := range row {
			cells[i] = &simpletable.Cell{
				Align: simpletable.AlignCenter,
				Text:  formatValue(val),
			}
		}
		table.Body.Cells = append(table.Body.Cells, cells)
	}

	var result strings.Builder
	if t.Name != "" {
		result.WriteString(fmt.Sprintf("Table: %s\n", t.Name))
	}
	result.WriteString(table.String())
	result.WriteString(fmt.Sprintf("\n%d row(s)\n", len(t.Rows)))
	return result.String()
}

// PrintAny auto-detects type from Response and prints accordingly.
func PrintAny(resp *Response) string {
	if resp == nil || resp.IsEmpty() {
		return "No data."
	}

	// Try to detect nodes/edges by checking all columns
	// First try to extract nodes from the first column
	if len(resp.Columns) > 0 {
		ar, err := resp.Get(0)
		if err == nil {
			// Try nodes first
			nodes, schemas, err := ar.AsNodes()
			if err == nil && len(nodes) > 0 {
				return PrintNodes(nodes, schemas)
			}

			// Try edges
			edges, schemas, err := ar.AsEdges()
			if err == nil && len(edges) > 0 {
				return PrintEdges(edges, schemas)
			}
		}
	}

	// Default to table format - print all columns as table
	// We need to build a table manually since we don't have AsTable() on Response anymore
	table := &Table{
		Name:    "result",
		Headers: make([]*Header, len(resp.Columns)),
		Rows:    make([][]interface{}, len(resp.Rows)),
	}

	// Build headers
	for i, col := range resp.Columns {
		propType := PropertyTypeString // Default to string
		if len(resp.Rows) > 0 && i < len(resp.Rows[0].Values) {
			propType = resp.Rows[0].Values[i].Type
		}
		table.Headers[i] = &Header{
			Name: col,
			Type: propType,
		}
	}

	// Build rows
	for i, row := range resp.Rows {
		table.Rows[i] = make([]interface{}, len(row.Values))
		for j, tv := range row.Values {
			val, err := tv.ToGo()
			if err != nil {
				return fmt.Sprintf("Error: %v", err)
			}
			table.Rows[i][j] = val
		}
	}

	return PrintTable(table)
}

// Helper functions

func buildNodeHeader(schemaName string, schemas map[string]*Schema) *simpletable.Header {
	cells := []*simpletable.Cell{
		{Align: simpletable.AlignCenter, Text: "ID"},
		{Align: simpletable.AlignCenter, Text: "Labels"},
	}

	// Add property columns from schema
	if schema, ok := schemas[schemaName]; ok && schema != nil {
		for _, prop := range schema.Properties {
			cells = append(cells, &simpletable.Cell{
				Align: simpletable.AlignCenter,
				Text:  prop.Name,
			})
		}
	}

	return &simpletable.Header{Cells: cells}
}

func buildNodeRow(node *Node, schemaName string, schemas map[string]*Schema) []*simpletable.Cell {
	cells := []*simpletable.Cell{
		{Align: simpletable.AlignCenter, Text: node.ID},
		{Align: simpletable.AlignCenter, Text: formatLabels(node.Labels)},
	}

	// Add property values
	if schema, ok := schemas[schemaName]; ok && schema != nil {
		for _, prop := range schema.Properties {
			val := node.Properties[prop.Name]
			cells = append(cells, &simpletable.Cell{
				Align: simpletable.AlignCenter,
				Text:  formatValue(val),
			})
		}
	}

	return cells
}

func buildEdgeHeader(schemaName string, schemas map[string]*Schema) *simpletable.Header {
	cells := []*simpletable.Cell{
		{Align: simpletable.AlignCenter, Text: "ID"},
		{Align: simpletable.AlignCenter, Text: "FROM"},
		{Align: simpletable.AlignCenter, Text: "TO"},
		{Align: simpletable.AlignCenter, Text: "Label"},
	}

	// Add property columns from schema
	if schema, ok := schemas[schemaName]; ok && schema != nil {
		for _, prop := range schema.Properties {
			cells = append(cells, &simpletable.Cell{
				Align: simpletable.AlignCenter,
				Text:  prop.Name,
			})
		}
	}

	return &simpletable.Header{Cells: cells}
}

func buildEdgeRow(edge *Edge, schemaName string, schemas map[string]*Schema) []*simpletable.Cell {
	cells := []*simpletable.Cell{
		{Align: simpletable.AlignCenter, Text: edge.ID},
		{Align: simpletable.AlignCenter, Text: edge.FromNodeID},
		{Align: simpletable.AlignCenter, Text: edge.ToNodeID},
		{Align: simpletable.AlignCenter, Text: edge.Label},
	}

	// Add property values
	if schema, ok := schemas[schemaName]; ok && schema != nil {
		for _, prop := range schema.Properties {
			val := edge.Properties[prop.Name]
			cells = append(cells, &simpletable.Cell{
				Align: simpletable.AlignCenter,
				Text:  formatValue(val),
			})
		}
	}

	return cells
}

func formatPath(path *Path) string {
	if path == nil {
		return "<nil>"
	}

	var parts []string
	nodeIdx := 0
	edgeIdx := 0

	// Alternate between nodes and edges
	for nodeIdx < len(path.Nodes) || edgeIdx < len(path.Edges) {
		if nodeIdx < len(path.Nodes) {
			node := path.Nodes[nodeIdx]
			label := ""
			if len(node.Labels) > 0 {
				label = ":" + node.Labels[0]
			}
			parts = append(parts, fmt.Sprintf("(%s%s)", node.ID, label))
			nodeIdx++
		}

		if edgeIdx < len(path.Edges) {
			edge := path.Edges[edgeIdx]
			parts = append(parts, fmt.Sprintf("-[%s:%s]->", edge.ID, edge.Label))
			edgeIdx++
		}
	}

	return strings.Join(parts, " ")
}

func formatLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	return "[" + strings.Join(labels, ", ") + "]"
}

func formatValue(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	switch val := v.(type) {
	case []interface{}:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatValue(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		parts := make([]string, 0, len(val))
		for k, v := range val {
			parts = append(parts, fmt.Sprintf("%s: %s", k, formatValue(v)))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func containsAny(slice []string, items ...string) bool {
	for _, s := range slice {
		for _, item := range items {
			if s == item {
				return true
			}
		}
	}
	return false
}
