package api

import (
	"errors"
	"fmt"
	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
	"strings"
)

func (api *UltipaAPI) ShowGraph(config *configuration.RequestConfig) (graphSets []*structs.GraphSet, err error) {
	res, err := api.Uql("show().graph().more()", config)
	if err != nil {
		return nil, err
	}

	if !res.IsSuccess() {
		return nil, fmt.Errorf(res.Status.Message)
	}

	graphSets, err = res.Alias(http.RESP_GRAPH_KEY).AsGraphSets()
	return graphSets, err
}

func (api *UltipaAPI) CreateGraphIfNotExist(graphSet *structs.GraphSet, config *configuration.RequestConfig) (rwc http.ResponseWithExistCheck, err error) {
	rwc.Exist, err = api.HasGraph(graphSet.Name, config)

	if rwc.Exist {
		return rwc, err
	}

	rwc.Response, err = api.CreateGraph(graphSet, config)
	return rwc, err
}

// CreateGraph 5.0 partitionByHash:Crc32/CityHash64
func (api *UltipaAPI) CreateGraph(graphSet *structs.GraphSet, config *configuration.RequestConfig) (*http.Response, error) {
	if graphSet == nil {
		return nil, errors.New("graphSet cannot be nil")
	}
	//if graphSet.Name == "" || graphSet.PartitionBy == "" || len(graphSet.Shards) == 0 {
	//	return nil, errors.New("graphSet Name/Shards/PartitionBy cannot be empty")
	//}

	if graphSet.Name == "" {
		return nil, errors.New("graphSet name is required")
	}

	uql := fmt.Sprintf(`create().graph("%v", "%v")`, graphSet.Name, graphSet.Description)
	if graphSet.Description == "" {
		uql = fmt.Sprintf(`create().graph("%v")`, graphSet.Name)
	}

	if len(graphSet.Shards) != 0 {
		uql = fmt.Sprintf("%s.shards([%s])", uql, strings.Join(graphSet.Shards, ","))
	}
	if graphSet.PartitionBy != "" {
		uql = fmt.Sprintf("%s.partitionByHash(%s,_id)", uql, graphSet.PartitionBy)
	}

	resp, err := api.Uql(uql, config)

	if err != nil {
		//api.Logger.Log("create graphSet failed : " + graphSet.Name + " " + err.Error())
		return nil, err
	}

	//if !resp.IsSuccess() {
	//	return nil, fmt.Errorf(resp.Status.Message)
	//}

	return resp, err
}

func (api *UltipaAPI) DropGraph(graphName string, config *configuration.RequestConfig) (*http.Response, error) {
	resp, err := api.Uql(fmt.Sprintf(`drop().graph("%v")`, graphName), config)

	if err != nil {
		return nil, err
	}

	//if !resp.IsSuccess() {
	//	return nil, fmt.Errorf(resp.Status.Message)
	//}

	return resp, err
}

func (api *UltipaAPI) HasGraph(graphName string, config *configuration.RequestConfig) (bool, error) {
	graphSets, err := api.ShowGraph(config)

	if err != nil {
		return false, err
	}

	for _, graph := range graphSets {
		if graph.Name == graphName {
			return true, nil
		}
	}

	return false, nil
}

func (api *UltipaAPI) GetGraph(graphName string, config *configuration.RequestConfig) (*structs.GraphSet, error) {
	grapSets, err := api.ShowGraph(config)

	if err != nil {
		return nil, err
	}

	for _, graph := range grapSets {
		if graph.Name == graphName {
			return graph, nil
		}
	}

	return nil, errors.New("graph not found")
}

func (api *UltipaAPI) AlterGraph(graphName string, alterGraphSet *structs.GraphSet, config *configuration.RequestConfig) (*http.Response, error) {
	if graphName == "" {
		return nil, errors.New("graphName is required")
	}
	if alterGraphSet == nil {
		return nil, errors.New("alterGraphSet cannot be nil")
	}

	if alterGraphSet.Name == "" && alterGraphSet.Description == "" {
		return nil, errors.New("alterGraphSet name/description cannot be empty at the same time")
	}

	uql := fmt.Sprintf(`alter().graph("%s").set({name: "%s", description: "%s"})`, graphName, alterGraphSet.Name, alterGraphSet.Description)

	if alterGraphSet.Description == "" {
		uql = fmt.Sprintf(`alter().graph("%s").set({name: "%s"})`, graphName, alterGraphSet.Name)
	} else if alterGraphSet.Name == "" {
		uql = fmt.Sprintf(`alter().graph("%s").set({description: "%s"})`, graphName, alterGraphSet.Description)
	}

	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	//if !resp.IsSuccess() {
	//	return nil, fmt.Errorf(resp.Status.Message)
	//}

	return resp, nil
}

func (api *UltipaAPI) Truncate(params *structs.TruncateParams, config *configuration.RequestConfig) (*http.Response, error) {
	uql := ""
	if params.DBType == nil {
		t := ultipa.DBType_DBGLOBAL
		params.DBType = &t
	}

	schemaName, err := CheckReplaceSchemaPropertyName(params.SchemaName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, graphName = %s", err.Error(), params.GraphName))
	}

	if params.SchemaName != "" {
		if !(*params.DBType == ultipa.DBType_DBNODE || *params.DBType == ultipa.DBType_DBEDGE) {
			//return nil, fmt.Errorf("to truncate schema, dbType must be DBType_DBNODE or DBType_DBEDGE")
			return nil, fmt.Errorf("to truncate schema, DBType is required in the parameters")
		}

		if params.SchemaName == "*" {
			params.SchemaName = `"*"`
		} else {
			params.SchemaName = "@" + schemaName
		}

	} else {
		if !(*params.DBType == ultipa.DBType_DBGLOBAL) {
			return nil, fmt.Errorf("to truncate graph, dbType must be DBType_DBGLOBAL or nil")
		}
	}

	switch *params.DBType {
	case ultipa.DBType_DBNODE:
		uql = fmt.Sprintf(`truncate().graph("%v").nodes(%v)`, params.GraphName, params.SchemaName)
	case ultipa.DBType_DBEDGE:
		uql = fmt.Sprintf(`truncate().graph("%v").edges(%v)`, params.GraphName, params.SchemaName)
	default:
		uql = fmt.Sprintf(`truncate().graph("%v")`, params.GraphName)
	}

	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	//if !resp.IsSuccess() {
	//	return nil, fmt.Errorf(resp.Status.Message)
	//}

	return resp, nil
}

func (api *UltipaAPI) Compact(graphName string, config *configuration.RequestConfig) (*http.JobResponse, error) {
	uql := fmt.Sprintf(`compact().graph("%v")`, graphName)

	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return http.GetJobResponseFromUqlResponse(resp)
}

//func (api *UltipaAPI) RebalanceGraph(graph *structs.GraphSet, config *configuration.RequestConfig) (*http.Response, error) {
//    if graph == nil {
//        return nil, errors.New("graph cannot be nil")
//    }
//    if graph.Name == "" || graph.PartitionBy == "" || len(graph.Shards) == 0 {
//        return nil, errors.New("graph Name/Shards/PartitionBy cannot be empty")
//    }
//    return api.Uql(fmt.Sprintf(`alter().graph("%v", "%v").shards([%v]).partitionByHash('%v',_id)`, graph.Name, graph.Description, strings.Join(graph.Shards, ","), graph.PartitionBy),config)
//}

//func (api *UltipaAPI) MountGraph(graphName string, config *configuration.RequestConfig) (*http.Response, error) {
//	uql := fmt.Sprintf(`mount().graph("%v")`, graphName)
//
//	resp, err := api.Uql(uql, config)
//
//	if err != nil {
//		return nil, err
//	}
//
//	return resp, nil
//}
//
//func (api *UltipaAPI) UnmountGraph(graphName string, config *configuration.RequestConfig) (*http.Response, error) {
//	uql := fmt.Sprintf(`unmount().graph("%v")`, graphName)
//
//	resp, err := api.Uql(uql, config)
//
//	if err != nil {
//		return nil, err
//	}
//
//	return resp, nil
//}
