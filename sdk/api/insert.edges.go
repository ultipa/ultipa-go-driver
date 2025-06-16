package api

import (
	"context"
	"errors"
	"fmt"
	"sync"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils/logger"
)

func (api *UltipaAPI) InsertEdgesBatch(table *ultipa.EntityTable, config *configuration.InsertRequestConfig) (*http.InsertResponse, error) {
	graphName := ""
	if config.Graph != "" {
		graphName = config.Graph
	} else if api.Config.DefaultGraph != "" {
		graphName = api.Config.DefaultGraph
	}

	//config.UseMaster = true
	client, _, err := api.GetClient(config.RequestConfig)

	if err != nil {
		return nil, err
	}

	ctx, cancel, err := api.Pool.NewContext(config.RequestConfig)
	if err != nil {
		return nil, err
	}
	defer cancel()

	resp, err := client.InsertEdges(ctx, &ultipa.InsertEdgesRequest{
		GraphName: graphName,
		EdgeTable: table,
		//CreateNodeIfNotExist: config.CreateNodeIfNotExist,
		InsertType: config.InsertType,
		Silent:     config.Silent,
	})

	if err != nil {
		return nil, err
	}

	if resp.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
		return nil, errors.New(resp.Status.Msg)
	}

	return http.NewEdgesInsertResponse(resp)
}

func (api *UltipaAPI) InsertEdgesBatchBySchema(schema *structs.Schema, rows []*structs.Edge, config *configuration.InsertRequestConfig) (*http.InsertResponse, error) {
	if config == nil {
		config = &configuration.InsertRequestConfig{}
	}

	if config.RequestConfig == nil {
		config.RequestConfig = &configuration.RequestConfig{}
	}

	graphName := ""
	if config.Graph != "" {
		graphName = config.Graph
	} else if api.Config.DefaultGraph != "" {
		graphName = api.Config.DefaultGraph
	}

	//config.UseMaster = true
	client, _, err := api.GetClient(config.RequestConfig)

	if err != nil {
		return nil, err
	}

	ctx, cancel, err := api.Pool.NewContext(config.RequestConfig)
	if err != nil {
		return nil, err
	}
	defer cancel()

	table := &ultipa.EntityTable{}

	table.Schemas = []*ultipa.Schema{
		{
			SchemaName: schema.Name,
			Properties: []*ultipa.Property{},
		},
	}

	for _, prop := range schema.Properties {

		if prop.IsIDType() || prop.IsIgnore() {
			continue
		}

		table.Schemas[0].Properties = append(table.Schemas[0].Properties, &ultipa.Property{
			PropertyName: prop.Name,
			PropertyType: prop.Type,
			SubTypes:     prop.SubTypes,
		})
	}

	err, edgeRows := setPropertiesToEdgeRow(schema, rows, config.RequestConfig)

	if err != nil {
		return nil, err
	}
	table.EntityRows = edgeRows
	resp, err := client.InsertEdges(ctx, &ultipa.InsertEdgesRequest{
		GraphName:  graphName,
		EdgeTable:  table,
		InsertType: config.InsertType,
		//CreateNodeIfNotExist: config.CreateNodeIfNotExist,
		Silent: config.Silent,
	})

	if err != nil {
		return nil, err
	}

	if resp.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
		return nil, errors.New(resp.Status.Msg)
	}

	return http.NewEdgesInsertResponse(resp)
}

func setPropertiesToEdgeRow(schema *structs.Schema, rows []*structs.Edge, config *configuration.RequestConfig) (error, []*ultipa.EntityRow) {
	wg := sync.WaitGroup{}
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	edgeRows := make([]*ultipa.EntityRow, len(rows))

	for index, row := range rows {
		err = checkEdgeProperties(schema, row, index)
		if err != nil {
			return err, edgeRows
		}

		wg.Add(1)

		go func(index int, row *structs.Edge) {
			defer wg.Done()
			var newEdge *ultipa.EntityRow
			newEdge, err = doConvertSdkEdgeRowToUltipaEdgeRow(schema, row, index, config)
			if err != nil {
				cancel()
				return
			}
			edgeRows[index] = newEdge
		}(index, row)
		select {
		case <-ctx.Done():
			return err, edgeRows
		default:
		}
	}
	wg.Wait()
	return err, edgeRows
}

func checkEdgeProperties(schema *structs.Schema, row *structs.Edge, index int) error {
	if row == nil {
		return errors.New(fmt.Sprintf("node row [%d] error: node row is nil.", index))
	}
	err := CheckEdgeRows(row, schema.Properties, index)
	if err != nil {
		return err
	}
	return nil
}

func convertSdkEdgeRowToUltipaEdgeRow(schema *structs.Schema, row *structs.Edge, index int, config *configuration.RequestConfig) (*ultipa.EntityRow, error) {
	err := checkEdgeProperties(schema, row, index)
	if err != nil {
		return nil, err
	}
	return doConvertSdkEdgeRowToUltipaEdgeRow(schema, row, index, config)
}

func doConvertSdkEdgeRowToUltipaEdgeRow(schema *structs.Schema, row *structs.Edge, index int, config *configuration.RequestConfig) (*ultipa.EntityRow, error) {
	newEdge := &ultipa.EntityRow{
		FromId:     row.From,
		FromUuid:   row.FromUUID,
		ToId:       row.To,
		ToUuid:     row.ToUUID,
		SchemaName: schema.Name,
		Uuid:       row.UUID,
	}
	for _, prop := range schema.Properties {

		if prop.IsIDType() || prop.IsIgnore() {
			continue
		}

		if !row.Values.Contain(prop.Name) {
			return nil, errors.New(fmt.Sprintf("edge row [%d] error: values doesn't contain property [%s]", index, prop.Name))
		}

		bs, err := row.GetBytesSafe(prop.Name, prop.Type, prop.SubTypes, config)

		if err != nil {
			logger.PrintError("Get row bytes value failed " + prop.Name + " " + err.Error())
			err = errors.New(fmt.Sprintf("edge row [%d] error: failed to serialize value of property %s,value=%v", index, prop.Name, row.Values.Get(prop.Name)))
			return nil, err
		}

		newEdge.Values = append(newEdge.Values, bs)
	}
	return newEdge, nil
}

// InsertEdgesBatchAuto Nodes interface values should be string
func (api *UltipaAPI) InsertEdgesBatchAuto(rows []*structs.Edge, config *configuration.InsertRequestConfig) (map[string]*http.InsertResponse, error) {
	if config == nil {
		config = &configuration.InsertRequestConfig{}
	}

	if config.RequestConfig == nil {
		config.RequestConfig = &configuration.RequestConfig{}
	}

	graphName := ""
	if config.Graph != "" {
		graphName = config.Graph
	} else if api.Config.DefaultGraph != "" {
		graphName = api.Config.DefaultGraph
	}

	resps := &http.InsertBatchAutoResponse{
		Resps:     map[string]*http.InsertResponse{},
		ErrorItem: map[int]string{},
		Statistic: &http.Statistic{},
	}

	// collect schema and edge index in rows
	m := map[string]map[int]int{}
	schemas, err := api.ShowEdgeSchema(config.RequestConfig)

	if err != nil {
		return nil, err
	}

	batches := map[string]*Batch{}

	for index, edge := range rows {

		if _, ok := m[edge.Schema]; !ok {
			m[edge.Schema] = map[int]int{}
		}
		// init schema
		if batches[edge.Schema] == nil {

			batches[edge.Schema] = &Batch{}

			s := utils.Find(schemas, func(i int) bool {
				return schemas[i].Name == edge.Schema
			})

			if schema, ok := s.(*structs.Schema); ok {
				batches[edge.Schema].Schema = schema
			} else {
				// schema not exit
				return nil, errors.New("Edge Schema not found : " + edge.Schema)
			}
		}

		batch := batches[edge.Schema]
		// add rows
		row, err := convertSdkEdgeRowToUltipaEdgeRow(batch.Schema, edge, index, config.RequestConfig)
		if err != nil {
			return nil, err
		}

		if row != nil {
			batch.Edges = append(batch.Edges, row)
			m[edge.Schema][len(batch.Edges)-1] = index
		}
		//batch.Edges = append(batch.Edges, edge)
	}

	for _, batch := range batches {
		batchSchema := batch.Schema

		//config.UseMaster = true
		client, _, err := api.GetClient(config.RequestConfig)

		if err != nil {
			return nil, err
		}

		ctx, cancel, err := api.Pool.NewContext(config.RequestConfig)
		if err != nil {
			return nil, err
		}
		defer cancel()

		table := &ultipa.EntityTable{}

		table.Schemas = []*ultipa.Schema{
			{
				SchemaName: batchSchema.Name,
				Properties: []*ultipa.Property{},
			},
		}

		for _, prop := range batchSchema.Properties {

			if prop.IsIDType() || prop.IsIgnore() {
				continue
			}

			table.Schemas[0].Properties = append(table.Schemas[0].Properties, &ultipa.Property{
				PropertyName: prop.Name,
				PropertyType: prop.Type,
				SubTypes:     prop.SubTypes,
			})
		}

		if err != nil {
			return nil, err
		}
		table.EntityRows = batch.Edges
		resp, err := client.InsertEdges(ctx, &ultipa.InsertEdgesRequest{
			GraphName:  graphName,
			EdgeTable:  table,
			InsertType: config.InsertType,
			//CreateNodeIfNotExist: config.CreateNodeIfNotExist,
			Silent: config.Silent,
		})

		if err != nil {
			return nil, err
		}

		if resp.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
			if resps.ErrorCode == "" {
				resps.ErrorCode = ultipa.ErrorCode_name[int32(resp.Status.ErrorCode)]
			}
			resps.Msg += batchSchema.Name + ":" + resp.Status.Msg + "\r\n"
		}

		response, err := http.NewEdgesInsertResponse(resp)
		resps.Resps[batchSchema.Name] = response

		for k, v := range response.ErrorItems {
			m3 := m[batchSchema.Name]
			vl := m3[k]
			resps.ErrorItem[vl] = v
		}
		resps.Statistic.TotalCost += response.Statistic.TotalCost
		resps.Statistic.EngineCost += response.Statistic.EngineCost
	}

	return resps.Resps, nil
}

func (api *UltipaAPI) InsertEdges(schemaName string, edges []*structs.Edge, config *configuration.InsertRequestConfig) (*http.Response, error) {
	params := ""
	switch config.InsertType {
	case ultipa.InsertType_NORMAL:
		params = "insert()"
	case ultipa.InsertType_OVERWRITE:
		params = "insert().overwrite()"
	case ultipa.InsertType_UPSERT:
		params = "upsert()"
	default:
		return nil, fmt.Errorf("InsertEdges error, unknown InsertType: %d", config.InsertType)
	}

	uql := fmt.Sprintf(`%s.into(@%s).edges([%s])`, params, schemaName, structs.EdgesToInsertUql(edges))
	if !config.Silent {
		uql = uql + " as edges return edges{*}"
	}
	resp, err := api.Uql(uql, config.RequestConfig)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
