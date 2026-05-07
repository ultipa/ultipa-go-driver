package services

import (
	"context"
	"fmt"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc/metadata"
)

// ServiceContext provides shared dependencies for all services.
type ServiceContext struct {
	// gRPC service clients
	SessionClient     pb.SessionServiceClient
	QueryClient       pb.QueryServiceClient
	DataClient        pb.DataServiceClient
	GraphClient       pb.GraphServiceClient
	TransactionClient pb.TransactionServiceClient
	HealthClient      pb.HealthClient
	AdminClient       pb.AdminServiceClient
	BulkImportClient  pb.BulkImportServiceClient

	// Session and config accessors
	GetSessionID         func() uint64
	GetServerVersion     func() string
	GetClientSessionID   func() string
	GetDefaultGraph      func() string
	GetTimeout           func() int
	SetDefaultGraph      func(name string)
	UpdateActivity       func()
	IsLoggedIn           func() bool
}

// WithSessionMetadata adds session-id (legacy) and, when present, the new
// `x-ultipa-session-id` (transaction-branch §2.1) headers to the outgoing
// gRPC metadata.
func (sc *ServiceContext) WithSessionMetadata(ctx context.Context) context.Context {
	pairs := []string{"session-id", fmt.Sprintf("%d", sc.GetSessionID())}
	if sc.GetClientSessionID != nil {
		if csid := sc.GetClientSessionID(); csid != "" {
			pairs = append(pairs, "x-ultipa-session-id", csid)
		}
	}
	md := metadata.Pairs(pairs...)
	return metadata.NewOutgoingContext(ctx, md)
}
