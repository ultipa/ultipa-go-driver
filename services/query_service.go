package services

import (
	"context"
	"io"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
)

// QueryService handles query execution operations.
type QueryService struct {
	ctx *ServiceContext
}

// NewQueryService creates a new QueryService.
func NewQueryService(ctx *ServiceContext) *QueryService {
	return &QueryService{
		ctx: ctx,
	}
}

// QueryConfig represents query configuration (mirrors main package).
type QueryConfig struct {
	GraphName     string
	TransactionID uint64
	// Timeout is the query timeout in seconds. If set to 0, the driver will use
	// the context deadline (if set) or fall back to the client's default timeout.
	// The timeout priority is: QueryConfig.Timeout > context deadline > client default.
	Timeout int
	// ReadOnly indicates if the query is read-only.
	ReadOnly bool
	// Parameters are the query parameters.
	Parameters map[string]interface{}
	// MaxPathResults limits the number of paths returned from path queries.
	// 0 means unlimited.
	MaxPathResults int64
}

// Response represents query response (mirrors main package).
type Response struct {
	Columns      []string
	Rows         []*Row
	RowCount     int64
	HasMore      bool
	Warnings     []string
	RowsAffected int64
}

// Row represents a result row (mirrors main package).
type Row struct {
	Values []*TypedValue
}

// Parameter represents a query parameter (mirrors main package).
type Parameter struct {
	Name  string
	Value *TypedValue
}

// Gql executes a GQL query and returns the results.
func (s *QueryService) Gql(ctx context.Context, query string, config *QueryConfig,
	newParameter func(name string, value interface{}) (*Parameter, error),
	getDefaultGraph func() string, getTimeout func() int) (*Response, error) {

	ctx = s.ctx.WithSessionMetadata(ctx)

	req, err := s.buildGqlRequest(ctx, query, config, newParameter, getDefaultGraph, getTimeout)
	if err != nil {
		return nil, err
	}

	resp, err := s.ctx.QueryClient.Gql(ctx, req)
	if err != nil {
		return nil, err
	}

	s.ctx.UpdateActivity()
	return s.convertGqlResponse(resp)
}

// GqlStream executes a GQL query and streams the results.
func (s *QueryService) GqlStream(ctx context.Context, query string, config *QueryConfig, callback func(*Response) error,
	newParameter func(name string, value interface{}) (*Parameter, error),
	getDefaultGraph func() string, getTimeout func() int) error {

	ctx = s.ctx.WithSessionMetadata(ctx)

	req, err := s.buildGqlRequest(ctx, query, config, newParameter, getDefaultGraph, getTimeout)
	if err != nil {
		return err
	}

	stream, err := s.ctx.QueryClient.GqlStream(ctx, req)
	if err != nil {
		return err
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		response, err := s.convertGqlResponse(resp)
		if err != nil {
			return err
		}

		if err := callback(response); err != nil {
			return err
		}
	}

	s.ctx.UpdateActivity()
	return nil
}

// Explain returns the execution plan for a query.
func (s *QueryService) Explain(ctx context.Context, query string, config *QueryConfig,
	newParameter func(name string, value interface{}) (*Parameter, error),
	getDefaultGraph func() string, getTimeout func() int) (string, error) {

	ctx = s.ctx.WithSessionMetadata(ctx)

	req, err := s.buildGqlRequest(ctx, query, config, newParameter, getDefaultGraph, getTimeout)
	if err != nil {
		return "", err
	}

	resp, err := s.ctx.QueryClient.Explain(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Plan, nil
}

// Profile executes a query with profiling and returns statistics.
func (s *QueryService) Profile(ctx context.Context, query string, config *QueryConfig,
	newParameter func(name string, value interface{}) (*Parameter, error),
	getDefaultGraph func() string, getTimeout func() int) (string, error) {

	ctx = s.ctx.WithSessionMetadata(ctx)

	req, err := s.buildGqlRequest(ctx, query, config, newParameter, getDefaultGraph, getTimeout)
	if err != nil {
		return "", err
	}

	resp, err := s.ctx.QueryClient.Profile(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Profile, nil
}

// buildGqlRequest builds a GQL request from query and config.
func (s *QueryService) buildGqlRequest(ctx context.Context, query string, config *QueryConfig,
	newParameter func(name string, value interface{}) (*Parameter, error),
	getDefaultGraph func() string, getTimeout func() int) (*pb.GqlRequest, error) {

	req := &pb.GqlRequest{
		Gql:       query,
		SessionId: s.ctx.GetSessionID(),
	}

	if config != nil {
		req.GraphName = config.GraphName
		req.TransactionId = config.TransactionID
		req.Timeout = int32(config.Timeout)
		req.ReadOnly = config.ReadOnly
		req.MaxPathResults = config.MaxPathResults

		if len(config.Parameters) > 0 {
			for name, value := range config.Parameters {
				param, err := newParameter(name, value)
				if err != nil {
					return nil, err
				}
				req.Parameters = append(req.Parameters, &pb.Parameter{
					Name: param.Name,
					Value: &pb.TypedValue{
						Type:   pb.PropertyType(param.Value.Type),
						Data:   param.Value.Data,
						IsNull: param.Value.IsNull,
					},
				})
			}
		}
	}

	// Use default graph if not specified
	if req.GraphName == "" {
		req.GraphName = getDefaultGraph()
	}

	// Use context deadline for timeout if not explicitly set
	if req.Timeout == 0 {
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining > 0 {
				req.Timeout = int32(remaining.Seconds())
			}
		}
	}

	// Fall back to client default timeout if still not set
	if req.Timeout == 0 {
		req.Timeout = int32(getTimeout())
	}

	return req, nil
}

// convertGqlResponse converts a proto response to a Response.
func (s *QueryService) convertGqlResponse(resp *pb.GqlResponse) (*Response, error) {
	rows := make([]*Row, len(resp.Rows))
	for i, pbRow := range resp.Rows {
		values := make([]*TypedValue, len(pbRow.Values))
		for j, pbVal := range pbRow.Values {
			values[j] = &TypedValue{
				Type:   PropertyType(pbVal.Type),
				Data:   pbVal.Data,
				IsNull: pbVal.IsNull,
			}
		}
		rows[i] = &Row{Values: values}
	}

	return &Response{
		Columns:      resp.Columns,
		Rows:         rows,
		RowCount:     resp.RowCount,
		HasMore:      resp.HasMore,
		Warnings:     resp.Warnings,
		RowsAffected: resp.RowsAffected,
	}, nil
}
