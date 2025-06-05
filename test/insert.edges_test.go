package test

import (
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"testing"

	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

func TestInsertEdge(t *testing.T) {
	//t.Run("insert node", TestInsertNodes)

	//client.SetCurrentGraph("go_sdk_test")
	schemaName := "default"
	prop := &structs.Property{
		Schema: schemaName,
		Name:   "value",
		Type:   ultipa.PropertyType_DOUBLE,
	}
	client.CreateEdgeProperty(prop, nil)

	var edges []*structs.Edge
	edge1 := structs.NewEdge()
	//edge1.UUID = 1
	edge1.From = "1"
	edge1.To = "2"
	edge1.Set("value", 1.0)

	edge2 := structs.NewEdge()
	//edge2.UUID = 2
	edge2.From = "11"
	edge2.To = "12"
	edge2.Set("value", "1212")

	edge3 := structs.NewEdge()
	edge3.UUID = 3
	edge3.FromUUID = 1
	edge3.ToUUID = 2

	edges = append(edges, edge1, edge2)

	uql := structs.EdgesToInsertUql(edges)
	t.Log(uql)

	config := &configuration.InsertRequestConfig{
		Silent: true,
	}

	response, err := client.InsertEdges(schemaName, edges, config)
	if err != nil {
		t.Error(err)
	}

	// overwrite
	delete(edge1.Values.Data, "value")

	config = &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_OVERWRITE,
		Silent:     true,
	}

	response, err = client.InsertEdges(schemaName, edges, config)
	if err != nil {
		t.Error(err)
	}

	t.Log(response)

	// upsert
	edge2.Set("value", 12.12)

	config = &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_UPSERT,
		Silent:     true,
	}

	response, err = client.InsertEdges(schemaName, edges, config)
	if err != nil {
		t.Error(err)
	}

	t.Log(response)
}
