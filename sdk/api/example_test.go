package api_test

import (
	"log"

	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk"
	"github.com/ultipa/ultipa-go-driver/sdk/api"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/printers"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

var client *api.UltipaAPI

func ExampleNewUltipaAPI() {

	config := &configuration.UltipaConfig{
		Hosts:    []string{"10.0.0.1:60061", "10.0.0.2:60061", "10.0.0.3:60061"},
		Username: "root",
		Password: "root",
	}
	client, err := sdk.NewUltipaDriver(config)

	if err != nil {
		log.Fatalln(err)
	}

	graph, _ := client.ShowGraph(nil)

	log.Println(graph)
}

func ExampleUltipaAPI_UQL_Nodes_Edges() {

	rConfig := &configuration.RequestConfig{
		Timeout: 20,
	}
	resp, err := client.Uql("find().nodes() return nodes limit 1", rConfig)

	nodes, schemas, err := resp.Alias("nodes").AsNodes()

	log.Println(nodes, schemas, err)

	respEdges, err := client.Uql("find().edges() return edges limit 1", nil)

	edges, edgeSchemas, err := respEdges.Alias("edges").AsEdges()

	log.Println(edges, edgeSchemas, err)
}

func ExampleUltipaAPI_CreateGraph() {

	graph := &structs.GraphSet{
		Name:        "new_graph",
		Description: "my new graph",
	}
	resp, err := client.CreateGraph(graph, nil)

	log.Println(resp.Status.Code, err)
}

func ExampleUltipaAPI_DropGraph() {

	resp, err := client.DropGraph("old_graph", nil)
	log.Println(resp.Status.Code, err)
}

func ExampleUltipaAPI_HasGraph() {

	exist, err := client.HasGraph("exist_graph", nil)
	log.Println(exist, err)
}

func ExampleUltipaAPI_ListSchema() {
	nodeSchemas, _ := client.ShowSchema(nil)
	log.Println(nodeSchemas)

	// or

}

func ExampleUltipaAPI_GetSchema() {
	//get node schema
	nodeSchema, _ := client.GetSchema("my_node_schema", ultipa.DBType_DBNODE, nil)
	log.Println(nodeSchema)

	//get edge schema
	edgeSchema, _ := client.GetSchema("my_edge_schema", ultipa.DBType_DBEDGE, nil)
	log.Println(edgeSchema)
}

func ExampleUltipaAPI_CreateSchema() {

	// create an empty schema
	newSchema := &structs.Schema{
		Name:   "new_node_schema",
		DBType: ultipa.DBType_DBNODE,
	}
	resp, _ := client.CreateSchema(newSchema, false, nil)

	log.Println(resp.Status.Code)

	// create schema with properties
	newSchemaWithProperties := &structs.Schema{
		Name:        "my_node_schema_prop",
		Description: "A Schema with 2 properties",
		Properties: []*structs.Property{
			{
				Name: "username",
				Type: ultipa.PropertyType_STRING,
			},
			{
				Name: "password",
				Type: ultipa.PropertyType_STRING,
			},
		},
	}

	resp2, _ := client.CreateSchema(newSchemaWithProperties, true, nil)
	log.Println(resp2)
}

func ExampleUltipaAPI_CreateSchemaIfNotExist() {

	schema := structs.Schema{
		Name: "new_schema",
	}

	resp, _ := client.CreateSchemaIfNotExist(&schema, false, nil)
	log.Println(resp)
}

func ExampleUltipaAPI_CreateNodeProperty() {

	// CreateTime Node Property
	newProp := &structs.Property{
		Schema: "target_schema",
		Name:   "name",
		Type:   ultipa.PropertyType_STRING,
	}

	resp, _ := client.CreateProperty(ultipa.DBType_DBNODE, newProp, nil)
	log.Println(resp.Status.Code)

	// CreateTime Edge Property
	newEdgeProp := &structs.Property{
		Schema: "target_schema",
		Name:   "relation",
		Type:   ultipa.PropertyType_STRING,
	}

	resp2, _ := client.CreateProperty(ultipa.DBType_DBEDGE, newEdgeProp, nil)
	log.Println(resp2.Status.Code)

	rwc, _ := client.CreatePropertyIfNotExist(ultipa.DBType_DBEDGE, newEdgeProp, nil)
	log.Println(rwc)
}

func ExampleUltipaAPI_GetProperty() {
	prop, _ := client.GetProperty(ultipa.DBType_DBNODE, "user", "name", nil)
	log.Println(prop)
}

func ExampleUltipaAPI_AlterNodeProperty() {
	prop := &structs.Property{
		Name:        "name",
		Description: "name Description",
		Schema:      "user",
	}
	newProp := &structs.Property{
		Name:        "newName",
		Description: "name change to newName",
	}
	resp, _ := client.AlterNodeProperty(prop, newProp, nil)
	log.Println(resp)
}

func ExampleUltipaAPI_AlterEdgeProperty() {
	prop := &structs.Property{
		Name:        "name",
		Description: "name Description",
		Schema:      "relation",
	}
	newProp := &structs.Property{
		Name:        "newName",
		Description: "name change to newName",
	}
	resp, _ := client.AlterEdgeProperty(prop, newProp, nil)
	log.Println(resp)
}

func ExampleUltipaAPI_DropNodeProperty() {
	prop := &structs.Property{
		Name:   "name",
		Schema: "user",
	}
	resp, _ := client.DropNodeProperty(prop, nil)
	log.Println(resp)
}

func ExampleUltipaAPI_DropEdgeProperty() {
	prop := &structs.Property{
		Name:   "name",
		Schema: "user",
	}
	resp, _ := client.DropNodeProperty(prop, nil)
	log.Println(resp)
}

func ExampleUltipaAPI_UQL() {

	resp, _ := client.Uql("find().nodes() as nodes return nodes limit 10", nil)
	nodes, schemas, err := resp.Alias("nodes").AsNodes()

	if err != nil {
		log.Fatalln(err)
	}

	printers.PrintNodes(nodes, schemas)
}

func ExampleUltipaAPI_UQL2() {
	resp, _ := client.Uql("find().edges() as edges return edges limit 10", nil)
	edges, schemas, err := resp.Alias("nodes").AsEdges()

	if err != nil {
		log.Fatalln(err)
	}

	printers.PrintEdges(edges, schemas)
}

func ExampleUltipaAPI_UQL3() {
	resp, _ := client.Uql("n().e()[2].n() as paths return paths{*} limit 1", nil)
	paths, err := resp.Get(0).AsPaths()

	if err != nil {
		log.Fatalln(err)
	}
	printers.PrintPaths(paths)
}

func ExampleUltipaAPI_UQL4() {
	resp, _ := client.Uql("n(as start).e()[2].n(as end) return table(start._id, end._id) as pairs limit 10", nil)
	table, err := resp.Get(0).AsTable()

	if err != nil {
		log.Fatalln(err)
	}
	printers.PrintTable(table)
}

func ExampleUltipaAPI_UQL5() {
	resp, _ := client.Uql(`n({_id == "ULTIPA"}).e().n(as friends) return collect(friends.name) as names`, nil)
	names, err := resp.Get(0).AsAttr()

	if err != nil {
		log.Fatalln(err)
	}
	printers.PrintAttr(names)
}

func ExampleUltipaAPI_UQL6() {
	resp, _ := client.Uql("show().graph()", nil)
	graphs, err := resp.Alias(http.RESP_GRAPH_KEY).AsGraphSets()

	if err != nil {
		log.Fatalln(err)
	}
	printers.PrintGraphSet(graphs)
}

func ExampleUltipaAPI_UQL7() {
	resp, _ := client.Uql("show().schemas()", nil)

	nodeSchemas, err := resp.Alias(http.RESP_NODE_SCHEMA_KEY).AsSchemas()
	if err != nil {
		log.Fatalln(err)
	}
	edgeSchemas, err := resp.Alias(http.RESP_EDGE_SCHEMA_KEY).AsSchemas()
	if err != nil {
		log.Fatalln(err)
	}

	printers.PrintSchema(nodeSchemas)
	printers.PrintSchema(edgeSchemas)
}

//func ExampleUltipaAPI_UQL8() {
//	resp, _ := client.Uql("show().algos()", nil)
//
//	algos, err := resp.Alias(http.RESP_ALGOS_KEY).AsAlgos()
//
//	if err != nil {
//		log.Fatalln(err)
//	}
//	printers.PrintAlgoList(algos)
//}

func ExampleUltipaAPI_UQL9() {

	resp, _ := client.Uql("stats()", nil)

	dataitem := resp.Alias(http.RESP_STATISTIC_KEY)

	printers.PrintAny(dataitem)
}

func ExampleUltipaAPI_InsertNodesBatchBySchema() {
	// insert 10000 nodes to a schema
	schema := &structs.Schema{
		Name: "PrivilegeToUser",
		Properties: []*structs.Property{
			{
				Name: "name",
				Type: ultipa.PropertyType_STRING,
			},
			{
				Name: "age",
				Type: ultipa.PropertyType_INT32,
			},
		},
	}

	var nodes []*structs.Node

	for i := 0; i < 10000; i++ {
		newNode := structs.NewNode()
		newNode.Set("name", "demo")
		newNode.Set("age", i)
		nodes = append(nodes, newNode)
	}

	_, err := client.InsertNodesBatchBySchema(schema, nodes, &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_OVERWRITE,
	})

	if err != nil {
		log.Fatalln(err)
	}

}
