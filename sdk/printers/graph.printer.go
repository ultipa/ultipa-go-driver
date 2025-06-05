package printers

import (
	"strconv"

	"github.com/alexeyco/simpletable"
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

//func PrintGraph(graph *structs.Graph) {
//	if graph == nil {
//		fmt.Println("No graph data found.")
//		return
//	}
//	fmt.Println(getSchemaNameContent(ultipa.DBType_DBNODE, graph.Edges))
//	fmt.Println()
//	fmt.Println()
//	fmt.Println(getSchemaNameContent(ultipa.DBType_DBEDGE, graph.EdgeSchemas))
//
//	fmt.Println(getNodeTableString(graph.Nodes, graph.NodeSchemas))
//	fmt.Println(getEdgeTableString(graph.Edges, graph.EdgeSchemas))
//}

func getSchemaNameContent(dbType ultipa.DBType, schemas map[string]*structs.Schema) string {
	table := simpletable.New()
	if ultipa.DBType_DBNODE == dbType {
		table.Header.Cells = []*simpletable.Cell{{Text: "#"}, {Text: "Node schemas"}}

	} else {
		table.Header.Cells = []*simpletable.Cell{{Text: "#"}, {Text: "Edge schemas"}}
	}
	i := 1
	for _, schema := range schemas {
		table.Body.Cells = append(table.Body.Cells, []*simpletable.Cell{
			{Text: strconv.Itoa(i)},
			{Text: schema.Name},
		})
		i++
	}
	return table.String()
}
