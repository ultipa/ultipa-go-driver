package session

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
)

// UltipaClient defines the interface for executing queries
// This breaks the circular dependency with sdk/api
type UltipaClient interface {
	Uql(uql string, config *configuration.RequestConfig) (*http.Response, error)
	Gql(gql string, config *configuration.RequestConfig) (*http.Response, error)
}

var (
	// ErrNoTransaction indicates no active transaction
	ErrNoTransaction = fmt.Errorf("no active transaction")

	// ErrTransactionActive indicates transaction already started
	ErrTransactionActive = fmt.Errorf("transaction already started")
)

// Session represents a database session with unique ID and optional transaction
// NOT thread-safe - caller must synchronize if used across goroutines
type Session struct {
	conn          UltipaClient
	sessionID     uint64
	transactionID uint64
	config        *configuration.SessionConfig
}

// NewSession creates a new session with auto-generated ID
func NewSession(conn UltipaClient, config *configuration.SessionConfig) (*Session, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	return &Session{
		conn:          conn,
		sessionID:     sessionID,
		transactionID: 0,
		config:        config,
	}, nil
}

// NewSessionWithID creates a session with specified ID
func NewSessionWithID(conn UltipaClient, sessionID uint64, config *configuration.SessionConfig) *Session {
	return &Session{
		conn:          conn,
		sessionID:     sessionID,
		transactionID: 0,
		config:        config,
	}
}

// SessionID returns the session identifier
func (s *Session) SessionID() uint64 {
	return s.sessionID
}

// TransactionID returns current transaction ID (0 if no active transaction)
func (s *Session) TransactionID() uint64 {
	return s.transactionID
}

// HasTransaction returns true if session has active transaction
func (s *Session) HasTransaction() bool {
	return s.transactionID != 0
}

// StartTransaction begins a new transaction
func (s *Session) StartTransaction(ctx context.Context) (*http.Response, error) {
	if s.HasTransaction() {
		return nil, fmt.Errorf("%w: transaction ID %d", ErrTransactionActive, s.transactionID)
	}

	config := s.buildRequestConfig()

	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	resp, err := s.conn.Gql("START TRANSACTION", config)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	if resp.IsSuccess() {
		s.transactionID = resp.TransactionID
	}

	return resp, nil
}

// Commit commits the current transaction
func (s *Session) Commit(ctx context.Context) (*http.Response, error) {
	if !s.HasTransaction() {
		return nil, ErrNoTransaction
	}

	config := s.buildRequestConfig()

	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	resp, err := s.conn.Gql("COMMIT", config)

	if err == nil && resp.IsSuccess() {
		s.transactionID = 0 // Clear transaction
	}

	return resp, err
}

// Rollback rolls back the current transaction
func (s *Session) Rollback(ctx context.Context) (*http.Response, error) {
	if !s.HasTransaction() {
		return nil, ErrNoTransaction
	}

	config := s.buildRequestConfig()

	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	resp, err := s.conn.Gql("ROLLBACK", config)

	if err == nil && resp.IsSuccess() {
		s.transactionID = 0 // Clear transaction
	}

	return resp, err
}

// UQL executes UQL within session context
func (s *Session) UQL(uql string, config *configuration.RequestConfig) (*http.Response, error) {
	mergedConfig := s.mergeConfig(config)
	return s.conn.Uql(uql, mergedConfig)
}

// GQL executes GQL within session context
func (s *Session) GQL(gql string, config *configuration.RequestConfig) (*http.Response, error) {
	mergedConfig := s.mergeConfig(config)
	return s.conn.Gql(gql, mergedConfig)
}

// UQLWithContext executes UQL with context support
func (s *Session) UQLWithContext(ctx context.Context, uql string, config *configuration.RequestConfig) (*http.Response, error) {
	// Check context before execution
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	mergedConfig := s.mergeConfig(config)
	return s.conn.Uql(uql, mergedConfig)
}

// GQLWithContext executes GQL with context support
func (s *Session) GQLWithContext(ctx context.Context, gql string, config *configuration.RequestConfig) (*http.Response, error) {
	// Check context before execution
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	mergedConfig := s.mergeConfig(config)
	return s.conn.Gql(gql, mergedConfig)
}

// buildRequestConfig creates config with session/transaction context
func (s *Session) buildRequestConfig() *configuration.RequestConfig {
	config := &configuration.RequestConfig{
		SessionID:     s.sessionID,
		TransactionID: s.transactionID,
	}

	if s.config != nil {
		config.Graph = s.config.Graph
		config.Timeout = s.config.Timeout
		config.Timezone = s.config.Timezone
		config.TimezoneOffset = s.config.TimezoneOffset
		config.Thread = s.config.Thread
	}

	return config
}

// mergeConfig merges session config with provided config
func (s *Session) mergeConfig(config *configuration.RequestConfig) *configuration.RequestConfig {
	if config == nil {
		return s.buildRequestConfig()
	}

	// Start with provided config
	merged := &configuration.RequestConfig{
		Graph:             config.Graph,
		Timeout:           config.Timeout,
		Host:              config.Host,
		Timezone:          config.Timezone,
		TimezoneOffset:    config.TimezoneOffset,
		Thread:            config.Thread,
		TransactionConfig: config.TransactionConfig,
	}

	// Override session/transaction from session context
	merged.SessionID = s.sessionID
	if s.HasTransaction() {
		merged.TransactionID = s.transactionID
	} else if config.TransactionID != 0 {
		merged.TransactionID = config.TransactionID
	}

	// Apply session config defaults if not overridden
	if s.config != nil {
		if merged.Graph == "" {
			merged.Graph = s.config.Graph
		}
		if merged.Timeout == 0 {
			merged.Timeout = s.config.Timeout
		}
		if merged.Timezone == "" {
			merged.Timezone = s.config.Timezone
		}
		if merged.TimezoneOffset == "" {
			merged.TimezoneOffset = s.config.TimezoneOffset
		}
		if merged.Thread == 0 {
			merged.Thread = s.config.Thread
		}
	}

	return merged
}

// generateSessionID creates a cryptographically random 64-bit session ID
func generateSessionID() (uint64, error) {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b[:]), nil
}

// Close gracefully closes session
// Automatically rolls back active transaction if any
// Note: Session keeps alive until explicitly closed or user logs out
func (s *Session) Close() error {
	if s.HasTransaction() {
		_, err := s.Rollback(nil)
		if err != nil {
			return fmt.Errorf("failed to rollback transaction on close: %w", err)
		}
	}
	return nil
}
