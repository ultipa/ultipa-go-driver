package session

import (
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
	hostName      string // Host affinity for transaction (ensures all transaction requests go to same host)
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

// StartTransaction begins a new transaction and returns a Transaction object
// Only one transaction can be active per session at a time
func (s *Session) StartTransaction(config *configuration.TransactionConfig) (*Transaction, error) {
	if s.HasTransaction() {
		return nil, fmt.Errorf("%w: transaction ID %d", ErrTransactionActive, s.transactionID)
	}

	reqConfig := s.buildRequestConfig()

	// Apply transaction config to the request
	if config != nil {
		reqConfig.TransactionConfig = config
	}

	resp, err := s.conn.Gql("START TRANSACTION", reqConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("start transaction failed: %s", resp.Status.Message)
	}

	// Server returns transaction ID in response
	if resp.TransactionID == 0 {
		return nil, fmt.Errorf("server did not return transaction ID")
	}

	s.transactionID = resp.TransactionID
	// Store the host that processed START TRANSACTION for affinity
	s.hostName = resp.HostName

	// Create and return Transaction object
	return newTransaction(s, resp.TransactionID, config), nil
}

// Uql executes UQL within session context
func (s *Session) Uql(uql string, config *configuration.RequestConfig) (*http.Response, error) {
	mergedConfig := configuration.MergeWithSessionDefaults(config, s.config, s.sessionID, s.transactionID)
	// Force to same host if in transaction (host affinity)
	if s.hostName != "" && mergedConfig.Host == "" {
		mergedConfig.Host = s.hostName
	}
	return s.conn.Uql(uql, mergedConfig)
}

// Gql executes GQL within session context
func (s *Session) Gql(gql string, config *configuration.RequestConfig) (*http.Response, error) {
	mergedConfig := configuration.MergeWithSessionDefaults(config, s.config, s.sessionID, s.transactionID)
	// Force to same host if in transaction (host affinity)
	if s.hostName != "" && mergedConfig.Host == "" {
		mergedConfig.Host = s.hostName
	}
	return s.conn.Gql(gql, mergedConfig)
}

// buildRequestConfig creates config with session/transaction context
func (s *Session) buildRequestConfig() *configuration.RequestConfig {
	return configuration.MergeWithSessionDefaults(nil, s.config, s.sessionID, s.transactionID)
}

// clearTransaction clears the transaction ID and host affinity (called by Transaction after commit/rollback)
func (s *Session) clearTransaction() {
	s.transactionID = 0
	s.hostName = "" // Clear host affinity when transaction ends
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

// Close gracefully closes session by sending SESSION CLOSE command
// If a transaction is active, it will be automatically rolled back by the server
func (s *Session) Close() error {
	config := s.buildRequestConfig()
	// Force to same host if in transaction (host affinity)
	if s.hostName != "" && config.Host == "" {
		config.Host = s.hostName
	}

	resp, err := s.conn.Gql("SESSION CLOSE", config)
	if err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("session close failed: %s", resp.Status.Message)
	}

	// Clear session state
	s.transactionID = 0
	s.hostName = ""

	return nil
}
