package utils

import (
	"fmt"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
)

func CheckAliasExist(reply *ultipa.QueryReply, alias string) bool {
	for _, a := range reply.Alias {
		if a.Alias == alias {
			return true
		}
	}

	return false
}

func FindAliasDataInReply(reply *ultipa.QueryReply, alias string) (data interface{}, t ultipa.ResultType) {

	if CheckAliasExist(reply, alias) == false {
		return nil, ultipa.ResultType_RESULT_TYPE_UNSET
	}

	for _, Alias := range reply.Alias {
		switch Alias.ResultType {
		case ultipa.ResultType_RESULT_TYPE_NODE:
			data = Find(reply.Nodes, func(index int) bool { return reply.Nodes[index].Alias == alias })
			t = Alias.ResultType
		case ultipa.ResultType_RESULT_TYPE_EDGE:
			data = Find(reply.Edges, func(index int) bool { return reply.Edges[index].Alias == alias })
			t = Alias.ResultType
		case ultipa.ResultType_RESULT_TYPE_TABLE:
			data = Find(reply.Tables, func(index int) bool { return reply.Tables[index].TableName == alias })
			t = Alias.ResultType
		case ultipa.ResultType_RESULT_TYPE_PATH:
			data = Find(reply.Paths, func(index int) bool { return reply.Paths[index].Alias == alias })
			t = Alias.ResultType
		//case ultipa.ResultType_RESULT_TYPE_ARRAY:
		//	data = Find(reply.Arrays, func(index int) bool { return reply.Arrays[index].Alias == alias })
		//	t = Alias.ResultType
		case ultipa.ResultType_RESULT_TYPE_ATTR:
			data = Find(reply.Attrs, func(index int) bool { return reply.Attrs[index].Alias == alias })
			t = Alias.ResultType
			//case ultipa.ResultType_RESULT_TYPE_GRAPH:
			//	data = Find(reply.Graphs, func(index int) bool { return reply.Graphs[index].Alias == alias })
			t = Alias.ResultType
		case ultipa.ResultType_RESULT_TYPE_UNSET:
			// When ResultType is UNSET, try to find data in different collections
			// Try Attrs first (common for property queries like n.record)
			if reply.Attrs != nil {
				data = Find(reply.Attrs, func(index int) bool { return reply.Attrs[index].Alias == alias })
				if data != nil {
					t = ultipa.ResultType_RESULT_TYPE_ATTR
					break
				}
			}
			// Try Nodes (for node queries like RETURN n)
			if reply.Nodes != nil {
				data = Find(reply.Nodes, func(index int) bool { return reply.Nodes[index].Alias == alias })
				if data != nil {
					t = ultipa.ResultType_RESULT_TYPE_NODE
					break
				}
			}
			// Try Edges
			if reply.Edges != nil {
				data = Find(reply.Edges, func(index int) bool { return reply.Edges[index].Alias == alias })
				if data != nil {
					t = ultipa.ResultType_RESULT_TYPE_EDGE
					break
				}
			}
			// Try Tables
			if reply.Tables != nil {
				data = Find(reply.Tables, func(index int) bool { return reply.Tables[index].TableName == alias })
				if data != nil {
					t = ultipa.ResultType_RESULT_TYPE_TABLE
					break
				}
			}
			// Try Paths
			if reply.Paths != nil {
				data = Find(reply.Paths, func(index int) bool { return reply.Paths[index].Alias == alias })
				if data != nil {
					t = ultipa.ResultType_RESULT_TYPE_PATH
					break
				}
			}
			t = Alias.ResultType
		default:
			errMsg := fmt.Sprintf("FindAliasDataInReply Not Supported DBType %v, it usually caused by unsupported server version.", Alias.ResultType)
			panic(errMsg)
		}

		if data != nil {
			break
		}
	}

	return data, t

}

//func MergeUQLItems(s1 interface{}, s2 interface{}, fieldName string, tableName string, rowName string) error {
//	var err error
//
//
//	s1v := reflect.ValueOf(s1).Elem().FieldByName(fieldName)
//	s2v := reflect.ValueOf(s2).Elem().FieldByName(fieldName)
//
//	if s1v.IsNil() {
//		s1v.Set(s2v)
//	}
//
//
//	for i := 0; i < s2v.Len(); i++ {
//		alias := ""
//
//		if fieldName == "Tables" {
//			alias = s2v.Field(i).Elem().FieldByName("TableName").Interface().(string)
//		} else {
//			alias = s2v.Field(i).Elem().FieldByName("Alias").Interface().(string)
//		}
//
//		if alias == "" {
//			continue
//		}
//
//		// find same alias and set values
//		found := false
//		for ii :=0; ii < s1v.Len(); ii++ {
//			alias2 := ""
//			if fieldName == "Tables" {
//				alias2 = s2v.Field(i).Elem().FieldByName("TableName").Interface().(string)
//			} else {
//				alias2 = s2v.Field(i).Elem().FieldByName("Alias").Interface().(string)
//			}
//
//			if alias == alias2 {
//				v1 := s1v.Field(i).Elem()
//				v2 := s2v.Field(ii).Elem()
//
//				if fieldName == "Tables" {
//					v1.Set(reflect.AppendSlice(v1.FieldByName(rowName), v2.FieldByName(rowName)))
//				} else {
//					v11 := v1.FieldByName(tableName)
//					v22 := v2.FieldByName(tableName)
//					v22.Set(reflect.AppendSlice(v11.FieldByName(rowName), v22.FieldByName(rowName)))
//				}
//
//				found = true
//			}
//		}
//
//		if found == false {
//			s1v.Set(reflect.AppendSlice(s1v, s2v))
//		}
//
//	}
//
//
//
//
//	return err
//}

func MergeUQLReply(reply1 *ultipa.QueryReply, reply2 *ultipa.QueryReply) *ultipa.QueryReply {

	//err := MergeSameStruct(reply1, reply2)

	//if err != nil {
	//	log.Fatalln(err)
	//}
	//
	//return reply1

	for _, Alias := range reply1.Alias {
		switch Alias.ResultType {
		case ultipa.ResultType_RESULT_TYPE_NODE:
			if reply1.Nodes == nil && reply2.Nodes == nil {
				return reply1
			}
			if reply1.Nodes == nil {
				return reply2
			}
			if reply2.Nodes == nil {
				return reply1
			}
			data1 := Find(reply1.Nodes, func(index int) bool { return reply1.Nodes[index].Alias == Alias.Alias })
			data2 := Find(reply2.Nodes, func(index int) bool { return reply2.Nodes[index].Alias == Alias.Alias })

			if data2 == nil {
				continue
			}

			if data1 == nil && data2 != nil {
				reply1.Nodes = append(reply1.Nodes, data2.(*ultipa.NodeAlias))
				continue
			}

			if data2 != nil {
				nodes1 := data1.(*ultipa.NodeAlias)
				nodes2 := data2.(*ultipa.NodeAlias)
				nodes1.NodeTable.EntityRows = append(nodes1.NodeTable.EntityRows, nodes2.NodeTable.EntityRows...)
			}
		case ultipa.ResultType_RESULT_TYPE_EDGE:
			if reply1.Edges == nil && reply2.Edges == nil {
				return reply1
			}
			if reply1.Edges == nil {
				return reply2
			}
			if reply2.Edges == nil {
				return reply1
			}
			data1 := Find(reply1.Edges, func(index int) bool { return reply1.Edges[index].Alias == Alias.Alias })
			data2 := Find(reply2.Edges, func(index int) bool { return reply2.Edges[index].Alias == Alias.Alias })
			if data2 == nil {
				continue
			}

			if data1 == nil && data2 != nil {
				reply1.Edges = append(reply1.Edges, data2.(*ultipa.EdgeAlias))
				continue
			}

			if data2 != nil {
				edges1 := data1.(*ultipa.EdgeAlias)
				edges2 := data2.(*ultipa.EdgeAlias)
				edges1.EdgeTable.EntityRows = append(edges1.EdgeTable.EntityRows, edges2.EdgeTable.EntityRows...)
			}
		case ultipa.ResultType_RESULT_TYPE_TABLE:
			if reply1.Tables == nil && reply2.Tables == nil {
				return reply1
			}
			if reply1.Tables == nil {
				return reply2
			}
			if reply2.Tables == nil {
				return reply1
			}
			table1 := Find(reply1.Tables, func(index int) bool { return reply1.Tables[index].TableName == Alias.Alias })
			table2 := Find(reply2.Tables, func(index int) bool { return reply2.Tables[index].TableName == Alias.Alias })

			if table2 == nil {
				continue
			}

			if table1 == nil && table2 != nil {
				reply1.Tables = append(reply1.Tables, table2.(*ultipa.Table))
				continue
			}

			if table2 != nil {
				data1 := table1.(*ultipa.Table)
				data2 := table2.(*ultipa.Table)
				data1.TableRows = append(data1.TableRows, data2.TableRows...)
			}

		case ultipa.ResultType_RESULT_TYPE_PATH:
			if reply1.Paths == nil && reply2.Paths == nil {
				return reply1
			}
			if reply1.Paths == nil {
				return reply2
			}
			if reply2.Paths == nil {
				return reply1
			}
			data1 := Find(reply1.Paths, func(index int) bool { return reply1.Paths[index].Alias == Alias.Alias })
			data2 := Find(reply2.Paths, func(index int) bool { return reply2.Paths[index].Alias == Alias.Alias })

			if data2 == nil {
				continue
			}

			if data1 == nil && data2 != nil {
				reply1.Paths = append(reply1.Paths, data2.(*ultipa.PathAlias))
				continue
			}

			if data2 != nil {
				paths1 := data1.(*ultipa.PathAlias)
				paths2 := data2.(*ultipa.PathAlias)
				paths1.Paths = append(paths1.Paths, paths2.Paths...)
			}
		//case ultipa.ResultType_RESULT_TYPE_ARRAY:
		//if reply1.Arrays == nil && reply2.Arrays == nil {
		//	return reply1
		//}
		//if reply1.Arrays == nil {
		//	return reply2
		//}
		//if reply2.Arrays == nil {
		//	return reply1
		//}
		//
		//data1 := Find(reply1.Arrays, func(index int) bool { return reply1.Arrays[index].Alias == Alias.Alias })
		//data2 := Find(reply2.Arrays, func(index int) bool { return reply2.Arrays[index].Alias == Alias.Alias })
		//
		//if data2 == nil {
		//	continue
		//}
		//
		//if data1 == nil && data2 != nil {
		//	reply1.Arrays = append(reply1.Arrays, data2.(*ultipa.ArrayAlias))
		//	continue
		//}
		//
		//if data2 != nil {
		//	array1 := data1.(*ultipa.ArrayAlias)
		//	array2 := data2.(*ultipa.ArrayAlias)
		//	array1.Elements = append(array1.Elements, array2.Elements...)
		//}
		case ultipa.ResultType_RESULT_TYPE_ATTR:
			if reply1.Attrs == nil && reply2.Attrs == nil {
				return reply1
			}
			if reply1.Attrs == nil {
				return reply2
			}
			if reply2.Attrs == nil {
				return reply1
			}
			data1 := Find(reply1.Attrs, func(index int) bool { return reply1.Attrs[index].Alias == Alias.Alias })
			data2 := Find(reply2.Attrs, func(index int) bool { return reply2.Attrs[index].Alias == Alias.Alias })

			if data2 == nil {
				continue
			}

			if data1 == nil && data2 != nil {
				reply1.Attrs = append(reply1.Attrs, data2.(*ultipa.AttrAlias))
				continue
			}

			if data2 != nil {
				attr1 := data1.(*ultipa.AttrAlias)
				attr2 := data2.(*ultipa.AttrAlias)

				if attr2.Attr == nil {
					continue
				}
				if attr1.Attr == nil && attr2.Attr != nil {
					reply1.Attrs = append(reply1.Attrs, attr2)
					continue
				}
				attr1.Attr.Values = append(attr1.Attr.Values, attr2.Attr.Values...)
			}
			//case ultipa.ResultType_RESULT_TYPE_GRAPH:
			//	if reply1.Graphs == nil && reply2.Graphs == nil {
			//		return reply1
			//	}
			//	if reply1.Graphs == nil {
			//		return reply2
			//	}
			//	if reply2.Graphs == nil {
			//		return reply1
			//	}
			//	data1 := Find(reply1.Graphs, func(index int) bool { return reply1.Graphs[index].Alias == Alias.Alias })
			//	data2 := Find(reply2.Graphs, func(index int) bool { return reply2.Graphs[index].Alias == Alias.Alias })
			//
			//	if data2 == nil {
			//		continue
			//	}
			//
			//	if data1 == nil && data2 != nil {
			//		reply1.Graphs = append(reply1.Graphs, data2.(*ultipa.GraphAlias))
			//		continue
			//	}
			//
			//	if data2 != nil {
			//		graph1 := data1.(*ultipa.GraphAlias)
			//		graph2 := data2.(*ultipa.GraphAlias)
			//
			//		graph1.Graph.NodeTable.EntityRows = append(graph1.Graph.NodeTable.EntityRows, graph2.Graph.NodeTable.EntityRows...)
			//		nodeSchemaMap := map[string]*ultipa.Schema{}
			//		for _, graph1Schema := range graph1.Graph.NodeTable.Schemas {
			//			nodeSchemaMap[graph1Schema.Schema] = graph1Schema
			//		}
			//		for _, graph2Schema := range graph2.Graph.NodeTable.Schemas {
			//			if _, ok := nodeSchemaMap[graph2Schema.Schema]; !ok {
			//				nodeSchemaMap[graph2Schema.Schema] = graph2Schema
			//				graph1.Graph.NodeTable.Schemas = append(graph1.Graph.NodeTable.Schemas, graph2Schema)
			//			}
			//		}
			//
			//		graph1.Graph.EdgeTable.EntityRows = append(graph1.Graph.EdgeTable.EntityRows, graph2.Graph.EdgeTable.EntityRows...)
			//		edgeSchemaMap := map[string]*ultipa.Schema{}
			//		for _, graph1Schema := range graph1.Graph.EdgeTable.Schemas {
			//			edgeSchemaMap[graph1Schema.Schema] = graph1Schema
			//		}
			//		for _, graph2Schema := range graph2.Graph.EdgeTable.Schemas {
			//			if _, ok := edgeSchemaMap[graph2Schema.Schema]; !ok {
			//				edgeSchemaMap[graph2Schema.Schema] = graph2Schema
			//				graph1.Graph.EdgeTable.Schemas = append(graph1.Graph.EdgeTable.Schemas, graph2Schema)
			//			}
			//		}
			//	}

		}
	}

	return reply1
}
