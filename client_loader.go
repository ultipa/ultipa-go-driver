package gqldb

import (
	"context"
	"io"

	"github.com/ultipa/ultipa-go-driver/v6/services"
)

// =============================================================================
// Loader Service — upload local files and load them into a graph.
//
// The file lives on the CLIENT; these methods stream its bytes to the server,
// which parses and loads (ontology schema / RDF instance data / CSV / prefixes).
// This closes the gap left by GQL `LOAD ... FROM '<server-path|url>'`, which
// only sees server-reachable sources. For that server-side form, use the
// *FromSource variants.
//
// Typical flow for an ontology graph (schema before data):
//
//	c.CreateGraph(ctx, "g", types.GraphTypeOntology, "")
//	c.LoadOntologyFile(ctx, "onto.ttl", gqldb.LoadOntologyOptions{GraphName: "g"})
//	c.LoadDataFile(ctx, "data.ttl", gqldb.LoadDataOptions{GraphName: "g"})
// =============================================================================

// Re-exported loader types for ergonomic main-package use.
type (
	LoadOntologyResult  = services.LoadOntologyResult
	LoadDataResult      = services.LoadDataResult
	LoadCsvResult       = services.LoadCsvResult
	LoadPrefixResult    = services.LoadPrefixResult
	LoaderCapabilities  = services.LoaderCapabilities
	ParseError          = services.ParseError
	LoadOntologyOptions = services.LoadOntologyOptions
	LoadDataOptions     = services.LoadDataOptions
	LoadCsvOptions      = services.LoadCsvOptions
	LoadPrefixOptions   = services.LoadPrefixOptions
	CsvColumnMapping    = services.CsvColumnMapping
)

// ---- Ontology ----

// LoadOntologyFile uploads and loads an ontology schema from a local file.
// Format is auto-detected from the extension when opts.Format is empty.
func (c *Client) LoadOntologyFile(ctx context.Context, path string, opts LoadOntologyOptions) (*LoadOntologyResult, error) {
	return c.loaderSvc.LoadOntologyFile(ctx, path, opts)
}

// LoadOntology uploads and loads an ontology schema streamed from r.
func (c *Client) LoadOntology(ctx context.Context, r io.Reader, opts LoadOntologyOptions) (*LoadOntologyResult, error) {
	return c.loaderSvc.LoadOntology(ctx, r, opts)
}

// LoadOntologyFromSource loads an ontology from a server-reachable path/URL.
func (c *Client) LoadOntologyFromSource(ctx context.Context, source string, opts LoadOntologyOptions) (*LoadOntologyResult, error) {
	return c.loaderSvc.LoadOntologyFromSource(ctx, source, opts)
}

// ---- RDF instance data ----

// LoadDataFile uploads and loads RDF instance data from a local file.
func (c *Client) LoadDataFile(ctx context.Context, path string, opts LoadDataOptions) (*LoadDataResult, error) {
	return c.loaderSvc.LoadDataFile(ctx, path, opts)
}

// LoadData uploads and loads RDF instance data streamed from r.
func (c *Client) LoadData(ctx context.Context, r io.Reader, opts LoadDataOptions) (*LoadDataResult, error) {
	return c.loaderSvc.LoadData(ctx, r, opts)
}

// LoadDataFromSource loads RDF instance data from a server-reachable path/URL.
func (c *Client) LoadDataFromSource(ctx context.Context, source string, opts LoadDataOptions) (*LoadDataResult, error) {
	return c.loaderSvc.LoadDataFromSource(ctx, source, opts)
}

// ---- CSV ----

// LoadCsvFile uploads and imports CSV rows from a local file as nodes or edges.
func (c *Client) LoadCsvFile(ctx context.Context, path string, opts LoadCsvOptions) (*LoadCsvResult, error) {
	return c.loaderSvc.LoadCsvFile(ctx, path, opts)
}

// LoadCsv uploads and imports CSV rows streamed from r.
func (c *Client) LoadCsv(ctx context.Context, r io.Reader, opts LoadCsvOptions) (*LoadCsvResult, error) {
	return c.loaderSvc.LoadCsv(ctx, r, opts)
}

// ---- Prefix + capabilities ----

// LoadPrefix registers namespace prefixes (single, standard set, or bulk-from-URL).
func (c *Client) LoadPrefix(ctx context.Context, opts LoadPrefixOptions) (*LoadPrefixResult, error) {
	return c.loaderSvc.LoadPrefix(ctx, opts)
}

// GetLoaderCapabilities reports the server's supported loader formats and limits.
func (c *Client) GetLoaderCapabilities(ctx context.Context) (*LoaderCapabilities, error) {
	return c.loaderSvc.GetLoaderCapabilities(ctx)
}
