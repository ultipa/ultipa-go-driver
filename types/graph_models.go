package types

// Node represents a node in the graph database.
type Node struct {
	ID         string
	Labels     []string
	Properties map[string]interface{}
}

// Edge represents an edge in the graph database.
type Edge struct {
	ID         string
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
