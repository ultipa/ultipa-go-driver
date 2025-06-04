package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ultipa/ultipa-go-driver/sdk/types"
	"log"
	"strconv"
	"strings"
	"time"

	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
	"github.com/ultipa/ultipa-go-driver/sdk/utils"
	"google.golang.org/protobuf/proto"
)

type DataItem struct {
	Alias string
	Type  ultipa.ResultType
	Data  interface{}
}

func NewDataItem() *DataItem {
	return &DataItem{}
}

func NodeTableToNodes(nt *ultipa.EntityTable, alias string) ([]*structs.Node, map[types.UUID]*structs.Node, map[string]*structs.Schema, error) {

	schemas := map[string]*structs.Schema{}
	var nodes []*structs.Node
	nodesMap := make(map[types.UUID]*structs.Node)

	for _, oSchema := range nt.Schemas {
		schema := structs.NewSchema(oSchema.SchemaName)
		schema.DBType = ultipa.DBType_DBNODE
		schemas[schema.Name] = schema
		for _, header := range oSchema.Properties {
			schema.Properties = append(schema.Properties, &structs.Property{Name: header.PropertyName, Type: header.PropertyType, SubTypes: header.SubTypes})
		}
	}

	for _, oNode := range nt.EntityRows {
		var node *structs.Node
		if oNode.IsNull {
			node = nil
			continue
		} else {
			node = &structs.Node{
				//Name:   alias,
				ID:     oNode.Id,
				UUID:   oNode.Uuid,
				Schema: oNode.SchemaName,
			}

			// set values
			node.Values = structs.NewValues()
			schema := schemas[oNode.SchemaName]
			for index, v := range oNode.Values {
				prop := schema.Properties[index]
				value, err := utils.ConvertBytesToInterface(v, prop.Type, prop.SubTypes)
				if err != nil {
					return nil, nil, nil, err
				}
				node.Values.Set(prop.Name, value)
			}
		}

		nodes = append(nodes, node)
		nodesMap[node.UUID] = node
	}

	return nodes, nodesMap, schemas, nil
}

func NodeTableToUUIDs(nt *ultipa.EntityTable) []types.UUID {
	uuids := make([]types.UUID, 0, len(nt.EntityRows))
	for _, oNode := range nt.EntityRows {
		if oNode.IsNull {
			continue
		}

		uuids = append(uuids, oNode.Uuid)

	}

	return uuids
}

func EdgeTableToEdges(et *ultipa.EntityTable, alias string) ([]*structs.Edge, map[types.UUID]*structs.Edge, map[string]*structs.Schema, error) {

	schemas := map[string]*structs.Schema{}
	var edges []*structs.Edge
	edgesMap := make(map[types.UUID]*structs.Edge)

	for _, oSchema := range et.Schemas {
		schema := structs.NewSchema(oSchema.SchemaName)
		schema.DBType = ultipa.DBType_DBEDGE
		schemas[schema.Name] = schema
		for _, header := range oSchema.Properties {
			schema.Properties = append(schema.Properties, &structs.Property{Name: header.PropertyName, Type: header.PropertyType, SubTypes: header.SubTypes})
		}
	}
	var edge *structs.Edge
	for _, oEdge := range et.EntityRows {
		if oEdge.IsNull {
			edge = nil
			continue
		} else {
			edge = &structs.Edge{
				//Name:     alias,
				UUID:     oEdge.Uuid,
				From:     oEdge.FromId,
				FromUUID: oEdge.FromUuid,
				To:       oEdge.ToId,
				ToUUID:   oEdge.ToUuid,
				Schema:   oEdge.SchemaName,
			}

			// set values
			edge.Values = structs.NewValues()
			schema := schemas[oEdge.SchemaName]
			for index, v := range oEdge.Values {
				prop := schema.Properties[index]
				value, err := utils.ConvertBytesToInterface(v, prop.Type, prop.SubTypes)
				if err != nil {
					return nil, nil, nil, err
				}
				edge.Values.Set(prop.Name, value)
			}
		}

		edges = append(edges, edge)
		edgesMap[edge.UUID] = edge

	}
	return edges, edgesMap, schemas, nil
}

func (di *DataItem) AsNodes() (nodes []*structs.Node, schemas map[string]*structs.Schema, err error) {
	if di.Data == nil {
		return nil, nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nodes, schemas, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_NODE && di.Type != ultipa.ResultType_RESULT_TYPE_ATTR {
		return nil, schemas, errors.New(fmt.Sprintf("dataItem %s is not either Node type or LIST Node type", di.Alias))
	}

	oNodes := di.Data.(*ultipa.NodeAlias)

	nodes, _, schemas, err = NodeTableToNodes(oNodes.NodeTable, oNodes.Alias)
	if err != nil {
		return nil, nil, err
	}
	return nodes, schemas, err
}

func (di *DataItem) AsEdges() (edges []*structs.Edge, schemas map[string]*structs.Schema, err error) {
	if di.Data == nil {
		return nil, nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return edges, schemas, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_EDGE && di.Type != ultipa.ResultType_RESULT_TYPE_ATTR {
		return nil, schemas, errors.New(fmt.Sprintf("dataItem %s is not either Edge type or LIST Edge type", di.Alias))
	}

	oEdges := di.Data.(*ultipa.EdgeAlias)

	edges, _, schemas, err = EdgeTableToEdges(oEdges.EdgeTable, oEdges.Alias)
	if err != nil {
		return nil, nil, err
	}
	return edges, schemas, err
}

func (di *DataItem) AsPaths() (paths []*structs.Path, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return paths, nil
	}

	//if di.Type != ultipa.ResultType_RESULT_TYPE_PATH && di.Type != ultipa.ResultType_RESULT_TYPE_ATTR {
	//    return nil, errors.New(fmt.Sprintf("dataItem %s is not either Path type or LIST Path type", di.Alias))
	//}

	if di.Type != ultipa.ResultType_RESULT_TYPE_PATH {
		return nil, errors.New(fmt.Sprintf("dataItem %s is not either Path type ", di.Alias))
	}

	pathAlias := di.Data.(*ultipa.PathAlias)

	//return parsePaths(pathAlias.Paths, pathAlias.Alias)
	return parsePaths(pathAlias.Paths)
}

func parsePaths(oPaths []*ultipa.Path) (paths []*structs.Path, err error) {
	for _, oPath := range oPaths {
		path := &structs.Path{}
		//path.Name = name
		_, nodesMap, _, err := NodeTableToNodes(oPath.NodeTable, "")
		if err != nil {
			return nil, err
		}
		path.Nodes = nodesMap
		path.NodeUUIDs = NodeTableToUUIDs(oPath.NodeTable)
		if err != nil {
			return nil, err
		}

		_, edgesMap, _, err := EdgeTableToEdges(oPath.EdgeTable, "")
		if err != nil {
			return nil, err
		}
		path.Edges = edgesMap
		path.EdgeUUIDs = NodeTableToUUIDs(oPath.EdgeTable)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func (di *DataItem) AsTable() (table *structs.Table, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return table, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	oTable := di.Data.(*ultipa.Table)

	table = structs.NewTable()
	table.Name = oTable.TableName

	for _, header := range oTable.Headers {
		h := &structs.Header{
			Name:         header.PropertyName,
			PropertyType: header.PropertyType,
		}
		table.Headers = append(table.Headers, h)
	}

	for _, row := range oTable.TableRows {

		r := structs.Value{}

		for index, field := range row.Values {
			value, err := utils.ConvertBytesToInterface(field, table.Headers[index].PropertyType, nil)
			if err != nil {
				return nil, err
			}
			r = append(r, value)
		}

		table.Rows = append(table.Rows, &r)
	}

	return table, err
}

//AsArray find().nodes() as nodes group by nodes.year as y return y,collect(nodes._id)
//func (di *DataItem) AsArray() (arr *structs.Array, err error) {
//
//	if di.DBType == ultipa.ResultType_RESULT_TYPE_UNSET {
//		return arr, nil
//	}
//
//	if di.DBType != ultipa.ResultType_RESULT_TYPE_ARRAY {
//		return nil, errors.New("DataItem " + di.Alias + " is not DBType Array")
//	}
//
//	arr = structs.NewArray()
//
//	oArray := di.Data.(*ultipa.ArrayAlias)
//
//	arr.Name = oArray.Alias
//
//	for _, oRow := range oArray.Elements {
//		r := structs.Value{}
//
//		for _, field := range oRow.Values {
//			//TODO, check has subTypes or not?
//			value, err := utils.ConvertBytesToInterface(field, oArray.PropertyType, nil)
//			if err != nil {
//				return nil, err
//			}
//			r = append(r, value)
//		}
//
//		arr.Values = append(arr.Values, &r)
//	}
//
//	return arr, err
//}

func (di *DataItem) AsAttr() (*structs.Attr, error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nil, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_ATTR {
		return nil, errors.New("DataItem " + di.Alias + " is not Type Attribute list")
	}

	attrAlias := di.Data.(*ultipa.AttrAlias)
	oAttr := attrAlias.Attr

	midAttr, err := parseAttr(oAttr, attrAlias.Alias)
	if err != nil {
		return nil, err
	}
	switch midAttr.PropertyType {
	case ultipa.PropertyType_LIST:
		return midAttr.ListAttrAsAttr()

	case ultipa.PropertyType_SET:
		return midAttr.ListAttrAsAttr()
	case ultipa.PropertyType_MAP:
		return nil, errors.New(fmt.Sprintf("DataItem %v is not either Type Attr or LIST Attr, but MAP, not supported yet.", di.Alias))
	default:
		return midAttr, nil
	}
	return nil, err
}

// AsAttrEdges parse DataItem as Attr with Values that is List<List<Node>>
func (di *DataItem) AsAttrNodes() (*structs.AttrNodes, error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nil, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_ATTR {
		return nil, errors.New("DataItem " + di.Alias + " is not Type Attr")
	}

	attrAlias := di.Data.(*ultipa.AttrAlias)
	oAttr := attrAlias.Attr

	midAttr, err := parseAttr(oAttr, attrAlias.Alias)
	if err != nil {
		return nil, err
	}
	return midAttr.ListAttrAsAttrNodes()
}

// AsAttrEdges parse DataItem as Attr with Values that is List<List<Edge>>
func (di *DataItem) AsAttrEdges() (*structs.AttrEdges, error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nil, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_ATTR {
		return nil, errors.New("DataItem " + di.Alias + " is not Type Attr")
	}

	attrAlias := di.Data.(*ultipa.AttrAlias)
	oAttr := attrAlias.Attr

	midAttr, err := parseAttr(oAttr, attrAlias.Alias)
	if err != nil {
		return nil, err
	}
	return midAttr.ListAttrAsAttrEdges()
}

// AsAttrPaths parse DataItem as Attr with Values that is List<List<Path>>
func (di *DataItem) AsAttrPaths() (*structs.AttrPaths, error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nil, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_ATTR {
		return nil, errors.New("DataItem " + di.Alias + " is not Type Attr")
	}

	attrAlias := di.Data.(*ultipa.AttrAlias)
	oAttr := attrAlias.Attr

	midAttr, err := parseAttr(oAttr, attrAlias.Alias)
	if err != nil {
		return nil, err
	}
	return midAttr.ListAttrAsAttrPaths()
}

func parseAttr(oAttr *ultipa.Attr, name string) (*structs.Attr, error) {
	attr := structs.NewAttr()
	attr.Name = name
	attr.PropertyType = oAttr.ValueType
	if oAttr == nil || oAttr.Values == nil {
		return attr, nil
	}

	err := handleAttrValues(oAttr, attr)
	if err != nil {
		return attr, err
	}
	return attr, nil
}

func handleAttrValues(oAttr *ultipa.Attr, attr *structs.Attr) error {
	switch oAttr.ValueType {
	case ultipa.PropertyType_SET:
		fallthrough
	case ultipa.PropertyType_LIST:
		err := parseAttrList(oAttr, attr)
		if err != nil {
			return err
		}
	case ultipa.PropertyType_MAP:
		mapDataRows, err := parseAttrMap(oAttr)
		if err != nil {
			return err
		}
		for _, row := range mapDataRows {
			attr.Values = append(attr.Values, row)
		}
	default:
		attr.ResultType = ultipa.ResultType_RESULT_TYPE_ATTR
		if oAttr.Values == nil {
			attr.Values = nil
		} else {
			for _, v := range oAttr.Values {
				value, err := utils.ConvertBytesToInterface(v, attr.PropertyType, nil)
				if err != nil {
					return err
				}
				attr.Values = append(attr.Values, value)
			}
		}
	}
	return nil
}

// parseAttrList parse the oAttr that PropertyType is ultipa.PropertyType_LIST or ultipa.PropertyType_SET, set the parsed value to attr.
func parseAttrList(oAttr *ultipa.Attr, attr *structs.Attr) error {
	var resultType ultipa.ResultType
	for _, v := range oAttr.Values {
		oListData := &ultipa.AttrListData{}
		err := proto.Unmarshal(v, oListData)
		if err != nil {
			return err
		}
		if oListData.IsNull {
			attr.Values = append(attr.Values, nil)
			continue
		}
		switch oListData.Type {
		case ultipa.ResultType_RESULT_TYPE_NODE:
			nodes, _, _, err := NodeTableToNodes(oListData.Nodes, "")
			if err != nil {
				return err
			}
			listData := structs.NewAttrListData()
			listData.ResultType = oListData.Type
			listData.Nodes = append(listData.Nodes, nodes...)
			attr.Values = append(attr.Values, listData)
			if ultipa.ResultType_RESULT_TYPE_UNSET == resultType {
				resultType = listData.ResultType
			}
		case ultipa.ResultType_RESULT_TYPE_EDGE:
			edges, _, _, err := EdgeTableToEdges(oListData.Edges, "")
			if err != nil {
				return err
			}
			listData := structs.NewAttrListData()
			listData.ResultType = oListData.Type
			listData.Edges = append(listData.Edges, edges...)
			attr.Values = append(attr.Values, listData)
			if ultipa.ResultType_RESULT_TYPE_UNSET == resultType {
				resultType = listData.ResultType
			}
		case ultipa.ResultType_RESULT_TYPE_PATH:
			paths, err := parsePaths(oListData.Paths)
			if err != nil {
				return err
			}
			listData := structs.NewAttrListData()
			listData.ResultType = oListData.Type
			listData.Paths = append(listData.Paths, paths...)
			attr.Values = append(attr.Values, listData)
			if ultipa.ResultType_RESULT_TYPE_UNSET == resultType {
				resultType = listData.ResultType
			}
		case ultipa.ResultType_RESULT_TYPE_ATTR:
			if ultipa.ResultType_RESULT_TYPE_UNSET == resultType {
				resultType = ultipa.ResultType_RESULT_TYPE_ATTR
			}
			//not null but len==0, then set an empty slice
			row := structs.Value{}

			for _, subOAttr := range oListData.Attrs {
				subAttr, err := parseAttr(subOAttr, "")
				if err != nil {
					return err
				}
				if subAttr == nil {
					row = append(row, nil)
				} else {
					row = append(row, subAttr.Values...)
				}
			}
			attr.Values = append(attr.Values, row)

		}
	}
	if ultipa.ResultType_RESULT_TYPE_UNSET == resultType {
		resultType = ultipa.ResultType_RESULT_TYPE_ATTR
	}
	attr.ResultType = resultType
	return nil
}

// parseAttrMap parse the Attr that PropertyType is ultipa.PropertyType_MAP
func parseAttrMap(oAttr *ultipa.Attr) ([]*structs.AttrMapData, error) {
	var mapDataRows []*structs.AttrMapData
	for _, v := range oAttr.Values {
		oMapData := &ultipa.AttrMapData{}
		mapData := structs.NewAttrMapData()
		err := proto.Unmarshal(v, oMapData)
		if err != nil {
			return nil, err
		}

		key, err := parseAttr(oMapData.Key, "")
		if err != nil {
			return nil, err
		}
		value, err := parseAttr(oMapData.Value, "")
		if err != nil {
			return nil, err
		}
		mapData.Key = key
		mapData.Value = value
		mapDataRows = append(mapDataRows, mapData)
	}
	return mapDataRows, nil
}

// AsGraphSets the types will be tables and alias is nodeSchema and edgeSchema
func (di *DataItem) AsGraphSets() (graphSets []*structs.GraphSet, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return graphSets, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_GRAPH_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a Graph list")
	}

	g, err := di.AsTable()
	if err != nil {
		return nil, err
	}

	values := g.ToKV()
	for _, v := range values {
		id := v.Get("id").(string)
		name := v.Get("name").(string)
		status := v.Get("status").(string)
		description := v.Get("description").(string)
		shards := v.Get("shards").(string)
		//replicaNum := v.Get("replica_num").(string)
		partitionBy := v.Get("partition_by").(string)

		var totalNodes uint64 = 0
		if v := v.Get("total_nodes"); v != nil {
			totalNodes, _ = strconv.ParseUint(v.(string), 10, 64)

		}
		var totalEdges uint64 = 0
		if v := v.Get("total_edges"); v != nil {
			totalEdges, err = strconv.ParseUint(v.(string), 10, 64)
		}
		var slotNum int
		if v := v.Get("slot_num"); v != nil {
			slotNum, err = strconv.Atoi(v.(string))
		}

		graphSets = append(graphSets, &structs.GraphSet{
			ID: id,
			//ClusterId:   clusterId,
			Name:        name,
			TotalNodes:  totalNodes,
			TotalEdges:  totalEdges,
			Status:      status,
			Description: description,
			Shards:      strings.Split(shards, ","),
			SlotNum:     slotNum,
			//ReplicaNum:  replicaNum,
			PartitionBy: partitionBy,
		})
	}

	return graphSets, err
}

func (di *DataItem) AsGraphCount() (graphCounts []*structs.GraphCount, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return graphCounts, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_GRAPH_COUNT_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a graph count list")
	}

	if len(table.Headers) <= 4 {
		return nil, nil
	}
	for _, row := range table.TableRows {

		//0:type, 1: schema, 2: from_schema,3:to_schema, 4:count
		values := row.GetValues()
		count, _ := strconv.ParseUint(string(values[4]), 10, 64)

		sp := &structs.SchemaStat{
			FromSchema: string(values[2]),
			ToSchema:   string(values[3]),
			Count:      count,
		}
		i := &structs.GraphCount{
			Type:   string(values[0]),
			Schema: string(values[1]),
			Stat:   sp,
		}
		graphCounts = append(graphCounts, i)
	}

	return graphCounts, err
}

// AsSchemas the types will be tables and alias is nodeSchema and edgeSchema
func (di *DataItem) AsSchemas() (schemas []*structs.Schema, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return schemas, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_NODE_SCHEMA_KEY && table.TableName != RESP_EDGE_SCHEMA_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a Schema list")
	}

	// node | edge
	//Type := ""
	dbType := ultipa.DBType_DBNODE
	// store index to get total number
	TotalIndex := 0
	switch table.TableName {
	case RESP_NODE_SCHEMA_KEY:
		dbType = ultipa.DBType_DBNODE
		//Type = "node"
		TotalIndex = 3
	case RESP_EDGE_SCHEMA_KEY:
		//Type = "edge"
		dbType = ultipa.DBType_DBEDGE
		TotalIndex = 3
	}

	var IdIndex = 0
	var NameIndex = 1
	//var StatusIndex = 2
	var DescIndex = 3
	var PropertyIndex = 4

	for index, header := range table.Headers {
		switch header.PropertyName {
		case "id":
			IdIndex = index
		case "name":
			NameIndex = index
		case "status":
			//StatusIndex = index
		case "description":
			DescIndex = index
		case "properties":
			PropertyIndex = index
		}
		//if header.PropertyName == "name" {
		//    NameIndex = index
		//} else if header.PropertyName == "description" {
		//    DescIndex = index
		//} else if header.PropertyName == "properties" {
		//    PropertyIndex = index
		//    //} else if header.PropertyName == "totalNodes" {
		//    //	TotalIndex = index
		//    //} else if header.PropertyName == "totalEdges" {
		//    //	TotalIndex = index
		//} else if header.PropertyName == "id" {
		//    IdIndex = index
		//}
	}

	for _, row := range table.TableRows {
		//0:name, 1: description, 2: json(properties),3:totalNodes, 4:totalEdges
		values := row.GetValues()
		schema := structs.NewSchema(string(values[NameIndex]))
		schema.Description = string(values[DescIndex])
		//schema.Type = Type
		//schema.Status = string(values[StatusIndex])
		propertyJson := values[PropertyIndex]
		schema.Total, _ = strconv.ParseUint(utils.AsString(values[TotalIndex]), 10, 64)
		schema.Id = string(values[IdIndex])
		schema.DBType = dbType

		var props []*struct {
			Name        string
			Type        string
			Description string
			Lte         string
			Extra       string
		}

		err = json.Unmarshal(propertyJson, &props)

		if err != nil {
			log.Fatalln(err)
		}

		for _, prop := range props {
			lte := false
			if prop.Lte != "" {
				lte, err = strconv.ParseBool(prop.Lte)
				if err != nil {
					log.Fatalln(err)
				}
			}

			var exta *structs.DecimalExtra
			if err := structs.UnmarshalDecimalExtra([]byte(prop.Extra), exta); err != nil {
				log.Fatalln(err)
			}

			p := structs.Property{
				Name:         prop.Name,
				Description:  prop.Description,
				Lte:          lte,
				Schema:       schema.Name,
				DecimalExtra: exta,
			}
			p.SetTypeByString(prop.Type)
			schema.Properties = append(schema.Properties, &p)
		}

		schemas = append(schemas, schema)
	}

	return schemas, err
}

// AsProperties the types will be tables and alias is nodeProperty and edgeProperty
func (di *DataItem) AsProperties() (properties []*structs.Property, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return properties, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_NODE_PROPERTY_KEY && table.TableName != RESP_EDGE_PROPERTY_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a Property list")
	}

	for _, row := range table.TableRows {
		//0:name, 1: type, 2: lte, 3: schema, 4: description
		values := row.GetValues()

		rowValues := map[string][]byte{}
		for idx, header := range table.Headers {
			rowValues[header.PropertyName] = values[idx]
		}

		name := getOrDefault("name", "", rowValues)
		lteStr := getOrDefault("lte", "false", rowValues)
		typeStr := getOrDefault("type", "", rowValues)
		read := getOrDefault("read", "0", rowValues)
		write := getOrDefault("write", "0", rowValues)
		schema := getOrDefault("schema", "0", rowValues)
		desc := getOrDefault("description", "", rowValues)
		lte, err := strconv.ParseBool(lteStr)
		extra := getOrDefault("extra", "", rowValues)
		encrypt := getOrDefault("encrypt", "", rowValues)
		if err != nil {
			log.Fatalln(err)
		}

		var exta *structs.DecimalExtra
		if err := structs.UnmarshalDecimalExtra([]byte(extra), exta); err != nil {
			log.Fatalln(err)
		}

		p := structs.Property{
			Name:         name,
			Description:  desc,
			Lte:          lte,
			Read:         "1" == read,
			Write:        "1" == write,
			Schema:       schema,
			DecimalExtra: exta,
			Encrypt:      encrypt,
		}
		p.SetTypeByString(typeStr)
		properties = append(properties, &p)

	}

	return properties, err
}

func getOrDefault(name string, defaultValue string, container map[string][]byte) string {
	bytes, ok := container[name]
	if ok {
		return string(bytes)
	}
	return defaultValue
}

// AsIndexes the types will be tables and alias is nodeIndex and edgeIndex
func (di *DataItem) AsIndexes() (indexes []*structs.Index, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return indexes, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)
	var indexType ultipa.DBType

	if table.TableName == RESP_NODE_INDEX_KEY {
		indexType = ultipa.DBType_DBNODE
	} else if table.TableName == RESP_EDGE_INDEX_KEY {
		indexType = ultipa.DBType_DBEDGE
	} else {
		return nil, errors.New("DataItem " + di.Alias + " is not a Index list")
	}

	for _, row := range table.TableRows {
		//0：id 1:name, 2: properties, 3: schema, 4: status
		values := row.GetValues()
		id := string(values[0])
		i := structs.Index{
			Id:         id,
			Name:       string(values[1]),
			Properties: string(values[2]),
			Schema:     string(values[3]),
			Status:     string(values[4]),
			//Size:       size,
			DBType: indexType,
		}
		indexes = append(indexes, &i)

	}

	return indexes, err
}

// AsFullTexts the types will be tables and alias is node fulltext Index and edge fulltext Index
func (di *DataItem) AsFullTexts() (fullTextIndexes []*structs.Index, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return fullTextIndexes, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	var indexType ultipa.DBType

	if table.TableName == RESP_NODE_FULLTEXT_KEY {
		indexType = ultipa.DBType_DBNODE
	} else if table.TableName == RESP_EDGE_FULLTEXT_KEY {
		indexType = ultipa.DBType_DBEDGE
	} else {
		return nil, errors.New("DataItem " + di.Alias + " is not a fullText list")
	}

	for _, row := range table.TableRows {
		//0:name, 1: properties, 2: schema, 3: status
		values := row.GetValues()

		i := structs.Index{
			Name:       string(values[0]),
			Properties: string(values[1]),
			Schema:     string(values[2]),
			Status:     string(values[3]),
			DBType:     indexType,
		}
		fullTextIndexes = append(fullTextIndexes, &i)

	}

	return fullTextIndexes, err
}

func (di *DataItem) AsAlgos() ([]*structs.Algo, error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table(algo) as pre-condition")
	}

	table, err := di.AsTable()

	if err != nil {
		return nil, err
	}

	var algos []*structs.Algo

	values := table.ToKV()

	for _, v := range values {
		algo := &structs.Algo{
			Name:             v.Get("name").(string),
			Version:          v.Get("version").(string),
			Type:             v.Get("type").(string),
			WriteSupportType: v.Get("write_support_type").(string),
			CanRollback:      v.Get("can_rollback").(string),
			Description:      v.Get("description").(string),
			ConfigContext:    v.Get("config_context").(string),
			Params:           nil,
		}

		algos = append(algos, algo)
	}

	return algos, nil
}

// AsGraph convert graphAlias to structs.Graph for uql syntax toGraph(listUnion(collect(n1), collect(n2)), collect(e)) as graph return graph
func (di *DataItem) AsGraph() (graph *structs.Graph, err error) {
	if di.Data == nil {
		return nil, nil
	}

	paths, err := di.AsPaths()
	if err != nil {
		return nil, err
	}

	if paths == nil {
		return nil, errors.New("empty graphs")
	}

	pathAlias := di.Data.(*ultipa.PathAlias)

	graph, err = parseGraphs(pathAlias.Paths)
	if err != nil {
		return nil, err
	}
	//for _, path := range paths {
	//	path.Nodes = nil
	//	path.Edges = nil
	//	graph.Paths = append(graph.Paths, path)
	//}
	graph.Paths = paths

	return graph, nil
}

func parseGraphs(oPaths []*ultipa.Path) (graph *structs.Graph, err error) {
	graph = structs.NewGraph()

	for _, oPath := range oPaths {
		//path.Name = name
		nodes, _, _, err := NodeTableToNodes(oPath.NodeTable, "")
		if err != nil {
			return nil, err
		}

		for _, node := range nodes {
			graph.Nodes[node.UUID] = node
		}

		edges, _, _, err := EdgeTableToEdges(oPath.EdgeTable, "")
		if err != nil {
			return nil, err
		}

		for _, edge := range edges {
			graph.Edges[edge.UUID] = edge
		}
	}
	return graph, nil
}

func (di *DataItem) AsAny() (interface{}, error) {

	switch di.Type {
	case ultipa.ResultType_RESULT_TYPE_ATTR:
		return di.AsAttr()
	//case ultipa.ResultType_RESULT_TYPE_ARRAY:
	//	return di.AsArray()
	case ultipa.ResultType_RESULT_TYPE_EDGE:
		edges, _, err := di.AsEdges()
		return edges, err
	case ultipa.ResultType_RESULT_TYPE_NODE:
		nodes, _, err := di.AsNodes()
		return nodes, err
	case ultipa.ResultType_RESULT_TYPE_TABLE:
		return di.AsTable()
	//case ultipa.ResultType_RESULT_TYPE_GRAPH:
	//	return di.AsGraph()
	default:
		return di.Data, nil
	}

}

func (di *DataItem) AsPolicies() (policies []*structs.Policy, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nil, errors.New("RESULT_TYPE_UNSET")
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_POLICY_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a policy list")
	}

	var policy *structs.Policy
	for _, row := range table.TableRows {
		//0:name, 1: properties, 2: schema, 3: status
		values := row.GetValues()
		policy, err = bytesToPolicy(values)
		if err != nil {
			return nil, err
		}

		policies = append(policies, policy)
	}

	return policies, err
}

func bytesToPolicy(data [][]byte) (*structs.Policy, error) {
	if len(data) != 5 {
		return nil, fmt.Errorf("invalid data length, expected 4 but got %d", len(data))
	}

	var policy structs.Policy

	// Name
	policy.Name = string(data[0])

	// GraphPrivileges
	if err := json.Unmarshal(data[1], &policy.GraphPrivileges); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GraphPrivileges: %w", err)
	}

	// SystemPrivileges
	if err := json.Unmarshal(data[2], &policy.SystemPrivileges); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SystemPrivileges: %w", err)
	}

	//PropertyPrivileges
	if err := json.Unmarshal(data[3], &policy.PropertyPrivileges); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PropertyPrivileges: %w", err)
	}

	// AsPolicies
	if err := json.Unmarshal(data[4], &policy.Policies); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AsPolicies: %w", err)
	}

	return &policy, nil
}

func (di *DataItem) AsExtas() (extas []*structs.Exta, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nil, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_EXTAS_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a exta list")
	}

	for _, row := range table.TableRows {
		values := row.GetValues()

		i := structs.Exta{
			Name:    string(values[0]),
			Author:  string(values[1]),
			Version: string(values[2]),
			Detail:  string(values[3]),
		}
		extas = append(extas, &i)

	}

	return extas, nil
}

func (di *DataItem) AsTasks() (tasks []*structs.Task, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nil, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_TASK_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a task list")
	}

	type TempTask struct {
		Param    string            `json:"param"`
		TaskInfo structs.TaskInfo  `json:"task_info"`
		Result   map[string]string `json:"result"`
		ErrorMsg string            `json:"error_msg"`
	}

	for _, row := range table.TableRows {
		values := row.GetValues()

		var tempTask TempTask
		err = json.Unmarshal(values[0], &tempTask)
		if err != nil {
			return nil, err
		}

		var param map[string]string
		err = json.Unmarshal([]byte(tempTask.Param), &param)
		if err != nil {
			return nil, err
		}

		task := structs.Task{
			Param:    param,
			TaskInfo: tempTask.TaskInfo,
			Result:   tempTask.Result,
			ErrorMsg: tempTask.ErrorMsg,
		}

		tasks = append(tasks, &task)
	}

	return tasks, nil
}

func (di *DataItem) AsProcesses() (tops []*structs.Process, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return tops, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_TOP_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a top list")
	}

	for _, row := range table.TableRows {
		values := row.GetValues()

		i := structs.Process{
			ProcessId:    string(values[0]),
			Status:       string(values[1]),
			ProcessQuery: string(values[2]),
			Duration:     string(values[3]),
		}
		tops = append(tops, &i)

	}

	return tops, err
}

func (di *DataItem) AsStats() (stat *structs.Stats, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nil, errors.New("ResultType_RESULT_TYPE_UNSET")
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_STATISTIC_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a stats list")
	}

	row := table.TableRows[0]
	values := row.GetValues()

	dateStr := string(values[2])
	parsedTime, err := time.Parse("Mon Jan 2 15:04:05 2006", dateStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing date: %v", err)
	}
	formattedDate := parsedTime.Format("2006-01-02 15:04:05")

	s := &structs.Stats{
		CPUUsage:    string(values[0]),
		MemUsage:    string(values[1]),
		ExpiredDate: formattedDate,
		CPUCores:    string(values[3]),
		Company:     string(values[4]),
		ServerType:  string(values[5]),
		Version:     string(values[6]),
	}
	stat = s

	return stat, err
}

func (di *DataItem) AsPrivileges() (privileges []*structs.Privilege, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nil, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_PRIVILEGE_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a privilege list")
	}

	for _, row := range table.TableRows {
		values := row.GetValues()

		var graphPrivileges, systemPrivileges []string
		err = json.Unmarshal(values[0], &graphPrivileges)
		if err != nil {
			return nil, errors.New("graphPrivileges Unmarshal failed" + err.Error())
		}
		err = json.Unmarshal(values[1], &systemPrivileges)
		if err != nil {
			return nil, errors.New("systemPrivileges Unmarshal failed" + err.Error())
		}
		for _, gp := range graphPrivileges {
			p := structs.Privilege{
				Name:  gp,
				Level: structs.GraphPrivilege,
			}
			privileges = append(privileges, &p)
		}
		for _, gp := range systemPrivileges {
			p := structs.Privilege{
				Name:  gp,
				Level: structs.SystemPrivilege,
			}
			privileges = append(privileges, &p)
		}

	}

	return privileges, nil
}

func (di *DataItem) AsUsers() (users []*structs.User, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nil, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_USER_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a user list")
	}

	g, err := di.AsTable()
	if err != nil {
		return nil, err
	}

	values := g.ToKV()

	for _, v := range values {
		user := &structs.User{
			UserName:    v.Get("username").(string),
			CreatedTime: v.Get("create").(*utils.UltipaTime).String(),
		}

		if err := json.Unmarshal([]byte(v.Get("graphPrivileges").(string)), &user.GraphPrivileges); err != nil {
			return nil, fmt.Errorf("failed to unmarshal GraphPrivileges: %w", err)
		}

		// SystemPrivileges
		if err := json.Unmarshal([]byte(v.Get("systemPrivileges").(string)), &user.SystemPrivileges); err != nil {
			return nil, fmt.Errorf("failed to unmarshal SystemPrivileges: %w", err)
		}

		// propertyPrivileges
		if err := json.Unmarshal([]byte(v.Get("propertyPrivileges").(string)), &user.PropertyPrivileges); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PropertyPrivileges: %w", err)
		}

		// AsPolicies
		if err := json.Unmarshal([]byte(v.Get("policies").(string)), &user.Policies); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Policies: %w", err)
		}

		users = append(users, user)
	}

	return users, err

	for _, row := range table.TableRows {
		values := row.GetValues()

		user, err := bytesToUser(values)
		if err != nil {
			return nil, err
		}

		users = append(users, user)

	}

	return users, nil
}

func bytesToUser(data [][]byte) (*structs.User, error) {
	if len(data) != 6 {
		return nil, fmt.Errorf("invalid data length, expected 6 but got %d", len(data))
	}

	var user structs.User

	// UserName
	user.UserName = string(data[0])

	// CreateTime
	timestamp, _ := strconv.ParseInt(string(data[1]), 10, 64)
	create := time.Unix(timestamp, 0)
	user.CreatedTime = create.Format("2006-01-02 15:04:05")

	// LastLogin
	//timestamp, _ = strconv.ParseInt(string(data[1]), 10, 64)
	//lastLogin := time.Unix(timestamp, 0)
	//user.LastLogin = lastLogin.Format("2006-01-02 15:04:05")

	// GraphPrivileges
	if err := json.Unmarshal(data[2], &user.GraphPrivileges); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GraphPrivileges: %w", err)
	}

	// SystemPrivileges
	if err := json.Unmarshal(data[3], &user.SystemPrivileges); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SystemPrivileges: %w", err)
	}

	// propertyPrivileges
	if err := json.Unmarshal(data[4], &user.PropertyPrivileges); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PropertyPrivileges: %w", err)
	}

	// AsPolicies
	if err := json.Unmarshal(data[5], &user.Policies); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Policies: %w", err)
	}

	return &user, nil
}

func (di *DataItem) AsJobs() (jobs []*structs.Job, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return nil, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_JOB_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a job list")
	}

	for _, row := range table.TableRows {
		values := row.GetValues()

		job := structs.Job{
			Id:        string(values[0]),
			GraphName: string(values[1]),
			Type:      string(values[2]),
			Query:     string(values[3]),
			Status:    string(values[4]),
			ErrMsg:    string(values[5]),
			Result:    bytes2result(values[6]),
			StartTime: string(values[7]),
			EndTime:   string(values[8]),
			Progress:  string(values[9]),
		}

		jobs = append(jobs, &job)
	}

	return jobs, nil
}

func bytes2result(data []byte) map[string]string {

	if len(data) == 0 {
		return nil
	}

	var result map[string]interface{}

	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println("Error Unmarshal result:", err)
		return nil
	}

	finalResult := make(map[string]string)

	for key, value := range result {
		finalResult[key] = fmt.Sprintf("%v", value)
	}

	return finalResult
}

func (di *DataItem) AsHDCGraphs() (projections []*structs.HDCGraph, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return projections, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_HDCGRAPH_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a Graph list")
	}

	g, err := di.AsTable()
	if err != nil {
		return nil, err
	}

	values := g.ToKV()

	for _, v := range values {
		projection := &structs.HDCGraph{
			Name:            v.Get("name").(string),
			GraphName:       v.Get("graph_name").(string),
			Status:          v.Get("status").(string),
			Stats:           v.Get("stats").(string),
			IsDefault:       v.Get("is_default").(string),
			HDCServerName:   v.Get("hdc_server_name").(string),
			HDCServerStatus: v.Get("hdc_server_status").(string),
			Config:          v.Get("config").(string),
		}

		projections = append(projections, projection)
	}

	return projections, err
}

func (di *DataItem) AsProjections() (projections []*structs.Projection, err error) {
	if di.Data == nil {
		return nil, nil
	}

	if di.Type == ultipa.ResultType_RESULT_TYPE_UNSET {
		return projections, nil
	}

	if di.Type != ultipa.ResultType_RESULT_TYPE_TABLE {
		return nil, errors.New("DataItem " + di.Alias + " should be a table as pre-condition")
	}

	table := di.Data.(*ultipa.Table)

	if table.TableName != RESP_PROJECTION_KEY {
		return nil, errors.New("DataItem " + di.Alias + " is not a Projection list")
	}

	g, err := di.AsTable()
	if err != nil {
		return nil, err
	}

	values := g.ToKV()

	for _, v := range values {
		projection := &structs.Projection{
			Name:      v.Get("name").(string),
			GraphName: v.Get("graph_name").(string),
			Status:    v.Get("status").(string),
			Stats:     v.Get("stats").(string),
			//IsDefault:       v.Get("is_default").(string),
			//HDCServerName:   v.Get("hdc_server_name").(string),
			//HDCServerStatus: v.Get("hdc_server_status").(string),
			Config: v.Get("config").(string),
		}

		projections = append(projections, projection)
	}

	return projections, err
}
