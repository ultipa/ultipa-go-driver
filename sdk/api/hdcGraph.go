package api

import (
	"encoding/json"
	"fmt"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
	"strings"
)

type HDCSyncType string
type HDCDirection string

type HDCBuilder struct {
	SyncType      HDCSyncType         `json:"update,omitempty"`
	HDCGraphName  string              `json:"-"`
	HDCServerName string              `json:"-"`
	NodeSchema    map[string][]string `json:"nodes,omitempty"`
	EdgeSchema    map[string][]string `json:"edges,omitempty"`
	Direction     HDCDirection        `json:"direction,omitempty"`
	LoadId        bool                `json:"load_id"`
	IsDefault     bool                `json:"default"`
}

const (
	STATIC     HDCSyncType  = "static"
	ASYNC      HDCSyncType  = "async"
	SYNC       HDCSyncType  = "sync"
	IN         HDCDirection = "in"
	OUT        HDCDirection = "out"
	UNDIRECTED HDCDirection = "undirected"
)

func (b *HDCBuilder) BuildUQL() (string, error) {
	if b.HDCGraphName == "" {
		return "", fmt.Errorf("HDCGraphName is required")
	}

	if b.HDCServerName == "" {
		return "", fmt.Errorf("HDCServerName is required")
	}

	type uqlBody struct {
		SyncType   HDCSyncType         `json:"update,omitempty"`
		NodeSchema map[string][]string `json:"nodes,omitempty"`
		EdgeSchema map[string][]string `json:"edges,omitempty"`
		Direction  HDCDirection        `json:"direction,omitempty"`
		LoadId     bool                `json:"load_id"`
		IsDefault  bool                `json:"default"`
		Query      string              `json:"query"`
		//Type       string              `json:"type"`
	}

	body := uqlBody{
		SyncType:   b.SyncType,
		NodeSchema: b.NodeSchema,
		EdgeSchema: b.EdgeSchema,
		Direction:  b.Direction,
		LoadId:     b.LoadId,
		IsDefault:  b.IsDefault,
		Query:      "query", // default
		//Type:       "Graph", // default
	}

	bodyJson, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	bodyStr := strings.Trim(string(bodyJson), "{}")

	uql := fmt.Sprintf(`hdc.graph.create("%s", {%s}).to("%s")`, b.HDCGraphName, bodyStr, b.HDCServerName)

	return uql, nil
}

func (api *UltipaAPI) CreateHDCGraphBySchema(builder HDCBuilder, config *configuration.RequestConfig) (*http.JobResponse, error) {
	uql, err := builder.BuildUQL()
	if err != nil {
		return nil, err
	}

	resp, err := api.Uql(uql, config)

	if err != nil {
		//api.Logger.Log("create hdc graph failed : " + graphName + " " + err.Error())
		return nil, err
	}

	//api.Logger.Log("Creating Graph Request OK! - " + graphName)

	return http.GetJobResponseFromUqlResponse(resp)
}

//func FormatHdcGraphSchemas(schemas []*structs.Schema) string {
//    if len(schemas) == 0 {
//        return `"*": ["*"]`
//    }
//
//    var result []string
//
//    for _, schema := range schemas {
//        if schema.Name == "" {
//            return ""
//        }
//
//        if len(schema.Properties) == 0 {
//            // if Properties is null
//            result = append(result, fmt.Sprintf(`%s: ["*"]`, schema.Name))
//        } else {
//            // join Property.Name
//            var propertyNames []string
//            for _, prop := range schema.Properties {
//                propertyNames = append(propertyNames, prop.Name)
//            }
//            result = append(result, fmt.Sprintf(`%s: ["%s"]`, schema.Name, strings.Join(propertyNames, `", "`)))
//        }
//    }
//
//    return strings.Join(result, ", ")
//
//}

func (api *UltipaAPI) ShowHDCGraph(config *configuration.RequestConfig) ([]*structs.HDCGraph, error) {
	resp, err := api.Uql("hdc.graph.show()", config)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	projections, err := resp.Alias(http.RESP_HDCGRAPH_KEY).AsHDCGraphs()
	if err != nil {
		return nil, err
	}

	return projections, nil
}

func (api *UltipaAPI) DropHDCGraph(hdcGraphName string, config *configuration.RequestConfig) (*http.Response, error) {
	uql := fmt.Sprintf(`hdc.graph.drop('%s')`, hdcGraphName)

	return api.Uql(uql, config)
}

func (api *UltipaAPI) ShowHDCAlgo(hdcServerName string, config *configuration.RequestConfig) ([]*structs.Algo, error) {
	if hdcServerName == "" {
		return nil, fmt.Errorf("hdcServerName is required")
	}

	uql := fmt.Sprintf(`hdc.server.show('%s')`, hdcServerName)
	resp, err := api.Uql(uql, config)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	algo, err := resp.Alias(http.RESP_ALGOS_KEY).AsAlgos()
	if err != nil {
		return nil, err
	}
	return algo, nil
}
