package services

import (
	"context"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// GraphService handles graph management operations.
type GraphService struct {
	ctx *ServiceContext
}

// NewGraphService creates a new GraphService.
func NewGraphService(ctx *ServiceContext) *GraphService {
	return &GraphService{
		ctx: ctx,
	}
}

// GraphInfo represents graph metadata (mirrors main package).
type GraphInfo struct {
	Name        string
	GraphType   types.GraphType
	NodeCount   int64
	EdgeCount   int64
	Description string
}

// CreateGraph creates a new graph.
func (s *GraphService) CreateGraph(ctx context.Context, name string, graphType types.GraphType, description string) (bool, string, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.CreateGraphRequest{
		Name:        name,
		GraphType:   GraphTypeToProto(graphType),
		Description: description,
	}

	resp, err := s.ctx.GraphClient.CreateGraph(ctx, req)
	if err != nil {
		return false, "", err
	}

	return resp.Success, resp.Message, nil
}

// DropGraph deletes a graph.
func (s *GraphService) DropGraph(ctx context.Context, name string, ifExists bool) (bool, string, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.DropGraphRequest{
		Name:     name,
		IfExists: ifExists,
	}

	resp, err := s.ctx.GraphClient.DropGraph(ctx, req)
	if err != nil {
		return false, "", err
	}

	return resp.Success, resp.Message, nil
}

// UseGraph sets the current graph for the session.
func (s *GraphService) UseGraph(ctx context.Context, name string) (bool, string, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.UseGraphRequest{
		Name:      name,
		SessionId: s.ctx.GetSessionID(),
	}

	resp, err := s.ctx.GraphClient.UseGraph(ctx, req)
	if err != nil {
		return false, "", err
	}

	if resp.Success {
		s.ctx.SetDefaultGraph(name)
	}

	return resp.Success, resp.Message, nil
}

// ListGraphs returns all available graphs.
func (s *GraphService) ListGraphs(ctx context.Context) ([]*GraphInfo, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.ListGraphsRequest{}

	resp, err := s.ctx.GraphClient.ListGraphs(ctx, req)
	if err != nil {
		return nil, err
	}

	graphs := make([]*GraphInfo, len(resp.Graphs))
	for i, g := range resp.Graphs {
		graphs[i] = &GraphInfo{
			Name:        g.Name,
			GraphType:   types.GraphType(g.GraphType),
			NodeCount:   g.NodeCount,
			EdgeCount:   g.EdgeCount,
			Description: g.Description,
		}
	}

	return graphs, nil
}

// GetGraphInfo returns information about a specific graph.
func (s *GraphService) GetGraphInfo(ctx context.Context, name string) (*GraphInfo, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.GetGraphInfoRequest{
		Name: name,
	}

	resp, err := s.ctx.GraphClient.GetGraphInfo(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Info == nil {
		return nil, nil
	}

	return &GraphInfo{
		Name:        resp.Info.Name,
		GraphType:   types.GraphType(resp.Info.GraphType),
		NodeCount:   resp.Info.NodeCount,
		EdgeCount:   resp.Info.EdgeCount,
		Description: resp.Info.Description,
	}, nil
}
