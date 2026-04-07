package unit

import (
	"strings"
	"testing"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestPrintNodes_Basic(t *testing.T) {
	nodes := []*gqldb.Node{
		{
			ID:         "n1",
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Alice", "age": int64(30)},
		},
		{
			ID:         "n2",
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Bob", "age": int64(25)},
		},
	}

	result := gqldb.PrintNodesWithoutSchema(nodes)

	if !strings.Contains(result, "n1") {
		t.Error("expected n1 in output")
	}
	if !strings.Contains(result, "n2") {
		t.Error("expected n2 in output")
	}
	if !strings.Contains(result, "Alice") {
		t.Error("expected Alice in output")
	}
	if !strings.Contains(result, "Bob") {
		t.Error("expected Bob in output")
	}
	if !strings.Contains(result, "Person") {
		t.Error("expected Person in output")
	}
	if !strings.Contains(result, "2 node(s)") {
		t.Error("expected '2 node(s)' in output")
	}
}

func TestPrintNodes_Empty(t *testing.T) {
	result := gqldb.PrintNodes(nil, nil)
	if !strings.Contains(result, "No nodes found") {
		t.Error("expected 'No nodes found' in output")
	}

	result = gqldb.PrintNodes([]*gqldb.Node{}, nil)
	if !strings.Contains(result, "No nodes found") {
		t.Error("expected 'No nodes found' in output")
	}
}

func TestPrintNodes_WithSchema(t *testing.T) {
	nodes := []*gqldb.Node{
		{
			ID:         "n1",
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Alice"},
		},
	}

	schemas := map[string]*gqldb.Schema{
		"Person": {
			Name: "Person",
			Properties: []*gqldb.PropertyDef{
				{Name: "name", Type: gqldb.PropertyTypeString},
			},
		},
	}

	result := gqldb.PrintNodes(nodes, schemas)

	if !strings.Contains(result, "n1") {
		t.Error("expected n1 in output")
	}
	if !strings.Contains(result, "Alice") {
		t.Error("expected Alice in output")
	}
	if !strings.Contains(result, "name") {
		t.Error("expected name in output")
	}
}

func TestPrintNodes_DifferentSchemas(t *testing.T) {
	nodes := []*gqldb.Node{
		{
			ID:         "n1",
			Labels:     []string{"Person"},
			Properties: map[string]interface{}{"name": "Alice"},
		},
		{
			ID:         "n2",
			Labels:     []string{"Company"},
			Properties: map[string]interface{}{"company_name": "Acme"},
		},
	}

	result := gqldb.PrintNodesWithoutSchema(nodes)

	if !strings.Contains(result, "Person") {
		t.Error("expected Person in output")
	}
	if !strings.Contains(result, "Company") {
		t.Error("expected Company in output")
	}
}

func TestPrintEdges_Basic(t *testing.T) {
	edges := []*gqldb.Edge{
		{
			ID:         "e1",
			Label:      "KNOWS",
			FromNodeID: "n1",
			ToNodeID:   "n2",
			Properties: map[string]interface{}{"since": int64(2020)},
		},
	}

	result := gqldb.PrintEdgesWithoutSchema(edges)

	if !strings.Contains(result, "e1") {
		t.Error("expected e1 in output")
	}
	if !strings.Contains(result, "KNOWS") {
		t.Error("expected KNOWS in output")
	}
	if !strings.Contains(result, "n1") {
		t.Error("expected n1 in output")
	}
	if !strings.Contains(result, "n2") {
		t.Error("expected n2 in output")
	}
	if !strings.Contains(result, "2020") {
		t.Error("expected 2020 in output")
	}
	if !strings.Contains(result, "1 edge(s)") {
		t.Error("expected '1 edge(s)' in output")
	}
}

func TestPrintEdges_Empty(t *testing.T) {
	result := gqldb.PrintEdges(nil, nil)
	if !strings.Contains(result, "No edges found") {
		t.Error("expected 'No edges found' in output")
	}

	result = gqldb.PrintEdges([]*gqldb.Edge{}, nil)
	if !strings.Contains(result, "No edges found") {
		t.Error("expected 'No edges found' in output")
	}
}

func TestPrintEdges_WithSchema(t *testing.T) {
	edges := []*gqldb.Edge{
		{
			ID:         "e1",
			Label:      "FOLLOWS",
			FromNodeID: "a",
			ToNodeID:   "b",
			Properties: map[string]interface{}{"weight": 0.5},
		},
	}

	schemas := map[string]*gqldb.Schema{
		"FOLLOWS": {
			Name: "FOLLOWS",
			Properties: []*gqldb.PropertyDef{
				{Name: "weight", Type: gqldb.PropertyTypeDouble},
			},
		},
	}

	result := gqldb.PrintEdges(edges, schemas)

	if !strings.Contains(result, "e1") {
		t.Error("expected e1 in output")
	}
	if !strings.Contains(result, "FOLLOWS") {
		t.Error("expected FOLLOWS in output")
	}
	if !strings.Contains(result, "weight") {
		t.Error("expected weight in output")
	}
}

func TestPrintEdges_DifferentSchemas(t *testing.T) {
	edges := []*gqldb.Edge{
		{
			ID:         "e1",
			Label:      "KNOWS",
			FromNodeID: "n1",
			ToNodeID:   "n2",
			Properties: map[string]interface{}{},
		},
		{
			ID:         "e2",
			Label:      "WORKS_AT",
			FromNodeID: "n1",
			ToNodeID:   "n3",
			Properties: map[string]interface{}{},
		},
	}

	result := gqldb.PrintEdgesWithoutSchema(edges)

	if !strings.Contains(result, "KNOWS") {
		t.Error("expected KNOWS in output")
	}
	if !strings.Contains(result, "WORKS_AT") {
		t.Error("expected WORKS_AT in output")
	}
	if !strings.Contains(result, "2 edge(s)") {
		t.Error("expected '2 edge(s)' in output")
	}
}

func TestPrintPaths_Basic(t *testing.T) {
	paths := []*gqldb.Path{
		{
			Nodes: []*gqldb.Node{
				{ID: "n1", Labels: []string{"Person"}, Properties: map[string]interface{}{}},
				{ID: "n2", Labels: []string{"Person"}, Properties: map[string]interface{}{}},
			},
			Edges: []*gqldb.Edge{
				{ID: "e1", Label: "KNOWS", FromNodeID: "n1", ToNodeID: "n2", Properties: map[string]interface{}{}},
			},
		},
	}

	result := gqldb.PrintPaths(paths)

	if !strings.Contains(result, "n1") {
		t.Error("expected n1 in output")
	}
	if !strings.Contains(result, "n2") {
		t.Error("expected n2 in output")
	}
	if !strings.Contains(result, "e1") {
		t.Error("expected e1 in output")
	}
	if !strings.Contains(result, "KNOWS") {
		t.Error("expected KNOWS in output")
	}
	if !strings.Contains(result, "Person") {
		t.Error("expected Person in output")
	}
	if !strings.Contains(result, "1 path(s)") {
		t.Error("expected '1 path(s)' in output")
	}
}

func TestPrintPaths_Empty(t *testing.T) {
	result := gqldb.PrintPaths(nil)
	if !strings.Contains(result, "No paths found") {
		t.Error("expected 'No paths found' in output")
	}

	result = gqldb.PrintPaths([]*gqldb.Path{})
	if !strings.Contains(result, "No paths found") {
		t.Error("expected 'No paths found' in output")
	}
}

func TestPrintPaths_MultiplePaths(t *testing.T) {
	paths := []*gqldb.Path{
		{
			Nodes: []*gqldb.Node{
				{ID: "a", Labels: []string{"A"}, Properties: map[string]interface{}{}},
			},
			Edges: []*gqldb.Edge{},
		},
		{
			Nodes: []*gqldb.Node{
				{ID: "b", Labels: []string{"B"}, Properties: map[string]interface{}{}},
			},
			Edges: []*gqldb.Edge{},
		},
	}

	result := gqldb.PrintPaths(paths)

	if !strings.Contains(result, "2 path(s)") {
		t.Error("expected '2 path(s)' in output")
	}
}

func TestPrintPaths_LongerPath(t *testing.T) {
	paths := []*gqldb.Path{
		{
			Nodes: []*gqldb.Node{
				{ID: "n1", Labels: []string{"A"}, Properties: map[string]interface{}{}},
				{ID: "n2", Labels: []string{"B"}, Properties: map[string]interface{}{}},
				{ID: "n3", Labels: []string{"C"}, Properties: map[string]interface{}{}},
			},
			Edges: []*gqldb.Edge{
				{ID: "e1", Label: "R1", FromNodeID: "n1", ToNodeID: "n2", Properties: map[string]interface{}{}},
				{ID: "e2", Label: "R2", FromNodeID: "n2", ToNodeID: "n3", Properties: map[string]interface{}{}},
			},
		},
	}

	result := gqldb.PrintPaths(paths)

	if !strings.Contains(result, "n1") {
		t.Error("expected n1 in output")
	}
	if !strings.Contains(result, "n2") {
		t.Error("expected n2 in output")
	}
	if !strings.Contains(result, "n3") {
		t.Error("expected n3 in output")
	}
	if !strings.Contains(result, "e1") {
		t.Error("expected e1 in output")
	}
	if !strings.Contains(result, "e2") {
		t.Error("expected e2 in output")
	}
}

func TestPrintTable_Basic(t *testing.T) {
	table := &gqldb.Table{
		Name: "TestTable",
		Headers: []*gqldb.Header{
			{Name: "col1", Type: gqldb.PropertyTypeString},
			{Name: "col2", Type: gqldb.PropertyTypeInt64},
		},
		Rows: [][]interface{}{
			{"value1", int64(100)},
			{"value2", int64(200)},
		},
	}

	result := gqldb.PrintTable(table)

	if !strings.Contains(result, "col1") {
		t.Error("expected col1 in output")
	}
	if !strings.Contains(result, "col2") {
		t.Error("expected col2 in output")
	}
	if !strings.Contains(result, "value1") {
		t.Error("expected value1 in output")
	}
	if !strings.Contains(result, "value2") {
		t.Error("expected value2 in output")
	}
	if !strings.Contains(result, "100") {
		t.Error("expected 100 in output")
	}
	if !strings.Contains(result, "200") {
		t.Error("expected 200 in output")
	}
	if !strings.Contains(result, "2 row(s)") {
		t.Error("expected '2 row(s)' in output")
	}
	if !strings.Contains(result, "TestTable") {
		t.Error("expected TestTable in output")
	}
}

func TestPrintTable_Empty(t *testing.T) {
	result := gqldb.PrintTable(nil)
	if !strings.Contains(result, "No table data") {
		t.Error("expected 'No table data' in output")
	}

	table := &gqldb.Table{
		Headers: []*gqldb.Header{},
		Rows:    [][]interface{}{},
	}
	result = gqldb.PrintTable(table)
	if !strings.Contains(result, "No table data") {
		t.Error("expected 'No table data' in output")
	}
}

func TestPrintTable_NilValues(t *testing.T) {
	table := &gqldb.Table{
		Name: "",
		Headers: []*gqldb.Header{
			{Name: "value", Type: gqldb.PropertyTypeString},
		},
		Rows: [][]interface{}{
			{nil},
		},
	}

	result := gqldb.PrintTable(table)
	if !strings.Contains(result, "<nil>") {
		t.Error("expected '<nil>' in output")
	}
}

func TestPrintTable_ArrayValues(t *testing.T) {
	table := &gqldb.Table{
		Name: "",
		Headers: []*gqldb.Header{
			{Name: "arr", Type: gqldb.PropertyTypeList},
		},
		Rows: [][]interface{}{
			{[]interface{}{int64(1), int64(2), int64(3)}},
		},
	}

	result := gqldb.PrintTable(table)
	if !strings.Contains(result, "[1, 2, 3]") {
		t.Error("expected '[1, 2, 3]' in output")
	}
}

func TestPrintTable_MapValues(t *testing.T) {
	table := &gqldb.Table{
		Name: "",
		Headers: []*gqldb.Header{
			{Name: "obj", Type: gqldb.PropertyTypeMap},
		},
		Rows: [][]interface{}{
			{map[string]interface{}{"key": "val"}},
		},
	}

	result := gqldb.PrintTable(table)
	if !strings.Contains(result, "key") || !strings.Contains(result, "val") {
		t.Error("expected key and val in output")
	}
}

func TestPrintAny_DetectsNodes(t *testing.T) {
	tvID, _ := gqldb.NewTypedValue("n1")
	tvLabels, _ := gqldb.NewTypedValue([]interface{}{"Person"})
	tvName, _ := gqldb.NewTypedValue("Alice")

	resp := gqldb.NewResponse(
		[]string{"id", "labels", "name"},
		[]*gqldb.Row{
			gqldb.NewRow([]*gqldb.TypedValue{tvID, tvLabels, tvName}),
		},
		1,
		false,
		nil,
		0,
	)

	result := gqldb.PrintAny(resp)

	if !strings.Contains(result, "n1") {
		t.Error("expected n1 in output")
	}
	if !strings.Contains(result, "Person") {
		t.Error("expected Person in output")
	}
}

func TestPrintAny_DetectsEdges(t *testing.T) {
	tvID, _ := gqldb.NewTypedValue("e1")
	tvLabel, _ := gqldb.NewTypedValue("KNOWS")
	tvFrom, _ := gqldb.NewTypedValue("n1")
	tvTo, _ := gqldb.NewTypedValue("n2")

	resp := gqldb.NewResponse(
		[]string{"id", "label", "from", "to"},
		[]*gqldb.Row{
			gqldb.NewRow([]*gqldb.TypedValue{tvID, tvLabel, tvFrom, tvTo}),
		},
		1,
		false,
		nil,
		0,
	)

	result := gqldb.PrintAny(resp)

	if !strings.Contains(result, "e1") {
		t.Error("expected e1 in output")
	}
	if !strings.Contains(result, "KNOWS") {
		t.Error("expected KNOWS in output")
	}
}

func TestPrintAny_EmptyResponse(t *testing.T) {
	resp := gqldb.NewResponse([]string{"col"}, nil, 0, false, nil, 0)

	result := gqldb.PrintAny(resp)

	if !strings.Contains(result, "No data") {
		t.Error("expected 'No data' in output")
	}
}

func TestPrintAny_FallbackToTable(t *testing.T) {
	tvX, _ := gqldb.NewTypedValue(int64(1))
	tvY, _ := gqldb.NewTypedValue(int64(2))

	resp := gqldb.NewResponse(
		[]string{"x", "y"},
		[]*gqldb.Row{
			gqldb.NewRow([]*gqldb.TypedValue{tvX, tvY}),
		},
		1,
		false,
		nil,
		0,
	)

	result := gqldb.PrintAny(resp)

	if !strings.Contains(result, "x") {
		t.Error("expected x in output")
	}
	if !strings.Contains(result, "y") {
		t.Error("expected y in output")
	}
}
