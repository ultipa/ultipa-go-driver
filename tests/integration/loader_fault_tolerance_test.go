//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// Exercises the P1 parser fault-tolerance fields (server >= b37ae12) through the
// thin driver API: continue_on_error surfaces a per-line error; validate_only is
// a dry run. Run:
//   go test -tags integration -run TestLoaderFaultTolerance -v ./tests/integration/
func TestLoaderFaultTolerance(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c := testClient
	const graph = "loader_ft_smoke"

	_ = c.DropGraph(ctx, graph, true)
	if err := c.CreateGraph(ctx, graph, types.GraphTypeOntology, ""); err != nil {
		t.Fatalf("create graph: %v", err)
	}
	defer func() { _ = c.UseGraph(ctx, "miniCircle"); _ = c.DropGraph(ctx, graph, true) }()

	const badTTL = `@prefix ax: <http://aichax.com/ontology/> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
ax:Person a owl:Class .
@@@ this line is broken turtle ;;; nonsense
ax:Place a owl:Class .
`
	res, err := c.LoadOntology(ctx, strings.NewReader(badTTL), gqldb.LoadOntologyOptions{
		GraphName: graph, Format: "TURTLE", ContinueOnError: true,
	})
	if err != nil {
		t.Fatalf("LoadOntology(continue_on_error): %v", err)
	}
	t.Logf("[continue_on_error] parsed=%d failed=%d skipped=%d parserVersion=%q errors=%d",
		res.Parsed, res.Failed, res.Skipped, res.ParserVersionUsed, len(res.Errors))
	for _, e := range res.Errors {
		t.Logf("   error: line=%d reason=%q snippet=%q", e.Line, e.Reason, e.Snippet)
	}
	// Driver-side assertion = the new fault-tolerance fields round-trip from the
	// server response. (Whether a given malformed line is *recorded* as a failure
	// is server parser behavior, not a driver concern — logged above.)
	if res.ParserVersionUsed == "" {
		t.Errorf("expected parser_version_used to be populated (new P1 field); got empty")
	}
	if res.Parsed == 0 {
		t.Errorf("expected parsed > 0 (new P1 field); got 0")
	}

	const goodTTL = `@prefix ax: <http://aichax.com/ontology/> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
ax:Animal a owl:Class .
`
	res2, err := c.LoadOntology(ctx, strings.NewReader(goodTTL), gqldb.LoadOntologyOptions{
		GraphName: graph, Format: "TURTLE", ValidateOnly: true,
	})
	if err != nil {
		t.Fatalf("LoadOntology(validate_only): %v", err)
	}
	t.Logf("[validate_only] parsed=%d failed=%d parserVersion=%q", res2.Parsed, res2.Failed, res2.ParserVersionUsed)
}
