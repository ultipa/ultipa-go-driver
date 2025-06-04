package printers

import (
	"log"
	"strconv"

	"github.com/alexeyco/simpletable"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func PrintProperty(properties []*structs.Property) {

	table := simpletable.New()
	table.Header.Cells = []*simpletable.Cell{{Text: "Name"}, {Text: "Description"},
		{Text: "DBType"}, {Text: "LTE"},
		{Text: "READ"}, {Text: "WRITE"},
		{Text: "Schema"}, {Text: "DecimalExtra"},
		{Text: "Encrypt"},
	}

	for _, prop := range properties {
		if prop == nil {
			continue
		}
		propertyTypeStr, err := prop.GetStringType()
		if err != nil {
			log.Panic(err)
		}

		var exta = prop.DecimalExtra
		extaStr, err := structs.MarshalDecimalExtra(exta)
		if err != nil {
			log.Panic(err)
		}

		cells := []*simpletable.Cell{
			{Text: prop.Name},
			{Text: prop.Description},
			{Text: propertyTypeStr},
			{Text: strconv.FormatBool(prop.Lte)},
			{Text: strconv.FormatBool(prop.Read)},
			{Text: strconv.FormatBool(prop.Write)},
			{Text: prop.Schema},
			{Text: string(extaStr)},
			{Text: prop.Encrypt},
		}
		table.Body.Cells = append(table.Body.Cells, cells)
	}
	if len(table.Body.Cells) > 0 {
		table.Println()
	}
}
