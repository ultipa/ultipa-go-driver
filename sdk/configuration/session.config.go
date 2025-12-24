package configuration

// SessionConfig defines session-level configuration
// Provides defaults for session scope, can be overridden per request
type SessionConfig struct {
	// Graph settings
	Graph string // Override default graph for this session

	// Timeout settings
	Timeout int32 // Request timeout in seconds

	// Execution settings
	Thread uint32 // Number of threads for query execution

	// Timezone settings
	Timezone       string // Timezone name, e.g., "Asia/Shanghai"
	TimezoneOffset string // Timezone offset, e.g., "+08:00"
}

// Validate checks if session config is valid
func (sc *SessionConfig) Validate() error {
	// Currently no validation needed
	// Can be extended in the future if needed
	return nil
}
