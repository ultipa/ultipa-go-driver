package gqldb

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
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
	// ErrPartlyCommitted matches any *PartlyCommittedError via errors.Is: a
	// commit that stored part of the transaction (5024). Never run it again.
	ErrPartlyCommitted = errors.New("gqldb: the commit stored part of the transaction")
	// ErrTransactionSignInExpired matches any *TransactionSignInExpiredError
	// via errors.Is: the sign-in that began the transaction expired.
	ErrTransactionSignInExpired = errors.New("gqldb: the sign-in that began the transaction expired")

	// Bulk import errors
	// ErrBulkImportInProgress matches any *BulkImportInProgressError via
	// errors.Is: the wait ended while the server still runs the End or Abort.
	ErrBulkImportInProgress = errors.New("gqldb: the bulk import End or Abort is still running on the server")

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
	// Code is the engine's error code, such as 5020 (CodeFulltextIndexLoading)
	// or 5024 (CodeWritesCommitted), taken from the error detail the server
	// sends with every error. 0 when the error has none: an error raised by
	// the driver itself, or a server that predates the detail.
	Code int
	// Reason is the stable reason the server sends with the code, such as
	// "FULLTEXT_INDEX_LOADING" (see the Reason constants). "" when there is none.
	Reason  string
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

// NewError creates a new GqldbError. With code 0, the code and the reason
// are taken from the error detail of the gRPC error in cause's chain, when the
// server sent one; they are never read from the message text.
func NewError(code int, message string, cause error) *GqldbError {
	e := &GqldbError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
	if code == 0 {
		e.Code, e.Reason = errorDetail(cause)
	}
	return e
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

// WriteConflictCode is the server error code carried by every write conflict
// (the same value as CodeWriteConflict).
const WriteConflictCode = CodeWriteConflict

// InTransactionHint is what a write conflict inside a transaction means for
// the caller. It ends the message of a *WriteConflictError from a statement of
// a transaction or from its Commit.
const InTransactionHint = "write conflict inside a transaction: the transaction is over and none of its " +
	"changes are stored; run the whole transaction again (WithTransactionRetry does this)"

// WriteConflictError reports that a request lost a write conflict (engine
// code 3011): nothing of it is stored.
//
// Under optimistic concurrency the first committer wins and the loser runs
// again, so this is a normal outcome rather than a failure. Surfacing it as a
// generic error makes correct programs look broken.
//
// Where it comes from:
//
//   - Commit, when an element this transaction read in an earlier statement
//     was changed by a transaction that committed in between.
//   - A statement, when a MERGE would have to wait on a key while this
//     transaction already holds another. Check for it around every statement
//     in the transaction body, not only around Commit.
//   - A request without a TransactionID: an auto-commit request whose
//     conflict the server could not settle by running the statement again
//     itself, or a request of a transaction opened with GQL text (START
//     TRANSACTION ... COMMIT), which the driver cannot tell apart.
//
// The driver never sends a request again after a write conflict. Inside a
// transaction the transaction is finished and none of its writes were
// applied: run the whole transaction again (its reads included), for example
// with WithTransactionRetry.
//
// Its Cause is the *GqldbError the call would otherwise have returned, so
// errors.As(err, &*GqldbError) and EngineCode(err) (3011) keep working. It is
// returned when the server's structured detail says so. A server older than
// the detail sends none: for a call that carries a transaction id (a statement
// with QueryConfig.TransactionID, or Commit) the driver then also returns it
// for gRPC status Aborted with a message that starts with "[3011]", and sets
// the Cause's Code and Reason to CodeWriteConflict and ReasonWriteConflict
// itself, so WithTransactionRetry works there too. Any other request's
// conflict from such a server is a plain *GqldbError (Code 0). Either way
// nothing is sent again by itself.
//
// Not to be confused with server code 3010 (nested BEGIN, rollback of a dead
// transaction), which is a caller mistake and must not be retried.
//
// One caveat, measured on 6.2.134, before building on this: retrying reduces
// but does not eliminate lost updates under concurrent load. An application
// that needs an exact count still needs a check of its own. (A single-statement
// read-modify-write such as `MATCH (n) SET n.v = n.v + 1`, which 6.2.134
// missed, is detected by the engine from dev 99601802 on.)
type WriteConflictError struct {
	// Message is the server's description of the conflict.
	Message string
	// Cause is the underlying error, if any.
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
// It is decided only from the server's structured detail: the ErrorInfo's
// engine code 3011 or its reason WRITE_CONFLICT, found anywhere in err's
// chain. An answer whose detail says part of the request is stored
// (PartlyStored) is not a write conflict: something was stored, so it must not
// be run again.
//
// The message text and the gRPC status alone never count here: a message can
// quote anything the caller stored, and an error without the detail (an older
// server) gives false. The driver reads such an error only for a call that
// carries a transaction id (isLegacyWriteConflict), where it returns a
// *WriteConflictError, for which IsWriteConflict is true.
func IsWriteConflict(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrWriteConflict) {
		return true
	}
	st, ok := grpcStatusOf(err)
	if !ok {
		return false
	}
	code, reason, hasInfo := errorInfoOf(st)
	if !hasInfo || PartlyStored(err) {
		return false
	}
	return code == CodeWriteConflict || reason == ReasonWriteConflict
}

// legacyConflictPrefix is how a server older than the error detail (gqldb-grpc
// 0246ef4, f7a7e78 and earlier) starts the message of a write conflict, which
// it answers with gRPC status Aborted.
const legacyConflictPrefix = "[3011]"

// isLegacyWriteConflict reports whether err is a write conflict as a server
// older than the error detail sends it: gRPC status Aborted, no GQLDB detail,
// and a message that starts with "[3011]".
//
// Only for a call that carries a transaction id (a statement of a transaction
// begun with BeginTransaction, or its Commit), where it makes the call return a
// *WriteConflictError and so lets WithTransactionRetry run the whole body
// again. It never sends one request again, and it is never used for a request
// without a transaction id, which may belong to a transaction opened with GQL
// text. A server that sends the detail is read from the detail only.
func isLegacyWriteConflict(err error) bool {
	st, ok := grpcStatusOf(err)
	if !ok || st.Code() != codes.Aborted || serverErrorInfo(st) != nil {
		return false
	}
	return strings.HasPrefix(st.Message(), legacyConflictPrefix)
}

// conflictError returns the *WriteConflictError a failed call returns, or nil
// when err is not a write conflict for it: the server's detail says so
// (IsWriteConflict), or, for a call that carries a transaction id
// (transactionID non-zero), an older server's answer does
// (isLegacyWriteConflict). wrapped is the *GqldbError the call would otherwise
// return; text is the conflict's message.
func conflictError(err error, transactionID uint64, wrapped *GqldbError, text string) error {
	switch {
	case IsWriteConflict(err):
	case transactionID != 0 && isLegacyWriteConflict(err):
		// An older server sends no code: the answer is a write conflict.
		wrapped.Code, wrapped.Reason = CodeWriteConflict, ReasonWriteConflict
	default:
		return nil
	}
	if transactionID != 0 {
		text += " (" + InTransactionHint + ")"
	}
	return &WriteConflictError{Message: text, Cause: wrapped}
}

// PartlyCommittedError reports a commit that stored part of the
// transaction's changes and then failed: the server's answer 5024
// (WRITES_COMMITTED) marked partly_stored=true, for example when a
// write-ahead log failed after part of the commit was written.
//
// The transaction is not rolled back -- what is stored stays stored -- and
// the driver never sends the commit again. Do not run the transaction again
// as it is: its changes would be applied twice. Find out what is stored
// first. Transaction.IsPartlyCommitted is true for it.
//
// Its Cause is the *GqldbError the commit would otherwise return, so
// EngineCode(err) is 5024 and ErrorReason(err) is "WRITES_COMMITTED".
type PartlyCommittedError struct {
	TransactionID uint64
	// Message is the server's own words, which say what part is stored.
	Message string
	Cause   error
}

func (e *PartlyCommittedError) Error() string {
	return fmt.Sprintf("gqldb: the commit of transaction %d stored part of its changes and then failed; "+
		"the transaction is not rolled back and must not be run again as it is (server: %s)", e.TransactionID, e.Message)
}

func (e *PartlyCommittedError) Unwrap() error { return e.Cause }

// Is lets errors.Is(err, ErrPartlyCommitted) match any partly committed commit.
func (e *PartlyCommittedError) Is(target error) bool { return target == ErrPartlyCommitted }

// TransactionSignInExpiredError reports a call made in a transaction whose
// sign-in expired. The driver signed in again with the stored credentials, so
// the next call works, but a transaction belongs to the sign-in that began it:
// the server refuses it from the new sign-in. None of its changes were
// committed. The driver does not send the call again and never begins a new
// transaction by itself: begin one and run the whole transaction again.
//
// It is returned for a statement, Commit or Rollback of such a transaction,
// whether the server answered UNAUTHENTICATED or the driver knew the
// transaction was begun under an earlier sign-in (then nothing is sent). It is
// returned also when signing in again failed at that moment (a dropped
// connection, say): the transaction is lost all the same, the message says the
// sign-in failed, and the next call signs in again.
type TransactionSignInExpiredError struct {
	TransactionID uint64
	// Cause is the server's answer, nil when nothing was sent.
	Cause error
	// signInErr is why signing in again failed, nil when it worked.
	signInErr error
}

func (e *TransactionSignInExpiredError) Error() string {
	var msg string
	if e.signInErr != nil {
		msg = fmt.Sprintf("gqldb: the sign-in expired while transaction %d was open, and signing in again failed "+
			"too (%v); the next call signs in again. A transaction belongs to the sign-in that began it, so "+
			"transaction %d cannot be used any more: none of its changes were committed; begin a new transaction "+
			"and run it again", e.TransactionID, e.signInErr, e.TransactionID)
	} else {
		msg = fmt.Sprintf("gqldb: the sign-in expired while transaction %d was open; the driver signed in again, "+
			"but a transaction belongs to the sign-in that began it, so transaction %d cannot be used any more: "+
			"none of its changes were committed; begin a new transaction and run it again", e.TransactionID, e.TransactionID)
	}
	if e.Cause != nil {
		msg += " (server: " + e.Cause.Error() + ")"
	}
	return msg
}

func (e *TransactionSignInExpiredError) Unwrap() error { return e.Cause }

// Is lets errors.Is(err, ErrTransactionSignInExpired) match it.
func (e *TransactionSignInExpiredError) Is(target error) bool {
	return target == ErrTransactionSignInExpired
}

// BulkImportInProgressError reports that EndBulkImport or AbortBulkImport
// stopped waiting -- the total wait (BulkImportWaitOptions.TotalTimeout,
// Config.BulkImportWaitTimeout) passed, or the context ended -- before the
// session reached a final state. State is the last state the server reported,
// and the message says what it means:
//
//   - ENDING or DISCARDING: the server still runs the End or the discard. It
//     has NOT failed. Call EndBulkImport / AbortBulkImport again: it waits
//     for the same work and returns its outcome.
//   - ACTIVE: the server had not reported the End or the discard as started.
//     The request was not sent (Attempts is 0), or it may not have reached
//     the server. Then the session is still open: send End (or Abort) again;
//     a repeated End is safe either way. An open session that receives
//     nothing is discarded by the server's idle cleanup, with its data.
type BulkImportInProgressError struct {
	SessionID string
	// Operation is "end" or "abort".
	Operation     string
	State         BulkImportState
	Progress      int64
	ProgressTotal int64
	// Message is the server's last words about it.
	Message string
	Waited  time.Duration
	// Attempts is how many End or Abort requests were sent.
	Attempts int
	// Cause is the context's error when the context ended the wait.
	Cause error
}

func (e *BulkImportInProgressError) Error() string {
	call := bulkCallName(e.Operation)
	work := "End"
	if e.Operation == "abort" {
		work = "discard"
	}
	waited := e.Waited.Round(time.Millisecond)
	var msg string
	switch {
	case e.State == BulkImportStateActive:
		if e.Attempts == 0 {
			msg = fmt.Sprintf("gqldb: %s of bulk import session %s stopped waiting after %s, before the request "+
				"was sent (state ACTIVE). The session is still open", call, e.SessionID, waited)
		} else {
			msg = fmt.Sprintf("gqldb: %s of bulk import session %s stopped waiting after %s, before the server "+
				"reported the %s as started (last state ACTIVE): the request may not have reached the server. "+
				"If it did not, the session is still open", call, e.SessionID, waited, work)
		}
		if e.Operation == "abort" {
			msg += " and holds the graph: call AbortBulkImport again, or the server's idle cleanup discards " +
				"the session after its idle time"
		} else {
			msg += ": call EndBulkImport again to keep its data (a repeated End is safe); unless End is sent, " +
				"the server's idle cleanup discards the session and its data after its idle time"
		}
	case e.State.IsFinal():
		msg = fmt.Sprintf("gqldb: %s of bulk import session %s stopped waiting after %s; the session reached "+
			"state %s on the server: call %s again for its outcome", call, e.SessionID, waited, e.State, call)
	default:
		msg = fmt.Sprintf("gqldb: %s of bulk import session %s is still running on the server after %s (state %s",
			call, e.SessionID, waited, e.State)
		if e.ProgressTotal > 0 {
			msg += fmt.Sprintf(", %d of %d records done", e.Progress, e.ProgressTotal)
		}
		msg += "); it has not failed: call " + call + " again to wait for its outcome (a repeated " +
			strings.TrimSuffix(call, "BulkImport") + " waits for the same work)"
	}
	if e.Message != "" {
		msg += " (server: " + e.Message + ")"
	}
	return msg
}

func (e *BulkImportInProgressError) Unwrap() error { return e.Cause }

// Is lets errors.Is(err, ErrBulkImportInProgress) match it.
func (e *BulkImportInProgressError) Is(target error) bool { return target == ErrBulkImportInProgress }
