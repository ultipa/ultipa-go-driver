package test

import (
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	http2 "github.com/ultipa/ultipa-go-driver/sdk/http"
	"log"
	"testing"

	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk/printers"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
	"github.com/ultipa/ultipa-go-driver/utils"
)

func TestShowSchemas(t *testing.T) {
	res, err := client.ShowSchema(nil)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf(utils.JSONString(res))
}

func TestListSchema(t *testing.T) {
	InitCases()
	res, err := client.ShowNodeSchema(nil)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf(utils.JSONString(res))
}

func TestCompareSchema(t *testing.T) {

	s1 := &structs.Schema{
		Name: "s1",
		Properties: []*structs.Property{
			{
				Name: "prop1",
				Type: ultipa.PropertyType_INT32,
			},
			{
				Name: "prop2",
				Type: ultipa.PropertyType_UINT64,
			},
		},
	}

	s2 := &structs.Schema{
		Name: "s1",
		Properties: []*structs.Property{
			{
				Name: "prop1",
				Type: ultipa.PropertyType_INT32,
			},
			{
				Name: "prop2",
				Type: ultipa.PropertyType_UINT64,
			},
		},
	}

	s3 := &structs.Schema{
		Name: "s1",
		Properties: []*structs.Property{
			{
				Name: "prop1",
				Type: ultipa.PropertyType_INT32,
			},
		},
	}

	s4 := &structs.Schema{
		Name: "s1",
		Properties: []*structs.Property{
			{
				Name: "prop1",
				Type: ultipa.PropertyType_UINT64,
			},
			{
				Name: "prop2",
				Type: ultipa.PropertyType_UINT64,
			},
		},
	}

	schemaParis := []struct {
		First  *structs.Schema
		Second *structs.Schema
		Fit    bool
		Expect bool
	}{
		{s1, s2, false, true},
		{s1, s3, false, false},
		{s1, s3, true, true},
		{s1, s4, false, false},
		{s1, s4, true, false},
		{s1, nil, false, false},
		{s1, nil, true, true},
		{nil, s1, false, false},
		{nil, s1, true, false},
	}
	for index, pair := range schemaParis {
		err, _ := structs.CompareSchemas(pair.First, pair.Second, false)

		if err != nil {
			t.Fatal("Test Compare schema failed ：", index, err)
		}
	}

}

func TestShowSchema(t *testing.T) {
	//client.SetCurrentGraph("call")
	response, _ := client.Uql("show().node_schema()", nil)

	printers.PrintAny(response.Alias(http2.RESP_NODE_SCHEMA_KEY))

	nodeSchemas, err := client.ShowNodeSchema(nil)
	if err != nil {
		t.Fatal(err)
	}
	edgeSchemas, err := client.ShowEdgeSchema(nil)
	if err != nil {
		t.Fatal(err)
	}

	printers.PrintSchema(nodeSchemas)
	printers.PrintSchema(edgeSchemas)
}

func TestCreateSchemaWithProperties(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	// create schema with properties
	newSchemaWithProperties := &structs.Schema{
		Name:        "_abc _acd",
		Description: "A Schema with 2 properties",
		Properties: []*structs.Property{
			{
				Name: "username用户@",
				Type: ultipa.PropertyType_STRING,
			},
			{
				Name: "123passwor@d",
				Type: ultipa.PropertyType_TEXT,
			},
		},
	}

	_, err := client.CreateSchema(newSchemaWithProperties, true, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateSchema(t *testing.T) {
	// create schema with properties
	schema, err := client.GetSchema("account", ultipa.DBType_DBNODE, nil)
	if err != nil {
		t.Log(err)
	}

	resp2, _ := client.CreateSchema(schema, true, &configuration.RequestConfig{Graph: "g1"})
	log.Println(resp2.Status)
}

func TestAlterSchema(t *testing.T) {
	// create schema with properties
	//client.SetCurrentGraph("test_go_sdk")
	schema := &structs.Schema{
		DBType: ultipa.DBType_DBNODE,
		Name:   `""`,
		//Description:   "People",
	}
	newSchema := &structs.Schema{
		Name:        `"""`,
		Description: "People2",
	}

	resp2, err := client.AlterSchema(schema, newSchema, nil)
	log.Println(resp2)
	if err != nil {
		t.Fatal(err)
	}
}
