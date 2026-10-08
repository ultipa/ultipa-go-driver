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
	mu              sync.RWMutex
	committed       bool
	rolledBack      bool
	// partlyCommitted: the commit answered 5024 (partly_stored): part of the
	// transaction's changes are stored. Never rolled back, never run again.
	partlyCommitted bool
	// signInLost: the sign-in that began the transaction expired; the server
	// refuses the transaction from any later sign-in. Nothing was committed.
	signInLost bool
	warnings   []string
}

// CommitResult is the server's answer to a commit (Client.CommitWithResult).
type CommitResult struct {
	Success bool
	Message string
	// Warnings of a commit that stored the transaction's changes and could
	// not finish something after them: a property index that missed a change
	// (out of use until ALTER INDEX ... REBUILD), the lookup of edges by _id
	// (rebuilt at the next open). The commit succeeded: do not run the
	// transaction again. Empty from a server older than the field (the warnings
	// are then only in Message). Transaction.Warnings returns them too.
	Warnings      []string
	TimeCostNs    int64
	DiskCostNs    int64
	ComputeCostNs int64
}

// TransactionManager manages transactions for the client.
type TransactionManager struct {
	transactions map[uint64]*Transaction
	// lost holds the ids of transactions whose sign-in expired, so a later
	// call naming one is refused without being sent.
	lost map[uint64]struct{}
	mu   sync.RWMutex
}

// NewTransactionManager creates a new transaction manager.
func NewTransactionManager() *TransactionManager {
	return &TransactionManager{
		transactions: make(map[uint64]*Transaction),
		lost:         make(map[uint64]struct{}),
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

// CommitWithWarnings marks a transaction as committed and keeps the
// warnings the server sent with the commit.
func (m *TransactionManager) CommitWithWarnings(txID uint64, warnings []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, ok := m.transactions[txID]
	if !ok {
		return ErrTransactionNotFound
	}

	tx.mu.Lock()
	tx.committed = true
	tx.warnings = append([]string(nil), warnings...)
	tx.mu.Unlock()

	delete(m.transactions, txID)
	return nil
}

// PartlyCommit marks a transaction whose commit stored part of its changes
// (the server's 5024, partly_stored=true). It is not rolled back: what is
// stored stays stored.
func (m *TransactionManager) PartlyCommit(txID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, ok := m.transactions[txID]
	if !ok {
		return ErrTransactionNotFound
	}

	tx.mu.Lock()
	tx.partlyCommitted = true
	tx.mu.Unlock()

	delete(m.transactions, txID)
	return nil
}

// MarkSignInLost marks a transaction whose sign-in expired: the server refuses
// it from a new sign-in, and nothing of it was committed. Later calls naming
// it are refused by the driver (IsSignInLost).
func (m *TransactionManager) MarkSignInLost(txID uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tx, ok := m.transactions[txID]; ok {
		tx.mu.Lock()
		tx.signInLost = true
		tx.mu.Unlock()
		delete(m.transactions, txID)
	}
	m.lost[txID] = struct{}{}
}

// IsSignInLost reports whether the transaction was marked by MarkSignInLost.
func (m *TransactionManager) IsSignInLost(txID uint64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.lost[txID]
	return ok
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
	m.lost = make(map[uint64]struct{})
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

// IsPartlyCommitted returns true if the commit stored part of the
// transaction's changes (the server's 5024, partly_stored=true). Such a
// transaction is not rolled back and must not be run again as it is.
func (tx *Transaction) IsPartlyCommitted() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.partlyCommitted
}

// IsSignInLost returns true if the sign-in that began the transaction expired.
// The driver signed in again, but the server refuses the transaction from the
// new sign-in; none of its changes were committed.
func (tx *Transaction) IsSignInLost() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.signInLost
}

// Warnings returns the warnings the server sent with the commit: the changes
// are stored, and something after them could not be finished (an index that
// missed a change, the lookup of edges by _id rebuilt at the next open).
func (tx *Transaction) Warnings() []string {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return append([]string(nil), tx.warnings...)
}

// IsActive returns true if the transaction is still active.
func (tx *Transaction) IsActive() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return !tx.committed && !tx.rolledBack && !tx.partlyCommitted && !tx.signInLost
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
