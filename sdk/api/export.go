package api

import (
	"io"
	"sync"

	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
	"google.golang.org/grpc"
)

func (api *UltipaAPI) Export(exportRequest *ultipa.ExportRequest, config *configuration.RequestConfig, cb func(nodes []*structs.Node, edges []*structs.Edge) error) error {
	var err error

	client, err := api.GetControlClient(config)
	if err != nil {
		return err
	}

	ctx, cancel, err := api.Pool.NewContext(config)
	if err != nil {
		return err
	}
	defer cancel()

	resp, respErr := client.Export(ctx, exportRequest, grpc.MaxCallRecvMsgSize(api.Config.MaxRecvSize), grpc.MaxCallSendMsgSize(api.Config.MaxRecvSize))

	if respErr != nil {
		return respErr
	}

	for {
		record, err := resp.Recv()

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		wg := sync.WaitGroup{}
		if record.NodeTable != nil {
			wg.Add(len(record.NodeTable.EntityRows))

			//record.NodeTable
			nodeSchemaMap := structs.NewSchemaMapFromProtoSchema(record.NodeTable.Schemas, ultipa.DBType_DBNODE)
			nodes := make([]*structs.Node, len(record.NodeTable.EntityRows))
			for index, nodeRow := range record.NodeTable.EntityRows {
				var parseErr error
				go func(index int, row *ultipa.EntityRow) {
					defer wg.Done()
					node, err := structs.NewNodeFromNodeRow(nodeSchemaMap[exportRequest.Schema], row)
					if err != nil {
						parseErr = err
					}
					nodes[index] = node
				}(index, nodeRow)
				if parseErr != nil {
					return parseErr
				}
			}

			wg.Wait()
			err = cb(nodes, nil)
		}

		if record.EdgeTable != nil {
			wg.Add(len(record.EdgeTable.EntityRows))
			//record.EdgeTable
			edgeSchemaMap := structs.NewSchemaMapFromProtoSchema(record.EdgeTable.Schemas, ultipa.DBType_DBEDGE)
			edges := make([]*structs.Edge, len(record.EdgeTable.EntityRows))
			for index, edgeRow := range record.EdgeTable.EntityRows {
				var parseErr error
				go func(index int, row *ultipa.EntityRow) {
					defer wg.Done()
					edge, err := structs.NewEdgeFromEdgeRow(edgeSchemaMap[row.SchemaName], row)
					if err != nil {
						parseErr = err
					}
					edges[index] = edge
				}(index, edgeRow)
				if parseErr != nil {
					return parseErr
				}
			}

			wg.Wait()
			err = cb(nil, edges)
		}

		if err != nil {
			return err
		}
	}

	return err
}
