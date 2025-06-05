package test

import (
	"log"
	"testing"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/models"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

func TestCreateModel(t *testing.T) {
	log.Println("[Test] Creating Graph Model")
	model := models.NewGraphModel(&structs.GraphSet{
		Name: "graph_by_model",
	})

	// create user schema
	model.AddSchema(&structs.Schema{
		Name:   "PrivilegeToUser",
		DBType: ultipa.DBType_DBNODE,
		Properties: []*structs.Property{
			{
				Name:        "username",
				Description: "user's name",
				Type:        ultipa.PropertyType_STRING,
			},
			{
				Name:        "password",
				Description: "user's password",
				Type:        ultipa.PropertyType_STRING,
			},
		},
	})

	// create indicator schema
	model.AddSchema(&structs.Schema{
		Name:   "Indicator",
		DBType: ultipa.DBType_DBNODE,
		Properties: []*structs.Property{
			{
				Name:        "name",
				Description: "indicator name",
				Type:        ultipa.PropertyType_STRING,
			},
			{
				Name:        "values",
				Description: "json content to save indicator values",
				Type:        ultipa.PropertyType_STRING,
			},
		},
	})

	model.AddSchema(&structs.Schema{
		Name:   "Privilege",
		DBType: ultipa.DBType_DBEDGE,
		Properties: []*structs.Property{
			{
				Name:        "type",
				Description: "type of privilege: r,w, rw",
				Type:        ultipa.PropertyType_STRING,
			},
		},
	})

	log.Println("[TEST] Initial Model")
	err := client.InitModel(model, nil)

	if err != nil {
		t.Fatalf("Test Error %v \n", err)
	}
}
