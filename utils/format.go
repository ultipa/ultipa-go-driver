package utils

//
//import (
//	"bytes"
//	"encoding/binary"
//	"errors"
//	"fmt"
//	"math"
//	"strconv"
//	ultipa " github.com/ultipa/ultipa-go-driver/rpc"
//	" github.com/ultipa/ultipa-go-driver/types"
//)
//
//var ultipaTime = UltipaTime{}
//
//func FormatNodeAliases(nodes []*ultipa.NodeAlias) *types.NodeAliases {
//	var arrs types.NodeAliases
//	for _, one := range nodes {
//		arrs = append(arrs, FormatNodeAlias(one))
//	}
//	return &arrs
//}
//func FormatEdgeAliases(edges []*ultipa.EdgeAlias) *types.EdgeAliases {
//	var arrs types.EdgeAliases
//	for _, one := range edges {
//		arrs = append(arrs, FormatEdgeAlias(one))
//	}
//	return &arrs
//}
//func FormatNodeAlias(node *ultipa.NodeAlias) *types.NodeAlias {
//	newNode := types.NodeAlias{}
//	newNode.Alias = node.Alias
//	newNode.Nodes = FormatNodeTable(node.GetNodeTable())
//	return &newNode
//}
//func FormatEdgeAlias(edge *ultipa.EdgeAlias) *types.EdgeAlias {
//	newEdge := types.EdgeAlias{}
//	newEdge.Alias = edge.Alias
//	newEdge.Edges = FormatEdgeTable(edge.GetEdgeTable())
//	return &newEdge
//}
//func FormatNodeTable(nodeTable *ultipa.NodeTable) *types.NodeTable {
//	var headers []string
//	var types_ []types.PropertyType
//	if nodeTable.Headers != nil {
//		for _, v := range nodeTable.Headers {
//			headers = append(headers, v.GetPropertyName())
//			types_ = append(types_, v.GetPropertyType())
//		}
//	}
//	var rows []*types.NodeRow
//	if nodeTable.NodeRows != nil {
//		for _, v := range nodeTable.GetNodeRows() {
//			_node := types.NodeRow{}
//			_node.ID = v.GetId()
//			_vs := _formatValues(v.GetValues(), types_, headers)
//			_node.Values = _vs
//			rows = append(rows, &_node)
//		}
//	}
//	return &rows
//}
//func _formatValues(values [][]byte, types_ []types.PropertyType, headers []string) *map[string]interface{} {
//	_vs := map[string]interface{}{}
//	_missHeaders := false
//	if len(values) > len(types_) || len(values) > len(headers) {
//		_missHeaders = true
//		fmt.Println("‼️ BUG server not return header")
//	}
//	for _index, vv := range values {
//		vvType := types.PROPERTY_TYPE_STRING // server side bug，so，hard-coded fix 0513
//		var key string
//		if _missHeaders == false {
//			vvType = types_[_index]
//			key = headers[_index]
//		} else {
//			key = fmt.Sprintf("Unknown %v", _index)
//		}
//		change := Deserialize(vv, vvType)
//		_vs[key] = change
//	}
//	return &_vs
//}
//func FormatEdgeTable(edgeTable *ultipa.EdgeTable) *types.EdgeTable {
//	var headers []string
//	var types_ []types.PropertyType
//	if edgeTable.Headers != nil {
//		for _, v := range edgeTable.Headers {
//			headers = append(headers, v.GetPropertyName())
//			types_ = append(types_, v.GetPropertyType())
//		}
//	}
//	var rows []*types.EdgeRow
//	if edgeTable.EdgeRows != nil {
//		for _, v := range edgeTable.GetEdgeRows() {
//			_edge := types.EdgeRow{}
//			_edge.From = v.GetFromId()
//			_edge.ID = v.GetId()
//			_edge.To = v.GetToId()
//			_vs := _formatValues(v.GetValues(), types_, headers)
//			_edge.Values = _vs
//			rows = append(rows, &_edge)
//		}
//	}
//	return &rows
//}
//func FormatPaths(paths []*ultipa.Path) *types.Paths {
//	var ps types.Paths
//	for _, path := range paths {
//		newPath := types.Path{
//			Nodes: FormatNodeTable(path.NodeTable),
//			Edges: FormatEdgeTable(path.EdgeTable),
//		}
//		ps = append(ps, &newPath)
//	}
//	return &ps
//}
//func FormatAttrAlias(attrAlias *ultipa.AttrAlias) *types.AttrAlias {
//	newAttrAlias := types.AttrAlias{}
//	newAttrAlias.Alias = attrAlias.GetAlias()
//	if attrAlias.GetValues() != nil {
//		var newValues []interface{}
//		for _, v := range attrAlias.GetValues() {
//			v1 := Deserialize(v, attrAlias.GetPropertyType())
//			newValues = append(newValues, v1)
//		}
//		newAttrAlias.Values = newValues
//	}
//	return &newAttrAlias
//}
//func FormatAttrs(attrs []*ultipa.AttrAlias) *types.Attrs {
//	var newAttrs types.Attrs
//	if attrs != nil {
//		for _, attr := range attrs {
//			newAttrs = append(newAttrs, FormatAttrAlias(attr))
//		}
//	}
//	return &newAttrs
//}
//func FormatTables(tables []*ultipa.Table) *types.Tables {
//	var newTables types.Tables
//	for _, table := range tables {
//		tb := types.Table{
//			TableName: table.TableName,
//			Headers:   table.Headers,
//		}
//		var newRows types.TableRows
//		tableRows := table.GetTableRows()
//		if tableRows != nil {
//			for _, row := range tableRows {
//				var _row []interface{}
//				for _, v := range row.GetValues() {
//					_v := Deserialize(v, types.PROPERTY_TYPE_STRING)
//					_row = append(_row, _v)
//				}
//				newRows = append(newRows, &_row)
//			}
//		}
//		tb.TableRows = &newRows
//		newTables = append(newTables, &tb)
//	}
//	return &newTables
//}
//func FormatKeyValues(values []*ultipa.Value) *map[string]interface{} {
//	if values == nil {
//		return nil
//	}
//	_values := map[string]interface{}{}
//	for _, ev := range values {
//		_values[ev.GetKey()] = ev.GetValue()
//	}
//	return &_values
//}
//func _bytesToRead(bs []byte, out interface{}) {
//	//bs[0] = (byte)(int(bs[0]) ^ 0x80)
//	buff := bytes.NewBuffer(bs)
//	binary.Read(buff, binary.BigEndian, out)
//}
//
//func Deserialize(bytes []byte, propertyType types.PropertyType) interface{} {
//	if len(bytes) == 0 && propertyType != types.PROPERTY_TYPE_STRING {
//		return nil
//	}
//	switch propertyType {
//	case types.PROPERTY_TYPE_STRING:
//		return string(bytes)
//	case types.PROPERTY_TYPE_INT32:
//		var num int32
//		_bytesToRead(bytes, &num)
//		return num
//	case types.PROPERTY_TYPE_INT64:
//		var num int64
//		_bytesToRead(bytes, &num)
//		return num
//	case types.PROPERTY_TYPE_UINT32:
//		var num uint32
//		_bytesToRead(bytes, &num)
//		return num
//	case types.PROPERTY_TYPE_UINT64:
//		var num uint64
//		_bytesToRead(bytes, &num)
//		return num
//	case types.PROPERTY_TYPE_FLOAT:
//		var num float32
//		_bytesToRead(bytes, &num)
//		return num
//	case types.PROPERTY_TYPE_DOUBLE:
//		var num float64
//		_bytesToRead(bytes, &num)
//		return num
//	case types.PROPERTY_TYPE_DATETIME:
//		var num uint64
//		_bytesToRead(bytes, &num)
//		return ultipaTime.New(num).ToString()
//	}
//	return "Unknown"
//}
//
////
////func FormatEdges(edges []*ultipa.Edge) []*Edge {
////	var newEdges []*Edge
////	for _, ev := range edges {
////		newEdges = append(newEdges, FormatEdge(ev))
////	}
////	return newEdges
////}
//
////func FormatEdge(edge *ultipa.Edge) *Edge {
////	newEdge := Edge{}
////	newEdge.ID = edge.GetId()
////	newEdge.From = edge.GetFromId()
////	newEdge.To = edge.GetToId()
////	newEdge.Values = FormatValues(edge.GetValues())
////	return &newEdge
////}
//func FormatStatus(status *ultipa.Status, err error) *types.Status {
//	return FormatStatusWithHost(status, err, "")
//}
//
//func FormatStatusWithHost(status *ultipa.Status, err error, host string) *types.Status {
//	if err != nil {
//		return &types.Status{
//			Code:    types.ErrorCode_UNKNOW,
//			Message: fmt.Sprint(err),
//		}
//	}
//	clusterInfo := types.ClusterInfo{}
//	newStatus := types.Status{
//		Code:        types.ErrorCode_SUCCESS,
//		Message:     "",
//		ClusterInfo: &clusterInfo,
//	}
//	newStatus.Code = status.GetErrorCode()
//	if status.GetErrorCode() != ultipa.ErrorCode_SUCCESS {
//		newStatus.Message = status.GetMsg()
//	}
//	_clusterInfo := status.GetClusterInfo()
//	isNotRaftMode := newStatus.Code == ultipa.ErrorCode_NOT_RAFT_MODE
//	if _clusterInfo != nil || isNotRaftMode {
//		if _clusterInfo == nil {
//			_clusterInfo = &ultipa.ClusterInfo{
//				Redirect:      "",
//				LeaderAddress: host,
//				Followers:     []*ultipa.RaftFollower{},
//			}
//		}
//		clusterInfo.Redirect = _clusterInfo.GetRedirect()
//		clusterInfo.RaftPeers = []*types.RaftPeerInfo{}
//		leaderIsAlgoExecutable := false
//		if isNotRaftMode {
//			leaderIsAlgoExecutable = true
//		}
//		clusterInfo.RaftPeers = append(clusterInfo.RaftPeers,
//			&types.RaftPeerInfo{
//				Host:               _clusterInfo.GetLeaderAddress(),
//				Status:             true,
//				IsLeader:           true,
//				IsAlgoExecutable:   leaderIsAlgoExecutable,
//				IsFollowerReadable: false,
//				IsUnset:            false,
//			})
//		for _, info := range _clusterInfo.GetFollowers() {
//			IsAlgoExecutable := false
//			IsFollowerReadable := false
//			Status := info.GetStatus() == 1
//			role := info.GetRole()
//			if Status {
//				IsAlgoExecutable = role&int32(types.RAFT_FOLLOWER_ROLE_ALGO_EXECUTABLE) > 0
//				IsFollowerReadable = role&int32(types.RAFT_FOLLOWER_ROLE_READABLE) > 0
//			}
//			clusterInfo.RaftPeers = append(clusterInfo.RaftPeers, &types.RaftPeerInfo{
//				Host:               info.GetAddress(),
//				Status:             Status,
//				IsLeader:           false,
//				IsAlgoExecutable:   IsAlgoExecutable,
//				IsFollowerReadable: IsFollowerReadable,
//				IsUnset:            role == int32(types.RAFT_FOLLOWER_ROLE_UNSET),
//			})
//		}
//
//	}
//	return &newStatus
//}
//
////
////func TableToValues(table *Table) *map[string][]string {
////	res := map[string][]string{}
////	for index, key := range table.Headers {
////		res[key] = table.TableRows[index]
////	}
////	return &res
////}
////
//func TableToArray(table *types.Table) *[]*map[string]interface{} {
//	var res []*map[string]interface{}
//	for _, rows := range *table.TableRows {
//		item := map[string]interface{}{}
//		for index, row := range *rows {
//			item[table.Headers[index]] = row
//		}
//		res = append(res, &item)
//	}
//	return &res
//}
//
//// ConvertToBytes data to bytes by Ultipa property type
//func ConvertToBytes(value interface{}, t ultipa.UltipaPropertyType) ([]byte, error) {
//	v := []byte{}
//
//	if value == nil || value == "" {
//		switch t {
//		case ultipa.UltipaPropertyType_PROPERTY_STRING:
//			value = ""
//		case ultipa.UltipaPropertyType_PROPERTY_INT32:
//			value = int32(0)
//		case ultipa.UltipaPropertyType_PROPERTY_INT64:
//			value = int64(0)
//		case ultipa.UltipaPropertyType_PROPERTY_UINT32:
//			value = uint32(0)
//		case ultipa.UltipaPropertyType_PROPERTY_UINT64:
//			value = uint64(0)
//		case ultipa.UltipaPropertyType_PROPERTY_FLOAT:
//			value = float32(0)
//		case ultipa.UltipaPropertyType_PROPERTY_DOUBLE:
//			value = float64(0)
//		case ultipa.UltipaPropertyType_PROPERTY_DATETIME:
//			value = uint64(0)
//		default:
//			return nil, errors.New(fmt.Sprint("not supported ultipa type : ", t))
//		}
//	}
//
//	switch t {
//	case ultipa.UltipaPropertyType_PROPERTY_INT32:
//		v = make([]byte, 4)
//		binary.BigEndian.PutUint32(v, uint32(value.(int32)))
//	case ultipa.UltipaPropertyType_PROPERTY_STRING:
//		v = []byte(value.(string))
//	case ultipa.UltipaPropertyType_PROPERTY_INT64:
//		v = make([]byte, 8)
//		binary.BigEndian.PutUint64(v, uint64(value.(int64)))
//	case ultipa.UltipaPropertyType_PROPERTY_UINT32:
//		v = make([]byte, 4)
//		binary.BigEndian.PutUint32(v, value.(uint32))
//	case ultipa.UltipaPropertyType_PROPERTY_UINT64:
//		v = make([]byte, 8)
//		binary.BigEndian.PutUint64(v, value.(uint64))
//	case ultipa.UltipaPropertyType_PROPERTY_FLOAT:
//		v = make([]byte, 4)
//		binary.BigEndian.PutUint32(v, math.Float32bits(value.(float32)))
//	case ultipa.UltipaPropertyType_PROPERTY_DOUBLE:
//		v = make([]byte, 8)
//		binary.BigEndian.PutUint64(v, math.Float64bits(value.(float64)))
//	case ultipa.UltipaPropertyType_PROPERTY_DATETIME:
//		v = make([]byte, 8)
//		n, err := ultipaTime.NewFromString(value.(string))
//		if err != nil {
//			return nil, err
//		}
//		binary.BigEndian.PutUint64(v, n.Datetime)
//
//	default:
//		return nil, errors.New(fmt.Sprint("not supported ultipa type : ", t))
//	}
//
//	return v, nil
//}
//
//// ConvertToID data to bytes by Ultipa property type
//func ConvertToID(value interface{}) int64 {
//	id := int64(0)
//	switch value.(type) {
//	case int64:
//		id = value.(int64)
//	case string:
//		v, _ := strconv.Atoi(value.(string))
//		id = int64(v)
//	}
//
//	return id
//}
