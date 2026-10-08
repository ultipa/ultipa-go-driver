package gqldb

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Review round 1 of the error-code batch, item 1: one auto-commit write ran
// twice. Its second statement failed with "node 'session expired' already
// exists"; the server of that time answered UNAUTHENTICATED (it read the
// words), and the driver signed in again and sent the whole call again, its
// committed first statement included: two server calls, two Audit nodes.
//
// The driver now signs in again on the UNAUTHENTICATED status alone (never on
// words), and sends the call again only when the server says it refused the
// call before running it (executed=false), or when the call is read-only; a
// stream only while no rows have reached the caller.

// reproducerText is the server's message in the reviewer's reproducer.
const reproducerText = "[5017] error in statement 2: [5017] failed to execute INSERT: INSERT: failed to insert node: node 'session expired' already exists"

// statusWith builds a status error with the server's ErrorInfo; metadata
// entries come in key, value pairs. No pairs and no reason: no detail at all,
// as from a server older than the detail.
func statusWith(c codes.Code, msg, reason string, md ...string) error {
	st := status.New(c, msg)
	if reason == "" && len(md) == 0 {
		return st.Err()
	}
	m := map[string]string{}
	for i := 0; i+1 < len(md); i += 2 {
		m[md[i]] = md[i+1]
	}
	st, err := st.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: ErrorInfoDomain, Metadata: m})
	if err != nil {
		panic(err)
	}
	return st.Err()
}

var (
	// The reviewer's reproducer as the 41da106 server answered it: the words
	// gave UNAUTHENTICATED and SESSION_EXPIRED, without executed=false.
	reproducerWithDetail = statusWith(codes.Unauthenticated, reproducerText, "SESSION_EXPIRED", "code", "5017")
	// The same from a server older than the detail.
	reproducerNoDetail = statusWith(codes.Unauthenticated, reproducerText, "")
	// A session refused by the authentication layer, before the call ran.
	expiredBeforeRun = statusWith(codes.Unauthenticated, "session expired", "SESSION_EXPIRED", "code", "7023", "executed", "false")
	// The reproducer as this round's server answers it.
	alreadyExists = statusWith(codes.AlreadyExists, reproducerText, "ALREADY_EXISTS", "code", "5017")
)

// sessionNode is a server whose first Gql, GqlStream, Explain and Profile call
// fails with err and whose later calls succeed. A call that ran (its error is
// not a refusal before running) stores one Audit node, as the reproducer's
// first statement does.
type sessionNode struct {
	pb.UnimplementedSessionServiceServer
	pb.UnimplementedQueryServiceServer
	err         error
	rowsBefore  int // rows a failing stream sends first
	logins      atomic.Int32
	calls       atomic.Int32 // Gql, GqlStream, Explain and Profile
	auditNodes  atomic.Int32
	mu          sync.Mutex
	failedFirst bool
	always      bool // every call fails with err, not only the first
}

func (n *sessionNode) answer() error {
	n.calls.Add(1)
	n.mu.Lock()
	first := !n.failedFirst || n.always
	n.failedFirst = true
	n.mu.Unlock()
	if first && n.err != nil {
		if !RefusedBeforeRun(n.err) {
			n.auditNodes.Add(1)
		}
		return n.err
	}
	n.auditNodes.Add(1)
	return nil
}

func (n *sessionNode) Login(context.Context, *pb.LoginRequest) (*pb.LoginResponse, error) {
	n.logins.Add(1)
	return &pb.LoginResponse{SessionId: 1, ServerVersion: "test"}, nil
}

func (n *sessionNode) Gql(context.Context, *pb.GqlRequest) (*pb.GqlResponse, error) {
	if err := n.answer(); err != nil {
		return nil, err
	}
	return &pb.GqlResponse{}, nil
}

func (n *sessionNode) GqlStream(_ *pb.GqlRequest, stream pb.QueryService_GqlStreamServer) error {
	err := n.answer()
	if err != nil {
		for i := 0; i < n.rowsBefore; i++ {
			if serr := stream.Send(&pb.GqlResponse{}); serr != nil {
				return serr
			}
		}
	}
	return err
}

func (n *sessionNode) Explain(context.Context, *pb.GqlRequest) (*pb.ExplainResponse, error) {
	if err := n.answer(); err != nil {
		return nil, err
	}
	return &pb.ExplainResponse{}, nil
}

func (n *sessionNode) Profile(context.Context, *pb.GqlRequest) (*pb.ProfileResponse, error) {
	if err := n.answer(); err != nil {
		return nil, err
	}
	return &pb.ProfileResponse{}, nil
}

// sessionClient starts a sessionNode failing first with err and a client
// signed in to it, with the 5020 retry off.
func sessionClient(t *testing.T, err error) (*Client, *sessionNode) {
	t.Helper()
	lis, lerr := net.Listen("tcp", "127.0.0.1:0")
	if lerr != nil {
		t.Fatal(lerr)
	}
	node := &sessionNode{err: err}
	srv := grpc.NewServer()
	pb.RegisterSessionServiceServer(srv, node)
	pb.RegisterQueryServiceServer(srv, node)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	cfg := NewConfigBuilder().Hosts(lis.Addr().String()).HealthCheckInterval(0).RetryCount(0).Build()
	client, cerr := NewClient(cfg)
	if cerr != nil {
		t.Fatal(cerr)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, cerr := client.Login(context.Background(), "admin", "pw"); cerr != nil {
		t.Fatal(cerr)
	}
	return client, node
}

func TestResend_ReloginReproducer(t *testing.T) {
	ctx := context.Background()
	write := "INSERT (:Audit {note:'one write'}); INSERT (:Status {_id:'session expired'})"
	readOnly := &QueryConfig{ReadOnly: true}

	cases := []struct {
		name       string
		err        error
		config     *QueryConfig
		wantCalls  int32
		wantAudit  int32
		wantLogins int32 // sign-ins after the first
		wantErr    bool
	}{
		// The reviewer's reproducer: one call, one server call, one Audit node.
		{"write, UNAUTHENTICATED with a detail but no executed mark", reproducerWithDetail, nil, 1, 1, 1, true},
		{"write, UNAUTHENTICATED from a server without the detail", reproducerNoDetail, nil, 1, 1, 1, true},
		{"write in a transaction, UNAUTHENTICATED without the mark", reproducerWithDetail, &QueryConfig{ReadOnly: true, TransactionID: 9}, 1, 1, 1, true},
		// Sent again: the server refused it before running it.
		{"write refused before it ran", expiredBeforeRun, nil, 2, 1, 1, false},
		// Sent again: a read-only call cannot have written.
		{"read-only, server without the detail", reproducerNoDetail, readOnly, 2, 2, 1, false},
		{"read-only, detail without the mark", reproducerWithDetail, readOnly, 2, 2, 1, false},
		// Words are not a status: no sign-in, no second call.
		{"words in an ALREADY_EXISTS message", alreadyExists, nil, 1, 1, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, node := sessionClient(t, c.err)
			_, err := client.Gql(ctx, write, c.config)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, want error: %v", err, c.wantErr)
			}
			if got := node.calls.Load(); got != c.wantCalls {
				t.Errorf("server calls = %d, want %d", got, c.wantCalls)
			}
			if got := node.auditNodes.Load(); got != c.wantAudit {
				t.Errorf("Audit nodes = %d, want %d", got, c.wantAudit)
			}
			if got := node.logins.Load() - 1; got != c.wantLogins {
				t.Errorf("sign-ins after the first = %d, want %d", got, c.wantLogins)
			}
		})
	}
}

func TestResend_StreamAfterRowsIsNeverSentAgain(t *testing.T) {
	ctx := context.Background()
	rows := 0
	count := func(*Response) error { rows++; return nil }

	// Refused before running, nothing delivered: sent again.
	client, node := sessionClient(t, expiredBeforeRun)
	if err := client.GqlStream(ctx, "MATCH (n) RETURN n", nil, count); err != nil {
		t.Fatalf("refused before running: %v", err)
	}
	if got := node.calls.Load(); got != 2 {
		t.Errorf("refused before running: %d calls, want 2", got)
	}

	// A read-only stream that delivered a row: never sent again.
	client, node = sessionClient(t, reproducerNoDetail)
	node.rowsBefore = 1
	rows = 0
	if err := client.GqlStream(ctx, "MATCH (n) RETURN n", &QueryConfig{ReadOnly: true}, count); err == nil {
		t.Fatal("a stream that failed after a row succeeded")
	}
	if got := node.calls.Load(); got != 1 || rows != 1 {
		t.Errorf("after a row: %d calls, %d rows, want 1 and 1", got, rows)
	}

	// Even with executed=false (it cannot be, after rows; the driver does
	// not rely on it).
	client, node = sessionClient(t, expiredBeforeRun)
	node.rowsBefore = 1
	rows = 0
	_ = client.GqlStream(ctx, "MATCH (n) RETURN n", nil, count)
	if got := node.calls.Load(); got != 1 || rows != 1 {
		t.Errorf("after a row, marked: %d calls, %d rows, want 1 and 1", got, rows)
	}

	// A write stream without the mark: not sent again.
	client, node = sessionClient(t, reproducerWithDetail)
	_ = client.GqlStream(ctx, "INSERT (:A) RETURN 1", nil, count)
	if got := node.calls.Load(); got != 1 {
		t.Errorf("write stream: %d calls, want 1", got)
	}
}

func TestResend_ExplainAndProfile(t *testing.T) {
	ctx := context.Background()
	for name, call := range map[string]func(*Client, *QueryConfig) error{
		"Explain": func(c *Client, cfg *QueryConfig) error { _, err := c.Explain(ctx, "INSERT (:A)", cfg); return err },
		"Profile": func(c *Client, cfg *QueryConfig) error { _, err := c.Profile(ctx, "INSERT (:A)", cfg); return err },
	} {
		client, node := sessionClient(t, reproducerWithDetail)
		if err := call(client, nil); err == nil || node.calls.Load() != 1 {
			t.Errorf("%s, may have run: err=%v calls=%d, want an error and 1 call", name, err, node.calls.Load())
		}
		client, node = sessionClient(t, expiredBeforeRun)
		if err := call(client, nil); err != nil || node.calls.Load() != 2 {
			t.Errorf("%s, refused before running: err=%v calls=%d, want success and 2 calls", name, err, node.calls.Load())
		}
	}
}

func TestRefusedBeforeRun(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want bool
	}{
		"marked":          {expiredBeforeRun, true},
		"detail, no mark": {reproducerWithDetail, false},
		"no detail":       {reproducerNoDetail, false},
		"executed=true":   {statusWith(codes.Unauthenticated, "x", "SESSION_EXPIRED", "code", "7023", "executed", "true"), false},
		"other domain": {func() error {
			st, _ := status.New(codes.Unauthenticated, "x").WithDetails(&errdetails.ErrorInfo{Domain: "example.com", Metadata: map[string]string{"executed": "false"}})
			return st.Err()
		}(), false},
		"wrapped":            {NewError(0, "query failed", expiredBeforeRun), true},
		"words in a message": {statusWith(codes.Unauthenticated, "executed=false", "X", "code", "0"), false},
		"nil":                {nil, false},
	} {
		if got := RefusedBeforeRun(tc.err); got != tc.want {
			t.Errorf("%s: RefusedBeforeRun = %v, want %v", name, got, tc.want)
		}
	}
}
