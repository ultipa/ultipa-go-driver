//go:build integration

package integration

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// AiRead / AiGql require a cloud DBaaS endpoint that has the ai.read /
// ai.gql procedures available. These tests connect directly to that
// endpoint (independent of the local testClient in main_test.go).
const (
	aiTestHost     = "f86fcd8d301b.dbaas.ultipa.com:443"
	aiTestUser     = "admin"
	aiTestPassword = "LZnPQqZ7Q4MNZ2kCbdG4"
	aiTestGraph    = "miniCircle"
)

// newAiTestClient dials the cloud DBaaS endpoint over TLS and logs in.
// Returns nil to signal a skip if the connection or login fails — the AI
// endpoint may be temporarily unavailable and we don't want to flake the
// broader test suite when that happens.
func newAiTestClient(t *testing.T) *gqldb.Client {
	t.Helper()

	// DBaaS endpoint at :443 is actually plain-text HTTP/2 (nginx gRPC gateway
	// in cleartext despite the port). Do NOT enable TLS — server rejects TLS
	// handshake. _ = strings keeps the import silent for now.
	_ = strings.SplitN(aiTestHost, ":", 2)
	_ = tls.Config{}
	config := gqldb.NewConfigBuilder().
		Hosts(aiTestHost).
		Username(aiTestUser).
		Password(aiTestPassword).
		DefaultGraph(aiTestGraph).
		Timeout(120 * time.Second).
		Build()

	client, err := gqldb.NewClient(config)
	if err != nil {
		t.Skipf("AI DBaaS endpoint unavailable (NewClient failed): %v", err)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := client.Login(ctx, aiTestUser, aiTestPassword); err != nil {
		client.Close()
		t.Skipf("AI DBaaS endpoint unavailable (Login failed): %v", err)
		return nil
	}
	return client
}

// aiRetry runs fn up to 4 times, backing off 20s/40s/60s/80s, when the result
// is a transient LLM rate-limit ("high demand" / "rate limit" / "quota" / "try
// again" / "overloaded") or a gRPC transport flake. Returns the final result.
type aiResultLike interface {
	ok() bool
	errMsg() string
}
type aiReadRes struct{ inner *gqldb.AiReadResult }
func (r aiReadRes) ok() bool     { return r.inner != nil && r.inner.Success }
func (r aiReadRes) errMsg() string { if r.inner != nil { return r.inner.Error }; return "" }

func aiRetry[T any](t *testing.T, fn func() (*T, error), isSuccess func(*T) bool, getErr func(*T) string) *T {
	t.Helper()
	var last *T
	var lastErr error
	// Retry up to 2 times to limit Gemini free-tier quota burn (each retry
	// consumes a request regardless of success/failure).
	for i := 0; i < 2; i++ {
		res, err := fn()
		if err != nil {
			lastErr = err
			msg := strings.ToLower(err.Error())
			if !(strings.Contains(msg, "unavailable") || strings.Contains(msg, "dropped") ||
				strings.Contains(msg, "timeout") || strings.Contains(msg, "handshake")) {
				t.Fatalf("non-transient transport error: %v", err)
			}
		} else if isSuccess(res) {
			return res
		} else {
			last = res
			msg := strings.ToLower(getErr(res))
			if !(strings.Contains(msg, "high demand") || strings.Contains(msg, "rate limit") ||
				strings.Contains(msg, "quota") || strings.Contains(msg, "try again") ||
				strings.Contains(msg, "overloaded")) {
				return res
			}
		}
		// Gemini free-tier per-minute rate limits require ~90s cooldown
		// between attempts; shorter waits keep re-triggering the quota error.
		time.Sleep(time.Duration(90) * time.Second)
	}
	if last != nil {
		return last
	}
	t.Fatalf("aiRetry: exhausted retries, last err: %v", lastErr)
	return nil
}

func TestAiReadAutoExecutesGeneratedGql(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping AI integration test in short mode")
	}

	client := newAiTestClient(t)
	if client == nil {
		return
	}
	defer client.Close()

	result := aiRetry(t,
		func() (*gqldb.AiReadResult, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			return client.AiRead(ctx, "count all nodes", nil)
		},
		func(r *gqldb.AiReadResult) bool { return r != nil && r.Success },
		func(r *gqldb.AiReadResult) string { if r != nil { return r.Error }; return "" },
	)
	var err error = nil
	if err != nil {
		t.Fatalf("AiRead returned transport error: %v", err)
	}
	if result == nil {
		t.Fatal("AiRead returned nil result")
	}

	t.Logf("AiRead: success=%v stages=%d generated_gql=%q total_elapsed_ms=%d total_tokens_input=%d total_tokens_output=%d",
		result.Success, len(result.Stages), result.GeneratedGql,
		result.TotalElapsedMs, result.TotalTokensInput, result.TotalTokensOutput)
	for i, s := range result.Stages {
		t.Logf("  stage[%d]: %s elapsed=%dms tokens_in=%d tokens_out=%d detail=%q",
			i, s.Stage, s.ElapsedMs, s.TokensInput, s.TokensOutput, s.Detail)
	}

	if !result.Success {
		t.Fatalf("AiRead reported failure: %s", result.Error)
	}

	if result.GeneratedGql == "" {
		t.Fatal("AiRead: expected GeneratedGql to be populated")
	}
	upperGql := strings.ToUpper(strings.TrimSpace(result.GeneratedGql))
	if !strings.HasPrefix(upperGql, "MATCH") {
		t.Errorf("AiRead: expected GeneratedGql to start with MATCH, got %q", result.GeneratedGql)
	}

	if result.Data == nil {
		t.Fatal("AiRead: expected Data to be a non-nil *Response (auto-executed)")
	}
	if len(result.Data.Rows) == 0 {
		t.Errorf("AiRead: expected at least one row in Data, got columns=%v rows=0", result.Data.Columns)
	} else {
		t.Logf("AiRead data: columns=%v rowCount=%d", result.Data.Columns, len(result.Data.Rows))
	}
}

// TestAiReadWithQueryConfigPropagates verifies that a per-call QueryConfig
// (graph_name + timeout) is propagated to the AI path — i.e., not silently
// dropped — so the call lands on the requested graph.
func TestAiReadWithQueryConfigPropagates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping AI integration test in short mode")
	}

	client := newAiTestClient(t)
	if client == nil {
		return
	}
	defer client.Close()

	cfg := &gqldb.QueryConfig{GraphName: aiTestGraph, Timeout: 120}
	result := aiRetry(t,
		func() (*gqldb.AiReadResult, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			return client.AiRead(ctx, "count all nodes", cfg)
		},
		func(r *gqldb.AiReadResult) bool { return r != nil && r.Success },
		func(r *gqldb.AiReadResult) string { if r != nil { return r.Error }; return "" },
	)
	if result == nil || !result.Success {
		if result != nil && (strings.Contains(strings.ToLower(result.Error), "rate") ||
			strings.Contains(strings.ToLower(result.Error), "quota")) {
			t.Skipf("AI provider transient: %s", result.Error)
		}
		t.Fatalf("AiRead with config failed: %v", result)
	}
	t.Logf("AiRead(config): success=%v generated_gql=%q elapsed_ms=%d",
		result.Success, result.GeneratedGql, result.TotalElapsedMs)
	if result.Data == nil || len(result.Data.Rows) == 0 {
		t.Fatalf("AiRead with config: expected non-empty Data")
	}
}

// TestAiReadWithNonExistentGraphErrors verifies that a per-call graph_name
// pointing at a non-existent graph fails the AI call cleanly instead of
// silently falling back to the session's current graph (server bug #12 on
// the AI path).
func TestAiReadWithNonExistentGraphErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping AI integration test in short mode")
	}

	client := newAiTestClient(t)
	if client == nil {
		return
	}
	defer client.Close()

	cfg := &gqldb.QueryConfig{GraphName: "ai_test_nonexistent_xxx_999", Timeout: 60}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	result, err := client.AiRead(ctx, "count all nodes", cfg)

	var msg string
	if err != nil {
		msg = err.Error()
	} else if result != nil && !result.Success {
		msg = result.Error
	} else if result != nil && result.Success {
		t.Fatalf("AiRead on non-existent graph unexpectedly succeeded: gen=%q rows=%d",
			result.GeneratedGql, func() int {
				if result.Data != nil {
					return len(result.Data.Rows)
				}
				return 0
			}())
	}
	t.Logf("AiRead(non-existent graph) err=%q", msg)
	low := strings.ToLower(msg)
	if !(strings.Contains(low, "not found") || strings.Contains(low, "does not exist") ||
		strings.Contains(low, "no such")) {
		t.Fatalf("expected a graph-not-found error, got: %q", msg)
	}
}

// TestAiGqlThenManualExecute exercises the documented "review-then-execute"
// workflow: ai_gql produces GQL with Data == nil, then the caller runs that
// GQL manually via client.Gql.
func TestAiGqlThenManualExecute(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping AI integration test in short mode")
	}

	client := newAiTestClient(t)
	if client == nil {
		return
	}
	defer client.Close()

	gen := aiRetry(t,
		func() (*gqldb.AiReadResult, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			return client.AiGql(ctx, "count all nodes", nil)
		},
		func(r *gqldb.AiReadResult) bool { return r != nil && r.Success },
		func(r *gqldb.AiReadResult) string { if r != nil { return r.Error }; return "" },
	)
	if gen == nil || !gen.Success {
		t.Fatalf("AiGql failed: %v", gen)
	}
	if gen.Data != nil {
		t.Fatalf("AiGql .Data must be nil (generate-only), got non-nil")
	}
	if gen.GeneratedGql == "" {
		t.Fatal("expected non-empty GeneratedGql")
	}
	t.Logf("AiGql then exec: gen=%q", gen.GeneratedGql)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := client.Gql(ctx, gen.GeneratedGql, nil)
	if err != nil {
		t.Fatalf("manual exec of generated GQL failed: %v", err)
	}
	if resp == nil || len(resp.Rows) == 0 {
		t.Fatalf("manual exec produced no rows; gql=%q", gen.GeneratedGql)
	}
	t.Logf("  manual exec rows=%d cols=%v", len(resp.Rows), resp.Columns)
}

func TestAiGqlGenerateOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping AI integration test in short mode")
	}

	client := newAiTestClient(t)
	if client == nil {
		return
	}
	defer client.Close()

	result := aiRetry(t,
		func() (*gqldb.AiReadResult, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			return client.AiGql(ctx, "find top 5 nodes by degree", nil)
		},
		func(r *gqldb.AiReadResult) bool { return r != nil && r.Success },
		func(r *gqldb.AiReadResult) string { if r != nil { return r.Error }; return "" },
	)
	var err error = nil
	if err != nil {
		t.Fatalf("AiGql returned transport error: %v", err)
	}
	if result == nil {
		t.Fatal("AiGql returned nil result")
	}

	t.Logf("AiGql: success=%v stages=%d generated_gql=%q total_elapsed_ms=%d total_tokens_input=%d total_tokens_output=%d",
		result.Success, len(result.Stages), result.GeneratedGql,
		result.TotalElapsedMs, result.TotalTokensInput, result.TotalTokensOutput)
	for i, s := range result.Stages {
		t.Logf("  stage[%d]: %s elapsed=%dms tokens_in=%d tokens_out=%d detail=%q",
			i, s.Stage, s.ElapsedMs, s.TokensInput, s.TokensOutput, s.Detail)
	}

	if !result.Success {
		t.Fatalf("AiGql reported failure: %s", result.Error)
	}
	if result.GeneratedGql == "" {
		t.Fatal("AiGql: expected GeneratedGql to be populated")
	}
	if result.Data != nil {
		t.Errorf("AiGql: expected Data to be nil (generate-only), got non-nil *Response")
	}
}
