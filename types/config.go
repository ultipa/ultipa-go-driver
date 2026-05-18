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

// InsertConfig extends QueryConfig with options for convenience insert methods
// (InsertNodes / InsertEdges). It mirrors the Python SDK's InsertConfig which
// inherits from QueryConfig and adds insert_type.
//
// Since Go does not have real inheritance, QueryConfig is embedded and
// InsertType is the additional field. Callers may pass nil; in that case the
// defaults (graph from session, InsertTypeNormal) are used.
type InsertConfig struct {
	QueryConfig
	InsertType InsertType
}

// DeleteConfig extends QueryConfig with options for the new GQL-emitter
// delete API (DeleteNodesByIDs / DeleteNodesByCondition /
// DeleteEdgesByIDs / DeleteEdgesByCondition).
//
// Two knobs:
//
//   - ReturnDeleted (default true) emits "RETURN ..." so the response
//     carries full deleted node/edge data. Set false on bulk deletes
//     to save bandwidth; RowsAffected still carries the count.
//
//   - AllowDeleteAll (default false) safety latch. With empty
//     labels/ids AND empty where, the SDK would otherwise emit a
//     graph-wide delete; this flag must be explicitly true to opt in.
//
// Note Go's zero-value semantics: ReturnDeleted defaults to false
// (Go zero value). The SDK treats `nil` config OR a freshly-built
// `&DeleteConfig{...}` as "RETURN by default" — the explicit way to
// suppress RETURN is `cfg.ReturnDeleted = false` after setting
// `cfg.ReturnDeleted = true` is normally not needed.
//
// To match the "default true" semantics consistently across languages,
// use the constructor `NewDeleteConfig()` instead of `&DeleteConfig{}`.
type DeleteConfig struct {
	QueryConfig
	ReturnDeleted  bool
	AllowDeleteAll bool
}

// NewDeleteConfig returns a DeleteConfig with the SDK defaults
// (ReturnDeleted=true, AllowDeleteAll=false). Prefer this over
// `&DeleteConfig{}` because Go's zero-value bool is false, which would
// otherwise silently disable RETURN.
func NewDeleteConfig() *DeleteConfig {
	return &DeleteConfig{ReturnDeleted: true}
}

// InsertNodesConfig represents configuration for the InsertNodes RPC,
// including bulk-import sessions.
//
// Mode selects duplicate-`_id` semantics. See InsertType docs (in
// types/convenience.go) for the three values and their differences.
// Defaults to InsertTypeNormal (zero value).
type InsertNodesConfig struct {
	Mode                InsertType // Normal / Overwrite / Upsert
	BulkImportSessionID string     // Optional: bulk import session ID for auto-checkpoint
}

// InsertEdgesConfig represents configuration for the InsertEdges RPC,
// including bulk-import sessions.
//
// See InsertNodesConfig for the Mode semantic split. Edge
// InsertTypeOverwrite and InsertTypeUpsert require EDGE_ID enabled on
// the target graph.
type InsertEdgesConfig struct {
	SkipInvalidNodes    bool       // Skip edges where source/target node doesn't exist
	Mode                InsertType // Normal / Overwrite / Upsert
	BulkImportSessionID string     // Optional: bulk import session ID for auto-checkpoint
}

// HealthWatcher watches the health status of a service.
type HealthWatcher struct {
	stream interface{} // Internal gRPC stream
	Status chan HealthStatus
	Done   chan error
}
