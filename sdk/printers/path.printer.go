package printers

import (
	"fmt"

	"github.com/alexeyco/simpletable"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func PrintPaths(paths []*structs.Path) {
	if len(paths) == 0 {
		fmt.Println("No path data found.")
		return
	}
	fmt.Println(getPathTableString(paths))
}

func getPathTableString(paths []*structs.Path) string {
	table := simpletable.New()

	table.Header.Cells = []*simpletable.Cell{
		{Text: "#"},
		{Text: "Path"},
	}

	for num, path := range paths {

		//log.Println(path.Nodes)
		row := []*simpletable.Cell{{Text: fmt.Sprint(num)}}
		pathString := ""
		for index, edge := range path.GetEdges() {
			node := path.GetNodes()[index]
			if edge == 0 || node == 0 {
				pathString = "nil"
				continue
			}
			d1 := "-"
			d2 := "-"

			//if edge.FromUUID == node.UUID {
			//	d2 = "->"
			//} else {
			//	d1 = "<-"
			//}

			pathString += fmt.Sprintf("(%v) %v [%v] %v ", node, d1, edge, d2)
		}
		if path.GetLastNode() != 0 {
			pathString += fmt.Sprintf("(%v)", path.GetLastNode())
		}
		row = append(row, &simpletable.Cell{Text: pathString})
		table.Body.Cells = append(table.Body.Cells, row)
	}

	return table.String()
}
