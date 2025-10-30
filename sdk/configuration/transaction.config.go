package configuration

// TransactionConfig holds transaction-specific configuration
type TransactionConfig struct {
	// ReadOnly indicates if this is a read-only transaction
	ReadOnly bool

	// IsolationLevel flag for transaction isolation
	IsolationLevel bool

	// Autocommit enables automatic commit mode
	Autocommit bool

	// TransactionTimeout is the transaction timeout in seconds
	TransactionTimeout uint32
}

// Validate checks if transaction config is valid
func (tc *TransactionConfig) Validate() error {
	// Currently no validation needed
	// Can be extended in the future if needed
	return nil
}
