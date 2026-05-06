package types

// Node represents a node in the graph database.
//
// `_id` and `_uuid` semantics (gqldb 6.1.147+):
//
//   - ID is the user-facing identifier — the user-provided string or
//     the system auto-generated form. Use this when round-tripping
//     identifiers through user code or external systems.
//   - UUID is the system numeric ID (uint64) formatted as decimal.
//     Stable, opaque, always available. Use this when you need a
//     stable internal handle: cache keys, log correlation, dedup
//     hashes, internal joins. Treat as a string in your code; do not
//     parse to uint64 unless you know your runtime preserves uint64
//     precision.
//
// UUID is empty when talking to a pre-6.1.147 server (old wire format
// has no InternalID trailer). Application code should fall back to ID
// for identity in that case.
type Node struct {
	ID         string
	UUID       string // System numeric ID, decimal-formatted; empty on pre-6.1.147 servers
	Labels     []string
	Properties map[string]interface{}
}

// Edge represents an edge in the graph database.
// See Node for the ID / UUID semantics — they apply identically.
type Edge struct {
	ID         string
	UUID       string // System numeric ID, decimal-formatted; empty on pre-6.1.147 servers
	Label      string
	FromNodeID string
	ToNodeID   string
	Properties map[string]interface{}
}

// Path represents a path in the graph database.
type Path struct {
	Nodes []*Node
	Edges []*Edge
}

// NodeData represents data for a node to be inserted.
type NodeData struct {
	ID         string                 // Optional custom node ID (if empty, auto-generated)
	Labels     []string
	Properties map[string]interface{}
}

// EdgeData represents data for an edge to be inserted.
type EdgeData struct {
	ID         string // Optional custom edge ID; requires EDGE_ID enabled on the target graph (if empty, auto-generated)
	Label      string
	FromNodeID string
	ToNodeID   string
	Properties map[string]interface{}
}

// ExportedNode represents a node exported from the database.
type ExportedNode struct {
	ID         string
	Labels     []string
	Properties map[string]interface{}
}

// ExportedEdge represents an edge exported from the database.
type ExportedEdge struct {
	ID         string
	Label      string
	FromNodeID string
	ToNodeID   string
	Properties map[string]interface{}
}

// GqldbError represents a database error (referenced from errors package).
type GqldbError struct {
	Code    int
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *GqldbError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}
