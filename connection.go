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
	mu         sync.RWMutex
}

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
			return conn.conn, nil
		}
	}

	return nil, ErrNoConnection
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

		// Check connection state
		state := conn.conn.GetState()
		healthy := state.String() == "READY" || state.String() == "IDLE"

		conn.mu.Lock()
		conn.healthy = healthy
		if healthy {
			conn.lastPing = time.Now()
		}
		conn.mu.Unlock()

		// Try to reconnect if unhealthy
		if !healthy {
			p.reconnect(host)
		}
	}
}

// reconnect attempts to reconnect to a host.
func (p *ConnectionPool) reconnect(host string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	// Close existing connection
	if conn, ok := p.connections[host]; ok {
		conn.conn.Close()
		delete(p.connections, host)
	}

	// Create new connection
	newConn, err := p.createConnection(host)
	if err != nil {
		return
	}

	p.connections[host] = newConn
}

// Close closes all connections in the pool.
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	close(p.stopCh)

	// Close all connections
	for _, conn := range p.connections {
		conn.conn.Close()
	}

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
