package types

// BulkCreateNodesOptions configures bulk node creation behavior.
type BulkCreateNodesOptions struct {
	Overwrite bool // Skip existence check and overwrite if node ID already exists
}

// BulkCreateEdgesOptions configures bulk edge creation behavior.
type BulkCreateEdgesOptions struct {
	SkipInvalidNodes bool // Skip edges where source/target node doesn't exist
}

// BulkImportOptions configures bulk import session creation.
type BulkImportOptions struct {
	EstimatedNodes int64 // Hint for pre-allocating node ID cache
	EstimatedEdges int64 // Hint for edge batch sizing
}

// BulkImportSession represents a bulk import session.
type BulkImportSession struct {
	SessionID string
	Success   bool
	Message   string
}

// CheckpointResult represents the result of a checkpoint operation.
type CheckpointResult struct {
	Success             bool
	RecordCount         int64
	LastCheckpointCount int64
	Message             string
}

// EndBulkImportResult represents the result of ending a bulk import session.
type EndBulkImportResult struct {
	Success      bool
	TotalRecords int64
	DurationMs   int64
	Message      string
}

// AbortBulkImportResult represents the result of aborting a bulk import session.
type AbortBulkImportResult struct {
	Success bool
	Message string
}

// BulkImportStatus represents the status of a bulk import session.
type BulkImportStatus struct {
	IsActive            bool
	GraphName           string
	RecordCount         int64
	LastCheckpointCount int64
	CreatedAt           int64
	LastActivity        int64
}
