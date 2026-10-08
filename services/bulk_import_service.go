package services

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc/metadata"
)

// CapabilitiesHeader is the request header in which a client names the
// optional answers it understands, separated by commas.
const CapabilitiesHeader = "x-gqldb-capabilities"

// CapabilityBulkProgress asks the server for the early answer of a long
// EndBulkImport or AbortBulkImport: shortly before the call's deadline,
// success=false, in_progress=true and the session's state and progress, while
// the End or the discard goes on. Only a client that sends the call again
// until the state is final may ask for it.
const CapabilityBulkProgress = "bulk-progress"

// withCapabilities adds the capabilities named to ctx's outgoing metadata.
func withCapabilities(ctx context.Context, capabilities []string) context.Context {
	if len(capabilities) == 0 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, CapabilitiesHeader, strings.Join(capabilities, ","))
}

// BulkImportStateName maps the proto's state to its short name ("ACTIVE",
// "ENDING", ...); "" for BULK_IMPORT_STATE_UNSPECIFIED (an older server).
func BulkImportStateName(s pb.BulkImportState) string {
	if s == pb.BulkImportState_BULK_IMPORT_STATE_UNSPECIFIED {
		return ""
	}
	return strings.TrimPrefix(s.String(), "BULK_IMPORT_STATE_")
}

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
// The server returns a single combined record count (no per-type split), plus
// wall-clock timing; both are surfaced here.
type EndBulkImportResult struct {
	Success      bool
	TotalRecords int64
	DurationMs   int64
	Message      string
	// State, InProgress, Progress and ProgressTotal as the server sent them;
	// State "" from a server older than the states.
	State         string
	InProgress    bool
	Progress      int64
	ProgressTotal int64
}

// AbortBulkImportResult represents an abort bulk import answer.
type AbortBulkImportResult struct {
	Success       bool
	Message       string
	State         string
	InProgress    bool
	Progress      int64
	ProgressTotal int64
	NodesRemoved  int64
	EdgesRemoved  int64
	DurationMs    int64
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

	// A server that refuses the session answers Success=false with an OK gRPC
	// status: a read-only database, the licence's read-only mode, a session
	// already open on the graph, the session limit. Returning that hands the
	// caller a session id it cannot use -- empty, or another session's -- and the
	// refusal then surfaces one call later, from BulkCreateNodes, as "bulk import
	// session is required", which names the driver instead of the server's reason.
	// A server from gqldb-grpc d368b17 on answers FAILED_PRECONDITION instead,
	// which arrives as the error above.
	if !resp.Success || resp.SessionId == "" {
		msg := resp.Message
		if msg == "" {
			msg = "the server refused the session and gave no reason"
		}
		return nil, fmt.Errorf("refused by the server: %s", msg)
	}

	// The refusal above is the only way Success is false, so a session that
	// reaches here is active.
	status := "ACTIVE"

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

// EndBulkImport ends a bulk import session. capabilities are sent in
// CapabilitiesHeader (CapabilityBulkProgress for the early answer).
func (s *BulkImportService) EndBulkImport(ctx context.Context, sessionID string, capabilities ...string) (*EndBulkImportResult, error) {
	ctx = withCapabilities(s.ctx.WithSessionMetadata(ctx), capabilities)

	req := &pb.EndBulkImportRequest{
		SessionId: sessionID,
	}

	resp, err := s.ctx.BulkImportClient.EndBulkImport(ctx, req)
	if err != nil {
		return nil, err
	}

	return &EndBulkImportResult{
		Success:       resp.Success,
		TotalRecords:  resp.TotalRecords,
		DurationMs:    resp.TimeCostNs / 1_000_000, // ns → ms
		Message:       resp.Message,
		State:         BulkImportStateName(resp.State),
		InProgress:    resp.InProgress,
		Progress:      resp.Progress,
		ProgressTotal: resp.ProgressTotal,
	}, nil
}

// AbortBulkImport aborts a bulk import session.
//
// Deprecated: it drops the server's answer; use AbortBulkImportWithResult.
func (s *BulkImportService) AbortBulkImport(ctx context.Context, sessionID string) error {
	_, err := s.AbortBulkImportWithResult(ctx, sessionID)
	return err
}

// AbortBulkImportWithResult aborts a bulk import session (its data is
// discarded) and returns the server's answer. capabilities are sent in
// CapabilitiesHeader (CapabilityBulkProgress for the early answer).
func (s *BulkImportService) AbortBulkImportWithResult(ctx context.Context, sessionID string, capabilities ...string) (*AbortBulkImportResult, error) {
	ctx = withCapabilities(s.ctx.WithSessionMetadata(ctx), capabilities)

	req := &pb.AbortBulkImportRequest{
		SessionId: sessionID,
	}

	resp, err := s.ctx.BulkImportClient.AbortBulkImport(ctx, req)
	if err != nil {
		return nil, err
	}
	return &AbortBulkImportResult{
		Success:       resp.Success,
		Message:       resp.Message,
		State:         BulkImportStateName(resp.State),
		InProgress:    resp.InProgress,
		Progress:      resp.Progress,
		ProgressTotal: resp.ProgressTotal,
		NodesRemoved:  resp.NodesRemoved,
		EdgesRemoved:  resp.EdgesRemoved,
		DurationMs:    resp.TimeCostNs / 1_000_000,
	}, nil
}

// BulkImportStatus represents the status of a bulk import session.
type BulkImportStatus struct {
	IsActive            bool
	GraphName           string
	RecordCount         int64
	LastCheckpointCount int64
	CreatedAt           int64
	LastActivity        int64
	State               string // "" from a server older than the states
	Progress            int64
	ProgressTotal       int64
	Message             string
	DiscardPending      bool
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
		State:               BulkImportStateName(resp.State),
		Progress:            resp.Progress,
		ProgressTotal:       resp.ProgressTotal,
		Message:             resp.Message,
		DiscardPending:      resp.DiscardPending,
	}, nil
}
