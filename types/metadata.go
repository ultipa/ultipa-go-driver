package types

// GraphInfo contains information about a graph.
type GraphInfo struct {
	Name        string
	GraphType   GraphType
	NodeCount   int64
	EdgeCount   int64
	Description string
}

// TransactionInfo contains information about a transaction.
type TransactionInfo struct {
	TransactionID uint64
	SessionID     uint64
	GraphName     string
	ReadOnly      bool
	CreatedAt     int64
	DurationMs    int64
	InternalTxID  string
}

// ASTCacheStats contains AST cache statistics.
type ASTCacheStats struct {
	Hits      uint64
	Misses    uint64
	Evictions uint64
	Entries   int32
	HitRate   float64
}

// PlanCacheStats contains plan cache statistics.
type PlanCacheStats struct {
	Size     int32
	Capacity int32
	Hits     uint64
	Misses   uint64
	HitRate  float64
}

// CacheStats contains combined cache statistics.
type CacheStats struct {
	ASTStats  *ASTCacheStats
	PlanStats *PlanCacheStats
}

// Statistics contains database statistics.
type Statistics struct {
	NodeCount       uint64
	EdgeCount       uint64
	LabelCounts     map[string]uint64
	EdgeLabelCounts map[string]uint64
}
