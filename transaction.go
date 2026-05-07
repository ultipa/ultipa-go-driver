package gqldb

import (
	"context"
	"sync"
	"time"
)

// Transaction represents an active database transaction.
type Transaction struct {
	ID        uint64
	SessionID uint64
	GraphName string
	ReadOnly  bool
	CreatedAt time.Time
	Timeout   time.Duration
	// ClientSessionID is the stable per-client logical session id surfaced
	// under the transaction-branch model. Distinct from SessionID (the
	// legacy uint64 from Login). Always populated by the driver. See
	// TRANSACTIONS_DRIVER_GUIDE.md §2.0–2.1.
	ClientSessionID string
	mu        sync.RWMutex
	committed bool
	rolledBack bool
}

// TransactionManager manages transactions for the client.
type TransactionManager struct {
	transactions map[uint64]*Transaction
	mu           sync.RWMutex
}

// NewTransactionManager creates a new transaction manager.
func NewTransactionManager() *TransactionManager {
	return &TransactionManager{
		transactions: make(map[uint64]*Transaction),
	}
}

// Begin creates a new transaction.
func (m *TransactionManager) Begin(txID, sessionID uint64, graphName string, readOnly bool, timeout time.Duration) *Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx := &Transaction{
		ID:        txID,
		SessionID: sessionID,
		GraphName: graphName,
		ReadOnly:  readOnly,
		CreatedAt: time.Now(),
		Timeout:   timeout,
	}

	m.transactions[txID] = tx
	return tx
}

// Commit marks a transaction as committed.
func (m *TransactionManager) Commit(txID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, ok := m.transactions[txID]
	if !ok {
		return ErrTransactionNotFound
	}

	tx.mu.Lock()
	tx.committed = true
	tx.mu.Unlock()

	delete(m.transactions, txID)
	return nil
}

// Rollback marks a transaction as rolled back.
func (m *TransactionManager) Rollback(txID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, ok := m.transactions[txID]
	if !ok {
		return ErrTransactionNotFound
	}

	tx.mu.Lock()
	tx.rolledBack = true
	tx.mu.Unlock()

	delete(m.transactions, txID)
	return nil
}

// Get returns a transaction by ID.
func (m *TransactionManager) Get(txID uint64) *Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transactions[txID]
}

// GetActive returns all active transactions.
func (m *TransactionManager) GetActive() []*Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	txs := make([]*Transaction, 0, len(m.transactions))
	for _, tx := range m.transactions {
		txs = append(txs, tx)
	}
	return txs
}

// GetActiveForSession returns all active transactions for a session.
func (m *TransactionManager) GetActiveForSession(sessionID uint64) []*Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var txs []*Transaction
	for _, tx := range m.transactions {
		if tx.SessionID == sessionID {
			txs = append(txs, tx)
		}
	}
	return txs
}

// HasActive returns true if there are any active transactions.
func (m *TransactionManager) HasActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.transactions) > 0
}

// Count returns the number of active transactions.
func (m *TransactionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.transactions)
}

// ClearAll clears all transactions (used during logout).
func (m *TransactionManager) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transactions = make(map[uint64]*Transaction)
}

// IsCommitted returns true if the transaction was committed.
func (tx *Transaction) IsCommitted() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.committed
}

// IsRolledBack returns true if the transaction was rolled back.
func (tx *Transaction) IsRolledBack() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.rolledBack
}

// IsActive returns true if the transaction is still active.
func (tx *Transaction) IsActive() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return !tx.committed && !tx.rolledBack
}

// Age returns how long the transaction has been active.
func (tx *Transaction) Age() time.Duration {
	return time.Since(tx.CreatedAt)
}

// IsExpired returns true if the transaction has exceeded its timeout.
func (tx *Transaction) IsExpired() bool {
	if tx.Timeout == 0 {
		return false
	}
	return tx.Age() > tx.Timeout
}

// TransactionContext is a helper for using transactions with context.
type TransactionContext struct {
	ctx context.Context
	tx  *Transaction
	client *Client
}

// NewTransactionContext creates a new transaction context.
func NewTransactionContext(ctx context.Context, client *Client, tx *Transaction) *TransactionContext {
	return &TransactionContext{
		ctx:    ctx,
		tx:     tx,
		client: client,
	}
}

// Execute runs a function within the transaction, automatically committing on success
// or rolling back on error.
func (tc *TransactionContext) Execute(fn func(ctx context.Context, txID uint64) error) error {
	err := fn(tc.ctx, tc.tx.ID)
	if err != nil {
		// Rollback on error
		tc.client.Rollback(tc.ctx, tc.tx.ID)
		return err
	}

	// Commit on success
	_, err = tc.client.Commit(tc.ctx, tc.tx.ID)
	return err
}
