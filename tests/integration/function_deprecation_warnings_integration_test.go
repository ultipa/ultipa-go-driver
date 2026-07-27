//go:build integration

// Deprecation warning smoke checks for server v6.2.38 (core v1.1.48+).
//
// Locks the 5 contracts from v1.1.48_tester_brief.html §4.1:
//   1. Legacy form still resolves to identical rows as canonical.
//   2. Canonical form returns no deprecation warning.
//   3. Legacy form emits a warning matching the brief format
//      `function "X" is deprecated; use "Y" instead`.
//   4. Warning is per-message deduped (1 warning per legacy fn, not per row).
//   5. Warning text contains BOTH legacy and canonical names so log-greppers
//      can find either.
//
// Port of Python test_function_deprecation_warnings.py.
package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

type fnPair struct {
	name           string
	legacy         string
	canonical      string
	legacyToken    string
	canonicalToken string
}

var deprecationPairs = []fnPair{
	{"TYPEOF", "typeOf(42)", "type_of(42)", "TYPEOF", "TYPE_OF"},
	{"TOSTRING", "toString(42)", "to_string(42)", "TOSTRING", "TO_STRING"},
	{"TOINTEGER", "toInteger('42')", "to_integer('42')", "TOINTEGER", "TO_INTEGER"},
	{"TOFLOAT", "toFloat('3.14')", "to_float('3.14')", "TOFLOAT", "TO_FLOAT"},
	{"TOBOOLEAN", "toBoolean('true')", "to_boolean('true')", "TOBOOLEAN", "TO_BOOLEAN"},
	{"LISTCONTAINS", "listContains([1,2,3], 2)", "list_contains([1,2,3], 2)", "LISTCONTAINS", "LIST_CONTAINS"},
	{"DATEFORMAT",
		"dateFormat(date('2026-05-25'), 'yyyy-MM-dd')",
		"date_format(date('2026-05-25'), 'yyyy-MM-dd')",
		"DATEFORMAT", "DATE_FORMAT"},
	{"DAYOFWEEK",
		"dayOfWeek(date('2026-05-25'))",
		"day_of_week(date('2026-05-25'))",
		"DAYOFWEEK", "DAY_OF_WEEK"},
}

// deprecationOnly filters a warnings slice down to ones containing "deprecated".
func deprecationOnly(ws []string) []string {
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		if strings.Contains(strings.ToLower(w), "deprecated") {
			out = append(out, w)
		}
	}
	return out
}

func TestFunctionDeprecationWarnings(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := testClient.UseGraph(ctx, "default"); err != nil {
		t.Fatalf("USE GRAPH default failed: %v", err)
	}

	for _, p := range deprecationPairs {
		p := p
		t.Run(p.name, func(t *testing.T) {
			legacyResp, err := testClient.Gql(ctx, "RETURN "+p.legacy, nil)
			if err != nil {
				t.Fatalf("legacy %q failed: %v", p.legacy, err)
			}
			canonResp, err := testClient.Gql(ctx, "RETURN "+p.canonical, nil)
			if err != nil {
				t.Fatalf("canonical %q failed: %v", p.canonical, err)
			}

			// 1. Identical row content
			lv, _ := legacyResp.Rows[0].Get(0)
			cv, _ := canonResp.Rows[0].Get(0)
			if fmt.Sprintf("%v", lv) != fmt.Sprintf("%v", cv) {
				t.Errorf("legacy %q -> %v, canonical %q -> %v",
					p.legacy, lv, p.canonical, cv)
			}

			// 2. Canonical: no deprecation warnings
			canonDep := deprecationOnly(canonResp.Warnings)
			if len(canonDep) > 0 {
				t.Errorf("canonical %q unexpectedly produced deprecation warnings: %v",
					p.canonical, canonDep)
			}

			// 3. Legacy: at least one deprecation warning
			legacyDep := deprecationOnly(legacyResp.Warnings)
			if len(legacyDep) == 0 {
				t.Fatalf("legacy %q produced no deprecation warning; got %v",
					p.legacy, legacyResp.Warnings)
			}

			msg := legacyDep[0]
			upper := strings.ToUpper(msg)
			// 5. Warning contains both legacy and canonical tokens
			if !strings.Contains(upper, p.legacyToken) {
				t.Errorf("warning %q missing legacy token %q", msg, p.legacyToken)
			}
			if !strings.Contains(upper, p.canonicalToken) {
				t.Errorf("warning %q missing canonical token %q", msg, p.canonicalToken)
			}
			// Format check
			if !strings.Contains(strings.ToLower(msg), "is deprecated; use") {
				t.Errorf("warning %q does not match 'function X is deprecated; use Y instead' format", msg)
			}
		})
	}
}

func TestLegacyMultiRowWarningIsDeduped(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graph := fmt.Sprintf("depr_dedup_%d", 10000+(int(time.Now().UnixNano())%90000))
	_ = testClient.DropGraph(ctx, graph, true)
	defer func() {
		_ = testClient.UseGraph(ctx, "default")
		_ = testClient.DropGraph(ctx, graph, true)
	}()

	if _, err := testClient.Gql(ctx, "CREATE GRAPH "+graph, nil); err != nil {
		t.Fatalf("CREATE GRAPH: %v", err)
	}
	if err := testClient.UseGraph(ctx, graph); err != nil {
		t.Fatalf("USE GRAPH: %v", err)
	}
	for i := 0; i < 25; i++ {
		if _, err := testClient.Gql(ctx,
			fmt.Sprintf("INSERT (:T {_id:'n%d', v:%d})", i, i), nil); err != nil {
			t.Fatalf("INSERT row %d: %v", i, err)
		}
	}

	r, err := testClient.Gql(ctx, "MATCH (n:T) RETURN typeOf(n.v)", nil)
	if err != nil {
		t.Fatalf("MATCH RETURN typeOf: %v", err)
	}
	if len(r.Rows) != 25 {
		t.Fatalf("sanity: expected 25 rows, got %d", len(r.Rows))
	}

	dep := deprecationOnly(r.Warnings)
	typeofCount := 0
	for _, w := range dep {
		if strings.Contains(strings.ToUpper(w), "TYPEOF") {
			typeofCount++
		}
	}
	if typeofCount != 1 {
		t.Errorf("expected exactly 1 dedup'd typeOf warning across 25 rows; got %d (all dep warnings: %v)",
			typeofCount, dep)
	}
}

func TestUnknownFunctionStillErrors(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = testClient.UseGraph(ctx, "default")

	_, err := testClient.Gql(ctx, "RETURN db.no_such_thing()", nil)
	if err == nil {
		t.Fatalf("expected error for db.no_such_thing(); got nil")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "unknown function") &&
		!strings.Contains(msg, "not found") &&
		!strings.Contains(msg, "no such") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWarningIsPerRequestNotGlobal(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = testClient.UseGraph(ctx, "default")

	r1, err := testClient.Gql(ctx, "RETURN typeOf(42)", nil)
	if err != nil {
		t.Fatalf("r1 failed: %v", err)
	}
	r2, err := testClient.Gql(ctx, "RETURN type_of(42)", nil)
	if err != nil {
		t.Fatalf("r2 failed: %v", err)
	}
	if len(deprecationOnly(r1.Warnings)) == 0 {
		t.Errorf("sanity: r1 (legacy) should have deprecation warning; got %v", r1.Warnings)
	}
	if len(deprecationOnly(r2.Warnings)) != 0 {
		t.Errorf("r2 (canonical) must not carry r1's warning; got %v", r2.Warnings)
	}
}
