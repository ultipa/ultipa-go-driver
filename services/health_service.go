package services

import (
	"context"
	"io"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
)

// HealthService handles health check operations.
type HealthService struct {
	ctx *ServiceContext
}

// NewHealthService creates a new HealthService.
func NewHealthService(ctx *ServiceContext) *HealthService {
	return &HealthService{
		ctx: ctx,
	}
}

// HealthCheck performs a health check for a service.
func (s *HealthService) HealthCheck(ctx context.Context, service string) (int32, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.HealthCheckRequest{
		Service: service,
	}

	resp, err := s.ctx.HealthClient.Check(ctx, req)
	if err != nil {
		return 0, err
	}

	return HealthStatusFromProto(resp.Status), nil
}

// Watch watches the health status of a service and streams status changes.
func (s *HealthService) Watch(ctx context.Context, service string, callback func(int32) error) error {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.HealthCheckRequest{
		Service: service,
	}

	stream, err := s.ctx.HealthClient.Watch(ctx, req)
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

		status := HealthStatusFromProto(resp.Status)
		if err := callback(status); err != nil {
			return err
		}
	}

	return nil
}
