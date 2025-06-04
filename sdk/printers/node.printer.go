package printers

import (
	"fmt"

	"github.com/alexeyco/simpletable"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func PrintNodes(nodes []*structs.Node, schemas map[string]*structs.Schema) {
	if len(nodes) == 0 {
		fmt.Println("No node data found.")
		return
	}
	fmt.Println(getNodeTableString(nodes, schemas))
}

func PrintNodesWithoutSchema(nodes []*structs.Node) {
	if len(nodes) == 0 {
		fmt.Println("No node data found.")
		return
	}
	fmt.Println(getNodeTableStringWithoutSchema(nodes))
}

func getNodeTableStringWithoutSchema(nodes []*structs.Node) string {

	schemaMap := structs.GetSchemasOfNodeList(nodes)
	if schemaMap == nil {
		schemaMap = map[string]*structs.Schema{}
	}
	return getNodeTableString(nodes, schemaMap)
}

func getNodeTableString(nodes []*structs.Node, schemas map[string]*structs.Schema) string {
	var lastSchemaName string
	var table *simpletable.Table
	switchSchema := false
	for _, node := range nodes {
		schemaName := ""
		if node != nil {
			schemaName = node.Schema
		}
		schema := schemas[schemaName]
		if schemaName != lastSchemaName {
			switchSchema = true
			lastSchemaName = schemaName
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
				&simpletable.Cell{Align: simpletable.AlignCenter, Text: "ID"},
				&simpletable.Cell{Align: simpletable.AlignCenter, Text: "UUID"},
				&simpletable.Cell{Align: simpletable.AlignCenter, Text: "Schema"})
			if schema != nil {
				for _, prop := range schema.Properties {
					table.Header.Cells = append(table.Header.Cells, &simpletable.Cell{Align: simpletable.AlignCenter, Text: prop.Name})
				}
			}

		}
		var row []*simpletable.Cell
		if node != nil {
			row = []*simpletable.Cell{
				{Align: simpletable.AlignCenter, Text: node.GetID()},
				{Align: simpletable.AlignCenter, Text: fmt.Sprint(node.GetUUID())},
				{Align: simpletable.AlignCenter, Text: fmt.Sprint(node.GetSchema())},
			}

			for i := 3; i < len(table.Header.Cells); i++ {

				headerKey := table.Header.Cells[i].Text
				vv := node.Values.Get(headerKey)
				row = append(row, &simpletable.Cell{Align: simpletable.AlignCenter, Text: fmt.Sprint(vv)})
			}
		} else {
			row = []*simpletable.Cell{
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
