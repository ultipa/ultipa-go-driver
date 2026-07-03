//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// End-to-end test of the high-level Loader API (services/loader_service.go +
// loader.go) against live 60063 (6.2.97). Run:
//   go test -tags integration -run TestLoaderAPI_EndToEnd -v ./tests/integration/
func TestLoaderAPI_EndToEnd(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	const (
		graph       = "loader_api_smoke"
		ontologyTTL = "/Users/ultipa/Downloads/demo/aichax-ontology.ttl"
		dataTTL     = "/Users/ultipa/Downloads/demo/aichax-demo.ttl"
	)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	c := testClient

	// Fresh ontology graph.
	_ = c.DropGraph(ctx, graph, true)
	if err := c.CreateGraph(ctx, graph, types.GraphTypeOntology, ""); err != nil {
		t.Fatalf("create ontology graph: %v", err)
	}
	defer func() { _ = c.DropGraph(ctx, graph, true) }()

	// Capabilities.
	if caps, err := c.GetLoaderCapabilities(ctx); err != nil {
		t.Logf("[caps] ERROR: %v", err)
	} else {
		t.Logf("[caps] ontology=%v data=%v maxUpload=%d remote=%v",
			caps.OntologyFormats, caps.DataFormats, caps.MaxUploadBytes, caps.RemoteSourceEnabled)
	}

	// LoadOntologyFile — format auto-detected from .ttl.
	onto, err := c.LoadOntologyFile(ctx, ontologyTTL, gqldb.LoadOntologyOptions{GraphName: graph})
	if err != nil {
		t.Fatalf("LoadOntologyFile: %v", err)
	}
	t.Logf("[LoadOntology] classes=%d objectProps=%d dataProps=%d prefixes=%d warnings=%v",
		onto.Classes, onto.ObjectProperties, onto.DataProperties, onto.PrefixesRegistered, onto.Warnings)
	if onto.Classes == 0 {
		t.Errorf("expected classes > 0")
	}

	// LoadDataFile.
	data, err := c.LoadDataFile(ctx, dataTTL, gqldb.LoadDataOptions{GraphName: graph})
	if err != nil {
		t.Fatalf("LoadDataFile: %v", err)
	}
	t.Logf("[LoadData] nodes=%d edges=%d warnings=%v", data.NodesCreated, data.EdgesCreated, data.Warnings)
	if data.NodesCreated == 0 || data.EdgesCreated == 0 {
		t.Errorf("expected nodes/edges > 0, got %d/%d", data.NodesCreated, data.EdgesCreated)
	}

	// Reasoning sanity: subclass roll-up should exceed the asserted base.
	count := func(label, stmt string) int {
		resp, err := c.Gql(ctx, stmt, &gqldb.QueryConfig{GraphName: graph})
		if err != nil {
			t.Logf("[query] %s ERROR: %v", label, err)
			return -1
		}
		n := len(resp.Rows)
		t.Logf("[query] %-22s rows=%d", label, n)
		return n
	}
	person := count("asserted Person", "MATCH (n@ax:Person) RETURN n LIMIT 100")
	agent := count("subclass Agent(infer)", "MATCH (n@ax:Agent) RETURN n LIMIT 100")
	if person > 0 && agent > 0 && agent < person {
		t.Errorf("subclass reasoning suspect: Agent(%d) < Person(%d)", agent, person)
	}
}
