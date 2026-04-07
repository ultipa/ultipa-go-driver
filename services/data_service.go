package services

import (
	"context"
	"io"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
)

// DataService handles node/edge insert/delete/export operations.
type DataService struct {
	ctx *ServiceContext
}

// NewDataService creates a new DataService.
func NewDataService(ctx *ServiceContext) *DataService {
	return &DataService{
		ctx: ctx,
	}
}

// NodeData represents node data for insertion (mirrors main package).
type NodeData struct {
	ID         string
	Labels     []string
	Properties map[string]interface{}
}

// EdgeData represents edge data for insertion (mirrors main package).
type EdgeData struct {
	ID         string
	Label      string
	From       string
	To         string
	Properties map[string]interface{}
}

// InsertNodesResult represents the result of inserting nodes.
type InsertNodesResult struct {
	Success      bool
	NodeIDs      []string
	NodesCreated int64
	Message      string
}

// InsertEdgesResult represents the result of inserting edges.
type InsertEdgesResult struct {
	Success      bool
	EdgesCreated int64
	Message      string
}

// DeleteResult represents the result of delete operations.
type DeleteResult struct {
	Success bool
	Deleted int64
	Message string
}

// InsertNodesConfig represents configuration for inserting nodes.
type InsertNodesConfig struct {
	Overwrite           bool
	BulkImportSessionID string
}

// InsertEdgesConfig represents configuration for inserting edges.
type InsertEdgesConfig struct {
	SkipInvalidNodes    bool
	BulkImportSessionID string
}

// InsertNodes inserts multiple nodes into a graph.
func (s *DataService) InsertNodes(ctx context.Context, graphName string, nodes []*NodeData, config *InsertNodesConfig,
	convertProps func(map[string]interface{}) (map[string]*pb.TypedValue, error)) (*InsertNodesResult, error) {

	ctx = s.ctx.WithSessionMetadata(ctx)

	pbNodes := make([]*pb.NodeData, len(nodes))
	for i, n := range nodes {
		props, err := convertProps(n.Properties)
		if err != nil {
			return nil, err
		}
		pbNodes[i] = &pb.NodeData{
			Id:         n.ID,
			Labels:     n.Labels,
			Properties: props,
		}
	}

	req := &pb.InsertNodesRequest{
		GraphName: graphName,
		Nodes:     pbNodes,
	}

	// Set options and bulk import session ID if provided
	if config != nil {
		// Always set options when config is provided
		req.Options = &pb.BulkCreateNodesOptions{
			Overwrite: config.Overwrite,
		}
		if config.BulkImportSessionID != "" {
			req.BulkImportSessionId = config.BulkImportSessionID
		}
	} else {
		// ALWAYS provide options, even when config is nil (defensive programming)
		req.Options = &pb.BulkCreateNodesOptions{
			Overwrite: false,
		}
	}

	resp, err := s.ctx.DataClient.InsertNodes(ctx, req)
	if err != nil {
		return nil, err
	}

	s.ctx.UpdateActivity()
	return &InsertNodesResult{
		Success:      resp.Success,
		NodeIDs:      resp.NodeIds,
		NodesCreated: resp.NodeCount,
		Message:      resp.Message,
	}, nil
}

// InsertEdges inserts multiple edges into a graph.
func (s *DataService) InsertEdges(ctx context.Context, graphName string, edges []*EdgeData, config *InsertEdgesConfig,
	convertProps func(map[string]interface{}) (map[string]*pb.TypedValue, error)) (*InsertEdgesResult, error) {

	ctx = s.ctx.WithSessionMetadata(ctx)

	pbEdges := make([]*pb.EdgeData, len(edges))
	for i, e := range edges {
		props, err := convertProps(e.Properties)
		if err != nil {
			return nil, err
		}
		pbEdges[i] = &pb.EdgeData{
			Label:      e.Label,
			FromNodeId: e.From,
			ToNodeId:   e.To,
			Properties: props,
		}
	}

	req := &pb.InsertEdgesRequest{
		GraphName: graphName,
		Edges:     pbEdges,
	}

	// Set options and bulk import session ID if provided
	if config != nil {
		// Always set options when config is provided
		req.Options = &pb.BulkCreateEdgesOptions{
			SkipInvalidNodes: config.SkipInvalidNodes,
		}
		if config.BulkImportSessionID != "" {
			req.BulkImportSessionId = config.BulkImportSessionID
		}
	} else {
		// ALWAYS provide options, even when config is nil (defensive programming)
		req.Options = &pb.BulkCreateEdgesOptions{
			SkipInvalidNodes: false,
		}
	}

	resp, err := s.ctx.DataClient.InsertEdges(ctx, req)
	if err != nil {
		return nil, err
	}

	s.ctx.UpdateActivity()
	return &InsertEdgesResult{
		Success:      resp.Success,
		EdgesCreated: resp.EdgeCount,
		Message:      resp.Message,
	}, nil
}

// DeleteNodes deletes nodes from a graph.
func (s *DataService) DeleteNodes(ctx context.Context, graphName string, nodeIDs, labels []string, where string) (*DeleteResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.DeleteNodesRequest{
		GraphName: graphName,
		NodeIds:   nodeIDs,
		Labels:    labels,
		Where:     where,
	}

	resp, err := s.ctx.DataClient.DeleteNodes(ctx, req)
	if err != nil {
		return nil, err
	}

	return &DeleteResult{
		Success: resp.Success,
		Deleted: resp.DeletedCount,
		Message: resp.Message,
	}, nil
}

// DeleteEdges deletes edges from a graph.
func (s *DataService) DeleteEdges(ctx context.Context, graphName string, edgeIDs []string, label, where string) (*DeleteResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.DeleteEdgesRequest{
		GraphName: graphName,
		EdgeIds:   edgeIDs,
		Label:     label,
		Where:     where,
	}

	resp, err := s.ctx.DataClient.DeleteEdges(ctx, req)
	if err != nil {
		return nil, err
	}

	return &DeleteResult{
		Success: resp.Success,
		Deleted: resp.DeletedCount,
		Message: resp.Message,
	}, nil
}

// ExportConfig represents configuration for the Export operation.
type ExportConfig struct {
	GraphName       string
	BatchSize       int32
	ExportNodes     bool
	ExportEdges     bool
	NodeLabels      []string
	EdgeLabels      []string
	IncludeMetadata bool
}

// ExportChunk represents a chunk of exported data.
type ExportChunk struct {
	Data    []byte
	IsFinal bool
	Stats   *ExportStats
}

// ExportStats contains export statistics.
type ExportStats struct {
	NodesExported int64
	EdgesExported int64
	BytesWritten  int64
	DurationMs    int64
}

// Export exports graph data in JSON Lines format (streaming).
func (s *DataService) Export(ctx context.Context, config *ExportConfig, callback func(*ExportChunk) error) error {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.ExportRequest{
		GraphName:       config.GraphName,
		BatchSize:       config.BatchSize,
		ExportNodes:     config.ExportNodes,
		ExportEdges:     config.ExportEdges,
		NodeLabels:      config.NodeLabels,
		EdgeLabels:      config.EdgeLabels,
		IncludeMetadata: config.IncludeMetadata,
	}

	stream, err := s.ctx.DataClient.Export(ctx, req)
	if err != nil {
		return err
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		chunk := &ExportChunk{
			Data:    resp.Data,
			IsFinal: resp.IsFinal,
		}

		// Convert stats if present (in final message)
		if resp.Stats != nil {
			chunk.Stats = &ExportStats{
				NodesExported: resp.Stats.NodesExported,
				EdgesExported: resp.Stats.EdgesExported,
				BytesWritten:  resp.Stats.BytesWritten,
				DurationMs:    resp.Stats.DurationMs,
			}
		}

		if err := callback(chunk); err != nil {
			return err
		}

		if resp.IsFinal {
			break
		}
	}

	s.ctx.UpdateActivity()
	return nil
}
