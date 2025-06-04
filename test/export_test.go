package test

import (
	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"testing"

	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func TestExportAsNodesEdges(t *testing.T) {
	//schema := &structs.Schema{
	//	Name: schemaName,
	//	Properties: []*structs.Property{
	//		{Name: "_id"}, {Name: "_uuid"}, {Name: "link"}, {Name: "type"}, {Name: "decisionDate"}, {Name: "officialDate"}, {Name: "year"}, {Name: "version"}, {Name: "decisionNumber"},
	//		{Name: "ssz"}, {Name: "szDec"}, {Name: "annullamentoConRinvio"}, {Name: "annullamentoSenzaRinvio"}, {Name: "rigettato"}, {Name: "inammissibile"}, {Name: "fullDocument"},
	//	}}

	schemaName := "nodeSchema"
	//client, _ := GetClient(hosts, graph)

	schema := &structs.Schema{
		Name: schemaName,
		Properties: []*structs.Property{
			{Name: "_id"}, {Name: "_uuid"}, {Name: "typeInt32"}, {Name: "typeFloat"}, {Name: "typeDouble"}, {Name: "typeInt64"}, {Name: "typeUint32"}, {Name: "typeUint64"}, {Name: "typeDatetime"},
			{Name: "typeString"}, {Name: "typeTimestamp"}, {Name: "typeNotMatch"}, {Name: "typeText"},
		}}

	properties := []string{}

	for _, prop := range schema.Properties {
		properties = append(properties, prop.Name)
	}
	exportRequest := &ultipa.ExportRequest{
		DbType:           ultipa.DBType_DBNODE,
		Limit:            10000,
		SelectProperties: properties,
		Schema:           schema.Name,
	}
	err := client.Export(exportRequest,
		&configuration.RequestConfig{},
		func(nodes []*structs.Node, edges []*structs.Edge) error {
			//printers.PrintNodes(nodes, map[string]*structs.Schema{schemaName: schema})
			t.Log(len(nodes))
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
}
