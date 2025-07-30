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

func (api *UltipaAPI) InsertNodesBatch(table *ultipa.EntityTable, config *configuration.InsertRequestConfig) (*http.InsertResponse, error) {
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

	resp, err := client.InsertNodes(ctx, &ultipa.InsertNodesRequest{
		GraphName:  graphName,
		NodeTable:  table,
		InsertType: config.InsertType,
		Silent:     config.Silent,
	})

	if err != nil {
		return nil, err
	}

	if resp.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
		return nil, errors.New(resp.Status.Msg)
	}

	return http.NewNodesInsertResponse(resp)
}

func (api *UltipaAPI) InsertNodesBatchBySchema(schema *structs.Schema, rows []*structs.Node, config *configuration.InsertRequestConfig) (*http.InsertResponse, error) {
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

	err, nodeRows := setPropertiesToNodeRow(schema, rows, config.RequestConfig)

	if err != nil {
		return nil, err
	}
	table.EntityRows = nodeRows
	//log.Printf("transporter debug UltipaRpcsClient.InsertNodes contexValue graph_name: %v \n", ctx.Value("graph_name"))
	resp, err := client.InsertNodes(ctx, &ultipa.InsertNodesRequest{
		GraphName:  graphName,
		NodeTable:  table,
		InsertType: config.InsertType,
		Silent:     config.Silent,
	})

	if err != nil {
		return nil, err
	}

	if resp.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
		return nil, errors.New(resp.Status.Msg)
	}

	return http.NewNodesInsertResponse(resp)
}

func setPropertiesToNodeRow(schema *structs.Schema, rows []*structs.Node, config *configuration.RequestConfig) (error, []*ultipa.EntityRow) {
	wg := sync.WaitGroup{}
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	nodeRows := make([]*ultipa.EntityRow, len(rows))

	for index, row := range rows {
		err = checkNodeProperties(schema, row, index)
		if err != nil {
			return err, nodeRows
		}
		wg.Add(1)
		go func(index int, row *structs.Node) {
			defer wg.Done()
			var newNode *ultipa.EntityRow
			newNode, err = doConvertSdkNodeRowToUltipaNodeRow(schema, row, index, config)
			if err != nil {
				cancel()
				return
			}
			nodeRows[index] = newNode
		}(index, row)
		select {
		case <-ctx.Done():
			return err, nodeRows
		default:
		}
	}
	wg.Wait()
	return err, nodeRows
}

func checkNodeProperties(schema *structs.Schema, row *structs.Node, index int) error {
	if row == nil {
		return errors.New(fmt.Sprintf("node row [%d] error: node row is nil.", index))
	}
	//err := CheckValuesAndProperties(schema.Properties, row.GetValues(), index)
	//if err != nil {
	//	return err
	//}
	return nil
}

func convertSdkNodeRowToUltipaNodeRow(schema *structs.Schema, row *structs.Node, index int, config *configuration.RequestConfig) (*ultipa.EntityRow, error) {
	err := checkNodeProperties(schema, row, index)
	if err != nil {
		return nil, err
	}
	return doConvertSdkNodeRowToUltipaNodeRow(schema, row, index, config)
}

func doConvertSdkNodeRowToUltipaNodeRow(schema *structs.Schema, row *structs.Node, index int, config *configuration.RequestConfig) (*ultipa.EntityRow, error) {
	newNode := &ultipa.EntityRow{
		Id:         row.ID,
		Uuid:       row.UUID,
		SchemaName: schema.Name,
	}
	for _, prop := range schema.Properties {
		if prop.IsIDType() || prop.IsIgnore() {
			continue
		}

		var bs []byte
		var err error
		if !row.Values.Contain(prop.Name) {
			bs = utils.GetNullBytes(prop.Type)
			//return nil, errors.New(fmt.Sprintf("node row [%d] error: values doesn't contain property [%s]", index, prop.Name))
		} else {
			bs, err = row.GetBytesSafe(prop.Name, prop.Type, prop.SubTypes, config)
		}

		if err != nil {
			logger.PrintError("Get row bytes value failed  " + prop.Name + " " + err.Error())
			err = errors.New(fmt.Sprintf("node row [%d] error: failed to serialize value of property %s,value=%v", index, prop.Name, row.Values.Get(prop.Name)))
			return nil, err
		}
		newNode.Values = append(newNode.Values, bs)
	}
	return newNode, nil
}

type Batch struct {
	Nodes  []*ultipa.EntityRow
	Edges  []*ultipa.EntityRow
	Schema *structs.Schema
}

// InsertNodesBatchAuto Nodes interface values should be string
func (api *UltipaAPI) InsertNodesBatchAuto(rows []*structs.Node, config *configuration.InsertRequestConfig) (map[string]*http.InsertResponse, error) {
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

	// collect schema and node index in rows
	m := map[string]map[int]int{}
	schemas, err := api.ShowNodeSchema(config.RequestConfig)

	if err != nil {
		return nil, err
	}

	batches := map[string]*Batch{}

	for index, node := range rows {
		if _, ok := m[node.Schema]; !ok {
			m[node.Schema] = map[int]int{}
		}
		// init schema
		if batches[node.Schema] == nil {

			batches[node.Schema] = &Batch{}

			s := utils.Find(schemas, func(i int) bool {
				return schemas[i].Name == node.Schema
			})

			if schema, ok := s.(*structs.Schema); ok {
				batches[node.Schema].Schema = schema
			} else {
				// schema not exit
				return nil, errors.New("Schema not found : " + node.Schema)
			}
		}

		batch := batches[node.Schema]
		// add rows
		row, err := convertSdkNodeRowToUltipaNodeRow(batch.Schema, node, index, config.RequestConfig)
		if err != nil {
			return nil, err
		}
		//node.UpdateByValueID()
		if row != nil {
			batch.Nodes = append(batch.Nodes, row)
			m[node.Schema][len(batch.Nodes)-1] = index
		}
		//batch.Nodes = append(batch.Nodes, node)
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
		table.EntityRows = batch.Nodes
		resp, err := client.InsertNodes(ctx, &ultipa.InsertNodesRequest{
			GraphName:  graphName,
			NodeTable:  table,
			InsertType: config.InsertType,
			Silent:     config.Silent,
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

		response, err := http.NewNodesInsertResponse(resp)
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

func (api *UltipaAPI) InsertNodes(schemaName string, nodes []*structs.Node, config *configuration.InsertRequestConfig) (*http.Response, error) {
	params := ""
	switch config.InsertType {
	case ultipa.InsertType_NORMAL:
		params = "insert()"
	case ultipa.InsertType_OVERWRITE:
		params = "insert().overwrite()"
	case ultipa.InsertType_UPSERT:
		params = "upsert()"
	default:
		return nil, fmt.Errorf("InsertNodes error, unknown InsertType: %d", config.InsertType)
	}

	uql := fmt.Sprintf(`%s.into(@%s).nodes([%s])`, params, schemaName, structs.NodesToInsertUql(nodes))
	if !config.Silent {
		uql = uql + " as nodes return nodes{*}"
	}
	resp, err := api.Uql(uql, config.RequestConfig)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
