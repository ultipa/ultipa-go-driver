package gqldb

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/emptypb"
)

// The connection pool's health check, over real connections. A connection
// that is ready or idle is healthy and stays; one that keeps failing is
// replaced after three ticks in a row. When the pool replaces a connection
// (its health check, or ForceReconnectAll after a dropped connection), every
// service sends its next call over the new one.

// callRecorder answers every unary call with an empty reply (a signed-in
// session for Login) and records each call's method and the client address
// it came from: each connection of the client has its own address.
type callRecorder struct {
	mu    sync.Mutex
	calls []recordedCall
}

type recordedCall struct{ method, from string }

func (r *callRecorder) handle(_ interface{}, stream grpc.ServerStream) error {
	method, _ := grpc.MethodFromServerStream(stream)
	from := ""
	if p, ok := peer.FromContext(stream.Context()); ok {
		from = p.Addr.String()
	}
	r.mu.Lock()
	r.calls = append(r.calls, recordedCall{method[strings.LastIndex(method, "/")+1:], from})
	r.mu.Unlock()
	if err := stream.RecvMsg(new(emptypb.Empty)); err != nil {
		return err
	}
	if strings.HasSuffix(method, "/Login") {
		return stream.SendMsg(&pb.LoginResponse{SessionId: 1, ServerVersion: "test"})
	}
	return stream.SendMsg(new(emptypb.Empty))
}

func (r *callRecorder) take() []recordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	calls := r.calls
	r.calls = nil
	return calls
}

func startRecorder(t *testing.T) (string, *callRecorder) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	rec := &callRecorder{}
	srv := grpc.NewServer(grpc.UnknownServiceHandler(rec.handle))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String(), rec
}

func closedAddress(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	lis.Close()
	return addr
}

func (p *ConnectionPool) entry(host string) *Connection {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.connections[host]
}

func TestHealthCheck_ReadyConnectionIsKept(t *testing.T) {
	addr, _ := startRecorder(t)
	pool, err := NewConnectionPool(NewConfigBuilder().Hosts(addr).HealthCheckInterval(20 * time.Millisecond).Build())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	cc, _ := pool.GetConnection()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pb.NewSessionServiceClient(cc).Ping(ctx, &pb.PingRequest{}); err != nil {
		t.Fatalf("ping: %v", err)
	}
	first := pool.entry(addr)
	first.mu.RLock()
	pingBefore := first.lastPing
	first.mu.RUnlock()

	time.Sleep(300 * time.Millisecond) // about 15 ticks

	now := pool.entry(addr)
	if now != first {
		t.Fatal("the pool replaced a ready connection")
	}
	now.mu.RLock()
	defer now.mu.RUnlock()
	if !now.healthy || now.consecutiveUnhealthy != 0 {
		t.Errorf("ready connection: healthy %v, unhealthy ticks %d", now.healthy, now.consecutiveUnhealthy)
	}
	if !now.lastPing.After(pingBefore) {
		t.Error("the health check never found the ready connection healthy")
	}
}

func TestHealthCheck_UnreachableConnectionIsReplacedAfterThreeTicks(t *testing.T) {
	addr := closedAddress(t)
	pool, err := NewConnectionPool(NewConfigBuilder().Hosts(addr).HealthCheckInterval(0).Build())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	first := pool.entry(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	first.conn.Connect()
	for s := first.conn.GetState(); s != connectivity.TransientFailure; s = first.conn.GetState() {
		if !first.conn.WaitForStateChange(ctx, s) {
			t.Fatalf("the connection to a closed port never failed (state %v)", s)
		}
	}

	for tick := 1; tick <= 2; tick++ {
		pool.checkHealth()
		if pool.entry(addr) != first {
			t.Fatalf("replaced after %d tick(s), want 3", tick)
		}
		if first.consecutiveUnhealthy != tick {
			t.Fatalf("unhealthy ticks %d after %d tick(s)", first.consecutiveUnhealthy, tick)
		}
	}
	pool.checkHealth()
	if pool.entry(addr) == first {
		t.Fatal("a failing connection was not replaced after three ticks")
	}
	first.conn.Close()
}

func TestRebuiltConnectionCarriesEveryServiceCall(t *testing.T) {
	addr, rec := startRecorder(t)
	client, err := NewClient(NewConfigBuilder().Hosts(addr).HealthCheckInterval(0).Build())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := client.Login(ctx, "u", "p"); err != nil {
		t.Fatalf("login: %v", err)
	}

	oneCallEach := func() {
		t.Helper()
		calls := []func() error{
			func() error { _, e := client.sessionClient.Ping(ctx, &pb.PingRequest{}); return e },
			func() error { _, e := client.queryClient.Gql(ctx, &pb.GqlRequest{}); return e },
			func() error { _, e := client.graphClient.ListGraphs(ctx, &pb.ListGraphsRequest{}); return e },
			func() error {
				_, e := client.transactionClient.ListTransactions(ctx, &pb.ListTransactionsRequest{})
				return e
			},
			func() error { _, e := client.dataClient.InsertNodes(ctx, &pb.InsertNodesRequest{}); return e },
			func() error { _, e := client.healthClient.Check(ctx, &pb.HealthCheckRequest{}); return e },
			func() error { _, e := client.adminClient.GetCacheStats(ctx, &pb.GetCacheStatsRequest{}); return e },
			func() error {
				_, e := client.bulkImportClient.GetBulkImportStatus(ctx, &pb.GetBulkImportStatusRequest{})
				return e
			},
			func() error {
				_, e := client.loaderClient.GetLoaderCapabilities(ctx, &pb.GetLoaderCapabilitiesRequest{})
				return e
			},
			func() error { // a streaming call
				stream, e := client.queryClient.GqlStream(ctx, &pb.GqlRequest{})
				for e == nil {
					_, e = stream.Recv()
				}
				if e == io.EOF {
					return nil
				}
				return e
			},
		}
		for i, call := range calls {
			if err := call(); err != nil {
				t.Fatalf("call %d: %v", i, err)
			}
		}
	}

	oneCallEach()
	before := rec.take()
	old := client.pool.entry(addr)

	// The pool replaces the connection, as its health check does.
	client.pool.ForceReconnectAll()
	if client.pool.entry(addr) == old {
		t.Fatal("ForceReconnectAll kept the old connection")
	}
	defer old.conn.Close()

	oneCallEach()
	after := rec.take()
	if len(after) != 10 {
		t.Fatalf("server saw %d calls, want 10: %v", len(after), after)
	}
	var stuck []string
	for _, c := range after {
		if c.from == before[0].from {
			stuck = append(stuck, c.method)
		}
	}
	if len(stuck) > 0 {
		t.Errorf("calls still sent over the connection the pool dropped: %v", stuck)
	}
}

// Close waits for the health check to stop. It used to wait while holding the
// pool's lock, which a running tick needs, so Close never returned when it
// came during a tick.
func TestHealthCheck_CloseDuringATickReturns(t *testing.T) {
	addr, _ := startRecorder(t)
	for round := 0; round < 100; round++ {
		pool, err := NewConnectionPool(NewConfigBuilder().Hosts(addr).HealthCheckInterval(time.Microsecond).Build())
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
		closed := make(chan struct{})
		go func() { pool.Close(); close(closed) }()
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
			t.Fatalf("Close did not return within 5 s (round %d)", round)
		}
	}
}
