package printers

import (
	"fmt"

	"github.com/alexeyco/simpletable"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

func PrintArray(arr *structs.Array) {
	table := simpletable.New()

	table.Header.Cells = []*simpletable.Cell{
		{
			Text: arr.Name,
		},
	}

	for _, row := range arr.Rows {

		table.Body.Cells = append(table.Body.Cells, []*simpletable.Cell{
			{
				Text: fmt.Sprint(*row),
			},
		})
	}

	table.Println()
}
