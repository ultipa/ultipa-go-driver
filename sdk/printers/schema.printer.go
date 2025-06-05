package printers

import (
	"fmt"
	"log"
	"strconv"

	"github.com/alexeyco/simpletable"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

func PrintSchema(schemas []*structs.Schema) {

	for _, schema := range schemas {
		if schema.Total != 0 {
			fmt.Println(fmt.Sprintf("%sSchema: %v - %v (%d)", structs.DBTypeToString(schema.DBType), schema.Id, schema.Name, schema.Total))
		} else {
			fmt.Println(fmt.Sprintf("%sSchema: %v - %v", structs.DBTypeToString(schema.DBType), schema.Id, schema.Name))
		}
		if schema.Description != "" {
			fmt.Println("Description: ", schema.Description)
		}
		if len(schema.Properties) != 0 {
			fmt.Println("Property: ")
		}

		table := simpletable.New()
		table.Header.Cells = []*simpletable.Cell{{Text: "Name"}, {Text: "Description"}, {Text: "Type"}, {Text: "LTE"}, {Text: "Schema"}}
		//table.Footer.Cells = []*simpletable.Cell{&simpletable.Cell{Span: 4, Text: fmt.Sprint("[", schema.DBType, "]Schema : "+schema.Name, "(", schema.Total, ")")}}

		for _, prop := range schema.Properties {

			propertyTypeStr, err := prop.GetStringType()
			if err != nil {
				log.Panic(err)
			}

			rowCells := []*simpletable.Cell{
				{Text: prop.Name},
				{Text: prop.Description},
				{Text: propertyTypeStr},
				{Text: strconv.FormatBool(prop.Lte)},
				{Text: prop.Schema},
			}

			table.Body.Cells = append(table.Body.Cells, rowCells)
		}

		if len(table.Body.Cells) > 0 {
			table.Println()
		}

		println("-")

	}
}
