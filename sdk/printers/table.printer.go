package printers

import (
	"fmt"

	"github.com/alexeyco/simpletable"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func PrintTable(tableData *structs.Table) {
	if tableData == nil {
		fmt.Println("No table data found.")
		return
	}

	table := simpletable.New()
	fmt.Println("Table name: ", tableData.Name)
	headers := tableData.GetHeaders()
	for _, header := range headers {
		table.Header.Cells = append(table.Header.Cells, &simpletable.Cell{Align: simpletable.AlignCenter, Text: header.Name})
	}

	for _, row := range tableData.GetRows() {
		r := []*simpletable.Cell{}
		if len(*row) != len(headers) {
			fmt.Printf("table row(%d) not equal to table head(%d) .\n", len(*row), len(headers))
			return
		}
		for _, field := range *row {
			r = append(r, &simpletable.Cell{Align: simpletable.AlignCenter, Text: fmt.Sprint(field)})
		}

		table.Body.Cells = append(table.Body.Cells, r)
	}

	table.Println()
}
