package gqldb

import (
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Common errors returned by the GQLDB client.
var (
	// Configuration errors
	ErrNoHosts = errors.New("gqldb: no hosts configured")
	// ErrGraphSwitchRejected matches any GraphSwitchRejectedError via
	// errors.Is, regardless of which keyword tripped the guard.
	ErrGraphSwitchRejected = errors.New("gqldb: query selects or replaces a graph")
	ErrInvalidTimeout      = errors.New("gqldb: invalid timeout value")

	// Connection errors
	ErrNoConnection      = errors.New("gqldb: no connection available")
	ErrConnectionClosed  = errors.New("gqldb: connection closed")
	ErrConnectionFailed  = errors.New("gqldb: connection failed")
	ErrAllHostsFailed    = errors.New("gqldb: all hosts failed to connect")
	ErrHealthCheckFailed = errors.New("gqldb: health check failed")

	// Session errors
	ErrNotLoggedIn    = errors.New("gqldb: not logged in")
	ErrLoginFailed    = errors.New("gqldb: login failed")
	ErrLogoutFailed   = errors.New("gqldb: logout failed")
	ErrSessionExpired = errors.New("gqldb: session expired")
	ErrInvalidSession = errors.New("gqldb: invalid session")

	// Transaction errors
	ErrNoTransaction          = errors.New("gqldb: no active transaction")
	ErrTransactionFailed      = errors.New("gqldb: transaction failed")
	ErrTransactionNotFound    = errors.New("gqldb: transaction not found")
	ErrTransactionAlreadyOpen = errors.New("gqldb: transaction already open")
	// ErrWriteConflict matches any *WriteConflictError via errors.Is. It is
	// the only retryable transaction error -- ErrTransactionFailed and the
	// 3010 conditions above are caller mistakes and must not be retried.
	ErrWriteConflict = errors.New("gqldb: write conflict: retry the transaction")

	// Query errors
	ErrQueryFailed  = errors.New("gqldb: query failed")
	ErrQueryTimeout = errors.New("gqldb: query timeout")
	ErrInvalidQuery = errors.New("gqldb: invalid query")
	ErrEmptyQuery   = errors.New("gqldb: empty query")

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

// GraphSwitchRejectedError is returned when a query selects or replaces a
// graph while Config.DisableUseGraph is enabled.
//
// See query_guard.go for what is blocked and why this is defense-in-depth
// rather than a tenant boundary.
type GraphSwitchRejectedError struct {
	// Keyword is the leading keyword phrase that triggered the rejection.
	Keyword string
}

func (e *GraphSwitchRejectedError) Error() string {
	if e.Keyword == "" {
		return "gqldb: query rejected: it selects or replaces a graph"
	}
	return "gqldb: query rejected: leading '" + e.Keyword + "' selects or " +
		"replaces a graph, and DisableUseGraph is enabled"
}

// Is lets errors.Is(err, ErrGraphSwitchRejected) match any keyword.
func (e *GraphSwitchRejectedError) Is(target error) bool {
	return target == ErrGraphSwitchRejected
}

// WriteConflictCode is the server error code carried by every write conflict.
const WriteConflictCode = 3011

// WriteConflictError reports that the transaction read a value another
// transaction changed. Retry it.
//
// Under optimistic concurrency the first committer wins and the loser
// retries, so this is a normal outcome rather than a failure. Surfacing it
// as a generic error makes correct programs look broken.
//
// Where it comes from:
//
//   - Commit, when an element this transaction read in an earlier statement
//     was changed by a transaction that committed in between.
//   - A statement, when a MERGE would have to wait on a key while this
//     transaction already holds another. Check for it around every statement
//     in the transaction body, not only around Commit.
//
// Either way the transaction is finished and its writes were never applied.
// Roll back best-effort and begin a new one to retry.
//
// Not to be confused with server code 3010 (nested BEGIN, rollback of a dead
// transaction), which is a caller mistake and must not be retried.
//
// Two caveats, both measured, before building on this:
//
//   - A single-statement read-modify-write such as
//     `MATCH (n) SET n.v = n.v + 1` is not detected yet, so the most natural
//     way to write a counter is the shape that is missed.
//   - Even for the detected two-statement form, retrying reduces but does not
//     eliminate lost updates under concurrent load. An application that needs
//     an exact count still needs a check of its own.
type WriteConflictError struct {
	// Message is the server's description of the conflict.
	Message string
	// Cause is the underlying gRPC error, if any.
	Cause error
}

func (e *WriteConflictError) Error() string {
	if e.Message == "" {
		return ErrWriteConflict.Error()
	}
	return "gqldb: " + e.Message
}

func (e *WriteConflictError) Unwrap() error { return e.Cause }

// Is lets errors.Is(err, ErrWriteConflict) match any write conflict.
func (e *WriteConflictError) Is(target error) bool { return target == ErrWriteConflict }

// IsWriteConflict reports whether err is a server-reported write conflict.
//
// It recognises two signals, because the server is mid-migration between
// them:
//
//   - the gRPC status Aborted, the standard "retry the transaction" code and
//     the one to rely on once the server maps conflicts to it;
//   - the "[3011]" marker in the message, which is how a conflict is
//     identifiable until that mapping lands. Today a conflict falls through
//     the server's error classifier and arrives as codes.Internal, so without
//     this arm nothing would match.
//
// Matching is on the bracketed "[3011]" specifically so that 3010 and any
// future 3011x code cannot match by accident.
func IsWriteConflict(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrWriteConflict) {
		return true
	}
	if st, ok := status.FromError(err); ok && st.Code() == codes.Aborted {
		return true
	}
	// status.FromError does not unwrap, so retry on the cause chain.
	for e := err; e != nil; e = errors.Unwrap(e) {
		if st, ok := status.FromError(e); ok && st.Code() == codes.Aborted {
			return true
		}
	}
	return strings.Contains(err.Error(), "[3011]")
}
