package gqldb

import (
	"crypto/tls"
	"time"
)

// Config holds the configuration for connecting to GQLDB.
type Config struct {
	// Hosts is a list of server addresses in the format "host:port".
	Hosts []string

	// Username for authentication.
	Username string

	// Password for authentication.
	Password string

	// DefaultGraph is the default graph to use for queries.
	DefaultGraph string

	// Timeout is the default timeout for queries, used when no timeout is specified
	// in QueryConfig.Timeout and no context deadline is set.
	// When a context with a deadline (e.g., context.WithTimeout) is used, the context
	// deadline takes precedence over this default timeout.
	// Default is 30 seconds.
	Timeout time.Duration

	// MaxRecvSize is the maximum message size in bytes that the client can receive.
	// Default is 64MB.
	MaxRecvSize int

	// TLSConfig holds optional TLS configuration for secure connections.
	TLSConfig *tls.Config

	// PoolSize is the maximum number of connections per host.
	// Default is 10.
	PoolSize int

	// HealthCheckInterval is the interval between health checks.
	// Default is 30 seconds.
	HealthCheckInterval time.Duration

	// RetryCount is the number of times to retry a failed request.
	// Default is 3.
	RetryCount int

	// RetryDelay is the delay between retries.
	// Default is 100ms.
	RetryDelay time.Duration

	// SessionID is an optional stable per-client logical session id used
	// by the transaction-branch model (sent as `x-ultipa-session-id`
	// metadata when §2.1 opt-in is enabled in TRANSACTIONS_DRIVER_GUIDE.md).
	// Empty string (default) means the driver auto-generates one at
	// NewClient time. Override only when you need a stable id across
	// reconnects or for cross-channel session continuity.
	SessionID string

	// DisableUseGraph rejects caller-supplied GQL whose leading
	// keyword is USE (with or without GRAPH), which switches the session
	// graph. For multi-tenant callers that pin each
	// request with QueryConfig.GraphName and embed end-user text in the
	// query.
	//
	// Defense-in-depth only - it inspects query text and cannot constrain
	// what the connected account may touch, and SHOW GRAPHS still
	// enumerates every graph. Use per-tenant users + RBAC for an actual
	// tenant boundary. Applies to Gql/GqlStream/Explain/Profile; GQL the
	// driver builds itself (convenience DDL, loaders) is unaffected.
	// Off by default.
	//
	// Graph-lifecycle DDL is NOT blocked: `DROP GRAPH <own>` followed by
	// `CREATE GRAPH <own> AS COPY OF <victim>` reaches another tenant with
	// no USE at all. Pair this with read_only, under which the server
	// rejects those writes ([4016]).
	//
	// INTERIM MEASURE. It filters query text, so it covers the forms known
	// when it shipped and silently stops covering any graph-selection
	// syntax the server adds later. It exists to bridge the gap until the
	// server can enforce this. Once server-side enforcement is available,
	// this flag is deprecated and removed at the next major version.
	DisableUseGraph bool
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Hosts:               []string{"localhost:9000"},
		Timeout:             30 * time.Second,
		MaxRecvSize:         64 * 1024 * 1024, // 64MB
		PoolSize:            10,
		HealthCheckInterval: 30 * time.Second,
		RetryCount:          3,
		RetryDelay:          100 * time.Millisecond,
	}
}

// ConfigBuilder provides a fluent interface for building Config.
type ConfigBuilder struct {
	config *Config
}

// NewConfigBuilder creates a new ConfigBuilder with default values.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: DefaultConfig(),
	}
}

// Hosts sets the server hosts.
func (b *ConfigBuilder) Hosts(hosts ...string) *ConfigBuilder {
	b.config.Hosts = hosts
	return b
}

// Username sets the username for authentication.
func (b *ConfigBuilder) Username(username string) *ConfigBuilder {
	b.config.Username = username
	return b
}

// Password sets the password for authentication.
func (b *ConfigBuilder) Password(password string) *ConfigBuilder {
	b.config.Password = password
	return b
}

// DefaultGraph sets the default graph.
func (b *ConfigBuilder) DefaultGraph(graph string) *ConfigBuilder {
	b.config.DefaultGraph = graph
	return b
}

// Timeout sets the query timeout.
func (b *ConfigBuilder) Timeout(timeout time.Duration) *ConfigBuilder {
	b.config.Timeout = timeout
	return b
}

// TimeoutSeconds sets the query timeout in seconds (convenience method).
func (b *ConfigBuilder) TimeoutSeconds(seconds int) *ConfigBuilder {
	b.config.Timeout = time.Duration(seconds) * time.Second
	return b
}

// MaxRecvSize sets the maximum receive message size.
func (b *ConfigBuilder) MaxRecvSize(bytes int) *ConfigBuilder {
	b.config.MaxRecvSize = bytes
	return b
}

// TLS sets the TLS configuration.
func (b *ConfigBuilder) TLS(config *tls.Config) *ConfigBuilder {
	b.config.TLSConfig = config
	return b
}

// PoolSize sets the connection pool size per host.
func (b *ConfigBuilder) PoolSize(size int) *ConfigBuilder {
	b.config.PoolSize = size
	return b
}

// HealthCheckInterval sets the health check interval.
func (b *ConfigBuilder) HealthCheckInterval(interval time.Duration) *ConfigBuilder {
	b.config.HealthCheckInterval = interval
	return b
}

// RetryCount sets the number of retries for failed requests.
func (b *ConfigBuilder) RetryCount(count int) *ConfigBuilder {
	b.config.RetryCount = count
	return b
}

// RetryDelay sets the delay between retries.
func (b *ConfigBuilder) RetryDelay(delay time.Duration) *ConfigBuilder {
	b.config.RetryDelay = delay
	return b
}

// Build returns the configured Config.
func (b *ConfigBuilder) Build() *Config {
	return b.config
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if len(c.Hosts) == 0 {
		return ErrNoHosts
	}
	if c.Timeout < 0 {
		return ErrInvalidTimeout
	}
	if c.MaxRecvSize <= 0 {
		c.MaxRecvSize = 64 * 1024 * 1024 // Default to 64MB
	}
	if c.PoolSize <= 0 {
		c.PoolSize = 10 // Default pool size
	}
	return nil
}

// TimeoutSeconds returns the timeout in seconds as int (for gRPC compatibility).
func (c *Config) TimeoutSeconds() int {
	return int(c.Timeout.Seconds())
}
