package gqldb

import (
	"context"
	"errors"
	"log"
	"runtime/debug"
	"time"

	"github.com/ultipa/ultipa-go-driver/v6/services"
	"google.golang.org/grpc/codes"
)

// BulkImportWaitOptions tunes how EndBulkImport and AbortBulkImport wait for
// the server to finish. The zero value is the default.
//
// An End can take longer than any one request should last (64 s at LSQB SF10)
// and an Abort, which discards what the session wrote, much longer (19 minutes
// there). The server keeps either running past a request's deadline. So the
// driver sends End or Abort with a short deadline of its own
// (ProgressInterval), and asks the server (header x-gqldb-capabilities:
// bulk-progress) to answer shortly before that deadline with the session's
// state and progress while the work goes on. It then sends the same call
// again, until the state is final (ENDED, ABORTED or FAILED) or a TotalTimeout
// you set passes (by default there is no limit). A repeated End or Abort is
// safe: it waits for the same work, or
// answers a finished session with its outcome.
//
// When a request's deadline passes with no answer (a slow network, or
// DisableEarlyAnswer), the driver reads GetBulkImportStatus, under a deadline
// of its own (10 s, cut only by the total wait): while the server names a
// state, it sends the call again. A server older than the states (before
// gqldb-grpc push 10) is detected by one GetBulkImportStatus before the first
// request: End or Abort is then sent once, as before, under the context's
// deadline alone. Such a server answers only when the work is done, so
// TotalTimeout, ProgressInterval and OnProgress have no effect there; only the
// context's deadline or cancel ends the wait.
//
// Cancelling the context ends the wait. If End or Abort had reached the server,
// the work goes on there; if it had not (the error's State is ACTIVE), the
// session stays open until End or Abort is sent again or the server's idle
// cleanup discards it.
type BulkImportWaitOptions struct {
	// TotalTimeout bounds the whole wait, all requests together. 0 means
	// Config.BulkImportWaitTimeout; when that is 0 too (the default) there is
	// no limit: the call waits until the session reaches a final state, as it
	// always did, only the context's deadline can end it sooner. When a limit
	// (or the context's deadline) ends the wait while the server still runs
	// the work, the call returns a *BulkImportInProgressError: the work has not
	// failed.
	TotalTimeout time.Duration
	// ProgressInterval is how long one request lasts at most (default 30 s,
	// at least 100 ms: a smaller value is raised to 100 ms). The server
	// answers shortly before it, so OnProgress is called about this often, and
	// never more than once a second.
	ProgressInterval time.Duration
	// OnProgress, when set, is called each time the server reports that the End
	// or the discard is still running. It runs on the calling goroutine. A
	// panic in it does not end the wait: the driver recovers it, logs the
	// first one of the wait with the standard log package, and goes on
	// waiting (and calling OnProgress) until the state is final.
	OnProgress func(BulkImportProgress)
	// DisableEarlyAnswer stops the driver asking for the early answer. Each
	// request then lasts until its deadline; the driver reads the status after
	// it and sends the call again while the session still runs.
	DisableEarlyAnswer bool
}

// DefaultBulkImportProgressInterval is the longest one End or Abort request
// lasts by default (BulkImportWaitOptions.ProgressInterval).
const DefaultBulkImportProgressInterval = 30 * time.Second

// MinBulkImportProgressInterval is the shortest ProgressInterval the driver
// uses; a smaller one is raised to it.
const MinBulkImportProgressInterval = 100 * time.Millisecond

// bulkStatusTimeout is the deadline of the status read that follows a request
// whose deadline passed with no answer. It is the read's own, not what was
// left of the request's: a short ProgressInterval must not leave it too
// little time to answer.
const bulkStatusTimeout = 10 * time.Second

// bulkLogf writes the driver's one log line about a failing OnProgress. A
// variable so tests can capture it.
var bulkLogf = log.Printf

// noBulkWaitLimit stands for "no total limit" (about 100 years).
const noBulkWaitLimit = 100 * 365 * 24 * time.Hour

// bulkResendPause is the least time between two sends of an End or an Abort,
// so a server that answers at once (a request with almost no time left)
// cannot make the driver spin.
const bulkResendPause = time.Second

// bulkAnswer is what the wait needs from one End or Abort answer.
type bulkAnswer struct {
	inProgress    bool
	state         BulkImportState
	progress      int64
	progressTotal int64
	nodesRemoved  int64
	edgesRemoved  int64
	message       string
}

// isDeadlineError reports whether err is a deadline that passed: the gRPC
// status DEADLINE_EXCEEDED or the context's own error.
func isDeadlineError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	st, ok := grpcStatusOf(err)
	return ok && st.Code() == codes.DeadlineExceeded
}

// waitBulkCall sends an End or an Abort (send) until the server reports a
// final state, as BulkImportWaitOptions describes. send gets the context of
// one request and the capabilities to ask for; it keeps the full answer for
// its caller. It returns how many requests were sent and how long it waited.
func (c *Client) waitBulkCall(ctx context.Context, op, sessionID string, opts []BulkImportWaitOptions,
	send func(callCtx context.Context, capabilities []string) (bulkAnswer, error)) (int, time.Duration, error) {
	var o BulkImportWaitOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	start := time.Now()

	// One status read tells a server that reports states (push 10 on) from an
	// older one. An older server drops a session's id while its End runs, so
	// an End sent again there is answered NOT_FOUND although the End goes on:
	// it gets the single request it always got.
	probe, perr := c.GetBulkImportStatus(ctx, sessionID)
	if perr != nil || probe.State == BulkImportStateUnspecified {
		err := c.withAutoReconnect(ctx, resendRule{}, func() error {
			_, e := send(ctx, nil)
			return e
		})
		return 1, time.Since(start), err
	}

	total := o.TotalTimeout
	if total <= 0 {
		c.mu.RLock()
		total = c.config.BulkImportWaitTimeout
		c.mu.RUnlock()
	}
	interval := o.ProgressInterval
	if interval <= 0 {
		interval = DefaultBulkImportProgressInterval
	}
	if interval < MinBulkImportProgressInterval {
		interval = MinBulkImportProgressInterval
	}
	// No total set: no limit of the driver's own (a time far beyond any wait
	// stands for it, so the arithmetic below needs no special case).
	waitEnd := start.Add(noBulkWaitLimit)
	if total > 0 {
		waitEnd = start.Add(total)
	}
	if d, ok := ctx.Deadline(); ok && d.Before(waitEnd) {
		waitEnd = d
	}
	var caps []string
	if !o.DisableEarlyAnswer {
		caps = []string{services.CapabilityBulkProgress}
	}

	last := bulkAnswer{state: probe.State, inProgress: !probe.State.IsFinal(), progress: probe.Progress,
		progressTotal: probe.ProgressTotal, message: probe.Message}
	// A panic in the caller's callback must not end the wait: the End or the
	// discard goes on on the server whatever the callback does, and the caller
	// is owed its outcome. The first panic of the wait is logged.
	callbackPanicked := false
	report := func(a bulkAnswer) {
		if o.OnProgress == nil {
			return
		}
		defer func() {
			if r := recover(); r != nil && !callbackPanicked {
				callbackPanicked = true
				bulkLogf("gqldb: the OnProgress callback of %s of bulk import session %s panicked: %v; "+
					"the driver goes on waiting for the final state and logs no further panic of this wait\n%s",
					bulkCallName(op), sessionID, r, debug.Stack())
			}
		}()
		o.OnProgress(BulkImportProgress{SessionID: sessionID, Operation: op, State: a.state, InProgress: a.inProgress,
			Progress: a.progress, ProgressTotal: a.progressTotal, NodesRemoved: a.nodesRemoved,
			EdgesRemoved: a.edgesRemoved, Message: a.message, Waited: time.Since(start)})
	}
	attempts := 0
	stillRunning := func(cause error) error {
		return &BulkImportInProgressError{SessionID: sessionID, Operation: op, State: last.state,
			Progress: last.progress, ProgressTotal: last.progressTotal, Message: last.message,
			Waited: time.Since(start), Attempts: attempts, Cause: cause}
	}

	for {
		left := time.Until(waitEnd)
		if left <= 0 || ctx.Err() != nil {
			return attempts, time.Since(start), stillRunning(ctx.Err())
		}
		attemptTimeout := interval
		if left < attemptTimeout {
			attemptTimeout = left
		}
		sent := time.Now()
		attempts++
		var ans bulkAnswer
		err := c.withAutoReconnect(ctx, resendRule{}, func() error {
			actx, cancel := context.WithTimeout(ctx, attemptTimeout)
			defer cancel()
			var e error
			ans, e = send(actx, caps)
			return e
		})
		switch {
		case err == nil && !ans.inProgress:
			return attempts, time.Since(start), nil
		case err == nil:
			last = ans
			report(ans)
		case ctx.Err() != nil:
			// The caller's context ended the request.
			if last.inProgress {
				return attempts, time.Since(start), stillRunning(ctx.Err())
			}
			return attempts, time.Since(start), err
		case !isDeadlineError(err):
			return attempts, time.Since(start), err
		default:
			// This request's own deadline passed with no answer. Ask where the
			// session stands; send the call again while the server says. The
			// read gets a deadline of its own, cut only by the total wait.
			left = time.Until(waitEnd)
			if left <= 0 {
				return attempts, time.Since(start), stillRunning(nil)
			}
			statusTimeout := bulkStatusTimeout
			cutByTotal := left <= statusTimeout
			if cutByTotal {
				statusTimeout = left
			}
			sctx, cancel := context.WithTimeout(ctx, statusTimeout)
			st, serr := c.GetBulkImportStatus(sctx, sessionID)
			cancel()
			if serr != nil && (ctx.Err() != nil || time.Until(waitEnd) <= 0 || (cutByTotal && isDeadlineError(serr))) {
				// The total wait or the caller ended the read (a timer can
				// fire a hair early): the work was last known running.
				return attempts, time.Since(start), stillRunning(ctx.Err())
			}
			if serr != nil || st.State == BulkImportStateUnspecified {
				return attempts, time.Since(start), err
			}
			last = bulkAnswer{state: st.State, inProgress: !st.State.IsFinal(), progress: st.Progress,
				progressTotal: st.ProgressTotal, message: st.Message}
			if last.inProgress {
				report(last)
			}
		}
		if pause := bulkResendPause - time.Since(sent); pause > 0 {
			// No room for another request after the pause: wait out the total
			// and stop, rather than send one with almost no time left.
			remaining := time.Until(waitEnd)
			stop := remaining <= pause
			if stop {
				pause = remaining
			}
			if pause > 0 {
				t := time.NewTimer(pause)
				select {
				case <-t.C:
				case <-ctx.Done():
					t.Stop()
				}
			}
			if stop {
				return attempts, time.Since(start), stillRunning(ctx.Err())
			}
		}
	}
}

// bulkCallName is the method a bulk wait reports: EndBulkImport or
// AbortBulkImport.
func bulkCallName(op string) string {
	if op == "abort" {
		return "AbortBulkImport"
	}
	return "EndBulkImport"
}
