package gqldb

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Connection represents a single gRPC connection to a GQLDB server.
type Connection struct {
	host       string
	conn       *grpc.ClientConn
	healthy    bool
	lastPing   time.Time
	// consecutiveUnhealthy counts how many consecutive health-check
	// ticks observed a non-READY state.  Reconnect is only triggered
	// after this reaches unhealthyReconnectThreshold, so a single
	// transient flicker during a busy in-flight RPC doesn't tear the
	// channel down and cancel every pending request.
	consecutiveUnhealthy int
	mu                   sync.RWMutex
}

// unhealthyReconnectThreshold is the number of consecutive unhealthy
// health-check ticks required before the pool replaces the channel.
// Three ticks gives a busy channel time to recover on its own before
// we interrupt it.
const unhealthyReconnectThreshold = 3

// ConnectionPool manages a pool of connections to GQLDB servers.
type ConnectionPool struct {
	config      *Config
	connections map[string]*Connection
	mu          sync.RWMutex
	closed      bool
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewConnectionPool creates a new connection pool.
func NewConnectionPool(config *Config) (*ConnectionPool, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	pool := &ConnectionPool{
		config:      config,
		connections: make(map[string]*Connection),
		stopCh:      make(chan struct{}),
	}

	// Initialize connections to all hosts
	for _, host := range config.Hosts {
		conn, err := pool.createConnection(host)
		if err != nil {
			// Log error but continue with other hosts
			continue
		}
		pool.connections[host] = conn
	}

	if len(pool.connections) == 0 {
		return nil, ErrAllHostsFailed
	}

	// Start health check goroutine
	if config.HealthCheckInterval > 0 {
		pool.wg.Add(1)
		go pool.healthCheckLoop()
	}

	return pool, nil
}

// createConnection creates a new gRPC connection to a host.
func (p *ConnectionPool) createConnection(host string) (*Connection, error) {
	var opts []grpc.DialOption

	// Set up credentials
	if p.config.TLSConfig != nil {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(p.config.TLSConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Set max receive message size
	opts = append(opts, grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(p.config.MaxRecvSize),
	))

	// Every call says which driver sends it (ClientHeader).
	opts = append(opts, clientHeaderDialOptions()...)

	// Set keepalive parameters
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                30 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: true,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, host, opts...)
	if err != nil {
		return nil, err
	}

	return &Connection{
		host:     host,
		conn:     conn,
		healthy:  true,
		lastPing: time.Now(),
	}, nil
}

// GetConnection returns a healthy connection from the pool.
func (p *ConnectionPool) GetConnection() (*grpc.ClientConn, error) {
	conn, err := p.healthyConnection()
	if err != nil {
		return nil, err
	}
	return conn.conn, nil
}

// healthyConnection returns a healthy pool entry.
func (p *ConnectionPool) healthyConnection() (*Connection, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, ErrConnectionClosed
	}

	// Find a healthy connection
	for _, conn := range p.connections {
		conn.mu.RLock()
		healthy := conn.healthy
		conn.mu.RUnlock()

		if healthy {
			return conn, nil
		}
	}

	return nil, ErrNoConnection
}

// currentConn returns the pool's connection to host as the pool holds it now,
// or nil when the pool has none.
func (p *ConnectionPool) currentConn(host string) *grpc.ClientConn {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if conn, ok := p.connections[host]; ok {
		return conn.conn
	}
	return nil
}

// hostConn sends each call over the pool's connection to one host as the pool
// holds it when the call starts. The pool replaces that connection when its
// health check finds it failing and on ForceReconnectAll; service clients
// built on a hostConn send their next call over the new connection, rather
// than staying on the one the pool let go of until the next Login.
type hostConn struct {
	pool *ConnectionPool
	host string
}

func (h *hostConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...grpc.CallOption) error {
	cc := h.pool.currentConn(h.host)
	if cc == nil {
		return ErrNoConnection
	}
	return cc.Invoke(ctx, method, args, reply, opts...)
}

func (h *hostConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	cc := h.pool.currentConn(h.host)
	if cc == nil {
		return nil, ErrNoConnection
	}
	return cc.NewStream(ctx, desc, method, opts...)
}

// GetConnectionForHost returns a connection to a specific host.
func (p *ConnectionPool) GetConnectionForHost(host string) (*grpc.ClientConn, error) {
	p.mu.RLock()
	conn, ok := p.connections[host]
	p.mu.RUnlock()

	if !ok {
		return nil, ErrNoConnection
	}

	conn.mu.RLock()
	defer conn.mu.RUnlock()

	if !conn.healthy {
		return nil, ErrConnectionFailed
	}

	return conn.conn, nil
}

// healthCheckLoop periodically checks the health of all connections.
func (p *ConnectionPool) healthCheckLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.checkHealth()
		}
	}
}

// checkHealth checks the health of all connections.
//
// Connectivity-state semantics:
//   - READY / IDLE        — healthy; reset the unhealthy counter.
//   - CONNECTING          — transient; leave the counter alone, don't
//     reconnect (the channel is healing itself).
//   - SHUTDOWN            — terminal; reconnect immediately.
//   - TRANSIENT_FAILURE   — count toward unhealthy; reconnect only after
//     unhealthyReconnectThreshold consecutive ticks.
//
// Reconnect replaces the pool's channel reference; it does NOT close
// the old channel, so in-flight RPCs continue to completion on it.
func (p *ConnectionPool) checkHealth() {
	p.mu.RLock()
	hosts := make([]string, 0, len(p.connections))
	for host := range p.connections {
		hosts = append(hosts, host)
	}
	p.mu.RUnlock()

	for _, host := range hosts {
		p.mu.RLock()
		conn, ok := p.connections[host]
		p.mu.RUnlock()

		if !ok {
			continue
		}

		state := conn.conn.GetState().String()

		switch state {
		case "READY", "IDLE":
			conn.mu.Lock()
			conn.healthy = true
			conn.consecutiveUnhealthy = 0
			conn.lastPing = time.Now()
			conn.mu.Unlock()

		case "CONNECTING":
			// Transient — let the channel heal itself.

		case "SHUTDOWN":
			conn.mu.Lock()
			conn.healthy = false
			conn.consecutiveUnhealthy = unhealthyReconnectThreshold
			conn.mu.Unlock()
			p.reconnect(host)

		default:
			// TRANSIENT_FAILURE or any other non-READY state.
			conn.mu.Lock()
			conn.consecutiveUnhealthy++
			shouldReconnect := conn.consecutiveUnhealthy >= unhealthyReconnectThreshold
			if shouldReconnect {
				conn.healthy = false
			}
			conn.mu.Unlock()
			if shouldReconnect {
				p.reconnect(host)
			}
		}
	}
}

// ForceReconnectAll rebuilds every host's gRPC connection in the pool.
// Called by the client on transport-level errors (UNAVAILABLE / connection
// reset) so the next RPC gets a fresh channel.  Like reconnect, this does
// NOT close old channels synchronously — in-flight RPCs on them complete
// before they're GC'd.
func (p *ConnectionPool) ForceReconnectAll() {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return
	}
	hosts := make([]string, 0, len(p.connections))
	for host := range p.connections {
		hosts = append(hosts, host)
	}
	p.mu.RUnlock()

	for _, host := range hosts {
		p.reconnect(host)
	}
}

// reconnect replaces the pool's channel for host with a fresh one.
//
// Critically, this does NOT close the old channel, so calls still running
// on it finish there; the next call goes over the new one (hostConn).
// Nothing closes the old channel later either: grpc-go has no finalizer
// for a *grpc.ClientConn, so it stays open until the process ends.
//
// Earlier versions closed the old channel synchronously here, which
// cancelled every pending RPC with a "Channel closed!"-style error.
func (p *ConnectionPool) reconnect(host string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	// Build a new connection up-front; only swap if it succeeds.
	newConn, err := p.createConnection(host)
	if err != nil {
		return
	}

	// Overwrite the pool entry.  The old Connection is now unreferenced
	// by the pool; calls still running on its *grpc.ClientConn finish
	// there.
	p.connections[host] = newConn
}

// Close closes all connections in the pool.
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}

	p.closed = true
	close(p.stopCh)

	// Close all connections
	for _, conn := range p.connections {
		conn.conn.Close()
	}
	p.mu.Unlock()

	// Wait for the health check only after letting go of the lock: a tick
	// that is running takes it to read the pool, and Close waiting for that
	// tick while holding it never returned.
	p.wg.Wait()
	return nil
}

// HealthyHostCount returns the number of healthy hosts.
func (p *ConnectionPool) HealthyHostCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := 0
	for _, conn := range p.connections {
		conn.mu.RLock()
		if conn.healthy {
			count++
		}
		conn.mu.RUnlock()
	}
	return count
}

// Hosts returns the list of configured hosts.
func (p *ConnectionPool) Hosts() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	hosts := make([]string, 0, len(p.connections))
	for host := range p.connections {
		hosts = append(hosts, host)
	}
	return hosts
}

// TLSConfigFromFiles creates a TLS configuration from certificate files.
func TLSConfigFromFiles(certFile, keyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
