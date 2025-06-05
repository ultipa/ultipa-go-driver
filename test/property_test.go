/**
 * @Author: zhaohaichao
 * @Description:
 * @File:  property_test
 * @Date: 2022/7/29 5:45 pm
 */

package test

import (
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"log"
	"testing"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/printers"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

func TestShowProperty(t *testing.T) {
	//pro3, err := client.GetProperty(ultipa.DBType_DBNODE, "insertNode", "typeSetString", nil)
	//if err != nil {
	//	log.Println(err)
	//}
	//printers.PrintProperty([]*structs.Property{pro3})

	nodeProp, edgeProp, err := client.ShowProperty(ultipa.DBType_DBNODE, "", nil)

	if err != nil {
		t.Fatal(err)
	}
	printers.PrintProperty(nodeProp)
	printers.PrintProperty(edgeProp)

	np, err := client.ShowNodeProperty("account", nil)
	if err != nil {
		t.Fatal(err)
	}

	npall, err := client.ShowNodeProperty("", nil)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := client.ShowEdgeProperty("rate", nil)
	if err != nil {
		t.Fatal(err)
	}

	printers.PrintProperty(np)
	printers.PrintProperty(npall)
	printers.PrintProperty(ep)
}

func TestShowNodeProperty(t *testing.T) {
	resp, err := client.Uql("show().node_property()", nil)
	if err != nil {
		t.Fatal(err)
	}

	nodeProperties, err := resp.Alias(http.RESP_NODE_PROPERTY_KEY).AsProperties()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintProperty(nodeProperties)
}

func TestShowEdgeProperty(t *testing.T) {
	resp, err := client.Uql("show().edge_property()", nil)
	if err != nil {
		t.Fatal(err)
	}

	edgeProperties, err := resp.Alias(http.RESP_EDGE_PROPERTY_KEY).AsProperties()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintProperty(edgeProperties)
}

func TestCreatePropertyWithUql(t *testing.T) {
	resp, err := client.Uql(`create().node_property(@People, "age", "int32[]")`, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err = client.Uql("show().node_property(@People)", nil)
	if err != nil {
		t.Fatal(err)
	}

	nodeProperties, err := resp.Alias(http.RESP_NODE_PROPERTY_KEY).AsProperties()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintProperty(nodeProperties)
}

func TestCreateProperty(t *testing.T) {
	// CreateTime Node Property
	newProp := &structs.Property{
		Schema: "default",
		Name:   `"""`,
		Type:   ultipa.PropertyType_BOOL,
	}

	resp, err := client.CreateProperty(ultipa.DBType_DBNODE, newProp, &configuration.RequestConfig{
		Graph: "go_sdk_test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
		t.Fatalf("resp code:%v,message:%v", resp.Status.Code, resp.Status.Message)
	}
	log.Println(resp.Status.Code)
}

func TestProperty(t *testing.T) {
	schema := &structs.Schema{
		Name:   "中文Schema",
		DBType: ultipa.DBType_DBNODE,
	}

	_, err := client.CreateSchemaIfNotExist(schema, false, nil)
	if err != nil {
		log.Println(err)
	}

	schema.DBType = ultipa.DBType_DBEDGE
	_, err = client.CreateSchemaIfNotExist(schema, false, nil)
	if err != nil {
		log.Println(err)
	}

	prop := &structs.Property{
		Schema:      schema.Name,
		Name:        "中文Property",
		Description: "中文描述",
		Type:        ultipa.PropertyType_STRING,
	}
	_, err = client.CreateProperty(ultipa.DBType_DBNODE, prop, nil)
	if err != nil {
		log.Println(err)
	}

	prop.Name = "中文Property1"
	_, err = client.CreateNodeProperty(prop, nil)
	if err != nil {
		log.Println(err)
	}

	_, err = client.CreateEdgeProperty(prop, nil)
	if err != nil {
		log.Println(err)
	}

	pro, err := client.ShowNodeProperty(schema.Name, nil)
	if err != nil {
		log.Println(err)
	}
	printers.PrintProperty(pro)

	pro1, err := client.ShowNodeProperty(schema.Name, nil)
	if err != nil {
		log.Println(err)
	}
	printers.PrintProperty(pro1)

	pro2, err := client.ShowEdgeProperty(schema.Name, nil)
	if err != nil {
		log.Println(err)
	}
	printers.PrintProperty(pro2)

	pro3, err := client.GetProperty(ultipa.DBType_DBNODE, schema.Name, prop.Name, nil)
	if err != nil {
		log.Println(err)
	}
	printers.PrintProperty([]*structs.Property{pro3})

	pro4, err := client.GetNodeProperty(schema.Name, prop.Name, nil)
	if err != nil {
		log.Println(err)
	}
	printers.PrintProperty([]*structs.Property{pro4})

	pro5, err := client.GetEdgeProperty(schema.Name, prop.Name, nil)
	if err != nil {
		log.Println(err)
	}
	printers.PrintProperty([]*structs.Property{pro5})

	prop1 := prop
	prop1.Name = "中文123"
	_, err = client.AlterProperty(ultipa.DBType_DBNODE, prop, prop1, nil)
	if err != nil {
		log.Println(err)
	}

	_, err = client.DropProperty(ultipa.DBType_DBNODE, prop1, nil)
	if err != nil {
		log.Println(err)
	}

	_, err = client.DropNodeProperty(prop1, nil)
	if err != nil {
		log.Println(err)
	}

	_, err = client.DropEdgeProperty(prop, nil)
	if err != nil {
		log.Println(err)
	}

}

func TestProperty2(t *testing.T) {
	prop := &structs.Property{
		Schema:      "default",
		Name:        "中文Property2",
		Description: "中文描述",
		Type:        ultipa.PropertyType_STRING,
	}
	_, err := client.CreatePropertyIfNotExist(ultipa.DBType_DBNODE, prop, nil)
	if err != nil {
		log.Println(err)
	}
}
