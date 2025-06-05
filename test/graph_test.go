package test

import (
	"github.com/ultipa/ultipa-go-driver/v5/sdk/printers"
	"log"
	"testing"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

func TestShowGraph(t *testing.T) {
	//InitCases()
	//client, _ := GetClient(hosts, graph)
	graphs, err := client.ShowGraph(nil)
	if err != nil {
		t.Fatal(err)
	}
	//
	if len(graphs) == 0 {
		t.Fatal("show().graph() no data return")
	}
	//
	//
	printers.PrintGraphSet(graphs)

	//log.Printf(utils.JSONString(res))
	//printers.PrintGraphSet(graphs)
	//res, _ := client.Uql("show().hdc(\"hdc-server-1\")", nil)
	res, _ := client.Uql("show().hdc()", nil)
	table, _ := res.Alias(http.RESP_GRAPH_KEY).AsTable()

	printers.PrintTable(table)

}

func TestCreateGraph(t *testing.T) {
	//client, err := GetClient(hosts, graph)
	graphName := "miniCircle"
	exit, err := client.HasGraph(graphName, nil)
	if err != nil {
		return
	}

	if exit {
		_, err := client.DropGraph(graphName, nil)
		if err != nil {
			t.Error(err)
		}
	}

	graphSet := &structs.GraphSet{
		Name:        graphName,
		Shards:      []string{"2,3"},
		PartitionBy: "Crc32",
	}
	_, err = client.CreateGraph(graphSet, nil)
	if err != nil {
		t.Fatal(err)
	}
	//err = client.SetCurrentGraph(graphName)
	if err != nil {
		t.Error(err)
	}

	_, err = client.Uql("insert().into(@default).nodes({_id:\"1\"})", nil)
	if err != nil {
		t.Error(err)
	}

	rwc, err := client.CreateGraphIfNotExist(graphSet, nil)
	if rwc.Exist {
		t.Logf("graph %s exist", graphSet.Name)
	}
	if err != nil {
		t.Fatal(err)
	}

	//client.DropGraph(graphName, nil)

}

func TestDropGraph(t *testing.T) {
	response, err := client.DropGraph("go_test_sdk", nil)
	if err != nil {
		t.Fatal("drop graph error: ", err)
	}
	t.Log(response.Status.Code.String())
}

func TestAsGraph(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	resp, _ := client.Uql("show().graph()", nil)
	graphs, err := resp.Alias(http.RESP_GRAPH_KEY).AsGraphSets()

	if err != nil {
		t.Fatal(err)
	}

	if len(graphs) == 0 {
		t.Fatal("show().graph() no data return")
	}

	//printers.PrintGraphSet(graphs)
}

func TestCreateGraphIfNotExist(t *testing.T) {

	//client, err := GetClient(hosts, graph)

	//if err != nil {
	//	t.Fatalf("failed to connect to server %v", err)
	//}

	_, err := client.CreateGraphIfNotExist(&structs.GraphSet{
		Name:        "test1123231",
		Shards:      []string{"1,2"},
		PartitionBy: "Crc32",
	}, nil)
	if err != nil {
		t.Fatalf("failed to create graph %v", err)
	}
	client.DropGraph("test1123231", nil)

}

func TestUltipaAPI_AlterGraph(t *testing.T) {
	type args struct {
		oldGraphName string
		newGraphName string
		description  string
		config       *configuration.RequestConfig
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Old graph name is empty",
			args: args{
				oldGraphName: "",
				newGraphName: "validNewGraphName",
				description:  "Valid description",
				config:       nil,
			},
			wantErr: true,
		},
		{
			name: "Old graph name contains illegal characters",
			args: args{
				oldGraphName: "1",
				newGraphName: "validNewGraphName",
				description:  "Valid description",
				config:       nil,
			},
			wantErr: true,
		},
		{
			name: "New graph name is empty",
			args: args{
				oldGraphName: "amz",
				newGraphName: "",
				description:  "new description",
				config:       nil,
			},
			wantErr: false,
		},
		{
			name: "Description is empty",
			args: args{
				oldGraphName: "amz",
				newGraphName: "amz1",
				description:  "",
				config:       nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//oldGraph := &structs.GraphSet{Name: tt.args.oldGraphName}
			newGraph := &structs.GraphSet{
				Name:        tt.args.newGraphName,
				Description: tt.args.description,
			}
			rsp, err := client.AlterGraph(tt.args.oldGraphName, newGraph, tt.args.config)
			_ = rsp
			if (tt.wantErr && err == nil) || (!tt.wantErr && err != nil) {
				t.Errorf("AlterGraph() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tr := &structs.TruncateParams{
		GraphName: "testaaa",
		DBType:    nil,
		//SchemaName: "*",
	}
	response, err := client.Truncate(tr, nil)
	if err != nil {
		log.Println(err)
	}
	log.Println(response)

	db := ultipa.DBType_DBNODE
	tr.DBType = &db
	response, err = client.Truncate(tr, nil)
	if err != nil {
		log.Println(err)
	}
	log.Println(response)

	db = ultipa.DBType_DBEDGE
	tr.DBType = &db
	tr.SchemaName = "中文"
	response, err = client.Truncate(tr, nil)
	if err != nil {
		log.Println(err)
	}
	log.Println(response)

	tr.DBType = nil
	tr.SchemaName = ""
	response, err = client.Truncate(tr, nil)
	if err != nil {
		log.Println(err)
	}
	log.Println(response)

}

func TestGetGraph(t *testing.T) {
	g, err := client.GetGraph("miniCircle", nil)
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintGraphSet(append([]*structs.GraphSet{}, g))

}

func TestCompact(t *testing.T) {
	job, err := client.Compact("go_sdk_test11", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(job)
}

func TestCreateGraphIfNotExist1(t *testing.T) {

	testData := []struct {
		graph  *structs.GraphSet
		result bool
		code   ultipa.ErrorCode
		errMsg string
	}{
		{graph: &structs.GraphSet{Name: "testCreateGraph1", Shards: []string{"1", "2", "3"}, PartitionBy: "CityHash64"}, result: true, errMsg: ""},
		{graph: &structs.GraphSet{Name: "testGraphIFNot", Shards: []string{"1", "2", "3"}, PartitionBy: "CityHash64"}, result: false, code: ultipa.ErrorCode_SUCCESS, errMsg: ""},
		{graph: &structs.GraphSet{Name: ""}, result: false, code: ultipa.ErrorCode_SUCCESS, errMsg: "graphSet name is required"},
	}
	for _, data := range testData {
		res, err := client.CreateGraphIfNotExist(data.graph, nil)
		t.Log(res, err)
	}
}
