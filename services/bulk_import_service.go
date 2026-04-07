package services

import (
	"context"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
)

// BulkImportService handles bulk import operations.
type BulkImportService struct {
	ctx *ServiceContext
}

// NewBulkImportService creates a new BulkImportService.
func NewBulkImportService(ctx *ServiceContext) *BulkImportService {
	return &BulkImportService{
		ctx: ctx,
	}
}

// BulkImportOptions represents bulk import options (mirrors main package).
type BulkImportOptions struct {
	EstimatedNodes  int64
	EstimatedEdges  int64
	BatchSize       int
	ParallelWorkers int
	SkipInvalidData bool
}

// BulkImportSession represents an active bulk import session.
type BulkImportSession struct {
	SessionID string
	GraphName string
	Success   bool
	Message   string
	Status    string
	CreatedAt int64
}

// CheckpointResult represents checkpoint result.
type CheckpointResult struct {
	Success       bool
	NodesImported int64
	EdgesImported int64
	Message       string
}

// EndBulkImportResult represents end bulk import result.
type EndBulkImportResult struct {
	Success       bool
	NodesImported int64
	EdgesImported int64
	DurationMs    int64
	Message       string
}

// StartBulkImport starts a bulk import session.
func (s *BulkImportService) StartBulkImport(ctx context.Context, graphName string, opts *BulkImportOptions) (*BulkImportSession, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.StartBulkImportRequest{
		GraphName: graphName,
	}

	if opts != nil {
		req.EstimatedNodes = opts.EstimatedNodes
		req.EstimatedEdges = opts.EstimatedEdges
	}

	resp, err := s.ctx.BulkImportClient.StartBulkImport(ctx, req)
	if err != nil {
		return nil, err
	}

	status := "ACTIVE"
	if !resp.Success {
		status = "FAILED"
	}

	return &BulkImportSession{
		SessionID: resp.SessionId,
		GraphName: graphName,
		Success:   resp.Success,
		Message:   resp.Message,
		Status:    status,
		CreatedAt: 0,
	}, nil
}

// Checkpoint is deprecated. Returns success without making an RPC call.
// Deprecated: Server checkpoint is now a no-op.
func (s *BulkImportService) Checkpoint(ctx context.Context, sessionID string) (*CheckpointResult, error) {
	return &CheckpointResult{
		Success: true,
		Message: "Checkpoint has been removed; use EndBulkImport which performs a final flush",
	}, nil
}

// EndBulkImport ends a bulk import session.
func (s *BulkImportService) EndBulkImport(ctx context.Context, sessionID string) (*EndBulkImportResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.EndBulkImportRequest{
		SessionId: sessionID,
	}

	resp, err := s.ctx.BulkImportClient.EndBulkImport(ctx, req)
	if err != nil {
		return nil, err
	}

	return &EndBulkImportResult{
		Success:       resp.Success,
		NodesImported: 0, // Proto doesn't have NodesImported
		EdgesImported: 0, // Proto doesn't have EdgesImported
		DurationMs:    0, // Proto doesn't have DurationMs
		Message:       resp.Message,
	}, nil
}

// AbortBulkImport aborts a bulk import session.
func (s *BulkImportService) AbortBulkImport(ctx context.Context, sessionID string) error {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.AbortBulkImportRequest{
		SessionId: sessionID,
	}

	_, err := s.ctx.BulkImportClient.AbortBulkImport(ctx, req)
	return err
}

// BulkImportStatus represents the status of a bulk import session.
type BulkImportStatus struct {
	IsActive            bool
	GraphName           string
	RecordCount         int64
	LastCheckpointCount int64
	CreatedAt           int64
	LastActivity        int64
}

// GetBulkImportStatus retrieves the status of a bulk import session.
func (s *BulkImportService) GetBulkImportStatus(ctx context.Context, sessionID string) (*BulkImportStatus, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.GetBulkImportStatusRequest{
		SessionId: sessionID,
	}

	resp, err := s.ctx.BulkImportClient.GetBulkImportStatus(ctx, req)
	if err != nil {
		return nil, err
	}

	return &BulkImportStatus{
		IsActive:            resp.IsActive,
		GraphName:           resp.GraphName,
		RecordCount:         resp.RecordCount,
		LastCheckpointCount: resp.LastCheckpointCount,
		CreatedAt:           resp.CreatedAt,
		LastActivity:        resp.LastActivity,
	}, nil
}
