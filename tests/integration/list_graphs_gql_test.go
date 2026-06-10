//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// ListGraphs / GetGraphInfo are now implemented over GQL `SHOW GRAPHS` (not
// the ListGraphs / GetGraphInfo RPCs, whose typed enum mis-mapped CLOSED and
// could not surface the new bounded_graph_type column). This pins: CLOSED is
// reported correctly, the BoundedGraphType field is reachable (value "" on
// these graphs), and GetGraphInfo agrees / returns ErrGraphNotFound when
// absent.
//
// Runs against the NO-AUTH 6.2.59 server. Override the host via
// GQLDB_LISTGRAPHS_HOST if needed.
func TestListGraphsGqlClosedMapping(t *testing.T) {
	host := os.Getenv("GQLDB_LISTGRAPHS_HOST")
	if host == "" {
		host = "192.168.1.87:60062"
	}

	config := gqldb.NewConfigBuilder().
		Hosts(host).
		Timeout(30 * time.Second).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Fatalf("NewClient(%s) failed: %v", host, err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// No-auth server: a Login is unnecessary, but if attempted the server may
	// reject with "authentication is not enabled" — tolerate that and proceed
	// without a session.
	if _, lerr := client.Login(ctx, "admin", "admin"); lerr != nil {
		t.Logf("Login returned (tolerated on no-auth server): %v", lerr)
	}

	// Unique graph names — 4 SDKs share this server, avoid clashes.
	suffix := fmt.Sprintf("%d_%d", time.Now().UnixNano(), os.Getpid())
	gOpen := "probe_go_listgraphs_open_" + suffix
	gClosed := "probe_go_listgraphs_closed_" + suffix

	// Cleanup: drop both graphs regardless of outcome.
	defer func() {
		_, _ = client.Gql(ctx, "DROP GRAPH "+gOpen+" IF EXISTS", nil)
		_, _ = client.Gql(ctx, "DROP GRAPH "+gClosed+" IF EXISTS", nil)
	}()

	// Create an OPEN graph via the typed CreateGraph OPEN path.
	if err := client.CreateGraph(ctx, gOpen, gqldb.GraphTypeOpen, "open one"); err != nil {
		t.Fatalf("CreateGraph(OPEN) failed: %v", err)
	}
	// Create a CLOSED graph via schema DDL (reliably reported as CLOSED).
	if _, err := client.Gql(ctx, "CREATE GRAPH "+gClosed+" {NODE P ({name STRING})}", nil); err != nil {
		t.Fatalf("CREATE GRAPH (closed) failed: %v", err)
	}

	graphs, err := client.ListGraphs(ctx)
	if err != nil {
		t.Fatalf("ListGraphs failed: %v", err)
	}

	var open, closed *gqldb.GraphInfo
	for _, g := range graphs {
		switch g.Name {
		case gOpen:
			open = g
		case gClosed:
			closed = g
		}
	}

	if open == nil {
		t.Fatalf("OPEN graph %q not listed", gOpen)
	}
	if closed == nil {
		t.Fatalf("CLOSED graph %q not listed", gClosed)
	}

	if open.GraphType != gqldb.GraphTypeOpen {
		t.Errorf("OPEN graph: expected GraphTypeOpen, got %v", open.GraphType)
	}
	// The core fix: the RPC enum mis-mapped CLOSED; GQL path must report it right.
	if closed.GraphType != gqldb.GraphTypeClosed {
		t.Errorf("CLOSED graph: expected GraphTypeClosed, got %v (RPC enum mis-mapped this)", closed.GraphType)
	}

	// BoundedGraphType field must be reachable; "" on these normal graphs.
	if closed.BoundedGraphType != "" {
		t.Logf("CLOSED graph BoundedGraphType = %q (expected empty on normal graphs)", closed.BoundedGraphType)
	}
	t.Logf("BoundedGraphType reachable: open=%q closed=%q", open.BoundedGraphType, closed.BoundedGraphType)

	// GetGraphInfo (also GQL-based) must agree.
	closedInfo, err := client.GetGraphInfo(ctx, gClosed)
	if err != nil {
		t.Fatalf("GetGraphInfo(closed) failed: %v", err)
	}
	if closedInfo.GraphType != gqldb.GraphTypeClosed {
		t.Errorf("GetGraphInfo(closed): expected GraphTypeClosed, got %v", closedInfo.GraphType)
	}

	openInfo, err := client.GetGraphInfo(ctx, gOpen)
	if err != nil {
		t.Fatalf("GetGraphInfo(open) failed: %v", err)
	}
	if openInfo.GraphType != gqldb.GraphTypeOpen {
		t.Errorf("GetGraphInfo(open): expected GraphTypeOpen, got %v", openInfo.GraphType)
	}

	// Missing graph must surface the not-found error.
	missing := "probe_go_listgraphs_missing_" + suffix
	if _, err := client.GetGraphInfo(ctx, missing); err == nil {
		t.Errorf("GetGraphInfo(%q) expected ErrGraphNotFound, got nil", missing)
	} else if !errors.Is(err, gqldb.ErrGraphNotFound) {
		t.Errorf("GetGraphInfo(%q): expected ErrGraphNotFound, got %v", missing, err)
	}
}
