package configuration

import ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"

type RequestConfig struct {
	// Existing fields
	Graph          string // Graphset Name
	Timeout        int32  // timeout (Seconds)
	Host           string // set for force host test
	Timezone       string // name of time zone , e.g. Aisa/Shanghai
	TimezoneOffset string // seconds that elapse from UTC, prior to TimeZone
	Thread         uint32 // used for uql request

	// Session/Transaction fields (s5.3)
	SessionID     uint64 // Session identifier (0 = not set)
	TransactionID uint64 // Transaction identifier (0 = not set)

	// Transaction configuration (grouped per design)
	TransactionConfig *TransactionConfig // Transaction-specific config (nil = no transaction config)
}

// HasSession returns true if SessionID is set
func (rc *RequestConfig) HasSession() bool {
	return rc.SessionID != 0
}

// HasTransaction returns true if TransactionID is set
func (rc *RequestConfig) HasTransaction() bool {
	return rc.TransactionID != 0
}

// ValidateTransaction validates transaction configuration
func (rc *RequestConfig) ValidateTransaction() error {
	if rc.TransactionConfig != nil {
		return rc.TransactionConfig.Validate()
	}
	return nil
}

// MergeWithSessionDefaults merges a user-provided RequestConfig with session defaults
// Priority (highest to lowest):
// 1. User-provided config (override)
// 2. Session defaults (sessionConfig)
// 3. Zero values (empty/0)
//
// sessionID and transactionID are always set from the parameters
// Note: sessionID=0 and transactionID=0 are valid (means no session/transaction)
func MergeWithSessionDefaults(override *RequestConfig, sessionConfig *SessionConfig, sessionID, transactionID uint64) *RequestConfig {
	// Start with session/transaction IDs
	merged := &RequestConfig{
		SessionID:     sessionID,
		TransactionID: transactionID,
	}

	// If override is provided, copy its values
	if override != nil {
		merged.Graph = override.Graph
		merged.Timeout = override.Timeout
		merged.Host = override.Host
		merged.Timezone = override.Timezone
		merged.TimezoneOffset = override.TimezoneOffset
		merged.Thread = override.Thread
		merged.TransactionConfig = override.TransactionConfig
	}

	// Apply session config defaults for empty values
	if sessionConfig != nil {
		if merged.Graph == "" {
			merged.Graph = sessionConfig.Graph
		}
		if merged.Timeout == 0 {
			merged.Timeout = sessionConfig.Timeout
		}
		if merged.Timezone == "" {
			merged.Timezone = sessionConfig.Timezone
		}
		if merged.TimezoneOffset == "" {
			merged.TimezoneOffset = sessionConfig.TimezoneOffset
		}
		if merged.Thread == 0 {
			merged.Thread = sessionConfig.Thread
		}
	}

	return merged
}

type InsertRequestConfig struct {
	*RequestConfig
	InsertType ultipa.InsertType // used for insertBulkNodes/Edges
	//CreateNodeIfNotExist bool              // used for insertBulkEdges
	Silent bool // if returns new ids
}
