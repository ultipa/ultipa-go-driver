package types

// QueryConfig represents configuration for a GQL query.
type QueryConfig struct {
	GraphName     string
	TransactionID uint64
	// Timeout is the query timeout in seconds. If set to 0, the driver will use
	// the context deadline (if set) or fall back to the client's default timeout.
	// The timeout priority is: QueryConfig.Timeout > context deadline > client default.
	Timeout int
	// ReadOnly indicates if the query is read-only.
	ReadOnly bool
	// Parameters are the query parameters.
	Parameters map[string]interface{}
	// MaxPathResults limits the number of paths returned from path queries.
	// 0 means unlimited.
	MaxPathResults int64
}

// InsertNodesConfig represents configuration for inserting nodes.
type InsertNodesConfig struct {
	Overwrite           bool   // Skip existence check and overwrite if node ID already exists
	BulkImportSessionID string // Optional: bulk import session ID for auto-checkpoint
}

// InsertEdgesConfig represents configuration for inserting edges.
type InsertEdgesConfig struct {
	SkipInvalidNodes    bool   // Skip edges where source/target node doesn't exist
	BulkImportSessionID string // Optional: bulk import session ID for auto-checkpoint
}

// HealthWatcher watches the health status of a service.
type HealthWatcher struct {
	stream interface{} // Internal gRPC stream
	Status chan HealthStatus
	Done   chan error
}
