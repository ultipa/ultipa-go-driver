package printers

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/alexeyco/simpletable"
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils/logger"
)

func PrintAttr(attr *structs.Attr) {
	if attr == nil {
		fmt.Println("No attr data found.")
		return
	}
	switch attr.PropertyType {
	case ultipa.PropertyType_SET:
		fallthrough
	case ultipa.PropertyType_LIST:
		switch attr.ResultType {
		case ultipa.ResultType_RESULT_TYPE_NODE:
			for _, row := range attr.Values {
				if row == nil {
					continue
				}
				attrNodes := row.(*structs.AttrNodes)
				PrintAttrNodes(attrNodes)
			}
			return
		case ultipa.ResultType_RESULT_TYPE_EDGE:
			for _, row := range attr.Values {
				if row == nil {
					continue
				}
				attrEdges := row.(*structs.AttrEdges)
				PrintAttrEdges(attrEdges)
			}
			return
		case ultipa.ResultType_RESULT_TYPE_PATH:
			for _, row := range attr.Values {
				if row == nil {
					continue
				}
				attrPaths := row.(*structs.AttrPaths)
				PrintAttrPaths(attrPaths)
			}
			return
		default:
			printSimpleAttr(attr)
			return
		}

	default:
		printSimpleAttr(attr)
	}
}

// PrintAttrNodes print Attr with values as List<List<Node>>
func PrintAttrNodes(attrNodes *structs.AttrNodes) {
	if attrNodes.NodesList == nil {
		logger.PrintInfo(fmt.Sprintf("Alias:%s, DBType:%s, resultType:%s, nodes is null.\r\n", attrNodes.Name, ultipa.PropertyType_LIST, attrNodes.ResultType))
		return
	}
	for i, nodes := range attrNodes.NodesList {
		logger.PrintInfo(fmt.Sprintf("Alias:%s, DBType:%s, resultType:%s\r\nIndex:%d", attrNodes.Name, ultipa.PropertyType_LIST, attrNodes.ResultType, i))
		PrintNodesWithoutSchema(nodes)
	}
}

// PrintAttrEdges print Attr with values as List<List<Edge>>
func PrintAttrEdges(attrEdges *structs.AttrEdges) {
	if attrEdges.EdgesList == nil {
		logger.PrintInfo(fmt.Sprintf("Alias:%s, DBType:%s, resultType:%s, edges is null.\r\n", attrEdges.Name, ultipa.PropertyType_LIST, attrEdges.ResultType))
		return
	}
	for i, edges := range attrEdges.EdgesList {
		logger.PrintInfo(fmt.Sprintf("Alias:%s, DBType:%s, resultType:%s\r\nIndex:%d", attrEdges.Name, ultipa.PropertyType_LIST, attrEdges.ResultType, i))
		PrintEdgesWithoutSchema(edges)
	}
}

// PrintAttrPaths print Attr with values as List<List<Path>>
func PrintAttrPaths(attrPaths *structs.AttrPaths) {
	if attrPaths.PathsList == nil {
		logger.PrintInfo(fmt.Sprintf("Alias:%s, DBType:%s, resultType:%s, paths is null.\r\n", attrPaths.Name, ultipa.PropertyType_LIST, attrPaths.ResultType))
		return
	}
	for i, paths := range attrPaths.PathsList {
		logger.PrintInfo(fmt.Sprintf("Alias:%s, DBType:%s, resultType:%s\r\nIndex:%d", attrPaths.Name, ultipa.PropertyType_LIST, attrPaths.ResultType, i))
		PrintPaths(paths)
	}
}

func printSimpleAttr(attr *structs.Attr) {
	table := simpletable.New()

	table.Header.Cells = []*simpletable.Cell{
		{
			Text: attr.Name,
		},
	}
	stringList := getAttrStr(attr)
	for _, str := range stringList {
		table.Body.Cells = append(table.Body.Cells, []*simpletable.Cell{
			{
				Text: str,
			},
		})
	}
	table.Println()
}

func getAttrStr(attr *structs.Attr) []string {
	var result []string
	for _, row := range attr.Values {
		if row == nil {
			result = append(result, "<nil>")
		} else {
			result = append(result, formatRow(row))
		}
	}
	return result
}

func formatRow(row interface{}) string {
	if row == nil {
		return "<nil>"
	} else {
		value := reflect.ValueOf(row)
		switch value.Type().Kind() {
		case reflect.Array, reflect.Slice:
			if value.Len() == 0 {
				return fmt.Sprintf("%v", row)
			} else if value.Len() == 1 && value.Index(0).Interface() == nil {
				// A list has and only has a nil element, will return [null]
				return "[<nil>]"
			} else {
				var subStrs []string
				for index := 0; index < value.Len(); index++ {
					subStr := formatRow(value.Index(index).Interface())
					subStrs = append(subStrs, subStr)
				}
				return "[" + strings.Join(subStrs, ", ") + "]"
			}

		default:
			return fmt.Sprintf("%v", row)
		}
	}
}

// getAttrStrWithList @Deprecated
func getAttrStrWithList(attr *structs.Attr) []string {
	switch attr.PropertyType {
	case ultipa.PropertyType_NULL_:
		return []string{}
	case ultipa.PropertyType_LIST:
		var result []string
		for _, row := range attr.Values {
			result = append(result, getAttrListCellString(row.(*structs.AttrListData)))
		}
		return result
	default:
		var result []string
		for _, row := range attr.Values {
			result = append(result, fmt.Sprintf("%v", row))
		}
		return result
	}
}

// getAttrStrWithList @Deprecated
func getAttrListCellString(attrListData *structs.AttrListData) string {
	switch attrListData.ResultType {
	case ultipa.ResultType_RESULT_TYPE_NODE:
		return getNodeTableStringWithoutSchema(attrListData.Nodes)
	case ultipa.ResultType_RESULT_TYPE_PATH:
		return getPathTableString(attrListData.Paths)
	case ultipa.ResultType_RESULT_TYPE_EDGE:
		return getEdgeTableStringWithoutSchema(attrListData.Edges)
	case ultipa.ResultType_RESULT_TYPE_ATTR:
		var result []string
		for _, subAttr := range attrListData.Attrs {
			subStrList := getAttrStr(subAttr)
			result = append(result, strings.Join(subStrList, ","))
		}
		return strings.Join(result, ",")
	}
	return ""
}
