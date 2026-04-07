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
	GetSessionID     func() uint64
	GetDefaultGraph  func() string
	GetTimeout       func() int
	SetDefaultGraph  func(name string)
	UpdateActivity   func()
	IsLoggedIn       func() bool
}

// WithSessionMetadata adds session-id to the context metadata.
func (sc *ServiceContext) WithSessionMetadata(ctx context.Context) context.Context {
	md := metadata.Pairs("session-id", fmt.Sprintf("%d", sc.GetSessionID()))
	return metadata.NewOutgoingContext(ctx, md)
}
