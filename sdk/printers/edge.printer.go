package printers

import (
	"fmt"

	"github.com/alexeyco/simpletable"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func PrintEdges(edges []*structs.Edge, schemas map[string]*structs.Schema) {
	if len(edges) == 0 {
		fmt.Println("No edge data found.")
		return
	}
	fmt.Println(getEdgeTableString(edges, schemas))
}

func PrintEdgesWithoutSchema(edges []*structs.Edge) {
	if len(edges) == 0 {
		fmt.Println("No edge data found.")
		return
	}
	fmt.Println(getEdgeTableStringWithoutSchema(edges))
}

func getEdgeTableStringWithoutSchema(edges []*structs.Edge) string {
	schemaMap := structs.GetSchemasOfEdgeList(edges)
	if schemaMap == nil {
		schemaMap = map[string]*structs.Schema{}
	}
	return getEdgeTableString(edges, schemaMap)
}

func getEdgeTableString(edges []*structs.Edge, schemas map[string]*structs.Schema) string {
	var lastSchemaName string
	var table *simpletable.Table
	switchSchema := false
	for _, edge := range edges {
		schemaName := ""
		if edge != nil {
			schemaName = edge.Schema
		}
		schema := schemas[schemaName]
		if schemaName != lastSchemaName {
			switchSchema = true
			lastSchemaName = edge.Schema
		} else {
			switchSchema = false
		}

		if table != nil && switchSchema {
			fmt.Println(table.String())
			table = nil
			switchSchema = false
		}
		if table == nil {
			table = simpletable.New()
			table.Header.Cells = append(table.Header.Cells,
				&simpletable.Cell{Align: simpletable.AlignCenter, Text: "UUID"},
				&simpletable.Cell{Align: simpletable.AlignCenter, Text: "FROM_UUID"},
				&simpletable.Cell{Align: simpletable.AlignCenter, Text: "FROM"},
				&simpletable.Cell{Align: simpletable.AlignCenter, Text: "TO_UUID"},
				&simpletable.Cell{Align: simpletable.AlignCenter, Text: "TO"},
				&simpletable.Cell{Align: simpletable.AlignCenter, Text: "SCHEMA"})
			if schema != nil {
				for _, prop := range schema.Properties {
					table.Header.Cells = append(table.Header.Cells, &simpletable.Cell{Align: simpletable.AlignCenter, Text: prop.Name})
				}
			}
		}
		var row []*simpletable.Cell
		if edge != nil {
			row = []*simpletable.Cell{
				{Align: simpletable.AlignCenter, Text: fmt.Sprint(edge.GetUUID())},
				{Align: simpletable.AlignCenter, Text: fmt.Sprint(edge.FromUUID)},
				{Align: simpletable.AlignCenter, Text: fmt.Sprint(edge.GetFrom())},
				{Align: simpletable.AlignCenter, Text: fmt.Sprint(edge.ToUUID)},
				{Align: simpletable.AlignCenter, Text: fmt.Sprint(edge.GetTo())},
				{Align: simpletable.AlignCenter, Text: fmt.Sprint(edge.GetSchema())},
			}

			for i := 6; i < len(table.Header.Cells); i++ {

				headerKey := table.Header.Cells[i].Text
				vv := edge.Values.Get(headerKey)
				row = append(row, &simpletable.Cell{Align: simpletable.AlignCenter, Text: fmt.Sprint(vv)})
			}
		} else {
			row = []*simpletable.Cell{
				{Align: simpletable.AlignCenter, Text: "nil"},
				{Align: simpletable.AlignCenter, Text: "nil"},
				{Align: simpletable.AlignCenter, Text: "nil"},
				{Align: simpletable.AlignCenter, Text: "nil"},
				{Align: simpletable.AlignCenter, Text: "nil"},
				{Align: simpletable.AlignCenter, Text: "nil"},
			}
		}
		table.Body.Cells = append(table.Body.Cells, row)
	}

	if table != nil {
		return table.String()
	}
	return ""
}
