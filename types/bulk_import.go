package types

import "time"

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

// BulkImportState is where a bulk import session stands, as the server
// reports it. ACTIVE, ENDING and DISCARDING are still running; ENDED, ABORTED
// and FAILED are final. BulkImportStateUnspecified ("") comes from a server
// older than the states (gqldb-grpc before push 10).
type BulkImportState string

const (
	BulkImportStateUnspecified BulkImportState = ""
	BulkImportStateActive      BulkImportState = "ACTIVE"     // open for inserts
	BulkImportStateEnding      BulkImportState = "ENDING"     // End is flushing and handing the load to compute
	BulkImportStateDiscarding  BulkImportState = "DISCARDING" // its data is being discarded (Abort, the idle cleanup, a discard resumed at start)
	BulkImportStateEnded       BulkImportState = "ENDED"      // final: End finished; the data is kept
	BulkImportStateAborted     BulkImportState = "ABORTED"    // final: the data is discarded (by its client, an administrator, the idle cleanup or a server stop)
	BulkImportStateFailed      BulkImportState = "FAILED"     // final: End failed, or the discard stopped on an error (Abort again retries it)
)

// IsFinal reports whether the state is final: ENDED, ABORTED or FAILED.
func (s BulkImportState) IsFinal() bool {
	return s == BulkImportStateEnded || s == BulkImportStateAborted || s == BulkImportStateFailed
}

// BulkImportProgress is what the server reported about an End or an Abort that
// is still running. EndBulkImport and AbortBulkImport pass one to
// BulkImportWaitOptions.OnProgress each time the server answers that the call
// is still running, and each time they read the state after a deadline.
type BulkImportProgress struct {
	SessionID string
	// Operation is "end" or "abort".
	Operation string
	State     BulkImportState
	// InProgress is true while the End or the discard still runs.
	InProgress bool
	// Progress and ProgressTotal: the records of the session's undo record
	// discarded so far, of all of them (0 and 0 for an End).
	Progress      int64
	ProgressTotal int64
	// NodesRemoved and EdgesRemoved: what an Abort has discarded so far.
	NodesRemoved int64
	EdgesRemoved int64
	// Message is the server's own words.
	Message string
	// Waited is how long the driver has waited for this call so far.
	Waited time.Duration
}

// EndBulkImportResult represents the result of ending a bulk import session.
type EndBulkImportResult struct {
	Success      bool
	TotalRecords int64
	// DurationMs is the server's time for its last answer.
	DurationMs int64
	Message    string
	// State is where the session stands: ENDED when it ended and kept its data;
	// "" from a server older than the states.
	State         BulkImportState
	Progress      int64
	ProgressTotal int64
	// Attempts is how many times End was sent (more than 1 when the server
	// answered that the End was still running, or a deadline passed).
	Attempts int
	// Waited is how long the driver waited for the End, all attempts together.
	Waited time.Duration
}

// AbortBulkImportResult represents the result of aborting a bulk import
// session: the server's own success and words. Abort discards what the
// session wrote.
type AbortBulkImportResult struct {
	Success bool
	Message string
	// State is ABORTED when the session's data is discarded; "" from a server
	// older than the states.
	State         BulkImportState
	Progress      int64
	ProgressTotal int64
	// NodesRemoved and EdgesRemoved: what the discard removed.
	NodesRemoved int64
	EdgesRemoved int64
	Attempts     int
	Waited       time.Duration
}

// BulkImportStatus represents the status of a bulk import session.
type BulkImportStatus struct {
	// IsActive is true while the session holds its graph: ACTIVE, ENDING,
	// DISCARDING, or a discard that has not finished.
	IsActive            bool
	GraphName           string
	RecordCount         int64
	LastCheckpointCount int64
	CreatedAt           int64
	LastActivity        int64
	// State is where the session stands; "" from a server older than the
	// states.
	State BulkImportState
	// Progress and ProgressTotal: the records of its undo record discarded so
	// far, of all of them (0 and 0 when it never discarded).
	Progress      int64
	ProgressTotal int64
	// Message is the session's record in words: its state, start, last write,
	// and how it ended ("aborted (idle)" for the idle cleanup's discard).
	Message string
	// DiscardPending: its discard has not finished and its undo record is
	// kept; the graph stays held. AbortBulkImport finishes it.
	DiscardPending bool
}
