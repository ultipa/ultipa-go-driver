package gqldb

import (
	"context"
	"errors"
	"testing"
)

// Unit tests for the in-query graph-switch guard.
//
// Every reject case below was verified against a live server (6.2.x) to
// actually reach another graph while the request was pinned elsewhere via
// QueryConfig.GraphName. Accept cases are queries the guard must not block;
// most also execute normally on that server, the exceptions being noted
// inline (the guard's job is to not be the thing that rejects them).
//
// This corpus is mirrored in the Python, Java, Node.js and C# SDK tests -
// keep the five in sync.

var guardRejectCases = []struct{ name, query string }{
	// bare USE - the customer's original payload; note there is no GRAPH
	{"bare use", "USE tenant_b\nMATCH (n) RETURN count(n)"},
	{"use graph", "USE GRAPH tenant_b\nMATCH (n) RETURN count(n)"},
	{"lowercase", "use graph tenant_b\nMATCH (n) RETURN count(n)"},
	{"mixed case", "UsE gRaPh tenant_b\nMATCH (n) RETURN count(n)"},
	{"tab/newline separated", "USE  \n\tGRAPH\n  tenant_b\nMATCH (n) RETURN 1"},
	{"CR separator", "USE tenant_b\r\nMATCH (n) RETURN 1"},
	{"formfeed separator", "USE\ftenant_b\nMATCH (n) RETURN 1"},
	{"vtab separator", "USE\vtenant_b\nMATCH (n) RETURN 1"},
	// NBSP: regexp's \s does not match U+00A0, but the server accepts it as
	// a separator - token scanning sidesteps the issue.
	{"NBSP separator", "USE\u00a0tenant_b\nMATCH (n) RETURN 1"},
	{"leading semicolon", ";USE tenant_b\nMATCH (n) RETURN 1"},
	{"double semicolon", "USE tenant_b;;MATCH (n) RETURN 1"},
	{"leading block comment", "/*c*/USE GRAPH tenant_b\nMATCH (n) RETURN 1"},
	{"inner block comment", "USE GRAPH /*c*/ tenant_b\nMATCH (n) RETURN 1"},
	{"glued block comment", "USE/*c*/tenant_b\nMATCH (n) RETURN 1"},
	{"nested block comment", "/*a/*b*/c*/USE tenant_b\nMATCH (n) RETURN 1"},
	{"slash line comment", "//c\nUSE tenant_b\nMATCH (n) RETURN 1"},
	// `--` is a comment server-side; not stripping it hides the USE
	{"dash line comment", "--c\nUSE tenant_b\nMATCH (n) RETURN 1"},
	{"USE as 2nd statement", "USE GRAPH tenant_b; MATCH (n) RETURN 1"},
	{"USE mid-compound", "MATCH (n) RETURN 1; USE tenant_b; MATCH (n) RETURN 1"},
	{"trailing USE", "MATCH (n) RETURN 1; USE GRAPH tenant_b"},
	{"leading whitespace", "   \n  USE tenant_b"},
}

var guardAcceptCases = []struct{ name, query string }{
	{"plain match", "MATCH (n) RETURN count(n)"},
	// Scope A blocks USE only, so graph-lifecycle DDL is allowed here even
	// though DROP GRAPH <own> + CREATE GRAPH <own> AS COPY OF <victim>
	// reaches another tenant. Pair the flag with read_only.
	{"create as copy of", "CREATE GRAPH mine AS COPY OF tenant_b"},
	{"drop graph", "DROP GRAPH mine"},
	{"alter graph rename", "ALTER GRAPH tenant_b RENAME TO mine"},
	{"create or replace graph", "CREATE OR REPLACE GRAPH mine AS COPY OF tenant_b"},
	{"string literal", "RETURN 'USE GRAPH tenant_b' AS s"},
	{"double-quoted literal", `RETURN "USE GRAPH b" AS s`},
	{"backtick literal", "RETURN `USE GRAPH b` AS s"},
	{"property compare", "MATCH (n WHERE n.name = 'use graph tenant_b') RETURN count(n)"},
	{"contains", "MATCH (n WHERE n.name CONTAINS 'use graph') RETURN count(n)"},
	{"comment mentioning use", "/* do not USE GRAPH here */ MATCH (n) RETURN 1"},
	{"line comment mentioning use", "-- do not USE GRAPH here\nMATCH (n) RETURN 1"},
	{"alias named use_graph", "MATCH (n) RETURN count(n) AS use_graph"},
	{"insert prop valued USE", "INSERT (:N {name:'USE GRAPH b'})"},
	{"semicolon inside literal", "INSERT (:N {name:'x; USE b'})"},
	{"escaped quote then USE text", `RETURN 'a\'; USE b' AS s`},
	{"identifier prefixed USE", "MATCH (n:USER) RETURN n"},
	{"USED as identifier", "MATCH (n) RETURN n.used"},
	// SHOW GRAPHS still enumerates every tenant's graph - out of scope for
	// this guard, and a reason it is not a tenant boundary.
	{"show graphs", "SHOW GRAPHS"},
	// Graph-type DDL names no graph, so the TYPE carve-out keeps it allowed.
	// Spellings from the GQL docs, verified on 6.2.127.
	{"create graph type", "CREATE GRAPH TYPE socialType"},
	{"create graph type inline", "CREATE GRAPH TYPE gType { NODE User ({name STRING, age UINT32}) }"},
	{"create graph type if not exists", "CREATE GRAPH TYPE IF NOT EXISTS s2 { NODE P ({name STRING}) }"},
	{"create or replace graph type", "CREATE OR REPLACE GRAPH TYPE socialType { NODE P ({name STRING}) }"},
	{"alter graph type", "ALTER GRAPH TYPE socialType RENAME TO communityType"},
	{"drop graph type", "DROP GRAPH TYPE gt"},
	// Not valid GQL - the server rejects both. They are here to pin down
	// that the *guard* is not what rejects them.
	{"empty", ""},
	{"only a comment", "-- nothing here"},
}

func TestGuardRejects(t *testing.T) {
	for _, tc := range guardRejectCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := findBlockedKeyword(tc.query); got == "" {
				t.Errorf("should be blocked: %q", tc.query)
			}
		})
	}
}

func TestGuardAccepts(t *testing.T) {
	for _, tc := range guardAcceptCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := findBlockedKeyword(tc.query); got != "" {
				t.Errorf("should be allowed: %q (blocked as %q)", tc.query, got)
			}
		})
	}
}

func TestGuardUnparseableFailsClosed(t *testing.T) {
	// The server rejects these as parse errors anyway, so refusing to guess
	// costs nothing and keeps the scanner from being fooled.
	for _, q := range []string{"RETURN 'unterminated", "/* unterminated USE b"} {
		if got := findBlockedKeyword(q); got != unparseableKeyword {
			t.Errorf("findBlockedKeyword(%q) = %q, want %q", q, got, unparseableKeyword)
		}
	}
}

func TestGuardReportsKeyword(t *testing.T) {
	for _, q := range []string{"USE b", "use graph b"} {
		if got := findBlockedKeyword(q); got != "USE" {
			t.Errorf("findBlockedKeyword(%q) = %q, want USE", q, got)
		}
	}
}

// guardClient builds a client against an unreachable port: any call that got
// as far as the network would fail with a connection error, so a
// GraphSwitchRejectedError proves the guard short-circuits before any RPC.
func guardClient(t *testing.T, enabled bool) *Client {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Hosts = []string{"127.0.0.1:1"}
	cfg.DisableUseGraph = enabled
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestGuardOffByDefault(t *testing.T) {
	if DefaultConfig().DisableUseGraph {
		t.Error("DisableUseGraph should default to false")
	}
}

func TestGuardEntryPoints(t *testing.T) {
	c := guardClient(t, true)
	defer c.Close()
	ctx := context.Background()
	q := "USE other_tenant\nMATCH (n) RETURN n"

	if _, err := c.Gql(ctx, q, nil); !errors.Is(err, ErrGraphSwitchRejected) {
		t.Errorf("Gql: got %v, want ErrGraphSwitchRejected", err)
	}
	if err := c.GqlStream(ctx, q, nil, func(*Response) error { return nil }); !errors.Is(err, ErrGraphSwitchRejected) {
		t.Errorf("GqlStream: got %v, want ErrGraphSwitchRejected", err)
	}
	if _, err := c.Explain(ctx, q, nil); !errors.Is(err, ErrGraphSwitchRejected) {
		t.Errorf("Explain: got %v, want ErrGraphSwitchRejected", err)
	}
	if _, err := c.Profile(ctx, q, nil); !errors.Is(err, ErrGraphSwitchRejected) {
		t.Errorf("Profile: got %v, want ErrGraphSwitchRejected", err)
	}
}

func TestGuardBlocksUseForms(t *testing.T) {
	c := guardClient(t, true)
	defer c.Close()
	ctx := context.Background()
	for _, q := range []string{
		"USE other_tenant\nMATCH (n) RETURN n",
		"use graph other_tenant",
		"--c\nUSE other_tenant",
		"MATCH (n) RETURN 1; USE other_tenant",
	} {
		if _, err := c.Gql(ctx, q, nil); !errors.Is(err, ErrGraphSwitchRejected) {
			t.Errorf("Gql(%q): got %v, want ErrGraphSwitchRejected", q, err)
		}
	}
}

// TestGuardAllowsGraphDDL pins down a known limitation.
// Scope A is USE-only. The graph-lifecycle DDL below reaches another tenant with no USE, so the flag must be paired with read_only; asserted here so the limitation cannot be forgotten.
func TestGuardAllowsGraphDDL(t *testing.T) {
	c := guardClient(t, true)
	defer c.Close()
	ctx := context.Background()
	for _, q := range []string{
		"DROP GRAPH victim",
		"CREATE GRAPH mine AS COPY OF victim",
		"ALTER GRAPH victim RENAME TO mine",
	} {
		_, err := c.Gql(ctx, q, nil)
		if errors.Is(err, ErrGraphSwitchRejected) {
			t.Errorf("Gql(%q) was rejected by the guard; scope A allows graph DDL", q)
		}
	}
}

func TestGuardPinDoesNotExempt(t *testing.T) {
	// The whole point: graph_name is only a per-request override, so an
	// embedded USE beats it. The guard must fire even when pinned.
	c := guardClient(t, true)
	defer c.Close()
	_, err := c.Gql(context.Background(), "USE b\nMATCH (n) RETURN n",
		&QueryConfig{GraphName: "tenant_a"})
	if !errors.Is(err, ErrGraphSwitchRejected) {
		t.Errorf("got %v, want ErrGraphSwitchRejected", err)
	}
}

func TestGuardDisabledIsNoOp(t *testing.T) {
	// Reaches the network and fails there, not at the guard.
	c := guardClient(t, false)
	defer c.Close()
	_, err := c.Gql(context.Background(), "USE other_tenant", nil)
	if errors.Is(err, ErrGraphSwitchRejected) {
		t.Error("guard fired while disabled")
	}
}
