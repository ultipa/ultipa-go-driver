package test

import (
	"fmt"
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/pieterclaerhout/go-waitgroup"
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/api"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils"
)

func TestBatchInsertNodes(t *testing.T) {
	//conn, _ := GetClient(hosts, graph)
	//client.SetCurrentGraph("go_sdk_test")
	schema := "text_schema"
	createSchema(t, schema, client)
	batchInsert(schema, client)
	checkInsertionResult(t, client, schema)
}

func batchInsert(schema string, conn *api.UltipaAPI) []*structs.Node {
	total := 500
	finished := 0

	wg := waitgroup.NewWaitGroup(20)

	var nodes []*structs.Node

	start := time.Now()
	rand.Seed(int64(time.Now().Second()))
	for {

		if total < 1 {
			break
		}

		node := structs.NewNode()

		node.ID = "AA" + fmt.Sprint(total)
		value := rand.Intn(1000)
		node.Set("username", fmt.Sprintf("user_%d", value))
		node.Set("password", RandStr(2000))
		node.Set("timestamp", 1234567890123456789)

		nodes = append(nodes, node)

		total--

		if total%30000 == 0 || total < 0 {

			wg.BlockAdd()
			go func(nodes []*structs.Node) {
				defer wg.Done()
				schema := structs.NewSchema(schema)
				schema.Properties = append(schema.Properties, &structs.Property{
					Name: "username",
					Type: ultipa.PropertyType_STRING,
				}, &structs.Property{
					Name: "password",
					Type: ultipa.PropertyType_TEXT,
				},
					&structs.Property{
						Name: "timestamp",
						Type: ultipa.PropertyType_TIMESTAMP,
					})

				_, err := conn.InsertNodesBatchBySchema(schema, nodes, &configuration.InsertRequestConfig{
					InsertType: ultipa.InsertType_OVERWRITE,
				})

				finished += len(nodes)
				if err != nil {
					log.Println(err)
					return
				}

				log.Printf("finished: %v, speed: %v", finished, float64(finished)/time.Since(start).Seconds())
			}(nodes)

			nodes = []*structs.Node{}
		}
	}

	wg.Wait()
	return nodes
}

func createSchema(t *testing.T, schema string, conn *api.UltipaAPI) {
	newSchemaWithProperties := &structs.Schema{
		Name:        schema,
		Description: "A Schema with 2 properties",
		Properties: []*structs.Property{
			{
				Schema: schema,
				Name:   "username",
				Type:   ultipa.PropertyType_STRING,
			},
			{
				Schema: schema,
				Name:   "password",
				Type:   ultipa.PropertyType_TEXT,
			},
			{
				Schema: schema,
				Name:   "timestamp",
				Type:   ultipa.PropertyType_TIMESTAMP,
			},
		},
	}

	_, err := conn.CreateSchemaIfNotExist(newSchemaWithProperties, true, nil)
	if err != nil {
		t.Error("failed to create schema", err)
	}
}

func checkInsertionResult(t *testing.T, conn *api.UltipaAPI, schema string) {
	// wait for data saved
	time.Sleep(time.Duration(2) * time.Second)
	resp3, err := conn.Uql(fmt.Sprintf("find().nodes({@%s}) as nodes return nodes{*}", schema), nil)
	if err != nil {
		t.Errorf("failed to query insertion result. %v", err)
	}
	//log.Println(resp3)
	nodes, _, err := resp3.Alias("nodes").AsNodes()
	//printers.PrintNodes(nodes, schemas)
	//printers.PrintNodes(nodes, schemas)
	if len(nodes) != 500 {
		t.Errorf("expected 500, got %d", len(nodes))
	}
}

func TestBatchInsertEdges(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	//client.SetCurrentGraph("go_sdk_test")

	total := 500
	finished := 0

	wg := waitgroup.NewWaitGroup(10)

	var edges []*structs.Edge

	schema := structs.NewSchema("default")
	//schema.Properties = append(schema.Properties, &structs.Property{
	//    Name: "e1",
	//    DBType: ultipa.PropertyType_STRING,
	//}, &structs.Property{
	//    Name: "e2",
	//    DBType: ultipa.PropertyType_STRING,
	//}, &structs.Property{
	//    Name: "e3",
	//    DBType: ultipa.PropertyType_STRING,
	//}, &structs.Property{
	//    Name: "e4",
	//    DBType: ultipa.PropertyType_STRING,
	//}, &structs.Property{
	//    Name: "e5",
	//    DBType: ultipa.PropertyType_STRING,
	//})

	start := time.Now()
	rand.Seed(int64(time.Now().Second()))
	for {

		if total < 1 {
			break
		}

		edge := structs.NewEdge()
		edge.From = "AA" + fmt.Sprint(total)
		edge.To = "AA" + fmt.Sprint(total+1)

		//edge.Set("e1", "abc")
		//edge.Set("e2", "abc")
		//edge.Set("e3", "abc")
		//edge.Set("e4", "abc")
		//edge.Set("e5", "abc")

		edges = append(edges, edge)

		total--

		if total%30000 == 0 || total < 0 {

			wg.BlockAdd()
			go func(edges []*structs.Edge) {
				defer wg.Done()

				_, err := client.InsertEdgesBatchBySchema(schema, edges, &configuration.InsertRequestConfig{
					InsertType: ultipa.InsertType_OVERWRITE,
				})

				finished += len(edges)
				if err != nil {
					log.Println(err)
					return
				}

				log.Printf("finished: %v, speed: %v", finished, float64(finished)/time.Since(start).Seconds())
			}(edges)

			edges = []*structs.Edge{}
		}
	}

	wg.Wait()

}

func TestCheckPropAndValueAutoData(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	timestamp1, _ := utils.NewTimestampFromString("2018-08-17T09:57:33+08:00", nil)
	schemaName := "nodeSchema2"
	//timestamp2, _ := utils.NewTimestampFromString("2018-08-17 09:57:33", nil)

	// create schema
	schema := structs.NewSchema(schemaName)
	schema.Properties = append(schema.Properties, &structs.Property{
		Name: "typeTimestamp",
		Type: ultipa.PropertyType_TIMESTAMP,
	}, &structs.Property{
		Name: "typeInt32",
		Type: ultipa.PropertyType_INT32,
	}, &structs.Property{
		Name: "typeNotMatch",
		Type: ultipa.PropertyType_UINT32,
	})

	_, err := client.CreateSchema(schema, true, nil)
	if err != nil {
		t.Fatalf("CreateSchema error ,%v", err)
	}

	defer func() {
		client.DropSchema(schema, nil)
	}()

	node1 := structs.Node{
		Values: &structs.Values{
			Data: map[string]interface {
			}{
				"typeTimestamp": timestamp1.GetTimeStamp(), "typeInt32": int32(1), "typeNotMatch": timestamp1.GetTimeStamp()}}, Schema: schemaName}
	node2 := structs.Node{
		Values: &structs.Values{
			Data: map[string]interface {
			}{
				"typeTimestamp": timestamp1.GetTimeStamp(), "typeInt32": int32(1), "typeNotMatch": timestamp1.GetTimeStamp(), "typeInt32Error": int32(1)}}, Schema: schemaName}
	node3 := structs.Node{
		Values: &structs.Values{
			Data: map[string]interface {
			}{
				"typeTimestamp": "2019-12-12 15:59:59"}}, Schema: schemaName}
	node4 := structs.Node{
		Values: &structs.Values{
			Data: map[string]interface {
			}{}}, Schema: schemaName}
	node5 := structs.Node{
		Values: &structs.Values{
			Data: map[string]interface {
			}{
				"typeTimestamp": "2019-12-12 15:59:59", "typeInt32": int32(1), "typeInt32Error": int32(1)}}, Schema: schemaName}
	rows1 := []*structs.Node{&node1, &node2}
	rows2 := []*structs.Node{&node1, &node1, &node3}
	rows3 := []*structs.Node{&node1, &node1, &node4}
	rows4 := []*structs.Node{&node5}
	//t.Log(timestamp2)
	cases := []struct {
		propertiesList []*structs.Property
		rows           []*structs.Node
		message        string
	}{
		{nil, rows1, "row [1] error: values size larger than properties size."},
		{nil, rows2, "row [2] error: values size smaller than properties size."},
		{nil, rows3, "row [2] error: values size smaller than properties size."},
		{nil, rows4, "row [0] error: values doesn't contain property [typeNotMatch]."},
	}
	for _, c := range cases {
		_, err1 := client.InsertNodesBatchAuto(c.rows, &configuration.InsertRequestConfig{
			InsertType: ultipa.InsertType_NORMAL})
		//fmt.Println(c.rows)
		//fmt.Println(re)
		if err1.Error() != c.message {
			t.Errorf("Returned message does not match the expected message. Expected: %s\nActual: %s", c.message, err1.Error())
		}
	}
}

func TestBatchInsert2(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	node1 := structs.Node{
		Values: &structs.Values{
			Data: map[string]interface {
			}{
				"typeTimestamp": "2038-01-19 03:14:07", "typeString": "haha", "typeDatetime": "2038-01-19 03:14:07"}}, Schema: "insertNode2"}
	rows1 := []*structs.Node{&node1}

	schema := structs.NewSchema("insertNode2")
	schema.Properties = append(schema.Properties, &structs.Property{
		Name: "typeTimestamp",
		Type: ultipa.PropertyType_TIMESTAMP,
	}, &structs.Property{
		Name: "typeString",
		Type: ultipa.PropertyType_STRING,
	}, &structs.Property{
		Name: "typeDatetime",
		Type: ultipa.PropertyType_DATETIME,
	})

	_, err := client.CreateSchema(schema, true, nil)
	if err != nil {
		t.Fatalf("CreateSchema error ,%v", err)
	}
	defer func() {
		client.DropSchema(schema, nil)
	}()

	cases := []struct {
		propertiesList []*structs.Property
		rows           []*structs.Node
		message        string
	}{
		{nil, rows1, "node row [1] error: values size larger than properties size."},
	}
	for _, c := range cases {
		insertRequestConfig := &configuration.InsertRequestConfig{
			InsertType: ultipa.InsertType_NORMAL,
		}

		requestConfig := &configuration.RequestConfig{
			TimezoneOffset: 3600,
		}
		insertRequestConfig.RequestConfig = requestConfig

		client.InsertNodesBatchBySchema(schema, c.rows, insertRequestConfig)
	}

}
