package services

import (
	"context"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
)

// SessionService handles authentication and session management.
type SessionService struct {
	ctx *ServiceContext
}

// NewSessionService creates a new SessionService.
func NewSessionService(ctx *ServiceContext) *SessionService {
	return &SessionService{
		ctx: ctx,
	}
}

// Session represents a user session (mirrors main package).
type Session struct {
	ID             uint64
	ServerVersion  string
	Roles          []string
	DefaultGraph   string
	IsCluster      bool
	ClusterID      string
	PartitionCount int32
}

// Login performs login against the gRPC service.
func (s *SessionService) Login(ctx context.Context, username, password, defaultGraph string) (*Session, error) {
	pbReq := &pb.LoginRequest{
		Username:     username,
		Password:     password,
		DefaultGraph: defaultGraph,
	}

	resp, err := s.ctx.SessionClient.Login(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	// Prefer the server's authoritative LoginResponse.current_graph
	// (new servers always populate it — equals the validated
	// req.default_graph or empty). Falls back to the request value for
	// older servers that don't populate the field.
	dg := resp.CurrentGraph
	if dg == "" {
		dg = defaultGraph
	}

	return &Session{
		ID:             resp.SessionId,
		ServerVersion:  resp.ServerVersion,
		Roles:          resp.Roles,
		DefaultGraph:   dg,
		IsCluster:      resp.IsCluster,
		ClusterID:      resp.ClusterId,
		PartitionCount: resp.PartitionCount,
	}, nil
}

// Logout performs logout against the gRPC service.
func (s *SessionService) Logout(ctx context.Context) error {
	ctx = s.ctx.WithSessionMetadata(ctx)
	req := &pb.LogoutRequest{}
	_, err := s.ctx.SessionClient.Logout(ctx, req)
	return err
}

// Ping performs ping against the gRPC service.
func (s *SessionService) Ping(ctx context.Context) (int64, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)
	req := &pb.PingRequest{}
	resp, err := s.ctx.SessionClient.Ping(ctx, req)
	if err != nil {
		return 0, err
	}
	return resp.LatencyNs, nil
}
