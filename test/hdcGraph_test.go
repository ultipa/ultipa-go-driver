package test

import (
	"github.com/ultipa/ultipa-go-driver/v5/sdk/api"
	"testing"
)

func TestShowHDCGraph(t *testing.T) {
	hdcGraph, err := client.ShowHDCGraph(nil)
	//hdcGraph, err := client.ShowProjection(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(hdcGraph)
}

func TestHDCGraph(t *testing.T) {
	builder := api.HDCBuilder{
		SyncType:      "",
		HDCGraphName:  "miniCircle_hdc",
		HDCServerName: "hdc-server-1",
		NodeSchema:    map[string][]string{"*": {"*"}},
		EdgeSchema:    map[string][]string{"agree": {"datetime", "timestampList"}},
		Direction:     "",
		LoadId:        false,
		IsDefault:     false,
	}

	t.Log(builder.BuildUQL())

	response, err := client.CreateHDCGraphBySchema(builder, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(response.Status.Code.String())

}

//func TestFormatSchemas(t *testing.T) {
//    tests := []struct {
//        name     string
//        schemas  []*structs.Schema
//        expected string
//    }{
//        {
//            name:     "EmptySchemas",
//            schemas:  []*structs.Schema{},
//            expected: `"*": ["*"]`,
//        },
//        {
//            name: "SingleSchemaWithProperties",
//            schemas: []*structs.Schema{
//                {
//                    Name: "User",
//                    Properties: []*structs.Property{
//                        {Name: "username"},
//                    },
//                },
//            },
//            expected: `User: ["username"]`,
//        },
//        {
//            name: "MultipleSchemasWithProperties",
//            schemas: []*structs.Schema{
//                {
//                    Name: "User",
//                    Properties: []*structs.Property{
//                        {Name: "username"},
//                        {Name: "email"},
//                    },
//                },
//                {
//                    Name: "Article",
//                    Properties: []*structs.Property{
//                        {Name: "title"},
//                    },
//                },
//            },
//            expected: `User: ["username", "email"], Article: ["title"]`,
//        },
//        {
//            name: "SchemaWithNoProperties",
//            schemas: []*structs.Schema{
//                {
//                    Name:       "EmptySchema",
//                    Properties: []*structs.Property{},
//                },
//            },
//            expected: `EmptySchema: ["*"]`,
//        },
//        {
//            name: "MixedSchemasWithAndWithoutProperties",
//            schemas: []*structs.Schema{
//                {
//                    Name: "User",
//                    Properties: []*structs.Property{
//                        {Name: "username"},
//                    },
//                },
//                {
//                    Name:       "Article",
//                    Properties: []*structs.Property{},
//                },
//            },
//            expected: `User: ["username"], Article: ["*"]`,
//        },
//    }
//
//    for _, tt := range tests {
//        t.Run(tt.name, func(t *testing.T) {
//            result := api.FormatHdcGraphSchemas(tt.schemas)
//            t.Log(result)
//            if result != tt.expected {
//                t.Errorf("formatSchemas() = %v, want %v", result, tt.expected)
//            }
//        })
//    }
//}
