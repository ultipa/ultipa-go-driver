package gqldb

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The server sends every error with one google.rpc.ErrorInfo in its gRPC
// status details: domain "gqldb.ultipa.com", a stable reason such as
// "FULLTEXT_INDEX_LOADING", and metadata["code"], the engine's error code in
// decimal ("0" when the error has none); metadata["executed"] = "false"
// when the server refused the request before any of it ran; and
// metadata["partly_stored"] = "true" when some or all of the request's
// changes are stored although it failed (never together with executed). The driver
// decides what an error means from the reason and the code, never from the
// message text, which can quote names the caller chose. A server that
// predates the detail sends none; the code is then 0 and the reason "".

// ErrorInfoDomain is the domain of the server's ErrorInfo.
const ErrorInfoDomain = "gqldb.ultipa.com"

// Engine codes an application commonly acts on. GqldbError.Code and
// EngineCode return them.
const (
	CodeWriteConflict        = 3011 // a write conflict, nothing stored: run the whole transaction again (the driver never sends it again)
	CodeReadOnly             = 4016 // a write in a read-only database or a read-only request
	CodeFulltextIndexLoading = 5020 // a fulltext index exists but cannot be searched yet
	CodeGraphReadOnly        = 5021 // a write on a read-only (lake) graph
	CodeGraphUnavailable     = 5022 // a lake graph that is declared but could not be opened
	CodeGraphRestoring       = 5023 // a lake graph being restored
	CodeWritesCommitted      = 5024 // the request's changes, or some of them, are stored; never run it again as it is
	CodeGraphBusy            = 5025 // a graph a bulk import holds (a session open, ending, or being discarded); the same action works once it finishes
	CodeLicenseNodeLimit     = 6010 // the write would pass the licence's node limit
	CodeLicenseEdgeLimit     = 6011 // the write would pass the licence's edge limit
	CodeLicenseGraphLimit    = 6012 // the graph would pass the licence's graph limit
	CodeLicenseReadOnly      = 6020 // a write while the licence is in read-only mode
	CodePermissionDenied     = 7021 // the account lacks a privilege
)

// Reasons an application or the driver acts on. GqldbError.Reason and
// ErrorReason return them.
const (
	ReasonLeaderChanged        = "LEADER_CHANGED"         // HA: the write reached a server that is not the leader
	ReasonFulltextIndexLoading = "FULLTEXT_INDEX_LOADING" // code 5020
	ReasonWritesCommitted      = "WRITES_COMMITTED"       // code 5024
	ReasonWritesPending        = "WRITES_PENDING"         // the statement's changes are in the open transaction; do not run it again in it
	ReasonWriteConflict        = "WRITE_CONFLICT"         // code 3011: a write conflict, nothing of the request stored
	ReasonGraphBusy            = "GRAPH_BUSY"             // code 5025: a bulk import holds the graph
)

// maxRetryBackoff caps the wait between two attempts of a read that met a
// loading fulltext index.
const maxRetryBackoff = 2 * time.Second

// grpcStatusOf returns the gRPC status inside err's chain, as the server sent
// it: its own message, not the text of the errors wrapped around it.
func grpcStatusOf(err error) (*status.Status, bool) {
	var gs interface{ GRPCStatus() *status.Status }
	if err == nil || !errors.As(err, &gs) {
		return nil, false
	}
	st := gs.GRPCStatus()
	return st, st != nil
}

// errorInfoOf returns the engine code and reason of the server's ErrorInfo in
// st; ok is false when st carries none (an older server).
func errorInfoOf(st *status.Status) (code int, reason string, ok bool) {
	info := serverErrorInfo(st)
	if info == nil {
		return 0, "", false
	}
	code, _ = strconv.Atoi(info.GetMetadata()["code"])
	return code, info.GetReason(), true
}

// serverErrorInfo returns the server's ErrorInfo in st, or nil.
func serverErrorInfo(st *status.Status) *errdetails.ErrorInfo {
	for _, d := range st.Details() {
		if info, isInfo := d.(*errdetails.ErrorInfo); isInfo && info.GetDomain() == ErrorInfoDomain {
			return info
		}
	}
	return nil
}

// RefusedBeforeRun reports whether the server's detail on err says it refused
// the request before any of it ran (the metadata executed=false): no session,
// an expired or unknown one, a method permission, the rate limit, or an HA
// follower's LEADER_CHANGED. Sending such a request again cannot repeat a
// change. False when the detail is missing (an older server) or does not say
// so: the request may have run.
func RefusedBeforeRun(err error) bool {
	st, ok := grpcStatusOf(err)
	if !ok {
		return false
	}
	info := serverErrorInfo(st)
	return info != nil && info.GetMetadata()["executed"] == "false"
}

// PartlyStored reports whether the server's detail on err says some or all of
// the request's changes are stored although it failed (the metadata
// partly_stored=true): a statement failed after earlier statements of the
// request had committed, or a [5024] answer. Never run such a request again as
// it is; the driver never sends it again on any path, a 5020 included. False
// when the detail is missing (an older server) or does not say so. The message
// text is never read.
func PartlyStored(err error) bool {
	st, ok := grpcStatusOf(err)
	if !ok {
		return false
	}
	info := serverErrorInfo(st)
	return info != nil && info.GetMetadata()["partly_stored"] == "true"
}

// resendRule decides whether a call may be sent again after it failed, on a
// path other than the 5020 retry: after signing in again, or to another host
// in HA. A call is sent again only when the server refused it before running
// it (RefusedBeforeRun), or when it is read-only, which the server enforces by
// refusing any write in it; and, for a stream, only while no rows have reached
// the caller. A request that may have written is never sent again.
type resendRule struct {
	readOnly  bool        // read-only, outside a transaction (isReadOnlyRequest)
	delivered func() bool // a stream: true once rows reached the callback
	// transactionID: the call is a statement of this transaction. After an
	// expired sign-in it is never sent again: the transaction belongs to the
	// sign-in that began it, and the server refuses it from a new one.
	transactionID uint64
}

// isDelivered reports whether rows of a stream have reached the caller: the
// stream is then never sent again, on any path.
func (r resendRule) isDelivered() bool {
	return r.delivered != nil && r.delivered()
}

// allows reports whether the call that failed with err may be sent again.
// Never when err says part of the request is stored (PartlyStored).
func (r resendRule) allows(err error) bool {
	if r.isDelivered() || PartlyStored(err) {
		return false
	}
	return r.readOnly || RefusedBeforeRun(err)
}

// mayGoToAnotherServer reports whether a call that a server answered with err
// may be sent to another server (HA: the leader after a follower, or the next
// host in the rotation): a LEADER_CHANGED answer, which a follower gives before
// running anything, or any error allows lets the call be sent again. Never a
// stream whose rows have reached the caller, and never an answer that says part
// of the request is stored, whatever else it says.
func (r resendRule) mayGoToAnotherServer(err error) bool {
	if r.isDelivered() || PartlyStored(err) {
		return false
	}
	return isLeaderChangedError(err) || r.allows(err)
}

// transactionIDOf returns the transaction a request runs in, 0 for none.
func transactionIDOf(config *QueryConfig) uint64 {
	if config == nil {
		return 0
	}
	return config.TransactionID
}

// isReadOnlyRequest reports whether config marks a request read-only outside
// a transaction: the server refuses any write in it.
func isReadOnlyRequest(config *QueryConfig) bool {
	return config != nil && config.ReadOnly && config.TransactionID == 0
}

// errorDetail returns the engine code and reason the server attached to err,
// found anywhere in err's chain. 0 and "" when there is none.
func errorDetail(err error) (int, string) {
	st, ok := grpcStatusOf(err)
	if !ok {
		return 0, ""
	}
	code, reason, _ := errorInfoOf(st)
	return code, reason
}

// EngineCode returns the engine error code carried by err: the Code of a
// *GqldbError in its chain, or the code the server attached to a gRPC error.
// 0 when there is none. It never reads the message text.
func EngineCode(err error) int {
	var ge *GqldbError
	if errors.As(err, &ge) && ge.Code != 0 {
		return ge.Code
	}
	code, _ := errorDetail(err)
	return code
}

// ErrorReason returns the stable reason the server attached to err (see the
// Reason constants), or "" when there is none.
func ErrorReason(err error) string {
	var ge *GqldbError
	if errors.As(err, &ge) && ge.Reason != "" {
		return ge.Reason
	}
	_, reason := errorDetail(err)
	return reason
}

// isLeaderChangedError reports whether err is the server's LEADER_CHANGED
// answer: a write that reached a server that is not the leader in HA mode
// (design §12). It is FAILED_PRECONDITION with the reason LEADER_CHANGED.
//
// A server that predates the error detail (the HA branch before it adopted
// it) sends no ErrorInfo; for such a status only, the server's own message
// must be the bare marker: exactly "LEADER_CHANGED", or "LEADER_CHANGED
// leader=<address>". When the status has the detail, its reason alone
// decides. The word anywhere else in a message never counts: a fulltext error
// naming an index "leader_changed_idx" used to send a write to another server.
func isLeaderChangedError(err error) bool {
	st, ok := grpcStatusOf(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		return false
	}
	if _, reason, hasInfo := errorInfoOf(st); hasInfo {
		return reason == ReasonLeaderChanged
	}
	msg := st.Message()
	return msg == ReasonLeaderChanged || strings.HasPrefix(msg, ReasonLeaderChanged+" leader=")
}

// retryBackoff is the wait before retry number attempt (0 for the first):
// base doubled each time, never more than maxRetryBackoff.
func retryBackoff(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		return 0
	}
	d := base
	for i := 0; i < attempt && d < maxRetryBackoff; i++ {
		d *= 2
	}
	if d > maxRetryBackoff {
		d = maxRetryBackoff
	}
	return d
}

// mayRetryRead reports whether a request that failed with err may be sent
// again: only a read-only request outside a transaction (the server refuses
// any write in a read-only request, so the failed attempt wrote nothing), and
// only for the engine code 5020, a fulltext index still loading, which clears
// by itself. Every other code is final, 5024, 4016, 6020 and 3011 included,
// and a request that may write is never sent twice: a write conflict is
// returned to the caller, in auto-commit as in a transaction. An answer that says part
// of the request is stored (PartlyStored) is never retried, a 5020 included.
func mayRetryRead(config *QueryConfig, err error) bool {
	if !isReadOnlyRequest(config) || PartlyStored(err) {
		return false
	}
	code, _ := errorDetail(err)
	return code == CodeFulltextIndexLoading
}

// retryRead runs attempt, and runs it again while it fails with an error
// mayRetryRead allows, at most retries more times, waiting retryBackoff
// between attempts. canRetry is asked before each retry (a stream that has
// handed rows to the caller cannot start over). It stops early, returning the
// last error, when ctx ends or its deadline is closer than the next wait.
func retryRead(ctx context.Context, config *QueryConfig, retries int, base time.Duration,
	sleep func(context.Context, time.Duration) error, canRetry func() bool, attempt func() error) error {
	err := attempt()
	for n := 0; err != nil && n < retries && mayRetryRead(config, err) && canRetry(); n++ {
		wait := retryBackoff(base, n)
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= wait {
			return err
		}
		if serr := sleep(ctx, wait); serr != nil {
			return err
		}
		err = attempt()
	}
	return err
}

// sleepContext waits for d, or until ctx ends.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
