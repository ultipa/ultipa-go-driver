package gqldb

import (
	"context"
	cryptoRand "crypto/rand"
	"fmt"
	"strings"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"github.com/ultipa/ultipa-go-driver/v6/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// insertTypeToProto maps the public types.InsertType enum to the
// proto-layer pb.InsertMode value. The two enums have aligned integer
// values; this function exists to make the boundary explicit and to
// default unknown values to NORMAL rather than silently round-tripping
// out-of-range integers.
func insertTypeToProto(t InsertType) pb.InsertMode {
	switch t {
	case InsertTypeOverwrite:
		return pb.InsertMode_INSERT_MODE_OVERWRITE
	case InsertTypeUpsert:
		return pb.InsertMode_INSERT_MODE_UPSERT
	default:
		return pb.InsertMode_INSERT_MODE_NORMAL
	}
}

// Client is the main entry point for interacting with GQLDB.
type Client struct {
	config    *Config
	pool      *ConnectionPool
	sessions  *SessionManager
	txManager *TransactionManager

	// Stored credentials for auto-reconnect
	storedUsername string
	storedPassword string
	storedGraph   string

	// Stable per-client logical session id surfaced under the
	// transaction-branch model (see TRANSACTIONS_DRIVER_GUIDE.md §2.0–2.1).
	// Initialized to a UUID v4 hex; users can override via Config.SessionID.
	clientSessionID string

	// gRPC service clients
	sessionClient     pb.SessionServiceClient
	queryClient       pb.QueryServiceClient
	dataClient        pb.DataServiceClient
	graphClient       pb.GraphServiceClient
	transactionClient pb.TransactionServiceClient
	healthClient      pb.HealthClient
	adminClient       pb.AdminServiceClient
	bulkImportClient  pb.BulkImportServiceClient

	// Service layer (delegation)
	sessionSvc     *services.SessionService
	querySvc       *services.QueryService
	graphSvc       *services.GraphService
	transactionSvc *services.TransactionService
	dataSvc        *services.DataService
	healthSvc      *services.HealthService
	adminSvc       *services.AdminService
	bulkImportSvc  *services.BulkImportService
}

// NewClient creates a new GQLDB client.
func NewClient(config *Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	pool, err := NewConnectionPool(config)
	if err != nil {
		return nil, err
	}

	csid := config.SessionID
	if csid == "" {
		csid = newClientSessionID()
	}

	client := &Client{
		config:          config,
		pool:            pool,
		sessions:        NewSessionManager(),
		txManager:       NewTransactionManager(),
		clientSessionID: csid,
	}

	// Initialize gRPC clients and services
	conn, err := client.getConn()
	if err != nil {
		return nil, err
	}
	client.initClients(conn)

	return client, nil
}

// getConn returns a gRPC connection from the pool.
func (c *Client) getConn() (*grpc.ClientConn, error) {
	return c.pool.GetConnection()
}

// initClients initializes the gRPC service clients.
func (c *Client) initClients(conn *grpc.ClientConn) {
	c.sessionClient = pb.NewSessionServiceClient(conn)
	c.queryClient = pb.NewQueryServiceClient(conn)
	c.dataClient = pb.NewDataServiceClient(conn)
	c.graphClient = pb.NewGraphServiceClient(conn)
	c.transactionClient = pb.NewTransactionServiceClient(conn)
	c.healthClient = pb.NewHealthClient(conn)
	c.adminClient = pb.NewAdminServiceClient(conn)
	c.bulkImportClient = pb.NewBulkImportServiceClient(conn)

	// Initialize service context
	ctx := &services.ServiceContext{
		SessionClient:     c.sessionClient,
		QueryClient:       c.queryClient,
		DataClient:        c.dataClient,
		GraphClient:       c.graphClient,
		TransactionClient: c.transactionClient,
		HealthClient:      c.healthClient,
		AdminClient:       c.adminClient,
		BulkImportClient:  c.bulkImportClient,
		GetSessionID:        func() uint64 { return c.sessions.GetSessionID() },
		GetServerVersion:    func() string { return c.sessions.GetServerVersion() },
		GetClientSessionID:  func() string { return c.clientSessionID },
		GetDefaultGraph:     func() string { return c.sessions.GetDefaultGraph() },
		GetTimeout:        func() int { return c.config.TimeoutSeconds() },
		SetDefaultGraph:   func(name string) { c.sessions.SetDefaultGraph(name) },
		UpdateActivity:    func() { c.sessions.UpdateActivity() },
		IsLoggedIn:        func() bool { return c.sessions.IsLoggedIn() },
	}

	// Initialize all services
	c.sessionSvc = services.NewSessionService(ctx)
	c.querySvc = services.NewQueryService(ctx)
	c.graphSvc = services.NewGraphService(ctx)
	c.transactionSvc = services.NewTransactionService(ctx)
	c.dataSvc = services.NewDataService(ctx)
	c.healthSvc = services.NewHealthService(ctx)
	c.adminSvc = services.NewAdminService(ctx)
	c.bulkImportSvc = services.NewBulkImportService(ctx)
}

// Close closes the client and all connections.
func (c *Client) Close() error {
	if c.sessions.IsLoggedIn() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c.Logout(ctx)
	}
	return c.pool.Close()
}

// =============================================================================
// Session Service - Delegates to SessionService
// =============================================================================

// Connect initializes the gRPC service clients without authentication.
func (c *Client) Connect(ctx context.Context) error {
	conn, err := c.getConn()
	if err != nil {
		return err
	}
	c.initClients(conn)
	return nil
}

// Login authenticates with the database and creates a session.
func (c *Client) Login(ctx context.Context, username, password string) (*Session, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}
	c.initClients(conn)

	defaultGraph := c.config.DefaultGraph
	svcSession, err := c.sessionSvc.Login(ctx, username, password, defaultGraph)
	if err != nil {
		return nil, NewError(0, "login failed", err)
	}

	// Store credentials for auto-reconnect
	c.storedUsername = username
	c.storedPassword = password

	// Register session with SessionManager
	session := c.sessions.Login(ctx, svcSession.ID, svcSession.ServerVersion, svcSession.Roles, svcSession.DefaultGraph, &ClusterInfo{
		IsCluster:      svcSession.IsCluster,
		ClusterID:      svcSession.ClusterID,
		PartitionCount: svcSession.PartitionCount,
	})
	return session, nil
}

// withAutoReconnect executes fn and, if the error is UNAUTHENTICATED,
// re-logs in with stored credentials and retries fn exactly once.
func (c *Client) withAutoReconnect(ctx context.Context, fn func() error) error {
	err := fn()
	if err == nil {
		return nil
	}

	// Only auto-reconnect if we have stored credentials
	if c.storedUsername == "" {
		return err
	}

	// Check if error is UNAUTHENTICATED (gRPC status code or message text)
	needsReconnect := false
	if s, ok := status.FromError(err); ok && s.Code() == codes.Unauthenticated {
		needsReconnect = true
	}
	if !needsReconnect {
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "session not found") || strings.Contains(errMsg, "session expired") {
			needsReconnect = true
		}
	}

	if needsReconnect {
		// Save current graph context before re-login
		savedGraph := c.storedGraph

		// Re-login with stored credentials
		_, loginErr := c.Login(ctx, c.storedUsername, c.storedPassword)
		if loginErr != nil {
			return err // Return original error if re-login fails
		}

		// Restore graph context after re-login
		if savedGraph != "" && savedGraph != "__system__" {
			_ = c.UseGraph(ctx, savedGraph)
		}

		// Retry the call once
		return fn()
	}

	return err
}

// Logout closes the current session.
func (c *Client) Logout(ctx context.Context) error {
	if !c.sessions.IsLoggedIn() {
		return nil
	}

	err := c.sessionSvc.Logout(ctx)
	if err != nil {
		return NewError(0, "logout failed", err)
	}

	c.sessions.Logout()
	return nil
}

// Ping sends a ping to the server and returns the latency in nanoseconds.
func (c *Client) Ping(ctx context.Context) (int64, error) {
	latency, err := c.sessionSvc.Ping(ctx)
	if err != nil {
		return 0, NewError(0, "ping failed", err)
	}
	return latency, nil
}

// =============================================================================
// Query Service - Delegates to QueryService
// =============================================================================

// Gql executes a GQL query and returns the results.
func (c *Client) Gql(ctx context.Context, query string, config *QueryConfig) (*Response, error) {
	if query == "" {
		return nil, ErrEmptyQuery
	}

	var resp *Response
	err := c.withAutoReconnect(ctx, func() error {
		svcConfig := c.convertToServiceQueryConfig(config)
		svcResp, err := c.querySvc.Gql(ctx, query, svcConfig, c.newParameterAdapter,
			c.sessions.GetDefaultGraph, func() int { return c.config.TimeoutSeconds() })
		if err != nil {
			return err
		}
		resp = c.convertFromServiceResponse(svcResp)
		return nil
	})
	if err != nil {
		return nil, NewError(0, "query failed", err)
	}

	return resp, nil
}

// GqlStream executes a GQL query and streams the results.
func (c *Client) GqlStream(ctx context.Context, query string, config *QueryConfig, callback func(*Response) error) error {
	if query == "" {
		return ErrEmptyQuery
	}

	err := c.withAutoReconnect(ctx, func() error {
		svcConfig := c.convertToServiceQueryConfig(config)
		return c.querySvc.GqlStream(ctx, query, svcConfig, func(svcResp *services.Response) error {
			resp := c.convertFromServiceResponse(svcResp)
			return callback(resp)
		}, c.newParameterAdapter, c.sessions.GetDefaultGraph, func() int { return c.config.TimeoutSeconds() })
	})

	if err != nil {
		return NewError(0, "stream query failed", err)
	}
	return nil
}

// Explain returns the execution plan for a query.
func (c *Client) Explain(ctx context.Context, query string, config *QueryConfig) (string, error) {
	if query == "" {
		return "", ErrEmptyQuery
	}

	var plan string
	err := c.withAutoReconnect(ctx, func() error {
		svcConfig := c.convertToServiceQueryConfig(config)
		var e error
		plan, e = c.querySvc.Explain(ctx, query, svcConfig, c.newParameterAdapter,
			c.sessions.GetDefaultGraph, func() int { return c.config.TimeoutSeconds() })
		return e
	})
	if err != nil {
		return "", NewError(0, "explain failed", err)
	}
	return plan, nil
}

// Profile executes a query with profiling and returns statistics.
func (c *Client) Profile(ctx context.Context, query string, config *QueryConfig) (string, error) {
	if query == "" {
		return "", ErrEmptyQuery
	}

	var profile string
	err := c.withAutoReconnect(ctx, func() error {
		svcConfig := c.convertToServiceQueryConfig(config)
		var e error
		profile, e = c.querySvc.Profile(ctx, query, svcConfig, c.newParameterAdapter,
			c.sessions.GetDefaultGraph, func() int { return c.config.TimeoutSeconds() })
		return e
	})
	if err != nil {
		return "", NewError(0, "profile failed", err)
	}
	return profile, nil
}

// =============================================================================
// Graph Service - Delegates to GraphService
// =============================================================================

// CreateGraph creates a new graph.
func (c *Client) CreateGraph(ctx context.Context, name string, graphType GraphType, description string) error {
	success, message, err := c.graphSvc.CreateGraph(ctx, name, graphType, description)
	if err != nil {
		return NewError(0, "create graph failed", err)
	}
	if !success {
		return NewError(0, message, nil)
	}
	return nil
}

// DropGraph deletes a graph.
func (c *Client) DropGraph(ctx context.Context, name string, ifExists bool) error {
	success, message, err := c.graphSvc.DropGraph(ctx, name, ifExists)
	if err != nil {
		return NewError(0, "drop graph failed", err)
	}
	if !success {
		return NewError(0, message, nil)
	}
	return nil
}

// UseGraph sets the current graph for the session.
func (c *Client) UseGraph(ctx context.Context, name string) error {
	success, message, err := c.graphSvc.UseGraph(ctx, name)
	if err != nil {
		return NewError(0, "use graph failed", err)
	}
	if !success {
		return NewError(0, message, nil)
	}
	c.storedGraph = name
	return nil
}

// ListGraphs returns all available graphs.
func (c *Client) ListGraphs(ctx context.Context) ([]*GraphInfo, error) {
	svcGraphs, err := c.graphSvc.ListGraphs(ctx)
	if err != nil {
		return nil, NewError(0, "list graphs failed", err)
	}

	graphs := make([]*GraphInfo, len(svcGraphs))
	for i, g := range svcGraphs {
		graphs[i] = &GraphInfo{
			Name:        g.Name,
			GraphType:   g.GraphType,
			NodeCount:   g.NodeCount,
			EdgeCount:   g.EdgeCount,
			Description: g.Description,
		}
	}
	return graphs, nil
}

// GetGraphInfo returns information about a specific graph.
func (c *Client) GetGraphInfo(ctx context.Context, name string) (*GraphInfo, error) {
	info, err := c.graphSvc.GetGraphInfo(ctx, name)
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.NotFound {
			return nil, ErrGraphNotFound
		}
		return nil, NewError(0, "get graph info failed", err)
	}
	if info == nil {
		return nil, ErrGraphNotFound
	}

	return &GraphInfo{
		Name:        info.Name,
		GraphType:   info.GraphType,
		NodeCount:   info.NodeCount,
		EdgeCount:   info.EdgeCount,
		Description: info.Description,
	}, nil
}

// =============================================================================
// Transaction Service - Delegates to TransactionService
// =============================================================================

// BeginTransaction starts a new transaction.
func (c *Client) BeginTransaction(ctx context.Context, graphName string, readOnly bool, timeout int) (*Transaction, error) {
	result, err := c.transactionSvc.BeginTransaction(ctx, graphName, readOnly, timeout)
	if err != nil {
		return nil, NewError(0, "begin transaction failed", err)
	}

	// Create transaction using transaction manager
	tx := c.txManager.Begin(result.TransactionID, result.SessionID, result.GraphName, result.ReadOnly, result.Timeout)
	// Surface the per-client logical session id on the returned tx
	// (transaction-branch ergonomic, see TRANSACTIONS_DRIVER_GUIDE.md).
	tx.ClientSessionID = c.clientSessionID
	return tx, nil
}

// ClientSessionID returns the stable per-client logical session id surfaced
// under the transaction-branch model. The same id is attached to every
// Transaction returned by BeginTransaction.
func (c *Client) ClientSessionID() string {
	return c.clientSessionID
}

// newClientSessionID generates a UUID v4 hex string for use as the
// per-client logical session id. Used at NewClient time when Config.SessionID
// is not provided. Falls back to time-based hex if crypto/rand fails (very
// unlikely; the fallback only ensures NewClient never errors on this).
func newClientSessionID() string {
	b := make([]byte, 16)
	if _, err := cryptoRand.Read(b); err != nil {
		// Extremely unlikely; emit a non-cryptographic fallback rather than
		// fail NewClient. Used only as the default when the user did not set
		// Config.SessionID.
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	// RFC 4122 v4 markers — actual values not strictly required for our
	// purposes (we use it as an opaque string), but make it look like a UUID.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x", b)
}

// Commit commits a transaction.
func (c *Client) Commit(ctx context.Context, transactionID uint64) (bool, error) {
	success, err := c.transactionSvc.Commit(ctx, transactionID)
	if err != nil {
		// Clean up local state even if server commit fails
		_ = c.txManager.Rollback(transactionID)
		return false, NewError(0, "commit failed", err)
	}

	// Mark transaction as committed in transaction manager
	c.txManager.Commit(transactionID)
	return success, nil
}

// Rollback aborts a transaction.
func (c *Client) Rollback(ctx context.Context, transactionID uint64) (bool, error) {
	success, err := c.transactionSvc.Rollback(ctx, transactionID)
	if err != nil {
		// Always clean up local state even if server rollback fails
		_ = c.txManager.Rollback(transactionID)
		return false, NewError(0, "rollback failed", err)
	}

	// Mark transaction as rolled back in transaction manager
	c.txManager.Rollback(transactionID)
	return success, nil
}

// ListTransactions returns active transactions.
func (c *Client) ListTransactions(ctx context.Context) ([]*TransactionInfo, error) {
	svcTxs, err := c.transactionSvc.ListTransactions(ctx)
	if err != nil {
		return nil, NewError(0, "list transactions failed", err)
	}

	txs := make([]*TransactionInfo, len(svcTxs))
	for i, t := range svcTxs {
		txs[i] = &TransactionInfo{
			TransactionID: t.TransactionID,
			SessionID:     t.SessionID,
			GraphName:     t.GraphName,
			ReadOnly:      t.ReadOnly,
			CreatedAt:     t.CreatedAt,
			DurationMs:    t.DurationMs,
			InternalTxID:  t.InternalTxID,
		}
	}
	return txs, nil
}

// WithTransaction executes a function within a transaction.
func (c *Client) WithTransaction(ctx context.Context, graphName string, readOnly bool, fn func(txID uint64) error) error {
	tx, err := c.BeginTransaction(ctx, graphName, readOnly, c.config.TimeoutSeconds())
	if err != nil {
		return err
	}

	if err := fn(tx.ID); err != nil {
		c.Rollback(ctx, tx.ID)
		return err
	}

	_, err = c.Commit(ctx, tx.ID)
	return err
}

// =============================================================================
// Admin DDL surface (transaction-branch)
// =============================================================================

// ShowTransactions returns active transactions via the GQL admin DDL
// `SHOW TRANSACTIONS`. Each row mirrors the 5-column server-side schema:
// TransactionID / Status / ReadOnly / StartTime / SessionID.
//
// SessionID is empty unless the server has auto-derived one from peer info
// or the driver explicitly surfaces `x-ultipa-session-id` metadata.
//
// Distinct from ListTransactions which uses the legacy gRPC.
func (c *Client) ShowTransactions(ctx context.Context) ([]*TransactionRow, error) {
	resp, err := c.Gql(ctx, "SHOW TRANSACTIONS", nil)
	if err != nil {
		return nil, err
	}
	rows := make([]*TransactionRow, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		v, _ := resp.GetByName(row, "transaction_id")
		txID, _ := v.(string)
		v, _ = resp.GetByName(row, "status")
		stat, _ := v.(string)
		v, _ = resp.GetByName(row, "read_only")
		ro, _ := v.(bool)
		v, _ = resp.GetByName(row, "start_time")
		st, _ := v.(string)
		v, _ = resp.GetByName(row, "session_id")
		sid, _ := v.(string)
		rows = append(rows, &TransactionRow{
			TransactionID: txID,
			Status:        stat,
			ReadOnly:      ro,
			StartTime:     st,
			SessionID:     sid,
		})
	}
	return rows, nil
}

// KillTransaction rolls back a single transaction by id via
// `KILL TRANSACTION '<id>'`. The id is the string form surfaced by
// ShowTransactions (e.g. "tx_a396c531-..."), distinct from the uint64
// returned by BeginTransaction.
func (c *Client) KillTransaction(ctx context.Context, transactionID string) (*Response, error) {
	escaped := strings.ReplaceAll(transactionID, "'", "''")
	return c.Gql(ctx, "KILL TRANSACTION '"+escaped+"'", nil)
}

// ResetTransactions rolls back every active transaction via
// `RESET TRANSACTIONS`. Admin-only; intended as an escape hatch when an
// orphan tx blocks new BEGINs.
func (c *Client) ResetTransactions(ctx context.Context) (*Response, error) {
	return c.Gql(ctx, "RESET TRANSACTIONS", nil)
}

// =============================================================================
// Data Service - Delegates to DataService
// =============================================================================

// InsertNodes inserts multiple nodes into a graph via the gRPC bulk-import RPC.
// This is the original 6.0.0 signature; preserved so callers written against
// 6.0.0 (e.g. gqldb-manager) keep compiling and running.
//
// For the GQL-emitter convenience helper added after 6.0.0, use InsertNodesGql.
func (c *Client) InsertNodes(ctx context.Context, graphName string, nodes []*NodeData, config *InsertNodesConfig) (*InsertNodesResult, error) {
	svcNodes := make([]*services.NodeData, len(nodes))
	for i, n := range nodes {
		svcNodes[i] = &services.NodeData{
			ID:         n.ID,
			Labels:     n.Labels,
			Properties: n.Properties,
		}
	}

	// Convert config (create default if nil to avoid deprecated bulk operations error)
	svcConfig := &services.InsertNodesConfig{}
	if config != nil {
		svcConfig.Mode = insertTypeToProto(config.Mode)
		svcConfig.BulkImportSessionID = config.BulkImportSessionID
	}

	result, err := c.dataSvc.InsertNodes(ctx, graphName, svcNodes, svcConfig, c.convertProperties)
	if err != nil {
		return nil, NewError(0, "insert nodes failed", err)
	}

	return &InsertNodesResult{
		Success:   result.Success,
		NodeIDs:   result.NodeIDs,
		NodeCount: result.NodesCreated,
		Message:   result.Message,
	}, nil
}

// InsertEdges inserts multiple edges into a graph via the gRPC bulk-import RPC.
// This is the original 6.0.0 signature; preserved so callers written against
// 6.0.0 keep compiling and running.
//
// For the GQL-emitter convenience helper added after 6.0.0, use InsertEdgesGql.
func (c *Client) InsertEdges(ctx context.Context, graphName string, edges []*EdgeData, config *InsertEdgesConfig) (*InsertEdgesResult, error) {
	svcEdges := make([]*services.EdgeData, len(edges))
	for i, e := range edges {
		svcEdges[i] = &services.EdgeData{
			ID:         e.ID,
			Label:      e.Label,
			From:       e.FromNodeID,
			To:         e.ToNodeID,
			Properties: e.Properties,
		}
	}

	// Convert config (create default if nil to avoid deprecated bulk operations error)
	svcConfig := &services.InsertEdgesConfig{}
	if config != nil {
		svcConfig.SkipInvalidNodes = config.SkipInvalidNodes
		svcConfig.Mode = insertTypeToProto(config.Mode)
		svcConfig.BulkImportSessionID = config.BulkImportSessionID
	}

	result, err := c.dataSvc.InsertEdges(ctx, graphName, svcEdges, svcConfig, c.convertProperties)
	if err != nil {
		return nil, NewError(0, "insert edges failed", err)
	}

	return &InsertEdgesResult{
		Success:   result.Success,
		EdgeCount: result.EdgesCreated,
		Message:   result.Message,
	}, nil
}

// InsertNodesBatchAuto is a deprecated alias for InsertNodes; kept for
// short-lived callers that adopted the post-6.0.0 rename. New code should
// use InsertNodes directly.
//
// Deprecated: use InsertNodes.
func (c *Client) InsertNodesBatchAuto(ctx context.Context, graphName string, nodes []*NodeData, config *InsertNodesConfig) (*InsertNodesResult, error) {
	return c.InsertNodes(ctx, graphName, nodes, config)
}

// InsertEdgesBatchAuto is a deprecated alias for InsertEdges; kept for
// short-lived callers that adopted the post-6.0.0 rename. New code should
// use InsertEdges directly.
//
// Deprecated: use InsertEdges.
func (c *Client) InsertEdgesBatchAuto(ctx context.Context, graphName string, edges []*EdgeData, config *InsertEdgesConfig) (*InsertEdgesResult, error) {
	return c.InsertEdges(ctx, graphName, edges, config)
}

// DeleteNodes deletes nodes from a graph.
func (c *Client) DeleteNodes(ctx context.Context, graphName string, nodeIDs []string, labels []string, where string) (*DeleteResult, error) {
	result, err := c.dataSvc.DeleteNodes(ctx, graphName, nodeIDs, labels, where)
	if err != nil {
		return nil, NewError(0, "delete nodes failed", err)
	}

	return &DeleteResult{
		Success:      result.Success,
		DeletedCount: result.Deleted,
		Message:      result.Message,
	}, nil
}

// DeleteEdges deletes edges from a graph.
func (c *Client) DeleteEdges(ctx context.Context, graphName string, edgeIDs []string, label string, where string) (*DeleteResult, error) {
	result, err := c.dataSvc.DeleteEdges(ctx, graphName, edgeIDs, label, where)
	if err != nil {
		return nil, NewError(0, "delete edges failed", err)
	}

	return &DeleteResult{
		Success:      result.Success,
		DeletedCount: result.Deleted,
		Message:      result.Message,
	}, nil
}

// Export exports graph data in JSON Lines format (streaming).
// The callback receives chunks of JSON Lines data. The final chunk will have IsFinal=true
// and contain export statistics.
func (c *Client) Export(ctx context.Context, config *ExportConfig, callback func(*ExportResult) error) error {
	svcConfig := &services.ExportConfig{
		GraphName:       config.GraphName,
		BatchSize:       config.BatchSize,
		ExportNodes:     config.ExportNodes,
		ExportEdges:     config.ExportEdges,
		NodeLabels:      config.NodeLabels,
		EdgeLabels:      config.EdgeLabels,
		IncludeMetadata: config.IncludeMetadata,
	}

	err := c.dataSvc.Export(ctx, svcConfig, func(chunk *services.ExportChunk) error {
		result := &ExportResult{
			Data:    chunk.Data,
			IsFinal: chunk.IsFinal,
		}

		if chunk.Stats != nil {
			result.Stats = &ExportStats{
				NodesExported: chunk.Stats.NodesExported,
				EdgesExported: chunk.Stats.EdgesExported,
				BytesWritten:  chunk.Stats.BytesWritten,
				DurationMs:    chunk.Stats.DurationMs,
			}
		}

		return callback(result)
	})

	if err != nil {
		return NewError(0, "export failed", err)
	}
	return nil
}

// =============================================================================
// Health Service - Delegates to HealthService
// =============================================================================

// HealthCheck performs a health check for a service.
func (c *Client) HealthCheck(ctx context.Context, service string) (HealthStatus, error) {
	status, err := c.healthSvc.HealthCheck(ctx, service)
	if err != nil {
		return HealthStatusUnknown, NewError(0, "health check failed", err)
	}
	return HealthStatus(status), nil
}

// Watch watches the health status of a service and returns a HealthWatcher.
func (c *Client) Watch(ctx context.Context, service string) (*HealthWatcher, error) {
	watcher := &HealthWatcher{
		Status: make(chan HealthStatus, 10),
		Done:   make(chan error, 1),
	}

	// Start watching in a goroutine
	go func() {
		err := c.healthSvc.Watch(ctx, service, func(status int32) error {
			select {
			case watcher.Status <- HealthStatus(status):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		// When streaming ends, close channels and signal done
		close(watcher.Status)
		if err != nil {
			watcher.Done <- err
		}
		close(watcher.Done)
	}()

	return watcher, nil
}

// WatchWithCallback watches health status and calls callback on changes.
func (c *Client) WatchWithCallback(ctx context.Context, service string, callback func(HealthStatus) error) error {
	err := c.healthSvc.Watch(ctx, service, func(status int32) error {
		return callback(HealthStatus(status))
	})

	if err != nil {
		return NewError(0, "watch failed", err)
	}
	return nil
}

// =============================================================================
// Admin Service - Delegates to AdminService
// =============================================================================

// WarmupParser warms up the parser cache.
func (c *Client) WarmupParser(ctx context.Context, count int) error {
	err := c.adminSvc.WarmupParser(ctx, count)
	if err != nil {
		return NewError(0, "warmup parser failed", err)
	}
	return nil
}

// GetCacheStats retrieves cache statistics.
func (c *Client) GetCacheStats(ctx context.Context, cacheType CacheType) (*CacheStats, error) {
	stats, err := c.adminSvc.GetCacheStats(ctx, int32(cacheType))
	if err != nil {
		return nil, NewError(0, "get cache stats failed", err)
	}

	result := &CacheStats{}
	if stats.ASTStats != nil {
		hitRate := float64(0)
		total := stats.ASTStats.Hits + stats.ASTStats.Misses
		if total > 0 {
			hitRate = float64(stats.ASTStats.Hits) / float64(total)
		}
		result.ASTStats = &ASTCacheStats{
			Entries:   int32(stats.ASTStats.Size),
			Hits:      uint64(stats.ASTStats.Hits),
			Misses:    uint64(stats.ASTStats.Misses),
			Evictions: uint64(stats.ASTStats.Evictions),
			HitRate:   hitRate,
		}
	}
	if stats.PlanStats != nil {
		hitRate := float64(0)
		total := stats.PlanStats.Hits + stats.PlanStats.Misses
		if total > 0 {
			hitRate = float64(stats.PlanStats.Hits) / float64(total)
		}
		result.PlanStats = &PlanCacheStats{
			Size:     int32(stats.PlanStats.Size),
			Capacity: 0, // Not available from service
			Hits:     uint64(stats.PlanStats.Hits),
			Misses:   uint64(stats.PlanStats.Misses),
			HitRate:  hitRate,
		}
	}
	return result, nil
}

// ClearCache clears the specified cache.
func (c *Client) ClearCache(ctx context.Context, cacheType CacheType) error {
	err := c.adminSvc.ClearCache(ctx, int32(cacheType))
	if err != nil {
		return NewError(0, "clear cache failed", err)
	}
	return nil
}

// GetStatistics retrieves database statistics.
func (c *Client) GetStatistics(ctx context.Context, graphName string) (*Statistics, error) {
	stats, err := c.adminSvc.GetStatistics(ctx, graphName)
	if err != nil {
		return nil, NewError(0, "get statistics failed", err)
	}

	return &Statistics{
		NodeCount:       uint64(stats.NodeCount),
		EdgeCount:       uint64(stats.EdgeCount),
		LabelCounts:     stats.LabelCounts,
		EdgeLabelCounts: stats.EdgeLabelCounts,
	}, nil
}

// InvalidatePermissionCache invalidates permission cache for a user.
func (c *Client) InvalidatePermissionCache(ctx context.Context, username string) error {
	err := c.adminSvc.InvalidatePermissionCache(ctx, username)
	if err != nil {
		return NewError(0, "invalidate permission cache failed", err)
	}
	return nil
}

// CompactResult represents the result of Compact operation.
type CompactResult struct {
	Success bool   // true if compaction was successful
	Message string // additional status message
}

// Compact triggers manual compaction of the database storage.
func (c *Client) Compact(ctx context.Context) (*CompactResult, error) {
	result, err := c.adminSvc.Compact(ctx)
	if err != nil {
		return nil, NewError(0, "compact failed", err)
	}
	return &CompactResult{
		Success: result.Success,
		Message: result.Message,
	}, nil
}

// ComputeTopologyResult represents the result of WaitForComputeTopology.
type ComputeTopologyResult struct {
	Ready   bool   // true if topology is ready
	Message string // additional status message
}

// WaitForComputeTopology waits for the computing engine topology to be ready.
// graphName: the graph to check (required)
// timeout: timeout duration (0 = check current status only)
func (c *Client) WaitForComputeTopology(ctx context.Context, graphName string, timeout time.Duration) (*ComputeTopologyResult, error) {
	timeoutMs := timeout.Milliseconds()
	result, err := c.adminSvc.WaitForComputeTopology(ctx, graphName, timeoutMs)
	if err != nil {
		return nil, NewError(0, "wait for compute topology failed", err)
	}

	return &ComputeTopologyResult{
		Ready:   result.Ready,
		Message: result.Message,
	}, nil
}

// CpuMetrics contains CPU usage metrics.
type CpuMetrics struct {
	ProcessPercent float64
	SystemPercent  float64
	NumCores       int32
}

// MemoryMetrics contains memory usage metrics.
type MemoryMetrics struct {
	ProcessRss        uint64
	HeapAlloc         uint64
	HeapSys           uint64
	StackInUse        uint64
	SystemTotal       uint64
	SystemAvailable   uint64
	SystemUsed        uint64
	SystemUsedPercent float64
}

// DiskIOMetrics contains disk I/O metrics.
type DiskIOMetrics struct {
	ReadBytes  uint64
	WriteBytes uint64
	ReadCount  uint64
	WriteCount uint64
}

// StorageMetrics contains storage metrics.
type StorageMetrics struct {
	DbPath      string
	DbSizeBytes uint64
	VolumeTotal uint64
	VolumeFree  uint64
	VolumeUsed  uint64
}

// NetworkMetrics contains network metrics.
type NetworkMetrics struct {
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
}

// SystemMetrics contains system-level metrics.
type SystemMetrics struct {
	Cpu     *CpuMetrics
	Memory  *MemoryMetrics
	DiskIO  *DiskIOMetrics
	Storage *StorageMetrics
	Network *NetworkMetrics
}

// GetSystemMetrics returns system-level metrics (CPU, memory, disk I/O, storage, network).
func (c *Client) GetSystemMetrics(ctx context.Context) (*SystemMetrics, error) {
	result, err := c.adminSvc.GetSystemMetrics(ctx)
	if err != nil {
		return nil, NewError(0, "get system metrics failed", err)
	}

	metrics := &SystemMetrics{}
	if result.Cpu != nil {
		metrics.Cpu = &CpuMetrics{
			ProcessPercent: result.Cpu.ProcessPercent,
			SystemPercent:  result.Cpu.SystemPercent,
			NumCores:       result.Cpu.NumCores,
		}
	}
	if result.Memory != nil {
		metrics.Memory = &MemoryMetrics{
			ProcessRss:        result.Memory.ProcessRss,
			HeapAlloc:         result.Memory.HeapAlloc,
			HeapSys:           result.Memory.HeapSys,
			StackInUse:        result.Memory.StackInUse,
			SystemTotal:       result.Memory.SystemTotal,
			SystemAvailable:   result.Memory.SystemAvailable,
			SystemUsed:        result.Memory.SystemUsed,
			SystemUsedPercent: result.Memory.SystemUsedPercent,
		}
	}
	if result.DiskIO != nil {
		metrics.DiskIO = &DiskIOMetrics{
			ReadBytes:  result.DiskIO.ReadBytes,
			WriteBytes: result.DiskIO.WriteBytes,
			ReadCount:  result.DiskIO.ReadCount,
			WriteCount: result.DiskIO.WriteCount,
		}
	}
	if result.Storage != nil {
		metrics.Storage = &StorageMetrics{
			DbPath:      result.Storage.DbPath,
			DbSizeBytes: result.Storage.DbSizeBytes,
			VolumeTotal: result.Storage.VolumeTotal,
			VolumeFree:  result.Storage.VolumeFree,
			VolumeUsed:  result.Storage.VolumeUsed,
		}
	}
	if result.Network != nil {
		metrics.Network = &NetworkMetrics{
			BytesSent:   result.Network.BytesSent,
			BytesRecv:   result.Network.BytesRecv,
			PacketsSent: result.Network.PacketsSent,
			PacketsRecv: result.Network.PacketsRecv,
		}
	}

	return metrics, nil
}

// =============================================================================
// Bulk Import Service - Delegates to BulkImportService
// =============================================================================

// StartBulkImport starts a bulk import session.
func (c *Client) StartBulkImport(ctx context.Context, graphName string, opts *BulkImportOptions) (*BulkImportSession, error) {
	svcOpts := (*services.BulkImportOptions)(nil)
	if opts != nil {
		svcOpts = &services.BulkImportOptions{
			EstimatedNodes: opts.EstimatedNodes,
			EstimatedEdges: opts.EstimatedEdges,
		}
	}

	session, err := c.bulkImportSvc.StartBulkImport(ctx, graphName, svcOpts)
	if err != nil {
		return nil, NewError(0, "start bulk import failed", err)
	}

	return &BulkImportSession{
		SessionID: session.SessionID,
		Success:   true,
		Message:   session.Status,
	}, nil
}

// Checkpoint is deprecated. Server checkpoint is now a no-op; use EndBulkImport which performs a final flush.
// Deprecated: This method will be removed in a future version.
func (c *Client) Checkpoint(ctx context.Context, sessionID string) (*CheckpointResult, error) {
	return &CheckpointResult{
		Success: true,
		Message: "Checkpoint has been removed; use EndBulkImport which performs a final flush",
	}, nil
}

// EndBulkImport ends a bulk import session.
func (c *Client) EndBulkImport(ctx context.Context, sessionID string) (*EndBulkImportResult, error) {
	result, err := c.bulkImportSvc.EndBulkImport(ctx, sessionID)
	if err != nil {
		return nil, NewError(0, "end bulk import failed", err)
	}

	return &EndBulkImportResult{
		Success:      result.Success,
		TotalRecords: result.NodesImported + result.EdgesImported,
		Message:      result.Message,
	}, nil
}

// AbortBulkImport aborts a bulk import session.
func (c *Client) AbortBulkImport(ctx context.Context, sessionID string) (*AbortBulkImportResult, error) {
	err := c.bulkImportSvc.AbortBulkImport(ctx, sessionID)
	if err != nil {
		return nil, NewError(0, "abort bulk import failed", err)
	}

	return &AbortBulkImportResult{
		Success: true,
		Message: "Bulk import aborted successfully",
	}, nil
}

// GetBulkImportStatus retrieves the status of a bulk import session.
func (c *Client) GetBulkImportStatus(ctx context.Context, sessionID string) (*BulkImportStatus, error) {
	result, err := c.bulkImportSvc.GetBulkImportStatus(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return &BulkImportStatus{
		IsActive:            result.IsActive,
		GraphName:           result.GraphName,
		RecordCount:         result.RecordCount,
		LastCheckpointCount: result.LastCheckpointCount,
		CreatedAt:           result.CreatedAt,
		LastActivity:        result.LastActivity,
	}, nil
}

// =============================================================================
// Convenience Methods
// =============================================================================

// GetSession returns the current session.
func (c *Client) GetSession() *Session {
	return c.sessions.GetSession()
}

// IsLoggedIn returns true if there is an active session.
func (c *Client) IsLoggedIn() bool {
	return c.sessions.IsLoggedIn()
}

// GetConfig returns the client configuration.
func (c *Client) GetConfig() *Config {
	return c.config
}

// =============================================================================
// Helper Methods (Type Conversion)
// =============================================================================

// convertToServiceQueryConfig converts QueryConfig to service QueryConfig.
func (c *Client) convertToServiceQueryConfig(config *QueryConfig) *services.QueryConfig {
	if config == nil {
		return nil
	}
	return &services.QueryConfig{
		GraphName:     config.GraphName,
		TransactionID: config.TransactionID,
		Timeout:       config.Timeout,
		ReadOnly:      config.ReadOnly,
		Parameters:    config.Parameters,
	}
}

// convertFromServiceResponse converts service Response to client Response.
func (c *Client) convertFromServiceResponse(svcResp *services.Response) *Response {
	rows := make([]*Row, len(svcResp.Rows))
	for i, svcRow := range svcResp.Rows {
		values := make([]*TypedValue, len(svcRow.Values))
		for j, svcVal := range svcRow.Values {
			values[j] = &TypedValue{
				Type:   PropertyType(svcVal.Type),
				Data:   svcVal.Data,
				IsNull: svcVal.IsNull,
			}
		}
		rows[i] = &Row{Values: values}
	}

	return &Response{
		Columns:  svcResp.Columns,
		Rows:     rows,
		RowCount: svcResp.RowCount,
		HasMore:  svcResp.HasMore,
		Warnings: svcResp.Warnings,
	}
}

// convertProperties converts Go map to proto TypedValue map.
func (c *Client) convertProperties(props map[string]interface{}) (map[string]*pb.TypedValue, error) {
	if props == nil {
		return nil, nil
	}

	result := make(map[string]*pb.TypedValue)
	for k, v := range props {
		tv, err := NewTypedValue(v)
		if err != nil {
			return nil, err
		}
		result[k] = &pb.TypedValue{
			Type:   pb.PropertyType(tv.Type),
			Data:   tv.Data,
			IsNull: tv.IsNull,
		}
	}
	return result, nil
}

// newParameterAdapter adapts the main package NewParameter to services.Parameter.
func (c *Client) newParameterAdapter(name string, value interface{}) (*services.Parameter, error) {
	param, err := NewParameter(name, value)
	if err != nil {
		return nil, err
	}
	return &services.Parameter{
		Name: param.Name,
		Value: &services.TypedValue{
			Type:   services.PropertyType(param.Value.Type),
			Data:   param.Value.Data,
			IsNull: param.Value.IsNull,
		},
	}, nil
}
