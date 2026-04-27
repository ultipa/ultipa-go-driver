package gqldb

// This file re-exports all types from the types/ package for backward compatibility.
// All type definitions have been moved to focused modules in the types/ subdirectory.

import "github.com/ultipa/ultipa-go-driver/v6/types"

// Re-export enumerations
type PropertyType = types.PropertyType
type GraphType = types.GraphType
type HealthStatus = types.HealthStatus
type CacheType = types.CacheType

const (
	PropertyTypeUnset         = types.PropertyTypeUnset
	PropertyTypeInt32         = types.PropertyTypeInt32
	PropertyTypeUint32        = types.PropertyTypeUint32
	PropertyTypeInt64         = types.PropertyTypeInt64
	PropertyTypeUint64        = types.PropertyTypeUint64
	PropertyTypeFloat         = types.PropertyTypeFloat
	PropertyTypeDouble        = types.PropertyTypeDouble
	PropertyTypeString        = types.PropertyTypeString
	PropertyTypeDatetime      = types.PropertyTypeDatetime
	PropertyTypeTimestamp     = types.PropertyTypeTimestamp
	PropertyTypeText          = types.PropertyTypeText
	PropertyTypeBlob          = types.PropertyTypeBlob
	PropertyTypePoint         = types.PropertyTypePoint
	PropertyTypeDecimal       = types.PropertyTypeDecimal
	PropertyTypeList          = types.PropertyTypeList
	PropertyTypeSet           = types.PropertyTypeSet
	PropertyTypeMap           = types.PropertyTypeMap
	PropertyTypeNull          = types.PropertyTypeNull
	PropertyTypeBool          = types.PropertyTypeBool
	PropertyTypeLocalDatetime = types.PropertyTypeLocalDatetime
	PropertyTypeZonedDatetime = types.PropertyTypeZonedDatetime
	PropertyTypeDate          = types.PropertyTypeDate
	PropertyTypeZonedTime     = types.PropertyTypeZonedTime
	PropertyTypeLocalTime     = types.PropertyTypeLocalTime
	PropertyTypeYearToMonth   = types.PropertyTypeYearToMonth
	PropertyTypeDayToSecond   = types.PropertyTypeDayToSecond
	PropertyTypeRecord        = types.PropertyTypeRecord
	PropertyTypePoint3D       = types.PropertyTypePoint3D
	PropertyTypeVector        = types.PropertyTypeVector
	PropertyTypeTable         = types.PropertyTypeTable
	PropertyTypePath          = types.PropertyTypePath
	PropertyTypeError         = types.PropertyTypeError
	PropertyTypeNode          = types.PropertyTypeNode
	PropertyTypeEdge          = types.PropertyTypeEdge

	GraphTypeOpen     = types.GraphTypeOpen
	GraphTypeClosed   = types.GraphTypeClosed
	GraphTypeOntology = types.GraphTypeOntology

	HealthStatusUnknown        = types.HealthStatusUnknown
	HealthStatusServing        = types.HealthStatusServing
	HealthStatusNotServing     = types.HealthStatusNotServing
	HealthStatusServiceUnknown = types.HealthStatusServiceUnknown

	CacheTypeAll  = types.CacheTypeAll
	CacheTypeAST  = types.CacheTypeAST
	CacheTypePlan = types.CacheTypePlan
)

// Re-export core types
type TypedValue = types.TypedValue
type Parameter = types.Parameter

// Re-export functions
var NewTypedValue = types.NewTypedValue
var NewTypedValueFromString = types.NewTypedValueFromString
var NewParameter = types.NewParameter

// Re-export data types
type Point = types.Point
type Point3D = types.Point3D
type Decimal = types.Decimal
type Datetime = types.Datetime
type LocalDateTime = types.LocalDateTime
type ZonedDateTime = types.ZonedDateTime
type LocalTime = types.LocalTime
type ZonedTime = types.ZonedTime
type YearToMonth = types.YearToMonth
type DayToSecond = types.DayToSecond
type Vector = types.Vector
type Record = types.Record
type Set = types.Set
type GqldbDate = types.GqldbDate
type GqldbTable = types.GqldbTable

// Re-export graph models
type Node = types.Node
type Edge = types.Edge
type Path = types.Path
type NodeData = types.NodeData
type EdgeData = types.EdgeData
type ExportedNode = types.ExportedNode
type ExportedEdge = types.ExportedEdge

// Re-export metadata types
type GraphInfo = types.GraphInfo
type TransactionInfo = types.TransactionInfo
type ASTCacheStats = types.ASTCacheStats
type PlanCacheStats = types.PlanCacheStats
type CacheStats = types.CacheStats
type Statistics = types.Statistics

// Re-export bulk import types
type BulkCreateNodesOptions = types.BulkCreateNodesOptions
type BulkCreateEdgesOptions = types.BulkCreateEdgesOptions
type BulkImportOptions = types.BulkImportOptions
type BulkImportSession = types.BulkImportSession
type CheckpointResult = types.CheckpointResult
type EndBulkImportResult = types.EndBulkImportResult
type AbortBulkImportResult = types.AbortBulkImportResult
type BulkImportStatus = types.BulkImportStatus

// Re-export schema types
type Schema = types.Schema
type PropertyDef = types.PropertyDef
type Table = types.Table
type Header = types.Header
type Attr = types.Attr

// Re-export config types
type QueryConfig = types.QueryConfig
type InsertConfig = types.InsertConfig
type InsertNodesConfig = types.InsertNodesConfig
type InsertEdgesConfig = types.InsertEdgesConfig
type HealthWatcher = types.HealthWatcher

// Re-export AI convenience types.
type AiStage = types.AiStage

// Re-export InsertType constants used by InsertConfig.
type InsertType = types.InsertType

const (
	InsertTypeNormal    = types.InsertTypeNormal
	InsertTypeOverwrite = types.InsertTypeOverwrite
)
