package gqldb

import "errors"

// Common errors returned by the GQLDB client.
var (
	// Configuration errors
	ErrNoHosts        = errors.New("gqldb: no hosts configured")
	ErrInvalidTimeout = errors.New("gqldb: invalid timeout value")

	// Connection errors
	ErrNoConnection      = errors.New("gqldb: no connection available")
	ErrConnectionClosed  = errors.New("gqldb: connection closed")
	ErrConnectionFailed  = errors.New("gqldb: connection failed")
	ErrAllHostsFailed    = errors.New("gqldb: all hosts failed to connect")
	ErrHealthCheckFailed = errors.New("gqldb: health check failed")

	// Session errors
	ErrNotLoggedIn     = errors.New("gqldb: not logged in")
	ErrLoginFailed     = errors.New("gqldb: login failed")
	ErrLogoutFailed    = errors.New("gqldb: logout failed")
	ErrSessionExpired  = errors.New("gqldb: session expired")
	ErrInvalidSession  = errors.New("gqldb: invalid session")

	// Transaction errors
	ErrNoTransaction          = errors.New("gqldb: no active transaction")
	ErrTransactionFailed      = errors.New("gqldb: transaction failed")
	ErrTransactionNotFound    = errors.New("gqldb: transaction not found")
	ErrTransactionAlreadyOpen = errors.New("gqldb: transaction already open")

	// Query errors
	ErrQueryFailed   = errors.New("gqldb: query failed")
	ErrQueryTimeout  = errors.New("gqldb: query timeout")
	ErrInvalidQuery  = errors.New("gqldb: invalid query")
	ErrEmptyQuery    = errors.New("gqldb: empty query")

	// Graph errors
	ErrGraphNotFound     = errors.New("gqldb: graph not found")
	ErrGraphExists       = errors.New("gqldb: graph already exists")
	ErrCreateGraphFailed = errors.New("gqldb: create graph failed")
	ErrDropGraphFailed   = errors.New("gqldb: drop graph failed")

	// Data errors
	ErrInsertFailed = errors.New("gqldb: insert failed")
	ErrDeleteFailed = errors.New("gqldb: delete failed")
	ErrExportFailed = errors.New("gqldb: export failed")

	// Type errors
	ErrInvalidType     = errors.New("gqldb: invalid type")
	ErrTypeConversion  = errors.New("gqldb: type conversion failed")
	ErrUnsupportedType = errors.New("gqldb: unsupported type")
)

// GqldbError represents a GQLDB-specific error with additional context.
type GqldbError struct {
	Code    int
	Message string
	Cause   error
}

func (e *GqldbError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *GqldbError) Unwrap() error {
	return e.Cause
}

// NewError creates a new GqldbError.
func NewError(code int, message string, cause error) *GqldbError {
	return &GqldbError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
