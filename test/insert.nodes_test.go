package test

import (
	"fmt"
	"log"
	"testing"

	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/printers"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
	"github.com/ultipa/ultipa-go-driver/sdk/types"
	ultipaUtils "github.com/ultipa/ultipa-go-driver/sdk/utils"
)

func TestInsertNodeWithListProperty(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	schema := structs.NewSchema("default")
	schema.Properties = append(schema.Properties, &structs.Property{
		Name:     "name",
		Type:     ultipa.PropertyType_LIST,
		SubTypes: []ultipa.PropertyType{ultipa.PropertyType_STRING},
	})

	var nodes []*structs.Node
	node := structs.NewNode()

	node.Set("name", []string{"lzq", "list", "set", "map"})

	nodes = append(nodes, node)

	resp, err := client.InsertNodesBatchBySchema(schema, nodes, &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_OVERWRITE,
	})

	if err != nil {
		t.Fatal(err)
	}

	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
		log.Println(resp.Status.Message)
		t.Log(resp.Status.Message)
	}
	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)
}

func TestInsertPointProperty(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	schema := structs.NewSchema("nodeSchemaList")
	schema.Properties = append(schema.Properties, &structs.Property{
		Name:     "typePoint",
		Type:     ultipa.PropertyType_POINT,
		SubTypes: nil,
	})

	var nodes []*structs.Node
	nodeWithPointer := structs.NewNode()
	nodeWithPointer.Set("typePoint", types.NewPoint(1.01, -2.01))

	nodeWithValue := structs.NewNode()
	nodeWithValue.Set("typePoint", types.Point{
		Latitude:  100.90,
		Longitude: 80.11,
	})

	nodeWithString := structs.NewNode()
	nodeWithString.Set("typePoint", "point(1.03 26.05)")

	nodes = append(nodes, nodeWithPointer, nodeWithValue, nodeWithString)

	resp, err := client.InsertNodesBatchBySchema(schema, nodes, &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_OVERWRITE,
	})

	if err != nil {
		t.Fatal(err)
	}

	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
		log.Println(resp.Status.Message)
		t.Log(resp.Status.Message)
	}
	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)

}

func TestInsertBlobProperty(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	schemaName := "node_schema"
	schema := structs.NewSchema(schemaName)
	schema.Properties = append(schema.Properties, &structs.Property{
		Name:     "name",
		Type:     ultipa.PropertyType_TEXT,
		SubTypes: nil,
	}, &structs.Property{
		Name:     "blob_prop",
		Type:     ultipa.PropertyType_BLOB,
		SubTypes: nil,
	})

	var nodes []*structs.Node
	node1 := structs.NewNode()
	node1.Set("name", "go_sdk")
	node1.Set("blob_prop", []byte{97, 98, 99})

	node2 := structs.NewNode()
	node2.Set("name", "test")
	node2.Set("blob_prop", "def")

	nodes = append(nodes, node1, node2)

	resp, err := client.InsertNodesBatchBySchema(schema, nodes, &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_OVERWRITE,
	})

	if err != nil {
		t.Fatal(err)
	}

	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
		log.Println(resp.Status.Message)
		t.Log(resp.Status.Message)
	}
	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)

	uql := fmt.Sprintf("find().nodes({@%s}) as nodes return nodes{*}", schemaName)
	response, err := client.Uql(uql, nil)

	if err != nil {
		t.Fatal(err)
	}

	nodes, schemas, err := response.Alias("nodes").AsNodes()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintNodes(nodes, schemas)
}

func TestInsertDecimalProperty(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	schemaName := "default"
	schema := structs.NewSchema(schemaName)
	schema.Properties = append(schema.Properties, &structs.Property{
		Name:     "name",
		Type:     ultipa.PropertyType_STRING,
		SubTypes: nil,
	}, &structs.Property{
		Name:     "salary",
		Type:     ultipa.PropertyType_DECIMAL,
		SubTypes: nil,
	})

	var nodes []*structs.Node
	node1 := structs.NewNode()
	node1.Set("name", "go_sdk")
	node1.Set("salary", "6.1")

	node2 := structs.NewNode()
	node2.Set("name", "test")
	node2.Set("salary", 6.1)

	nodes = append(nodes, node1, node2)

	resp, err := client.InsertNodesBatchBySchema(schema, nodes, &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_OVERWRITE,
	})

	if err != nil {
		t.Fatal(err)
	}

	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
		log.Println(resp.Status.Message)
		t.Log(resp.Status.Message)
	}
	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)

	uql := fmt.Sprintf("find().nodes({@%s}) as nodes return nodes{*}", schemaName)
	response, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	nodes, schemas, err := response.Alias("nodes").AsNodes()
	if err != nil {
		t.Fatal(err)
	}

	printers.PrintNodes(nodes, schemas)
}

func TestInsertNodeWithSetProperty(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	schema := structs.NewSchema("default")
	schema.Properties = append(schema.Properties, &structs.Property{
		Name:     "string_set",
		Type:     ultipa.PropertyType_SET,
		SubTypes: []ultipa.PropertyType{ultipa.PropertyType_STRING},
	})

	var nodes []*structs.Node
	node := structs.NewNode()

	node.Set("string_set", []string{"list", "set", "map"})

	nodes = append(nodes, node)

	resp, err := client.InsertNodesBatchBySchema(schema, nodes, &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_OVERWRITE,
	})

	if err != nil {
		t.Fatal(err)
	}

	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
		log.Println(resp.Status.Message)
		t.Log(resp.Status.Message)
	}
	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)
}

func TestInsertNodes(t *testing.T) {
	//client.SetCurrentGraph("go_sdk_test")
	schemaName := "default"

	ty := ultipa.DBType_DBNODE
	client.Truncate(&structs.TruncateParams{
		GraphName:  graph,
		DBType:     &ty,
		SchemaName: schemaName,
	}, nil)

	prop := &structs.Property{
		Schema: schemaName,
		Name:   "name",
		Type:   ultipa.PropertyType_STRING,
	}
	client.CreateNodeProperty(prop, nil)
	prop = &structs.Property{
		Schema: schemaName,
		Name:   "salary",
		Type:   ultipa.PropertyType_DOUBLE,
	}
	client.CreateNodeProperty(prop, nil)

	var nodes []*structs.Node
	node1 := structs.NewNode()
	node1.ID = "11131"
	node1.Set("name", "go_sdk")
	node1.Set("salary", "6.1")

	node2 := structs.NewNode()
	node2.ID = "223222"
	node2.Set("name", "test")
	node2.Set("salary", 6.1)

	node3 := structs.NewNode()
	node3.ID = "333433"
	//node3.Set("name", "test2")

	nodes = append(nodes, node1, node2, node3)

	uql := structs.NodesToInsertUql(nodes)
	t.Log(uql)

	config := &configuration.InsertRequestConfig{
		RequestConfig: &configuration.RequestConfig{},
		Silent:        true,
	}

	response, err := client.InsertNodes(schemaName, nodes, config)
	if err != nil {
		t.Error(err)
	}

	t.Log(response)

	// overwrite
	node1.Set("name", "go_sdk2")

	uql = structs.NodesToInsertUql(nodes)
	t.Log(uql)

	config = &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_OVERWRITE,
		Silent:     true,
	}

	response, err = client.InsertNodes(schemaName, nodes, config)
	if err != nil {
		t.Error(err)
	}

	t.Log(response)

	// upsert
	node2.Set("salary", 10.1)

	uql = structs.NodesToInsertUql(nodes)
	log.Println(uql)

	config = &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_UPSERT,
		Silent:     true,
	}

	response, err = client.InsertNodes(schemaName, nodes, config)
	if err != nil {
		t.Error(err)
	}

	t.Log(response)
}

func TestInsertBoolProperty(t *testing.T) {
	client, _ := GetClient([]string{"192.168.1.85:61099"}, "sdk_test")
	schema := structs.NewSchema("People")
	schema.Properties = append(schema.Properties, &structs.Property{
		Name: "bool_prop",
		Type: ultipa.PropertyType_BOOL,
	})

	var nodes []*structs.Node
	node1 := structs.NewNode()
	node1.ID = "1"
	node1.Set("bool_prop", true)

	node2 := structs.NewNode()
	node2.ID = "2"
	node2.Set("bool_prop", "true")

	node3 := structs.NewNode()
	node3.ID = "3"
	node3.Set("bool_prop", "false")

	node4 := structs.NewNode()
	node4.ID = "4"
	node4.Set("bool_prop", false)

	nodes = append(nodes, node1, node2, node3, node4)

	resp, err := client.InsertNodesBatchBySchema(schema, nodes, &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_OVERWRITE,
	})

	if err != nil {
		log.Fatalln(err)
	}

	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
		log.Println(resp.Status.Message)
		t.Log(resp.Status.Message)
	}
	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)

}

func TestRpcNode(t *testing.T) {

	p := []*structs.Property{
		{Name: "typeTimestamp", Type: ultipa.PropertyType_TIMESTAMP},
		{Name: "typeInt32", Type: ultipa.PropertyType_INT32},
		{Name: "typeNotMatch", Type: ultipa.PropertyType_TIMESTAMP},
		{Name: "testPoint", Type: ultipa.PropertyType_POINT},
		{Name: "typeFloat", Type: ultipa.PropertyType_FLOAT},
		{Name: "typeDouble", Type: ultipa.PropertyType_DOUBLE},
		{Name: "typeInt64", Type: ultipa.PropertyType_INT64},
		{Name: "typeUint32", Type: ultipa.PropertyType_UINT32},
		{Name: "typeUint64", Type: ultipa.PropertyType_UINT64},
		{Name: "typeDatetime", Type: ultipa.PropertyType_DATETIME},
		{Name: "typeString", Type: ultipa.PropertyType_STRING},
		{Name: "typeText", Type: ultipa.PropertyType_TEXT},
		{Name: "typeListString", Type: ultipa.PropertyType_LIST, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_STRING}},
		{Name: "typeListInt32", Type: ultipa.PropertyType_LIST, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_INT32}},
		{Name: "typeListInt64", Type: ultipa.PropertyType_LIST, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_INT64}},
		{Name: "typeListUint32", Type: ultipa.PropertyType_LIST, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_UINT32}},
		{Name: "typeListUint64", Type: ultipa.PropertyType_LIST, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_UINT64}},
		{Name: "typeListFloat", Type: ultipa.PropertyType_LIST, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_FLOAT}},
		{Name: "typeListDouble", Type: ultipa.PropertyType_LIST, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_DOUBLE}},
		{Name: "typeListDatetime", Type: ultipa.PropertyType_LIST, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_DATETIME}},
		{Name: "typeListTimestamp", Type: ultipa.PropertyType_LIST, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_TIMESTAMP}},
		{Name: "typeListText", Type: ultipa.PropertyType_LIST, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_TEXT}},
		{Name: "typeDecimal", Type: ultipa.PropertyType_DECIMAL},
		{Name: "typeSetString", Type: ultipa.PropertyType_SET, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_STRING}},
		{Name: "typeSetInt32", Type: ultipa.PropertyType_SET, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_INT32}},
		{Name: "typeSetInt64", Type: ultipa.PropertyType_SET, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_INT64}},
		{Name: "typeSetUint32", Type: ultipa.PropertyType_SET, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_UINT32}},
		{Name: "typeSetUint64", Type: ultipa.PropertyType_SET, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_UINT64}},
		{Name: "typeSetFloat", Type: ultipa.PropertyType_SET, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_FLOAT}},
		{Name: "typeSetDouble", Type: ultipa.PropertyType_SET, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_DOUBLE}},
		{Name: "typeSetDatetime", Type: ultipa.PropertyType_SET, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_DATETIME}},
		{Name: "typeSetTimestamp", Type: ultipa.PropertyType_SET, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_TIMESTAMP}},
		{Name: "typeSetText", Type: ultipa.PropertyType_SET, SubTypes: []ultipa.PropertyType{ultipa.PropertyType_TEXT}},
	}
	schema := structs.Schema{Name: "insertNode", Properties: p}
	timestamp1, _ := ultipaUtils.NewTimestampFromString("2023-1-2", nil)
	timestamp2, _ := ultipaUtils.NewTimestampFromString("2023-1-2T15:04:05.000+0700", nil)
	row := []*structs.Node{
		{ID: "49", Schema: "insertNode3", Values: &structs.Values{Data: map[string]interface{}{"typeTimestamp": 1734591775,
			"typeInt32": int32(1), "typeNotMatch": timestamp2.GetTimeStamp(), "testPoint": "Point(13.12 15.22)",
			"typeFloat": float32(-3.4028234e38), "typeDouble": float64(-1.7976931e308),
			"typeInt64": int64(-9223372036854775807), "typeUint32": uint32(0), "typeUint64": uint64(0),
			"typeDatetime": timestamp1.Datetime, "typeString": "encryptString2",
			"typeText": "encryptText2", "typeListString": []string{"encryptString1",
				"typeListString2", ""}, "typeListInt32": []int32{int32(-2147483648), int32(2147483647)},
			"typeListInt64":  []int64{int64(-9223372036854775808), int64(9223372036854775807)},
			"typeListUint32": []uint32{uint32(0), uint32(4294967295)}, "typeListUint64": []uint64{uint64(0), uint64(18446744073709551615)},
			"typeListFloat":  []float32{float32(-3.4028234e38), float32(3.4028234e38)},
			"typeListDouble": []float64{float64(-1.7976931e308), float64(1.7976931e308)}, "typeListDatetime": []uint64{timestamp1.Datetime, timestamp1.Datetime},
			"typeListTimestamp": []uint32{timestamp2.GetTimeStamp(), timestamp1.GetTimeStamp(), timestamp1.GetTimeStamp(), timestamp2.GetTimeStamp()},
			"typeListText":      []string{"encryptText1", "typeListText2", ""}, "typeDecimal": int32(1),
			"typeSetString":    []interface{}{"setString", "setString1", "setString1"},
			"typeSetInt32":     []interface{}{int32(1), int32(2), int32(3), int32(2)},
			"typeSetInt64":     []interface{}{int64(1), int64(2), int64(3), int64(2)},
			"typeSetUint32":    []interface{}{uint32(1), uint32(0), uint32(2), uint32(1)},
			"typeSetUint64":    []interface{}{uint64(1), uint64(0), uint64(2), uint64(1)},
			"typeSetFloat":     []interface{}{float32(1.23), float32(2.9), float32(0.892), float32(1.23)},
			"typeSetDouble":    []interface{}{float64(22.11), float64(9.01), float64(22.11), float64(1.2314)},
			"typeSetDatetime":  []interface{}{timestamp1.Datetime, timestamp1.Datetime},
			"typeSetTimestamp": []interface{}{timestamp2.GetTimeStamp(), timestamp2.GetTimeStamp(), timestamp1.GetTimeStamp()},
			"typeSetText":      []interface{}{"1", "2", "1", "3"},
		},
		}},
		{ID: "50", Schema: "insertNode4", Values: &structs.Values{Data: map[string]interface{}{"typeTimestamp": timestamp1.GetTimeStamp(),
			"typeInt32": int32(1), "typeNotMatch": timestamp2.GetTimeStamp(), "testPoint": "Point(13.12 15.22)",
			"typeFloat": float32(-3.4028234e38), "typeDouble": float64(-1.7976931e308),
			"typeInt64": int64(-9223372036854775807), "typeUint32": uint32(0), "typeUint64": uint64(0),
			"typeDatetime": timestamp1.Datetime, "typeString": "encryptString2",
			"typeText": "encryptText2", "typeListString": []string{"encryptString1",
				"typeListString2", ""}, "typeListInt32": []int32{int32(-2147483648), int32(2147483647)},
			"typeListInt64":  []int64{int64(-9223372036854775808), int64(9223372036854775807)},
			"typeListUint32": []uint32{uint32(0), uint32(4294967295)}, "typeListUint64": []uint64{uint64(0), uint64(18446744073709551615)},
			"typeListFloat":  []float32{float32(-3.4028234e38), float32(3.4028234e38)},
			"typeListDouble": []float64{float64(-1.7976931e308), float64(1.7976931e308)}, "typeListDatetime": []uint64{timestamp1.Datetime},
			"typeListTimestamp": []uint32{timestamp2.GetTimeStamp(), timestamp1.GetTimeStamp(), timestamp1.GetTimeStamp(), timestamp2.GetTimeStamp()},
			"typeListText":      []string{"encryptText1", "typeListText2", ""}, "typeDecimal": int32(1),
			"typeSetString":    []interface{}{"setString", "setString1", "setString1"},
			"typeSetInt32":     []interface{}{int32(1), int32(2), int32(3), int32(2)},
			"typeSetInt64":     []interface{}{int64(1), int64(2), int64(3), int64(2)},
			"typeSetUint32":    []interface{}{uint32(1), uint32(0), uint32(2), uint32(1)},
			"typeSetUint64":    []interface{}{uint64(1), uint64(0), uint64(2), uint64(1)},
			"typeSetFloat":     []interface{}{float32(1.23), float32(2.9), float32(0.892), float32(1.23)},
			"typeSetDouble":    []interface{}{float64(22.11), float64(9.01), float64(22.11), float64(1.2314)},
			"typeSetDatetime":  []interface{}{timestamp1.Datetime, timestamp1.Datetime},
			"typeSetTimestamp": []interface{}{timestamp2.GetTimeStamp(), timestamp2.GetTimeStamp(), timestamp1.GetTimeStamp()},
			"typeSetText":      []interface{}{"1", "2", "1", "3"},
		},
		}}}
	res, err := client.InsertNodesBatchBySchema(&schema, row, &configuration.InsertRequestConfig{InsertType: ultipa.InsertType_NORMAL})
	//rr,er := conn.InsertNodesBatchAuto()
	re, _ := client.Uql("find().nodes({_id=='rpc_1'}) as nodes return nodes{*}", nil)
	nodes, c, err := re.Alias("nodes").AsNodes()
	printers.PrintNodes(nodes, c)
	fmt.Println(res)
	fmt.Println(re)
	fmt.Println(err)
}
