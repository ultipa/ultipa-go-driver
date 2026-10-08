package services

import (
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The server's ErrorInfo domain, and the reason of its answer to a result too
// large for the single-response call (Gql).
const (
	errorInfoDomain      = "gqldb.ultipa.com"
	reasonResultRowLimit = "RESULT_ROW_LIMIT"
)

// streamFallbackAllowed reports whether Gql may send a request again through
// GqlStream after err. Both must hold:
//
//   - err is the server's refusal of a result too large for Gql:
//     RESOURCE_EXHAUSTED with the reason RESULT_ROW_LIMIT, or, from a server
//     that predates the error detail, RESOURCE_EXHAUSTED whose message says
//     "use streaming API";
//   - the request is safe to send again: read-only outside a transaction (the
//     server refuses any write in it), or refused before it ran (the detail's
//     executed=false); and the detail does not say part of it is stored
//     (partly_stored=true).
//
// A request that may have written is never sent again: its error is returned,
// and the caller marks it read-only or sends it with GqlStream. The text is
// read only to recognise an old server's answer, and only decides anything
// for a request that is safe to send again whatever the answer was.
func streamFallbackAllowed(err error, config *QueryConfig) bool {
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.ResourceExhausted {
		return false
	}
	readOnly := config != nil && config.ReadOnly && config.TransactionID == 0
	info := serverErrorInfo(st)
	if info == nil {
		return readOnly && strings.Contains(st.Message(), "use streaming API")
	}
	if info.GetReason() != reasonResultRowLimit || info.GetMetadata()["partly_stored"] == "true" {
		return false
	}
	return readOnly || info.GetMetadata()["executed"] == "false"
}

// serverErrorInfo returns the server's ErrorInfo in st, or nil.
func serverErrorInfo(st *status.Status) *errdetails.ErrorInfo {
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok && info.GetDomain() == errorInfoDomain {
			return info
		}
	}
	return nil
}
