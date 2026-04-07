package types

// PropertyType represents the type of a property value.
type PropertyType int32

const (
	PropertyTypeUnset         PropertyType = 0
	PropertyTypeInt32         PropertyType = 1
	PropertyTypeUint32        PropertyType = 2
	PropertyTypeInt64         PropertyType = 3
	PropertyTypeUint64        PropertyType = 4
	PropertyTypeFloat         PropertyType = 5
	PropertyTypeDouble        PropertyType = 6
	PropertyTypeString        PropertyType = 7
	PropertyTypeDatetime      PropertyType = 8  // Deprecated, use Timestamp
	PropertyTypeTimestamp     PropertyType = 9
	PropertyTypeText          PropertyType = 10
	PropertyTypeBlob          PropertyType = 11
	PropertyTypePoint         PropertyType = 12
	PropertyTypeDecimal       PropertyType = 13
	PropertyTypeList          PropertyType = 14
	PropertyTypeSet           PropertyType = 15
	PropertyTypeMap           PropertyType = 16
	PropertyTypeNull          PropertyType = 17
	PropertyTypeBool          PropertyType = 18
	PropertyTypeLocalDatetime PropertyType = 19
	PropertyTypeZonedDatetime PropertyType = 20
	PropertyTypeDate          PropertyType = 21
	PropertyTypeZonedTime     PropertyType = 22
	PropertyTypeLocalTime     PropertyType = 23
	PropertyTypeYearToMonth   PropertyType = 24
	PropertyTypeDayToSecond   PropertyType = 25
	PropertyTypeRecord        PropertyType = 26
	PropertyTypePoint3D       PropertyType = 27
	PropertyTypeVector        PropertyType = 28
	PropertyTypeTable         PropertyType = 29
	PropertyTypePath          PropertyType = 30 // Graph path with nodes and edges
	PropertyTypeError         PropertyType = 31 // Error value for TRY/CATCH handling
	PropertyTypeNode          PropertyType = 32 // Graph node
	PropertyTypeEdge          PropertyType = 33 // Graph edge
)

// GraphType represents the type of a graph.
type GraphType int32

const (
	GraphTypeOpen     GraphType = 0 // Schema-less graph
	GraphTypeClosed   GraphType = 1 // Schema-enforced graph
	GraphTypeOntology GraphType = 2 // Ontology-enabled graph
)

// HealthStatus represents the health status of a service.
type HealthStatus int32

const (
	HealthStatusUnknown        HealthStatus = 0
	HealthStatusServing        HealthStatus = 1
	HealthStatusNotServing     HealthStatus = 2
	HealthStatusServiceUnknown HealthStatus = 3
)

// CacheType represents the type of cache.
type CacheType int32

const (
	CacheTypeAll  CacheType = 0
	CacheTypeAST  CacheType = 1
	CacheTypePlan CacheType = 2
)
