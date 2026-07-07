//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// Characterization test for a typed vector column VECTOR(N, ELEMTYPE).
// Current SERVER behavior (verified 6.2.109 + 6.3.1): dimension N is enforced,
// but the element type is NOT — float values are accepted into VECTOR(3, INTEGER)
// and stored as-is (no reject, no coercion). Flip these assertions once the server
// decides enforcement. See server_issue_vector_element_type_not_enforced.md.
func TestVectorElementTypeNotEnforced(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := "test_vecelem_" + time.Now().Format("20060102150405")
	if _, err := testClient.Gql(ctx,
		fmt.Sprintf("CREATE GRAPH %s {NODE A ({embedding VECTOR(3, INTEGER)})}", graphName), nil); err != nil {
		t.Fatalf("CreateGraph with typed vector failed: %v", err)
	}
	defer dropTestGraph(graphName)

	cfg := &gqldb.QueryConfig{GraphName: graphName}

	// Element type NOT enforced: float values are accepted.
	if _, err := testClient.Gql(ctx,
		"INSERT (n:A {embedding: ai.vector([0.12, 0.45, 0.78])}) RETURN n", cfg); err != nil {
		t.Fatalf("float insert into VECTOR(3, INTEGER) should be accepted (element type ignored), got: %v", err)
	}

	// Stored as float, not coerced to integer.
	resp, err := testClient.Gql(ctx, "MATCH (n:A) RETURN n.embedding AS e", cfg)
	if err != nil || resp.RowCount != 1 {
		t.Fatalf("read back failed: err=%v rows=%d", err, resp.RowCount)
	}
	val, err := resp.GetByName(resp.Rows[0], "e")
	if err != nil {
		t.Fatalf("GetByName(e) failed: %v", err)
	}
	if s := fmt.Sprintf("%v", val); !strings.Contains(s, "0.1") {
		t.Errorf("expected float value ~0.12 stored unchanged, got %q", s)
	}

	// Dimension IS enforced: wrong length is rejected.
	if _, err := testClient.Gql(ctx,
		"INSERT (n:A {embedding: ai.vector([1.0, 2.0])}) RETURN n", cfg); err == nil {
		t.Errorf("dimension mismatch (2 vs VECTOR(3)) should be rejected, but insert succeeded")
	}
}
