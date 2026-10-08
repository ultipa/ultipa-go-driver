package gqldb

import (
	"context"
	cryptoRand "crypto/rand"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"github.com/ultipa/ultipa-go-driver/v6/services"
	"github.com/ultipa/ultipa-go-driver/v6/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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

	// HA leader-aware routing (design §12). activeHostIdx is the index into
	// config.Hosts of the endpoint that RPCs are currently pinned to; it advances
	// on a LEADER_CHANGED reply so the next write lands on the new leader. Guarded
	// by mu.
	mu            sync.RWMutex
	activeHostIdx int

	// followerRouter caches per-host HA status (role + applied index) for opt-in follower-read
	// routing (design §12). Lazily initialized; guarded by its own mutex. nil until the first
	// ReadPreferenceFollower query.
	followerRouter *followerRouter

	// retrySleep waits between two attempts of a read that met a loading
	// fulltext index (retryReadWhileIndexLoads). nil means sleepContext;
	// tests replace it so they need not wait.
	retrySleep func(context.Context, time.Duration) error

	// Stored credentials for auto-reconnect. Guarded by credMu: written by
	// Login (which can run on the reconnect path from arbitrary caller
	// goroutines) and read on classification in withAutoReconnect.
	credMu         sync.Mutex
	storedUsername string
	storedPassword string

	// Serializes auto-reconnect / channel-rebuild so concurrent session
	// expiries re-login at most once (single-flight) and don't race on the
	// service-stub references. The authoritative active graph is NOT tracked
	// here — it lives in sessions.GetDefaultGraph(), which is updated by BOTH
	// UseGraph() and Gql("USE GRAPH ...").
	reconnectMu sync.Mutex

	// Stable per-client logical session id surfaced under the
	// transaction-branch model (see TRANSACTIONS_DRIVER_GUIDE.md §2.0–2.1).
	// Initialized to a UUID v4 hex; users can override via Config.SessionID.
	// Replaced only when signing in again finds transactions of the expired
	// sign-in (detachLostTransactions), and only when the driver generated it.
	// Guarded by csidMu.
	csidMu          sync.RWMutex
	clientSessionID string
	// csidConfigured is true when the application set Config.SessionID. Such
	// an id may be shared with other clients, so the driver never replaces it
	// and never sends anything under it on another client's behalf.
	csidConfigured bool

	// gRPC service clients
	sessionClient     pb.SessionServiceClient
	queryClient       pb.QueryServiceClient
	dataClient        pb.DataServiceClient
	graphClient       pb.GraphServiceClient
	transactionClient pb.TransactionServiceClient
	healthClient      pb.HealthClient
	adminClient       pb.AdminServiceClient
	bulkImportClient  pb.BulkImportServiceClient
	loaderClient      pb.LoaderServiceClient

	// Service layer (delegation)
	sessionSvc     *services.SessionService
	querySvc       *services.QueryService
	graphSvc       *services.GraphService
	transactionSvc *services.TransactionService
	dataSvc        *services.DataService
	healthSvc      *services.HealthService
	adminSvc       *services.AdminService
	bulkImportSvc  *services.BulkImportService
	loaderSvc      *services.LoaderService
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
		csidConfigured:  config.SessionID != "",
	}

	// Initialize gRPC clients and services
	conn, err := client.getConn()
	if err != nil {
		return nil, err
	}
	client.initClients(conn)

	return client, nil
}

// getConn returns the host RPCs are currently pinned to, as a connection that
// sends each call over the pool's current connection to that host (hostConn):
// the active host (config.Hosts[activeHostIdx]) when it is healthy, otherwise
// any healthy host in the pool. The active host is advanced by leader-aware
// routing on LEADER_CHANGED so writes follow the leader across a failover.
func (c *Client) getConn() (grpc.ClientConnInterface, error) {
	c.mu.RLock()
	hosts := c.config.Hosts
	idx := c.activeHostIdx
	c.mu.RUnlock()
	if n := len(hosts); n > 0 {
		if _, err := c.pool.GetConnectionForHost(hosts[idx%n]); err == nil {
			return &hostConn{pool: c.pool, host: hosts[idx%n]}, nil
		}
	}
	conn, err := c.pool.healthyConnection()
	if err != nil {
		return nil, err
	}
	return &hostConn{pool: c.pool, host: conn.host}, nil
}

// initClients initializes the gRPC service clients.
func (c *Client) initClients(conn grpc.ClientConnInterface) {
	c.sessionClient = pb.NewSessionServiceClient(conn)
	c.queryClient = pb.NewQueryServiceClient(conn)
	c.dataClient = pb.NewDataServiceClient(conn)
	c.graphClient = pb.NewGraphServiceClient(conn)
	c.transactionClient = pb.NewTransactionServiceClient(conn)
	c.healthClient = pb.NewHealthClient(conn)
	c.adminClient = pb.NewAdminServiceClient(conn)
	c.bulkImportClient = pb.NewBulkImportServiceClient(conn)
	c.loaderClient = pb.NewLoaderServiceClient(conn)

	// Initialize service context
	ctx := &services.ServiceContext{
		SessionClient:      c.sessionClient,
		QueryClient:        c.queryClient,
		DataClient:         c.dataClient,
		GraphClient:        c.graphClient,
		TransactionClient:  c.transactionClient,
		HealthClient:       c.healthClient,
		AdminClient:        c.adminClient,
		BulkImportClient:   c.bulkImportClient,
		LoaderClient:       c.loaderClient,
		GetSessionID:       func() uint64 { return c.sessions.GetSessionID() },
		GetServerVersion:   func() string { return c.sessions.GetServerVersion() },
		GetClientSessionID: c.currentClientSessionID,
		GetDefaultGraph:    func() string { return c.sessions.GetDefaultGraph() },
		GetTimeout:         func() int { return c.config.TimeoutSeconds() },
		SetDefaultGraph:    func(name string) { c.sessions.SetDefaultGraph(name) },
		UpdateActivity:     func() { c.sessions.UpdateActivity() },
		IsLoggedIn:         func() bool { return c.sessions.IsLoggedIn() },
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
	c.loaderSvc = services.NewLoaderService(ctx)
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
	c.credMu.Lock()
	c.storedUsername = username
	c.storedPassword = password
	c.credMu.Unlock()

	// Register session with SessionManager
	session := c.sessions.Login(ctx, svcSession.ID, svcSession.ServerVersion, svcSession.Roles, svcSession.DefaultGraph, &ClusterInfo{
		IsCluster:      svcSession.IsCluster,
		ClusterID:      svcSession.ClusterID,
		PartitionCount: svcSession.PartitionCount,
	})
	return session, nil
}

// withAutoReconnect executes fn with two recovery behaviours:
//
//   - UNAUTHENTICATED — if stored credentials are available, re-login
//     (single-flight), so the next call works, and send fn again once, but
//     only when rule allows it: the server refused the call before running it
//     (RefusedBeforeRun), or the call is read-only, and for a stream no rows
//     have reached the caller. A call that may have written is not sent again;
//     its error is returned. Decided by the gRPC status alone: words such as
//     "session expired" in a message never count, since a message can quote a
//     value the caller stored.
//   - UNAVAILABLE / "connection reset by peer" — rebuild every channel
//     in the pool so the NEXT RPC gets a fresh one.  The current call
//     is NOT retried (caller decides; write-side calls are often not
//     idempotent).
func (c *Client) withAutoReconnect(ctx context.Context, rule resendRule, fn func() error) error {
	// Capture the session id BEFORE the RPC so autoReconnect can single-flight:
	// if another goroutine already re-logged in (the id rotates on login), this
	// caller skips its own re-login and just retries.
	sid := c.sessions.GetSessionID()

	err := fn()
	if err == nil {
		return nil
	}

	// Classify the error once.
	grpcCode := codes.Unknown
	if s, ok := grpcStatusOf(err); ok {
		grpcCode = s.Code()
	}
	errMsg := strings.ToLower(err.Error())

	user, _ := c.storedCredentials()
	if grpcCode == codes.Unauthenticated && user != "" {
		// Re-login (single-flight) and restore the active graph. A re-login
		// or graph-restore failure surfaces the original error rather than
		// silently retrying against the wrong/default graph.
		reconnectErr := c.autoReconnect(ctx, sid)
		if rule.transactionID != 0 {
			// The transaction belongs to the sign-in that expired: the server
			// refuses it from the new one, so the call is not sent again,
			// whether or not signing in again worked (when it failed, the
			// error says so and the next call signs in again).
			c.txManager.MarkSignInLost(rule.transactionID)
			return &TransactionSignInExpiredError{TransactionID: rule.transactionID, Cause: err, signInErr: reconnectErr}
		}
		if reconnectErr != nil {
			return err
		}
		if !rule.allows(err) {
			// Signed in again for the next call; this one may have run.
			return err
		}
		return fn()
	}

	// LEADER_CHANGED (design §12): a write reached a non-leader in HA mode. Rotate
	// across the configured endpoints to find the new leader and retry there. This
	// is checked before the generic UNAVAILABLE handling because it carries a
	// distinct FailedPrecondition reason and a different, write-safe recovery.
	// Decided from the error detail's reason, never from a word in the message.
	// A stream whose rows have reached the caller is never sent again: the
	// caller would see them twice.
	if isLeaderChangedError(err) {
		if rule.isDelivered() {
			return err
		}
		return c.retryOnLeaderChange(ctx, rule, fn, err)
	}

	isUnavailable := grpcCode == codes.Unavailable ||
		strings.Contains(errMsg, "connection reset by peer") ||
		strings.Contains(errMsg, "socket closed") ||
		strings.Contains(errMsg, "transport is closing")
	if isUnavailable && c.pool != nil {
		// Rebuild every channel so the next call gets a fresh one.
		// Do NOT retry the current call — caller decides. ForceReconnectAll
		// has its own pool mutex, so no client-level lock is needed here.
		c.pool.ForceReconnectAll()
	}

	return err
}

// retryOnLeaderChange rotates across the configured endpoints to find the new
// leader and retries the write there. Sessions are per-node, so each rotation
// re-logins and restores the graph context. Bounded by the number of hosts (each
// tried once) with capped backoff, so a cluster mid-election settles but a cluster
// with no leader fails fast instead of spinning. If a transaction is open the write
// is NOT re-routed: the new leader has no knowledge of that transaction, so it is
// aborted with a clear, retryable error (design §13 — explicit-tx writes are not
// replicated in HA mode; a failover invalidates the open transaction).
//
// The call is sent on only after an answer that says it did not run: a
// LEADER_CHANGED (a follower refuses a write before running it), or an error
// rule allows a resend on (executed=false, or a read-only call); and a stream
// only while none of its rows has reached the caller. Any other
// answer — 5024, 4016, a timeout, a dropped connection — may come from a write
// that ran, and ends the call with that error. A host that could not be signed
// in to was sent nothing, and the rotation moves on. The waits end with ctx.
func (c *Client) retryOnLeaderChange(ctx context.Context, rule resendRule, fn func() error, origErr error) error {
	if c.txManager != nil && c.txManager.HasActive() {
		return NewError(0, "transaction aborted: the leader changed mid-transaction; retry the whole transaction", origErr)
	}
	c.mu.RLock()
	nHosts := len(c.config.Hosts)
	c.mu.RUnlock()
	if nHosts <= 1 {
		return origErr // a single endpoint cannot route to a different leader
	}
	// mayGoOn reports whether a call that failed with rerr may be sent to
	// another host.
	mayGoOn := rule.mayGoToAnotherServer

	lastErr := origErr
	// Proactive routing (item C7): ask the cluster who the leader is via HAService.GetStatus and
	// jump straight there, instead of rotating host-by-host. Falls through to the rotation loop when
	// no leader is reported yet (mid-election) or HAService is unavailable (older server), so the
	// reactive path remains the robust backstop.
	if idx, ok := c.proactiveLeaderIdx(ctx); ok {
		c.mu.Lock()
		c.activeHostIdx = idx
		c.mu.Unlock()
		if rerr := c.reloginActiveHost(ctx); rerr == nil {
			rerr := fn()
			if rerr == nil {
				return nil // landed on the leader directly, no rotation
			}
			if !mayGoOn(rerr) {
				return rerr // the leader answered a call that may have run: never send it again
			}
			lastErr = rerr
		}
		// the jump did not stick (stale status, not-yet-ready leader, or a
		// refusal before running): fall through to rotation
	}

	backoff := 50 * time.Millisecond
	for attempt := 0; attempt < nHosts; attempt++ {
		if rerr := c.rotateActiveHostAndRelogin(ctx); rerr != nil {
			lastErr = rerr
		} else {
			rerr := fn()
			if rerr == nil {
				return nil // landed on the leader
			}
			if !isLeaderChangedError(rerr) || rule.isDelivered() {
				// a real error on the candidate, or a stream whose rows reached
				// the caller: surface it, do not keep rotating
				return rerr
			}
			lastErr = rerr
		}
		if sleepContext(ctx, backoff) != nil {
			return lastErr
		}
		backoff = capLeaderBackoff(backoff)
	}
	return lastErr
}

func capLeaderBackoff(d time.Duration) time.Duration {
	d *= 2
	if d > time.Second {
		return time.Second
	}
	return d
}

// rotateActiveHostAndRelogin advances the pinned endpoint to the next configured
// host and re-establishes a session there. Login re-targets the service clients via
// getConn (which now reads the advanced activeHostIdx), so this both moves routing
// and refreshes the per-node session.
func (c *Client) rotateActiveHostAndRelogin(ctx context.Context) error {
	c.mu.Lock()
	if n := len(c.config.Hosts); n > 0 {
		c.activeHostIdx = (c.activeHostIdx + 1) % n
	}
	c.mu.Unlock()
	return c.reloginActiveHost(ctx)
}

// reloginActiveHost re-establishes a session at the CURRENT activeHostIdx and re-points the service
// clients there (Login → getConn reads activeHostIdx → initClients), restoring the graph context.
// Used by both rotation (after advancing the index) and proactive leader routing (after jumping the
// index straight to the GetStatus-reported leader).
func (c *Client) reloginActiveHost(ctx context.Context) error {
	// Restore the authoritative active graph (sessions.GetDefaultGraph is updated
	// by both UseGraph and Gql "USE GRAPH ..."), and re-login under the
	// single-flight credentials accessor — consistent with autoReconnect.
	savedGraph := c.sessions.GetDefaultGraph()
	user, pass := c.storedCredentials()
	if user == "" {
		// No stored credentials yet: just re-point the service clients.
		conn, err := c.getConn()
		if err != nil {
			return err
		}
		c.initClients(conn)
		return nil
	}
	if _, err := c.Login(ctx, user, pass); err != nil {
		return err
	}
	if savedGraph != "" && savedGraph != "__system__" {
		_ = c.UseGraph(ctx, savedGraph)
	}
	return nil
}

// autoReconnect re-logs in with stored credentials and restores the active
// graph. Single-flight: expiredSessionID is the session id the caller observed
// before the failing RPC. Under the lock, if another goroutine has already
// re-logged in (the id rotates on login), this is a no-op so concurrent
// expiries trigger exactly one re-login.
//
// The graph to restore is read from sessions.GetDefaultGraph() — the
// authoritative active graph, updated by BOTH UseGraph() and
// Gql("USE GRAPH ...") — captured BEFORE re-login resets it to the config
// default. (The old code restored a stale storedGraph that only tracked
// explicit UseGraph() calls, so a gql-switched graph was silently dropped on
// reconnect.)
//
// A graph-restore failure is NOT swallowed: it propagates so the caller
// surfaces an error rather than silently retrying against the wrong graph
// (which returns empty rows with no error).
func (c *Client) autoReconnect(ctx context.Context, expiredSessionID uint64) error {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	if c.sessions.GetSessionID() != expiredSessionID {
		return nil // another goroutine already re-logged in
	}

	activeGraph := c.sessions.GetDefaultGraph()
	orphanCSID, orphanGraph := c.detachLostTransactions(expiredSessionID)

	user, pass := c.storedCredentials()
	if _, loginErr := c.Login(ctx, user, pass); loginErr != nil {
		return loginErr
	}
	if orphanCSID != "" {
		c.discardOrphanTransaction(ctx, orphanCSID, orphanGraph)
	}

	if activeGraph != "" && activeGraph != "__system__" {
		if useErr := c.UseGraph(ctx, activeGraph); useErr != nil {
			return useErr
		}
	}
	return nil
}

// storedCredentials returns the credentials captured at the last successful
// Login, under credMu, for safe concurrent reads on the reconnect path.
func (c *Client) storedCredentials() (string, string) {
	c.credMu.Lock()
	defer c.credMu.Unlock()
	return c.storedUsername, c.storedPassword
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
//
// Returns *GraphSwitchRejectedError when Config.DisableUseGraph is
// enabled and the query selects or replaces a graph.
func (c *Client) Gql(ctx context.Context, query string, config *QueryConfig) (*Response, error) {
	if err := c.checkGraphSwitch(query); err != nil {
		return nil, err
	}
	return c.gqlUnchecked(ctx, query, config)
}

// checkGraphSwitch applies the graph-switch guard to caller-supplied query
// text. No-op unless Config.DisableUseGraph is set.
//
// GQL the driver builds itself bypasses this by calling gqlUnchecked
// directly - convenience DDL legitimately emits ALTER GRAPH / CREATE GRAPH.
func (c *Client) checkGraphSwitch(query string) error {
	if c.config == nil || !c.config.DisableUseGraph {
		return nil
	}
	if keyword := findBlockedKeyword(query); keyword != "" {
		return &GraphSwitchRejectedError{Keyword: keyword}
	}
	return nil
}

// gqlUnchecked executes GQL, bypassing the graph-switch guard. For
// driver-authored query text only.
func (c *Client) gqlUnchecked(ctx context.Context, query string, config *QueryConfig) (*Response, error) {
	if query == "" {
		return nil, ErrEmptyQuery
	}
	if err := c.transactionSignInLost(transactionIDOf(config)); err != nil {
		return nil, err
	}

	rule := resendRule{readOnly: isReadOnlyRequest(config), transactionID: transactionIDOf(config)}

	// HA follower-read routing (design §12): when the caller opts into ReadPreferenceFollower, try a
	// fresh-enough follower first. With no eligible follower nothing was sent, and the call takes the
	// normal leader-routed path. A follower's error sends the call on to the leader only when the
	// resend rule allows it (read-only outside a transaction, or refused before running); otherwise
	// it is the call's answer, since a follower that ran a write may have stored it. NOTE: follower
	// reads are eventually consistent within a freshness bound, NOT read-your-writes.
	if config != nil && config.ReadPreference == ReadPreferenceFollower {
		resp, sent, ferr := c.tryFollowerRead(ctx, query, config)
		if sent && ferr == nil {
			return resp, nil
		}
		if sent && !rule.mayGoToAnotherServer(ferr) {
			return nil, NewError(0, "query failed", ferr)
		}
	}

	var resp *Response
	err := c.retryReadWhileIndexLoads(ctx, config, nil, func() error {
		return c.withAutoReconnect(ctx, rule, func() error {
			svcConfig := c.convertToServiceQueryConfig(config)
			svcResp, err := c.querySvc.Gql(ctx, query, svcConfig, c.newParameterAdapter,
				c.sessions.GetDefaultGraph, func() int { return c.config.TimeoutSeconds() })
			if err != nil {
				return err
			}
			resp = c.convertFromServiceResponse(svcResp)
			return nil
		})
	})
	if err != nil {
		return nil, queryError("query failed", config, err)
	}

	return resp, nil
}

// GqlStream executes a GQL query and streams the results.
func (c *Client) GqlStream(ctx context.Context, query string, config *QueryConfig, callback func(*Response) error) error {
	if err := c.checkGraphSwitch(query); err != nil {
		return err
	}
	if query == "" {
		return ErrEmptyQuery
	}

	if err := c.transactionSignInLost(transactionIDOf(config)); err != nil {
		return err
	}

	// A stream starts over only while no rows have reached the callback.
	delivered := false
	rule := resendRule{readOnly: isReadOnlyRequest(config), delivered: func() bool { return delivered },
		transactionID: transactionIDOf(config)}
	err := c.retryReadWhileIndexLoads(ctx, config, func() bool { return !delivered }, func() error {
		return c.withAutoReconnect(ctx, rule, func() error {
			svcConfig := c.convertToServiceQueryConfig(config)
			return c.querySvc.GqlStream(ctx, query, svcConfig, func(svcResp *services.Response) error {
				delivered = true
				resp := c.convertFromServiceResponse(svcResp)
				return callback(resp)
			}, c.newParameterAdapter, c.sessions.GetDefaultGraph, func() int { return c.config.TimeoutSeconds() })
		})
	})

	if err != nil {
		return queryError("stream query failed", config, err)
	}
	return nil
}

// queryError is the error a failed Gql or GqlStream returns: a *GqldbError,
// or, for a write conflict (conflictError: the server's detail, or an older
// server's "[3011]" answer to a call with a TransactionID), a
// *WriteConflictError whose Cause is that *GqldbError. A write
// conflict can arrive from a statement, not only from Commit: a MERGE that
// would have to wait on a key while this transaction already holds another is
// refused rather than allowed to deadlock. Inside a transaction its message
// says the whole transaction has to run again.
func queryError(message string, config *QueryConfig, err error) error {
	var tse *TransactionSignInExpiredError
	if errors.As(err, &tse) {
		return tse
	}
	wrapped := NewError(0, message, err)
	var txID uint64
	if config != nil {
		txID = config.TransactionID
	}
	if wce := conflictError(err, txID, wrapped, message+": "+err.Error()); wce != nil {
		return wce
	}
	return wrapped
}

// retryReadWhileIndexLoads runs attempt and, for a read-only request outside a
// transaction that fails because a fulltext index is still loading (engine
// code 5020), runs it again up to Config.RetryCount times with a doubling wait
// starting at Config.RetryDelay (see retryRead). Nothing else is sent again, a
// write conflict (3011) included. canRetry, when not nil, is asked before each
// retry.
func (c *Client) retryReadWhileIndexLoads(ctx context.Context, config *QueryConfig, canRetry func() bool, attempt func() error) error {
	if canRetry == nil {
		canRetry = func() bool { return true }
	}
	c.mu.RLock()
	retries, base := c.config.RetryCount, c.config.RetryDelay
	c.mu.RUnlock()
	sleep := c.retrySleep
	if sleep == nil {
		sleep = sleepContext
	}
	return retryRead(ctx, config, retries, base, sleep, canRetry, attempt)
}

// Explain returns the execution plan for a query.
func (c *Client) Explain(ctx context.Context, query string, config *QueryConfig) (string, error) {
	if err := c.checkGraphSwitch(query); err != nil {
		return "", err
	}
	if query == "" {
		return "", ErrEmptyQuery
	}

	if err := c.transactionSignInLost(transactionIDOf(config)); err != nil {
		return "", err
	}

	var plan string
	err := c.withAutoReconnect(ctx, resendRule{readOnly: isReadOnlyRequest(config), transactionID: transactionIDOf(config)}, func() error {
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
	if err := c.checkGraphSwitch(query); err != nil {
		return "", err
	}
	if query == "" {
		return "", ErrEmptyQuery
	}

	if err := c.transactionSignInLost(transactionIDOf(config)); err != nil {
		return "", err
	}

	var profile string
	err := c.withAutoReconnect(ctx, resendRule{readOnly: isReadOnlyRequest(config), transactionID: transactionIDOf(config)}, func() error {
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
	return c.CreateGraphWithEdgeId(ctx, name, graphType, description, EdgeIdUnset)
}

// CreateGraphWithEdgeId creates a new graph and optionally sets its EDGE_ID
// mode. The gRPC CreateGraph RPC has no EDGE_ID field, so when edgeId is not
// EdgeIdUnset this method first calls the existing gRPC CreateGraph (which
// preserves description) and then issues an
// `ALTER GRAPH <name> SET EDGE_ID ENABLED|DISABLED` via GQL.
// When edgeId is EdgeIdUnset the EDGE_ID setting is left untouched.
func (c *Client) CreateGraphWithEdgeId(ctx context.Context, name string, graphType GraphType, description string, edgeId EdgeIdMode) error {
	success, message, err := c.graphSvc.CreateGraph(ctx, name, graphType, description)
	if err != nil {
		return NewError(0, "create graph failed", err)
	}
	if !success {
		return NewError(0, message, nil)
	}
	if edgeId != EdgeIdUnset {
		state := "DISABLED"
		if edgeId == EdgeIdEnabled {
			state = "ENABLED"
		}
		if _, gqlErr := c.gqlUnchecked(ctx, fmt.Sprintf("ALTER GRAPH %s SET EDGE_ID %s", name, state), nil); gqlErr != nil {
			return NewError(0, "alter graph set edge_id failed", gqlErr)
		}
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
	// The active graph is tracked authoritatively in sessions (GraphService
	// calls SetDefaultGraph); autoReconnect restores from there.
	return nil
}

// ListGraphs returns all available graphs.
//
// Implemented over GQL `SHOW GRAPHS` rather than the ListGraphs RPC: the
// RPC's typed enum mis-maps CLOSED and cannot surface the new
// bounded_graph_type column. The result is parsed by column name — reading
// graph_mode (renamed from graph_type in 6.2.59) with a fall-back to the old
// graph_type name, and passing through bounded_graph_type when present — so
// it works against both old and new servers.
func (c *Client) ListGraphs(ctx context.Context) ([]*GraphInfo, error) {
	resp, err := c.gqlUnchecked(ctx, "SHOW GRAPHS", nil)
	if err != nil {
		return nil, NewError(0, "list graphs failed", err)
	}

	graphs := make([]*GraphInfo, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		name := strCol(row, "graph_name")
		mode := strCol(row, "graph_mode")
		if !row.Has("graph_mode") {
			// Old server (<= 6.2.50): the mode column is named graph_type.
			mode = strCol(row, "graph_type")
		}
		// bounded_graph_type is new in 6.2.59; absent on older servers (Go
		// has no null string, so "column absent" maps to "").
		bounded := strCol(row, "bounded_graph_type")
		graphs = append(graphs, &GraphInfo{
			Name:             name,
			GraphType:        GraphTypeFromMode(mode),
			NodeCount:        longCol(row, "node_count"),
			EdgeCount:        longCol(row, "edge_count"),
			Description:      strCol(row, "comment"),
			BoundedGraphType: bounded,
		})
	}
	return graphs, nil
}

// GetGraphInfo returns information about a specific graph.
//
// Reuses the GQL-based ListGraphs (SHOW GRAPHS) and filters by name — same
// reason as ListGraphs: the GetGraphInfo RPC's typed enum mis-maps CLOSED and
// can't surface bounded_graph_type. Returns ErrGraphNotFound when absent,
// preserving the prior not-found behavior.
func (c *Client) GetGraphInfo(ctx context.Context, name string) (*GraphInfo, error) {
	graphs, err := c.ListGraphs(ctx)
	if err != nil {
		return nil, err
	}
	for _, g := range graphs {
		if g.Name == name {
			return g, nil
		}
	}
	return nil, ErrGraphNotFound
}

// strCol reads a SHOW-GRAPHS cell by column name as a string ("" when the
// column is absent or the value is nil).
func strCol(row *Row, col string) string {
	if !row.Has(col) {
		return ""
	}
	v, err := row.GetByName(col)
	if err != nil || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// longCol reads a SHOW-GRAPHS cell by column name as an int64 (0 when the
// column is absent, nil, or unparsable).
func longCol(row *Row, col string) int64 {
	if !row.Has(col) {
		return 0
	}
	v, err := row.GetByName(col)
	if err != nil || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	case uint64:
		return int64(n)
	case uint32:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case string:
		parsed, perr := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		if perr != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

// =============================================================================
// Transaction Service - Delegates to TransactionService
// =============================================================================

// BeginTransaction starts a new transaction.
//
// An expired sign-in is signed in again, and Begin is sent again only when the
// server refused it before running it (executed=false): the caller asked for
// this transaction. No other path of the driver begins one.
func (c *Client) BeginTransaction(ctx context.Context, graphName string, readOnly bool, timeout int) (*Transaction, error) {
	var result *services.BeginTransactionResult
	err := c.withAutoReconnect(ctx, resendRule{}, func() error {
		var e error
		result, e = c.transactionSvc.BeginTransaction(ctx, graphName, readOnly, timeout)
		return e
	})
	if err != nil {
		return nil, NewError(0, "begin transaction failed", err)
	}

	// Create transaction using transaction manager
	tx := c.txManager.Begin(result.TransactionID, result.SessionID, result.GraphName, result.ReadOnly, result.Timeout)
	// Surface the per-client logical session id on the returned tx
	// (transaction-branch ergonomic, see TRANSACTIONS_DRIVER_GUIDE.md).
	tx.ClientSessionID = c.currentClientSessionID()
	return tx, nil
}

// ClientSessionID returns the stable per-client logical session id surfaced
// under the transaction-branch model. The same id is attached to every
// Transaction returned by BeginTransaction.
//
// When the driver generated it (Config.SessionID empty), it changes when the
// driver signs in again after the sign-in expired while a transaction begun
// with BeginTransaction was open (see TransactionSignInExpiredError): a server
// without the session-binding fix keeps such a transaction under the old id,
// and a statement sent with that id would join it. An id set in
// Config.SessionID never changes.
func (c *Client) ClientSessionID() string {
	return c.currentClientSessionID()
}

// currentClientSessionID returns the client session id sent with every call.
func (c *Client) currentClientSessionID() string {
	c.csidMu.RLock()
	defer c.csidMu.RUnlock()
	return c.clientSessionID
}

// detachLostTransactions handles the transactions begun under a sign-in that
// expired, before signing in again. The server refuses them from any new
// sign-in, yet keeps them open under this client's session id
// (x-ultipa-session-id): a plain statement sent with that id afterwards
// joined the dead transaction -- its write reported success and was never
// stored -- and a new BeginTransaction was refused ([3010] the session already
// has an active transaction). So they are marked lost: every call on them
// returns TransactionSignInExpiredError.
//
// When the driver generated the client session id, no other client holds it:
// the client takes a new id, so nothing it sends from now on joins them, and
// the old id and the graph of a lost transaction are returned for
// discardOrphanTransaction. When the application set the id (Config.SessionID),
// other clients may share it and may have a live transaction under it, which a
// ROLLBACK sent with that id would end. So the id is kept and nothing is sent:
// "" is returned. A server with the session-binding fix ends an expired
// sign-in's transactions itself; an older one keeps them until they time out.
func (c *Client) detachLostTransactions(expiredSessionID uint64) (string, string) {
	if expiredSessionID == 0 {
		return "", ""
	}
	lost := c.txManager.GetActiveForSession(expiredSessionID)
	if len(lost) == 0 {
		return "", ""
	}
	for _, tx := range lost {
		c.txManager.MarkSignInLost(tx.ID)
	}
	if c.csidConfigured {
		return "", ""
	}
	c.csidMu.Lock()
	old := c.clientSessionID
	c.clientSessionID = newClientSessionID()
	c.csidMu.Unlock()
	return old, lost[0].GraphName
}

// discardOrphanTransaction asks the server, from the new sign-in, to roll back
// the transaction it keeps open under the client's old session id
// (detachLostTransactions): a plain ROLLBACK sent with that id. Best effort:
// nothing of the transaction was committed either way, and an error (the
// server already ended it) is ignored.
func (c *Client) discardOrphanTransaction(ctx context.Context, orphanClientSessionID, graphName string) {
	sid := c.sessions.GetSessionID()
	md := metadata.Pairs("session-id", strconv.FormatUint(sid, 10), "x-ultipa-session-id", orphanClientSessionID)
	rctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.WithoutCancel(ctx), md), 10*time.Second)
	defer cancel()
	_, _ = c.queryClient.Gql(rctx, &pb.GqlRequest{Gql: "ROLLBACK", GraphName: graphName, SessionId: sid})
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

// Commit commits a transaction. See CommitWithResult for the server's
// warnings and for what each error means.
func (c *Client) Commit(ctx context.Context, transactionID uint64) (bool, error) {
	r, err := c.CommitWithResult(ctx, transactionID)
	if err != nil {
		return false, err
	}
	return r.Success, nil
}

// CommitWithResult commits a transaction and returns the server's answer,
// with the warnings of a commit that stored the changes but could not finish
// something after them (do not run such a transaction again: it is stored).
//
// The commit is never sent again by the driver. The errors:
//   - *PartlyCommittedError: the server answered 5024 marked partly_stored:
//     part of the changes are stored. The transaction is marked partly
//     committed, not rolled back (Transaction.IsPartlyCommitted).
//   - *TransactionSignInExpiredError: the sign-in that began the transaction
//     expired. The driver signs in again for the next call, but the server
//     refuses the transaction from the new sign-in; nothing was committed.
//   - *WriteConflictError: a write conflict; nothing is stored.
//   - otherwise a *GqldbError; the transaction is marked rolled back.
func (c *Client) CommitWithResult(ctx context.Context, transactionID uint64) (*CommitResult, error) {
	if err := c.transactionSignInLost(transactionID); err != nil {
		return nil, err
	}
	sid := c.sessions.GetSessionID()
	res, err := c.transactionSvc.CommitWithResult(ctx, transactionID)
	if err != nil {
		if tse := c.transactionSignInExpired(ctx, sid, transactionID, err); tse != nil {
			return nil, tse
		}
		wrapped := NewError(0, "commit failed", err)
		if PartlyStored(err) {
			// Part of the transaction is stored: it is neither rolled back nor
			// a conflict to retry.
			_ = c.txManager.PartlyCommit(transactionID)
			msg := err.Error()
			if st, ok := grpcStatusOf(err); ok {
				msg = st.Message()
			}
			return nil, &PartlyCommittedError{TransactionID: transactionID, Message: msg, Cause: wrapped}
		}
		// Clean up local state even if server commit fails
		_ = c.txManager.Rollback(transactionID)
		// A refused commit under optimistic concurrency is a normal outcome --
		// the first committer won and this one must retry. Surfacing it as a
		// generic GqldbError would make correct programs look broken.
		// A commit always carries its transaction id, so an older server's
		// "[3011]" answer counts too.
		if wce := conflictError(err, transactionID, wrapped, "commit failed: "+err.Error()); wce != nil {
			return nil, wce
		}
		return nil, wrapped
	}

	// Mark transaction as committed in transaction manager
	_ = c.txManager.CommitWithWarnings(transactionID, res.Warnings)
	return &CommitResult{
		Success:       res.Success,
		Message:       res.Message,
		Warnings:      res.Warnings,
		TimeCostNs:    res.TimeCostNs,
		DiskCostNs:    res.DiskCostNs,
		ComputeCostNs: res.ComputeCostNs,
	}, nil
}

// Rollback aborts a transaction. A transaction whose sign-in expired returns
// a *TransactionSignInExpiredError: the server refuses it from the new
// sign-in, and none of its changes were committed.
func (c *Client) Rollback(ctx context.Context, transactionID uint64) (bool, error) {
	if err := c.transactionSignInLost(transactionID); err != nil {
		return false, err
	}
	sid := c.sessions.GetSessionID()
	success, err := c.transactionSvc.Rollback(ctx, transactionID)
	if err != nil {
		if tse := c.transactionSignInExpired(ctx, sid, transactionID, err); tse != nil {
			return false, tse
		}
		// Always clean up local state even if server rollback fails
		_ = c.txManager.Rollback(transactionID)
		return false, NewError(0, "rollback failed", err)
	}

	// Mark transaction as rolled back in transaction manager
	c.txManager.Rollback(transactionID)
	return success, nil
}

// transactionSignInLost returns a *TransactionSignInExpiredError, without
// sending anything, when transactionID names a transaction whose sign-in
// expired: one marked so already, or one this client began under a sign-in
// that a new one has since replaced. The server refuses such a transaction
// ("transaction belongs to different session").
func (c *Client) transactionSignInLost(transactionID uint64) error {
	if transactionID == 0 {
		return nil
	}
	if c.txManager.IsSignInLost(transactionID) {
		return &TransactionSignInExpiredError{TransactionID: transactionID}
	}
	tx := c.txManager.Get(transactionID)
	if tx == nil || tx.SessionID == 0 {
		return nil
	}
	if cur := c.sessions.GetSessionID(); cur != 0 && cur != tx.SessionID {
		c.txManager.MarkSignInLost(transactionID)
		return &TransactionSignInExpiredError{TransactionID: transactionID}
	}
	return nil
}

// transactionSignInExpired handles a call of a transaction answered
// UNAUTHENTICATED: the transaction belongs to the sign-in that expired, so it
// is lost. It signs in again (single-flight, from sid) so the next call works,
// marks the transaction lost and returns the error to give the caller, whether
// or not signing in again worked (when it failed, the error says so and the
// next call signs in again). The call is never sent again. nil when err is not
// that answer, or no credentials are stored (the caller then returns err
// itself).
func (c *Client) transactionSignInExpired(ctx context.Context, sid, transactionID uint64, err error) error {
	st, ok := grpcStatusOf(err)
	if !ok || st.Code() != codes.Unauthenticated || transactionID == 0 {
		return nil
	}
	if user, _ := c.storedCredentials(); user == "" {
		return nil
	}
	signInErr := c.autoReconnect(ctx, sid)
	c.txManager.MarkSignInLost(transactionID)
	return &TransactionSignInExpiredError{TransactionID: transactionID, Cause: err, signInErr: signInErr}
}

// ListTransactions returns active transactions.
//
// Wrapped like every other call: a listing left idle past the sign-in TTL used
// to fail with 7023 while the calls around it signed in again and answered. The
// rule marks it read-only because it is one -- it reads and writes nothing and
// carries no transaction id, so no transaction is at stake -- which is what lets
// the resend happen on a server older than the executed="false" mark as well.
// The service re-reads the session id when it is sent again, so the second
// attempt uses the new sign-in.
func (c *Client) ListTransactions(ctx context.Context) ([]*TransactionInfo, error) {
	var svcTxs []*services.TransactionInfo
	err := c.withAutoReconnect(ctx, resendRule{readOnly: true}, func() error {
		var e error
		svcTxs, e = c.transactionSvc.ListTransactions(ctx)
		return e
	})
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
//
// Equivalent to WithTransactionRetry with a retry budget of 0.
func (c *Client) WithTransaction(ctx context.Context, graphName string, readOnly bool, fn func(txID uint64) error) error {
	return c.WithTransactionRetry(ctx, graphName, readOnly, 0, fn)
}

// WithTransactionRetry executes a function within a transaction, retrying it
// up to retryOnConflict times if the transaction loses a write conflict.
//
// Retrying is offered, not imposed, because fn is re-run from the start on
// each attempt and only the caller knows whether that is safe: anything fn
// does outside the transaction (sending mail, charging a card, appending to a
// log) happens again. Retries back off exponentially with jitter; the conflict
// is returned once the budget is spent.
//
// A write conflict is the only condition retried -- every other error is
// returned on the first attempt. In particular server code 3010 (nested BEGIN,
// rollback of a dead transaction) is a caller mistake and is never retried. A
// conflict is the *WriteConflictError a statement or the Commit of this
// transaction returns: from the server's structured detail, or, from a server
// older than the detail, gRPC status Aborted with a message that starts with
// "[3011]" (these calls carry the transaction id).
func (c *Client) WithTransactionRetry(ctx context.Context, graphName string, readOnly bool, retryOnConflict int, fn func(txID uint64) error) error {
	for attempt := 0; ; attempt++ {
		tx, err := c.BeginTransaction(ctx, graphName, readOnly, c.config.TimeoutSeconds())
		if err != nil {
			return err
		}

		err = fn(tx.ID)
		if err == nil {
			_, err = c.Commit(ctx, tx.ID)
			if err == nil {
				return nil
			}
			// Commit has set the local state: rolled back, partly committed
			// (a *PartlyCommittedError, never retried: part of it is stored)
			// or lost with its sign-in (never retried: the caller decides).
		} else {
			c.Rollback(ctx, tx.ID)
		}

		if attempt >= retryOnConflict || !IsWriteConflict(err) {
			return err
		}
		select {
		case <-time.After(conflictBackoff(attempt + 1)):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

const (
	// conflictBackoffBase is the base delay for the first conflict retry.
	conflictBackoffBase = 5 * time.Millisecond
	// conflictBackoffMax caps the exponential part, so a high retry count
	// cannot stall a caller.
	conflictBackoffMax = 200 * time.Millisecond
)

// conflictBackoff returns the delay before conflict retry number attempt
// (1-based): exponential with full jitter.
//
// The jitter is not decoration: two clients refused in the same round will
// retry in lockstep and collide again if they back off by the same amount.
func conflictBackoff(attempt int) time.Duration {
	ceiling := conflictBackoffBase << uint(attempt-1)
	if ceiling > conflictBackoffMax || ceiling <= 0 {
		ceiling = conflictBackoffMax
	}
	return time.Duration(rand.Int63n(int64(ceiling) + 1))
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
	resp, err := c.gqlUnchecked(ctx, "SHOW TRANSACTIONS", nil)
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
	return c.gqlUnchecked(ctx, "KILL TRANSACTION '"+escaped+"'", nil)
}

// ResetTransactions rolls back every active transaction via
// `RESET TRANSACTIONS`. Admin-only; intended as an escape hatch when an
// orphan tx blocks new BEGINs.
func (c *Client) ResetTransactions(ctx context.Context) (*Response, error) {
	return c.gqlUnchecked(ctx, "RESET TRANSACTIONS", nil)
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

// DeleteNodesByIDs deletes nodes by id list. Emits
// `MATCH (n) WHERE n._id IN [...] DETACH DELETE n RETURN n`.
// Empty/nil nodeIDs short-circuits without contacting the server.
// Use response.Alias("n").AsNodes() to access the deleted nodes.
// With config.ReturnDeleted=false, only RowsAffected is meaningful.
func (c *Client) DeleteNodesByIDs(ctx context.Context, nodeIDs []string, config *DeleteConfig) (*Response, error) {
	cfg := normalizeDeleteConfig(config)
	if len(nodeIDs) == 0 {
		return &Response{}, nil
	}
	gql := "MATCH (n) WHERE n._id IN [" + formatStringList(nodeIDs) + "] DETACH DELETE n"
	if cfg.ReturnDeleted {
		gql += " RETURN n"
	}
	return c.gqlUnchecked(ctx, gql, deleteConfigToQueryConfig(cfg))
}

// DeleteNodesByCondition deletes nodes matching labels and/or where.
// Emits `MATCH (n:L1|L2) WHERE <where> [LIMIT N] DETACH DELETE n RETURN n`.
//
// limit caps the number of nodes deleted (LIMIT N before DETACH DELETE).
// `limit <= 0` = no cap. Pass 0 if you don't want a limit.
//
// Returns an error if both labels and where are empty unless
// config.AllowDeleteAll is true.
func (c *Client) DeleteNodesByCondition(ctx context.Context, labels []string, where string, limit int, config *DeleteConfig) (*Response, error) {
	cfg := normalizeDeleteConfig(config)
	noLabels := len(labels) == 0
	noWhere := strings.TrimSpace(where) == ""
	if noLabels && noWhere && !cfg.AllowDeleteAll {
		return nil, fmt.Errorf(
			"DeleteNodesByCondition with no labels and no where would delete every " +
				"node in the graph. If this is intentional, set DeleteConfig.AllowDeleteAll = true")
	}
	gql := "MATCH (n"
	if !noLabels {
		parts := make([]string, len(labels))
		for i, l := range labels {
			parts[i] = "`" + l + "`"
		}
		gql += ":" + strings.Join(parts, "|")
	}
	gql += ")"
	if !noWhere {
		gql += " WHERE " + where
	}
	if limit > 0 {
		gql += fmt.Sprintf(" LIMIT %d", limit)
	}
	gql += " DETACH DELETE n"
	if cfg.ReturnDeleted {
		gql += " RETURN n"
	}
	return c.gqlUnchecked(ctx, gql, deleteConfigToQueryConfig(cfg))
}

// DeleteEdgesByIDs deletes edges by id list. Emits 5-column GQL keyed on
// e._id, reshapes the response into a single "e" column holding *Edge.
// Use response.Alias("e").AsEdges() — symmetric with the node path.
// Requires EDGE_ID ENABLED: on a disabled graph the server returns [5017]
// guiding the caller to ALTER GRAPH <g> SET EDGE_ID ENABLED (deletion does
// NOT fall back to the discouraged internal_id(e)).
func (c *Client) DeleteEdgesByIDs(ctx context.Context, edgeIDs []string, config *DeleteConfig) (*Response, error) {
	cfg := normalizeDeleteConfig(config)
	if len(edgeIDs) == 0 {
		return &Response{}, nil
	}
	gql := "MATCH ()-[e]->() WHERE e._id IN [" + formatStringList(edgeIDs) + "] DELETE e"
	if cfg.ReturnDeleted {
		gql += " RETURN e._id, e._from, e._to, labels(e)[0], properties(e)"
	}
	raw, err := c.gqlUnchecked(ctx, gql, deleteConfigToQueryConfig(cfg))
	if err != nil || !cfg.ReturnDeleted {
		return raw, err
	}
	return reshapeEdgeDelete(raw), nil
}

// DeleteEdgesByCondition deletes edges matching label and/or where.
// Emits 5-column GQL with optional LIMIT, then reshapes into single
// "e" column.  `limit <= 0` = no cap.
func (c *Client) DeleteEdgesByCondition(ctx context.Context, label string, where string, limit int, config *DeleteConfig) (*Response, error) {
	cfg := normalizeDeleteConfig(config)
	noLabel := label == ""
	noWhere := strings.TrimSpace(where) == ""
	if noLabel && noWhere && !cfg.AllowDeleteAll {
		return nil, fmt.Errorf(
			"DeleteEdgesByCondition with no label and no where would delete every " +
				"edge in the graph. If this is intentional, set DeleteConfig.AllowDeleteAll = true")
	}
	gql := "MATCH ()-[e"
	if !noLabel {
		gql += ":`" + label + "`"
	}
	gql += "]->()"
	if !noWhere {
		gql += " WHERE " + where
	}
	if limit > 0 {
		gql += fmt.Sprintf(" LIMIT %d", limit)
	}
	gql += " DELETE e"
	if cfg.ReturnDeleted {
		gql += " RETURN e._id, e._from, e._to, labels(e)[0], properties(e)"
	}
	raw, err := c.gqlUnchecked(ctx, gql, deleteConfigToQueryConfig(cfg))
	if err != nil || !cfg.ReturnDeleted {
		return raw, err
	}
	return reshapeEdgeDelete(raw), nil
}

// normalizeDeleteConfig returns a non-nil *DeleteConfig with sane
// defaults. Passing nil yields ReturnDeleted=true; passing a non-nil
// config returns it unchanged so caller-set values win.
func normalizeDeleteConfig(c *DeleteConfig) *DeleteConfig {
	if c == nil {
		return types.NewDeleteConfig()
	}
	return c
}

func deleteConfigToQueryConfig(dc *DeleteConfig) *types.QueryConfig {
	if dc == nil {
		return nil
	}
	qc := dc.QueryConfig
	return &qc
}

// formatStringList escapes single quotes and backslashes for safe GQL
// injection, returning `'id1', 'id2', ...`.
func formatStringList(ids []string) string {
	var sb strings.Builder
	for i, id := range ids {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteByte('\'')
		for _, ch := range id {
			if ch == '\'' || ch == '\\' {
				sb.WriteByte('\\')
			}
			sb.WriteRune(ch)
		}
		sb.WriteByte('\'')
	}
	return sb.String()
}

// reshapeEdgeDelete converts a raw 5-column edge-delete Response
// (e._id, _from, _to, labels(e)[0], properties(e)) into a single
// "e" column holding *Edge — symmetric with the node path.
// Rows with a null id are skipped.
func reshapeEdgeDelete(raw *Response) *Response {
	out := &Response{
		Columns:       []string{"e"},
		Rows:          nil,
		HasMore:       raw.HasMore,
		Warnings:      raw.Warnings,
		RowsAffected:  raw.RowsAffected,
		CurrentGraph:  raw.CurrentGraph,
		TimeCostNs:    raw.TimeCostNs,
		DiskCostNs:    raw.DiskCostNs,
		ComputeCostNs: raw.ComputeCostNs,
		DmlStats:      raw.DmlStats,
	}
	for _, r := range raw.Rows {
		if len(r.Values) == 0 {
			continue
		}
		idTv := r.Values[0]
		if idTv == nil || idTv.IsNull {
			continue
		}
		idV, _ := idTv.ToGo()
		idStr, _ := idV.(string)
		if idStr == "" {
			continue
		}
		var fromStr, toStr, labelStr string
		props := map[string]interface{}{}
		if len(r.Values) > 1 && r.Values[1] != nil && !r.Values[1].IsNull {
			if v, _ := r.Values[1].ToGo(); v != nil {
				fromStr, _ = v.(string)
			}
		}
		if len(r.Values) > 2 && r.Values[2] != nil && !r.Values[2].IsNull {
			if v, _ := r.Values[2].ToGo(); v != nil {
				toStr, _ = v.(string)
			}
		}
		if len(r.Values) > 3 && r.Values[3] != nil && !r.Values[3].IsNull {
			if v, _ := r.Values[3].ToGo(); v != nil {
				labelStr, _ = v.(string)
			}
		}
		if len(r.Values) > 4 && r.Values[4] != nil && !r.Values[4].IsNull {
			if v, _ := r.Values[4].ToGo(); v != nil {
				if m, ok := v.(map[string]interface{}); ok {
					props = m
				}
			}
		}
		edge := &Edge{
			ID:         idStr,
			Label:      labelStr,
			FromNodeID: fromStr,
			ToNodeID:   toStr,
			Properties: props,
		}
		out.Rows = append(out.Rows, &Row{Values: []*types.TypedValue{
			types.FromGo(types.PropertyTypeEdge, edge),
		}})
	}
	out.RowCount = int64(len(out.Rows))
	return out
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
				NodesExpected: chunk.Stats.NodesExpected,
				EdgesExpected: chunk.Stats.EdgesExpected,
				NodesSkipped:  chunk.Stats.NodesSkipped,
				EdgesSkipped:  chunk.Stats.EdgesSkipped,
				Warnings:      chunk.Stats.Warnings,
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
// A bulk import runs for hours, well past the sign-in TTL, so it is the path
// most likely to meet an expired sign-in — far more so than a query, which
// lasts milliseconds and has been wrapped all along. The resend rule is the
// same one the query paths use: a sign-in refusal carries executed="false", so
// RefusedBeforeRun allows it, and nothing that may have run is sent again.
func (c *Client) StartBulkImport(ctx context.Context, graphName string, opts *BulkImportOptions) (*BulkImportSession, error) {
	svcOpts := (*services.BulkImportOptions)(nil)
	if opts != nil {
		svcOpts = &services.BulkImportOptions{
			EstimatedNodes: opts.EstimatedNodes,
			EstimatedEdges: opts.EstimatedEdges,
		}
	}

	var session *services.BulkImportSession
	err := c.withAutoReconnect(ctx, resendRule{}, func() error {
		var e error
		session, e = c.bulkImportSvc.StartBulkImport(ctx, graphName, svcOpts)
		return e
	})
	if err != nil {
		return nil, NewError(0, "start bulk import failed", err)
	}

	// Success comes from the server; StartBulkImport above returns an error when
	// the server refused the session, so reaching here means it was granted.
	return &BulkImportSession{
		SessionID: session.SessionID,
		Success:   session.Success,
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

// EndBulkImport ends a bulk import session; its data is kept.
//
// It waits until the server reports a final state, sending End again while
// the server answers that the End is still running (see
// BulkImportWaitOptions; pass one to change the total wait, the progress
// interval or to get progress reports). The result's State is ENDED when the
// session ended and kept its data. A final answer with Success=false (the
// session was aborted meanwhile, or its End failed) is returned as a result,
// not an error. When the wait ends first, the error is a
// *BulkImportInProgressError: the End still runs and has not failed.
//
// Each request signs in again on an expired sign-in and is sent again when the
// server refused it before running it (executed=false).
func (c *Client) EndBulkImport(ctx context.Context, sessionID string, opts ...BulkImportWaitOptions) (*EndBulkImportResult, error) {
	var result *services.EndBulkImportResult
	attempts, waited, err := c.waitBulkCall(ctx, "end", sessionID, opts,
		func(callCtx context.Context, capabilities []string) (bulkAnswer, error) {
			r, e := c.bulkImportSvc.EndBulkImport(callCtx, sessionID, capabilities...)
			if e != nil {
				return bulkAnswer{}, e
			}
			result = r
			return bulkAnswer{inProgress: r.InProgress, state: BulkImportState(r.State), progress: r.Progress,
				progressTotal: r.ProgressTotal, message: r.Message}, nil
		})
	if err != nil {
		var inProgress *BulkImportInProgressError
		if errors.As(err, &inProgress) {
			return nil, inProgress
		}
		return nil, NewError(0, "end bulk import failed", err)
	}

	return &EndBulkImportResult{
		Success:       result.Success,
		TotalRecords:  result.TotalRecords,
		DurationMs:    result.DurationMs,
		Message:       result.Message,
		State:         BulkImportState(result.State),
		Progress:      result.Progress,
		ProgressTotal: result.ProgressTotal,
		Attempts:      attempts,
		Waited:        waited,
	}, nil
}

// AbortBulkImport aborts a bulk import session: the server discards what the
// session wrote. A large discard takes about as long as the load.
//
// It waits as EndBulkImport does, sending Abort again while the server answers
// that the discard is still running; a repeated Abort waits for the same
// discard or answers with its outcome. The result carries the server's own
// Success and Message, the State (ABORTED once the data is discarded) and the
// nodes and edges removed. When the wait ends first, the error is a
// *BulkImportInProgressError: the discard still runs and has not failed.
//
// Like EndBulkImport, each request signs in again on an expired sign-in and is
// sent again when the server refused it before running it.
func (c *Client) AbortBulkImport(ctx context.Context, sessionID string, opts ...BulkImportWaitOptions) (*AbortBulkImportResult, error) {
	var result *services.AbortBulkImportResult
	attempts, waited, err := c.waitBulkCall(ctx, "abort", sessionID, opts,
		func(callCtx context.Context, capabilities []string) (bulkAnswer, error) {
			r, e := c.bulkImportSvc.AbortBulkImportWithResult(callCtx, sessionID, capabilities...)
			if e != nil {
				return bulkAnswer{}, e
			}
			result = r
			return bulkAnswer{inProgress: r.InProgress, state: BulkImportState(r.State), progress: r.Progress,
				progressTotal: r.ProgressTotal, nodesRemoved: r.NodesRemoved, edgesRemoved: r.EdgesRemoved,
				message: r.Message}, nil
		})
	if err != nil {
		var inProgress *BulkImportInProgressError
		if errors.As(err, &inProgress) {
			return nil, inProgress
		}
		return nil, NewError(0, "abort bulk import failed", err)
	}

	return &AbortBulkImportResult{
		Success:       result.Success,
		Message:       result.Message,
		State:         BulkImportState(result.State),
		Progress:      result.Progress,
		ProgressTotal: result.ProgressTotal,
		NodesRemoved:  result.NodesRemoved,
		EdgesRemoved:  result.EdgesRemoved,
		Attempts:      attempts,
		Waited:        waited,
	}, nil
}

// GetBulkImportStatus retrieves the status of a bulk import session: an open
// one, or one that finished in the last hour (IsActive false, State final).
func (c *Client) GetBulkImportStatus(ctx context.Context, sessionID string) (*BulkImportStatus, error) {
	var result *services.BulkImportStatus
	err := c.withAutoReconnect(ctx, resendRule{readOnly: true}, func() error {
		var e error
		result, e = c.bulkImportSvc.GetBulkImportStatus(ctx, sessionID)
		return e
	})
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
		State:               BulkImportState(result.State),
		Progress:            result.Progress,
		ProgressTotal:       result.ProgressTotal,
		Message:             result.Message,
		DiscardPending:      result.DiscardPending,
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

	resp := &Response{
		Columns:       svcResp.Columns,
		Rows:          rows,
		RowCount:      svcResp.RowCount,
		HasMore:       svcResp.HasMore,
		Warnings:      svcResp.Warnings,
		RowsAffected:  svcResp.RowsAffected,
		CurrentGraph:  svcResp.CurrentGraph,
		TimeCostNs:    svcResp.TimeCostNs,
		DiskCostNs:    svcResp.DiskCostNs,
		ComputeCostNs: svcResp.ComputeCostNs,
		DmlStats:      convertServiceDmlStats(svcResp.DmlStats),
	}
	resp.PropagateColumnNames()
	return resp
}

// convertServiceDmlStats bridges the internal services.DmlStats onto the
// public DmlStats. nil stays nil (not a DML query / old server), preserving
// the "absent != changed nothing" contract.
func convertServiceDmlStats(s *services.DmlStats) *DmlStats {
	if s == nil {
		return nil
	}
	return &DmlStats{
		InsertedNodes: s.InsertedNodes,
		InsertedEdges: s.InsertedEdges,
		DeletedNodes:  s.DeletedNodes,
		DeletedEdges:  s.DeletedEdges,
		SetNodes:      s.SetNodes,
		SetEdges:      s.SetEdges,
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
