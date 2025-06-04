package utils

//
//import (
//	" github.com/ultipa/ultipa-go-driver/types"
//)
//
//func UqlResponseAppend(uqlReply1 *types.UqlReply, uqlReply2 *types.UqlReply)  {
//	//resJson, _ := StructToJSONBytes(uqlReply2)
//	//log.Printf("\nuql res ->\n %s\n", resJson)
//	//fmt.Println("----- merge uqlreply")
//	paths := append(*uqlReply1.Paths, *uqlReply2.Paths...)
//	uqlReply1.Paths = &paths
//
//	nodes := append(*uqlReply1.Nodes, *uqlReply2.Nodes...)
//	var newNodes types.NodeAliases
//	for _, n := range nodes {
//		var nodeAliasFind *types.NodeAlias
//		for _, newN := range newNodes {
//			if newN.Alias == n.Alias {
//				nodeAliasFind = newN
//			}
//		}
//		if nodeAliasFind == nil {
//			nodeAliasFind = n
//			newNodes = append(newNodes, nodeAliasFind)
//		} else {
//			rows := append(*nodeAliasFind.Nodes, *n.Nodes...)
//			nodeAliasFind.Nodes = &rows
//		}
//	}
//	uqlReply1.Nodes = &newNodes
//
//	edges := append(*uqlReply1.Edges, *uqlReply2.Edges...)
//	var newEdges types.EdgeAliases
//	for _, n := range edges {
//		var edgeAliasFind *types.EdgeAlias
//		for _, newN := range newEdges {
//			if newN.Alias == n.Alias {
//				edgeAliasFind = newN
//			}
//		}
//		if edgeAliasFind == nil {
//			edgeAliasFind = n
//			newEdges = append(newEdges, edgeAliasFind)
//		} else {
//			rows := append(*edgeAliasFind.Edges, *n.Edges...)
//			edgeAliasFind.Edges = &rows
//		}
//	}
//	uqlReply1.Edges = &newEdges
//
//	attrs := append(*uqlReply1.Attrs, *uqlReply2.Attrs...)
//	var newAttrs types.Attrs
//	for _, n := range attrs {
//		var attrAliasFind *types.AttrAlias
//		for _, newN := range newAttrs {
//			if newN.Alias == n.Alias {
//				attrAliasFind = newN
//			}
//		}
//		if attrAliasFind == nil {
//			attrAliasFind = n
//			newAttrs = append(newAttrs, attrAliasFind)
//		} else {
//			rows := append(attrAliasFind.Values, n.Values...)
//			attrAliasFind.Values = rows
//		}
//	}
//	uqlReply1.Attrs = &newAttrs
//
//	tables := append(*uqlReply1.Tables, *uqlReply2.Tables...)
//	var newTables types.Tables
//	for _, n := range tables {
//		var tableAliasFind *types.Table
//		for _, newN := range newTables {
//			if newN.TableName == n.TableName {
//				tableAliasFind = newN
//			}
//		}
//		if tableAliasFind == nil {
//			tableAliasFind = n
//			newTables = append(newTables, tableAliasFind)
//		} else {
//			rows := append(*tableAliasFind.TableRows, *n.TableRows...)
//			tableAliasFind.TableRows = &rows
//		}
//	}
//	uqlReply1.Tables = &newTables
//
//	if uqlReply1.Values != nil {
//		values := *uqlReply1.Values
//		if uqlReply2.Values != nil {
//			for k, v := range *uqlReply2.Values {
//				values[k] = v
//			}
//		}
//		uqlReply1.Values = &values
//	}
//}
