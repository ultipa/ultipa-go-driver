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

type InsertRequestConfig struct {
	*RequestConfig
	InsertType ultipa.InsertType // used for insertBulkNodes/Edges
	//CreateNodeIfNotExist bool              // used for insertBulkEdges
	Silent bool // if returns new ids
}
