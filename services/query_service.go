package services

import (
	"context"
	"io"
	"regexp"
	"strings"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// useGraphRE matches a SINGLE `USE GRAPH <ident>` statement (case-insensitive)
// with optional trailing whitespace and semicolons. Compound queries do not
// match — see comment in (*QueryService).Gql below.
var useGraphRE = regexp.MustCompile(`(?i)^\s*USE\s+GRAPH\s+(\S+?)\s*;*\s*$`)

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
	// CurrentGraph is the session's current graph after this RPC
	// executed, as authoritatively reported by the server. Always
	// populated on success against new servers (covers single/compound
	// USE GRAPH at any position, last-write-wins). Empty when running
	// against a pre-fix server, which the driver detects to fall back
	// to its USE GRAPH text-parsing path.
	CurrentGraph string
	// Server-side timing (nanoseconds), read from the engine's
	// ResultSet. Network / client-side time is NOT included. Old
	// servers omit these proto3 fields → 0 means "not reported", not
	// "took zero time". Streaming queries populate only on the final
	// batch (HasMore=false) — matches CurrentGraph / RowsAffected.
	TimeCostNs    int64
	DiskCostNs    int64
	ComputeCostNs int64
	// DmlStats holds per-category data-modification counts for a DML query
	// (INSERT / SET / REMOVE / DELETE / MERGE). nil for a pure read or a
	// pre-DmlStats server — treat "absent" as "not a data-modifying query",
	// NOT as "changed nothing". RowsAffected stays the sum across categories.
	// Streaming queries populate only on the final batch (HasMore=false),
	// matching CurrentGraph / RowsAffected / the timing trio.
	DmlStats *DmlStats
}

// DmlStats reports per-category data-modification counts, mirroring the
// server's DmlStats proto message (and the engine ResultSet's DMLStats).
// Each field counts that op category; RowsAffected is their sum. A nil
// *DmlStats means "not a data-modifying query" (pure read or a pre-DmlStats
// server), NOT "changed nothing".
type DmlStats struct {
	InsertedNodes int64
	InsertedEdges int64
	DeletedNodes  int64
	DeletedEdges  int64
	SetNodes      int64
	SetEdges      int64
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
//
// Falls back to GqlStream + client-side aggregation when the server
// rejects the result set as too large for non-streaming RPC
// (RESOURCE_EXHAUSTED with "use streaming API" detail).
func (s *QueryService) Gql(ctx context.Context, query string, config *QueryConfig,
	newParameter func(name string, value interface{}) (*Parameter, error),
	getDefaultGraph func() string, getTimeout func() int) (*Response, error) {
	return s.GqlVia(ctx, s.ctx.QueryClient, query, config, newParameter, getDefaultGraph, getTimeout)
}

// GqlVia executes a GQL query against an EXPLICIT QueryServiceClient — identical to Gql except the
// caller chooses the gRPC client. Used by HA follower-read routing to run a read on a follower's
// connection (design §12); the too-large-result streaming fallback still uses the service's default
// (leader-pinned) streaming path.
func (s *QueryService) GqlVia(ctx context.Context, qc pb.QueryServiceClient, query string, config *QueryConfig,
	newParameter func(name string, value interface{}) (*Parameter, error),
	getDefaultGraph func() string, getTimeout func() int) (*Response, error) {

	ctx = s.ctx.WithSessionMetadata(ctx)

	req, err := s.buildGqlRequest(ctx, query, config, newParameter, getDefaultGraph, getTimeout)
	if err != nil {
		return nil, err
	}

	resp, err := qc.Gql(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok &&
			st.Code() == codes.ResourceExhausted &&
			strings.Contains(st.Message(), "use streaming API") {
			return s.gqlStreamCollect(ctx, query, config, newParameter, getDefaultGraph, getTimeout)
		}
		return nil, err
	}

	s.ctx.UpdateActivity()

	// Phase 2 dual-source cache update: prefer the server's
	// authoritative `current_graph` (covers compound queries, multiple
	// embedded `USE GRAPH`, and last-write-wins) and fall back to the
	// strict client-side regex for older servers that don't populate
	// the field. The regex stays in place until the minimum-supported
	// server version bumps to one that always populates current_graph
	// (Phase 3, regex deleted at next major).
	//
	// Per the server contract, an empty `current_graph` against a
	// req.GraphName-only query (no session in flight) is also valid —
	// e.g. unauthenticated/no-RBAC mode where the engine has no
	// session-scoped notion of "current graph". In that case the regex
	// path still does the right thing.
	if resp.CurrentGraph != "" {
		s.ctx.SetDefaultGraph(resp.CurrentGraph)
	} else if m := useGraphRE.FindStringSubmatch(query); m != nil {
		// Compound queries that merely *start* with `USE GRAPH`
		// (e.g. `USE GRAPH g1; SHOW EDGE_ID STATUS`) must NOT poison
		// the default-graph state — the previous prefix-match logic
		// captured everything after `USE GRAPH` (mid-string semicolons
		// / newlines kept), wrote that as the graph name, and broke
		// every subsequent gql() call with INVALID_ARGUMENT once the
		// poisoned name was sent in the gRPC `graph_name` field.
		s.ctx.SetDefaultGraph(m[1])
	} else {
		// Round-21 #5: a successful DROP GRAPH X must clear our cached
		// default_graph if it pointed at X, otherwise the next RPC keeps
		// sending graph_name='X' on the wire, the server's
		// ValidateGraphExists check fails, and unrelated statements
		// surface a confusing NOT_FOUND. New servers handle this
		// authoritatively via current_graph (returns "" after self-drop)
		// — this branch only fires on older servers without the field.
		trimmed := strings.TrimSpace(query)
		if strings.HasPrefix(strings.ToUpper(trimmed), "DROP GRAPH") {
			rest := strings.TrimSpace(trimmed[len("DROP GRAPH"):])
			if strings.HasPrefix(strings.ToUpper(rest), "IF EXISTS") {
				rest = strings.TrimSpace(rest[len("IF EXISTS"):])
			}
			dropped := strings.Trim(strings.TrimSpace(strings.TrimRight(rest, ";")), "`\"'")
			current := s.ctx.GetDefaultGraph()
			if dropped != "" && current != "" && dropped == current {
				s.ctx.SetDefaultGraph("")
			}
		}
	}

	return s.convertGqlResponse(resp)
}

// gqlStreamCollect runs GqlStream and aggregates chunks into a single Response.
// Used as fallback from Gql when the server rejects the result set as too large.
func (s *QueryService) gqlStreamCollect(ctx context.Context, query string, config *QueryConfig,
	newParameter func(name string, value interface{}) (*Parameter, error),
	getDefaultGraph func() string, getTimeout func() int) (*Response, error) {

	merged := &Response{}
	first := true
	err := s.GqlStream(ctx, query, config, func(chunk *Response) error {
		if first {
			merged.Columns = chunk.Columns
			first = false
		}
		merged.Rows = append(merged.Rows, chunk.Rows...)
		merged.Warnings = append(merged.Warnings, chunk.Warnings...)
		if chunk.RowsAffected != 0 {
			merged.RowsAffected = chunk.RowsAffected
		}
		// Server populates timing / current_graph only on the final
		// batch (has_more=false); take the latest non-zero value.
		if chunk.CurrentGraph != "" {
			merged.CurrentGraph = chunk.CurrentGraph
		}
		if chunk.TimeCostNs != 0 {
			merged.TimeCostNs = chunk.TimeCostNs
		}
		if chunk.DiskCostNs != 0 {
			merged.DiskCostNs = chunk.DiskCostNs
		}
		if chunk.ComputeCostNs != 0 {
			merged.ComputeCostNs = chunk.ComputeCostNs
		}
		// DmlStats arrives only on the final batch (has_more=false); take
		// the latest non-nil value (nil means the chunk carried no stats).
		if chunk.DmlStats != nil {
			merged.DmlStats = chunk.DmlStats
		}
		return nil
	}, newParameter, getDefaultGraph, getTimeout)
	if err != nil {
		return nil, err
	}
	merged.RowCount = int64(len(merged.Rows))
	return merged, nil
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
		Columns:       resp.Columns,
		Rows:          rows,
		RowCount:      resp.RowCount,
		HasMore:       resp.HasMore,
		Warnings:      resp.Warnings,
		RowsAffected:  resp.RowsAffected,
		CurrentGraph:  resp.CurrentGraph,
		TimeCostNs:    resp.TimeCostNs,
		DiskCostNs:    resp.DiskCostNs,
		ComputeCostNs: resp.ComputeCostNs,
		DmlStats:      convertDmlStats(resp.GetDmlStats()),
	}, nil
}

// convertDmlStats maps the proto DmlStats sub-message onto the package-level
// DmlStats. Returns nil when the sub-message is absent (pure read / old
// server) or all-zero, mirroring the server which omits the message for a
// non-data-modifying query — so callers can treat "absent" as "not a DML
// query", not "changed nothing".
func convertDmlStats(raw *pb.DmlStats) *DmlStats {
	if raw == nil {
		return nil
	}
	stats := &DmlStats{
		InsertedNodes: raw.GetInsertedNodes(),
		InsertedEdges: raw.GetInsertedEdges(),
		DeletedNodes:  raw.GetDeletedNodes(),
		DeletedEdges:  raw.GetDeletedEdges(),
		SetNodes:      raw.GetSetNodes(),
		SetEdges:      raw.GetSetEdges(),
	}
	total := stats.InsertedNodes + stats.InsertedEdges +
		stats.DeletedNodes + stats.DeletedEdges +
		stats.SetNodes + stats.SetEdges
	if total == 0 {
		return nil
	}
	return stats
}
