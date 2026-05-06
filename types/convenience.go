package types

import "strings"

// DBType distinguishes between node and edge operations.
type DBType int

const (
	DBTypeNode DBType = iota
	DBTypeEdge
)

// LabelInfo represents a label from SHOW LABELS.
//
// GQL supports multi-label nodes (e.g. (n:Person:Employee)), so the server
// returns labels as a list even when there is only one. Labels holds the raw
// list as returned by the server; single-label groups are length-1.
type LabelInfo struct {
	Labels []string // label names (multi-label aware)
	Type   string   // "NODE" or "EDGE"
}

// NodeTypeInfo represents a node type from SHOW NODE TYPES.
type NodeTypeInfo struct {
	Name       string
	Properties []PropertyDef
}

// EdgeTypeInfo represents an edge type from SHOW EDGE TYPES.
type EdgeTypeInfo struct {
	Name       string
	Properties []PropertyDef
}

// LabelDef defines a label with its properties, used when creating closed graphs.
type LabelDef struct {
	Name       string
	Properties []PropertyDef
}

// IndexProperty represents a property in an index definition.
type IndexProperty struct {
	Name         string
	PrefixLength int // 0 = no prefix, >0 = prefix length for STRING
}

// IndexInfo represents an index from SHOW INDEX.
type IndexInfo struct {
	IndexName    string
	EntityType   string // "NODE" or "EDGE"
	Label        string
	Property     string
	PrefixLength *int
	Status       string
	Progress     string
	IndexedCount int64
	TotalCount   int64
	Error        string
}

// FulltextInfo represents a fulltext index from SHOW FULLTEXT.
type FulltextInfo struct {
	IndexName  string
	EntityType string // "NODE" or "EDGE"
	SchemaName string
	Properties string
	Analyzer   string
	Status     string
	DocCount   int64
	Progress   string
}

// TaskInfo represents a task from SHOW TASKS.
type TaskInfo struct {
	TaskId        string
	Type          string
	Query         string
	AlgoName      string
	Status        string
	StartedAt     string
	Progress      string
	Parameters    string
	NodesWritten  int64
	ComputeTimeMs int64
	WriteTimeMs   int64
}

// ProcessInfo represents a process from TOP.
type ProcessInfo struct {
	QueryId    string
	QueryText  string
	StartTime  string
	DurationMs int64
	Status     string
}

// GraphStats represents statistics from db.stats().
type GraphStats struct {
	GraphName         string
	NodeCount         int64
	EdgeCount         int64
	LabelCounts       map[string]int64
	EdgeLabelCounts   map[string]int64
	NodePropertyStats map[string]map[string]int64
	EdgePropertyStats map[string]map[string]int64
}

// InsertType controls the insert mode for GQL-based insert operations.
//
//   - InsertTypeNormal:    INSERT — error if `_id` already exists.
//   - InsertTypeOverwrite: INSERT OVERWRITE — REPLACE existing entity
//     wholesale on duplicate `_id`. Properties not in the write are LOST.
//   - InsertTypeUpsert:    UPSERT — MERGE new properties into existing
//     entity on duplicate `_id`. Properties not in the write are
//     PRESERVED; properties in the write OVERWRITE existing values.
//     Falls back to plain insert when no `_id` matches.
//
// Overwrite and Upsert are different semantics. Choose Upsert (merge)
// for partial-update workloads where you don't want to lose existing
// fields; choose Overwrite (replace) for known-clean re-ingest of an
// entity's full state.
type InsertType int

const (
	InsertTypeNormal    InsertType = 0
	InsertTypeOverwrite InsertType = 1
	InsertTypeUpsert    InsertType = 2
)

// InsertResponse represents the response for GQL-based insert operations.
type InsertResponse struct {
	RowsAffected int64
}

// AlgoInfo represents an algorithm from SHOW ALGOS.
type AlgoInfo struct {
	Name        string
	Description string
	Version     string
	Parameters  string
}

// AiStage represents a single streaming stage row returned by
// CALL ai.read(...) or CALL ai.gql(...). Known stage values are:
// start / routing / intent_extraction / describe_algorithm /
// generation / validation / execution / final / error.
type AiStage struct {
	Stage        string      // stage name (see above)
	Detail       string      // human-readable detail for the stage
	ElapsedMs    int64       // elapsed milliseconds at this stage
	TokensInput  int64       // input tokens consumed at this stage
	TokensOutput int64       // output tokens produced at this stage
	TokensCached int64       // cached tokens reused at this stage
	Data         interface{} // stage-specific payload (typically map[string]interface{})
}

// ParsePropertyTypeString converts a GQL type name string (e.g. "INTEGER", "STRING")
// to a PropertyType. Returns PropertyTypeString for unrecognized types.
func ParsePropertyTypeString(s string) PropertyType {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "INT32":
		return PropertyTypeInt32
	case "UINT32":
		return PropertyTypeUint32
	case "INT64", "INTEGER":
		return PropertyTypeInt64
	case "UINT64":
		return PropertyTypeUint64
	case "FLOAT":
		return PropertyTypeFloat
	case "DOUBLE":
		return PropertyTypeDouble
	case "STRING":
		return PropertyTypeString
	case "DATETIME":
		return PropertyTypeDatetime
	case "TIMESTAMP":
		return PropertyTypeTimestamp
	case "TEXT":
		return PropertyTypeText
	case "BLOB":
		return PropertyTypeBlob
	case "POINT":
		return PropertyTypePoint
	case "DECIMAL":
		return PropertyTypeDecimal
	case "LIST":
		return PropertyTypeList
	case "SET":
		return PropertyTypeSet
	case "MAP":
		return PropertyTypeMap
	case "NULL":
		return PropertyTypeNull
	case "BOOL", "BOOLEAN":
		return PropertyTypeBool
	case "LOCAL_DATETIME":
		return PropertyTypeLocalDatetime
	case "ZONED_DATETIME":
		return PropertyTypeZonedDatetime
	case "DATE":
		return PropertyTypeDate
	case "ZONED_TIME":
		return PropertyTypeZonedTime
	case "LOCAL_TIME":
		return PropertyTypeLocalTime
	case "YEAR_TO_MONTH":
		return PropertyTypeYearToMonth
	case "DAY_TO_SECOND":
		return PropertyTypeDayToSecond
	case "POINT3D":
		return PropertyTypePoint3D
	case "VECTOR":
		return PropertyTypeVector
	default:
		return PropertyTypeString
	}
}

// PropertyTypeToGQL converts a PropertyType to its GQL type name string.
func PropertyTypeToGQL(pt PropertyType) string {
	switch pt {
	case PropertyTypeInt32:
		return "INT32"
	case PropertyTypeUint32:
		return "UINT32"
	case PropertyTypeInt64:
		return "INT64"
	case PropertyTypeUint64:
		return "UINT64"
	case PropertyTypeFloat:
		return "FLOAT"
	case PropertyTypeDouble:
		return "DOUBLE"
	case PropertyTypeString:
		return "STRING"
	case PropertyTypeDatetime:
		return "DATETIME"
	case PropertyTypeTimestamp:
		return "TIMESTAMP"
	case PropertyTypeText:
		return "TEXT"
	case PropertyTypeBlob:
		return "BLOB"
	case PropertyTypePoint:
		return "POINT"
	case PropertyTypeDecimal:
		return "DECIMAL"
	case PropertyTypeList:
		return "LIST"
	case PropertyTypeSet:
		return "SET"
	case PropertyTypeMap:
		return "MAP"
	case PropertyTypeBool:
		return "BOOL"
	case PropertyTypeLocalDatetime:
		return "LOCAL_DATETIME"
	case PropertyTypeZonedDatetime:
		return "ZONED_DATETIME"
	case PropertyTypeDate:
		return "DATE"
	case PropertyTypeZonedTime:
		return "ZONED_TIME"
	case PropertyTypeLocalTime:
		return "LOCAL_TIME"
	case PropertyTypeYearToMonth:
		return "YEAR_TO_MONTH"
	case PropertyTypeDayToSecond:
		return "DAY_TO_SECOND"
	case PropertyTypePoint3D:
		return "POINT3D"
	case PropertyTypeVector:
		return "VECTOR"
	default:
		return "STRING"
	}
}

// ParsePropertyString parses a comma-separated property definition string like
// "age INTEGER, name STRING" into a slice of PropertyDef.
func ParsePropertyString(s string) []PropertyDef {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	result := make([]PropertyDef, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		tokens := strings.Fields(part)
		if len(tokens) < 2 {
			continue
		}
		result = append(result, PropertyDef{
			Name: tokens[0],
			Type: ParsePropertyTypeString(tokens[1]),
		})
	}
	return result
}
