//go:build wcprobe

// Manual probe: does a real server conflict reach the Go driver as
// *WriteConflictError? Run against a build that has the transaction work:
//
//	go test -tags wcprobe ./tests/ -run TestWriteConflictProbe -v -probe-host 127.0.0.1:60077
package tests

import (
	"context"
	"errors"
	"flag"
	"testing"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

var probeHost = flag.String("probe-host", "127.0.0.1:60077", "server to probe")

func TestWriteConflictProbe(t *testing.T) {
	ctx := context.Background()
	const graph = "zz_wcgo"

	dial := func() *gqldb.Client {
		c, err := gqldb.NewClient(&gqldb.Config{Hosts: []string{*probeHost}})
		if err != nil {
			t.Fatalf("new client: %v", err)
		}
		if _, err := c.Login(ctx, "admin", "admin11"); err != nil {
			t.Fatalf("login: %v", err)
		}
		return c
	}

	adm := dial()
	adm.Gql(ctx, "USE GRAPH default", nil)
	adm.Gql(ctx, "DROP GRAPH "+graph, nil)
	if _, err := adm.Gql(ctx, "CREATE GRAPH "+graph+" { NODE P ({ pid STRING, v INT }) }", nil); err != nil {
		t.Fatalf("create graph: %v", err)
	}
	if _, err := adm.Gql(ctx, "USE GRAPH "+graph+" INSERT (:P {pid:'k', v:0})", nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	defer func() {
		adm.Gql(ctx, "USE GRAPH default", nil)
		adm.Gql(ctx, "DROP GRAPH "+graph, nil)
	}()

	a, b := dial(), dial()
	ta, err := a.BeginTransaction(ctx, graph, false, 30)
	if err != nil {
		t.Fatalf("begin a: %v", err)
	}
	tb, err := b.BeginTransaction(ctx, graph, false, 30)
	if err != nil {
		t.Fatalf("begin b: %v", err)
	}

	read := "MATCH (n:P WHERE n.pid='k') RETURN n.v"
	if _, err := a.Gql(ctx, read, &gqldb.QueryConfig{TransactionID: ta.ID}); err != nil {
		t.Fatalf("read a: %v", err)
	}
	if _, err := b.Gql(ctx, read, &gqldb.QueryConfig{TransactionID: tb.ID}); err != nil {
		t.Fatalf("read b: %v", err)
	}
	write := "MATCH (n:P WHERE n.pid='k') SET n.v = 1"
	a.Gql(ctx, write, &gqldb.QueryConfig{TransactionID: ta.ID})
	b.Gql(ctx, write, &gqldb.QueryConfig{TransactionID: tb.ID})

	if _, err := a.Commit(ctx, ta.ID); err != nil {
		t.Fatalf("commit a should succeed: %v", err)
	}
	_, err = b.Commit(ctx, tb.ID)
	if err == nil {
		t.Fatalf("commit b succeeded: the server did not detect the conflict")
	}
	var wce *gqldb.WriteConflictError
	if !errors.As(err, &wce) {
		t.Fatalf("commit b failed but not as *WriteConflictError: %T: %v", err, err)
	}
	if !errors.Is(err, gqldb.ErrWriteConflict) {
		t.Fatalf("errors.Is(err, ErrWriteConflict) = false")
	}
	if !gqldb.IsWriteConflict(err) {
		t.Fatalf("IsWriteConflict(err) = false")
	}
	t.Logf("OK: %v", err)
}
