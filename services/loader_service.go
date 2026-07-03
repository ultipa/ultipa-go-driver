package services

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
)

// loaderChunkSize is the upload chunk size for the client-streaming Load* RPCs.
const loaderChunkSize = 256 * 1024

// LoaderService wraps the gRPC LoaderService (upload-and-load: ontology / RDF
// data / CSV / prefixes). The file lives on the CLIENT; these RPCs stream its
// bytes to the server, which parses and loads. (GQL `LOAD ... FROM` only sees
// server-reachable paths/URLs — these close that gap.)
type LoaderService struct {
	ctx *ServiceContext
}

// NewLoaderService creates a new LoaderService.
func NewLoaderService(ctx *ServiceContext) *LoaderService {
	return &LoaderService{ctx: ctx}
}

// ---- Result types ----

// LoadOntologyResult is the typed outcome of loading an ontology schema.
type LoadOntologyResult struct {
	IRI                string
	Classes            int64
	ObjectProperties   int64
	DataProperties     int64
	PrefixesRegistered int64
	Prefixes           map[string]string
	Warnings           []string
	// Parser fault-tolerance accounting (server ≥ b37ae12; zero/empty on older servers).
	Parsed            int64
	Failed            int64
	Skipped           int64
	ParserVersionUsed string
	Errors            []ParseError
	TimeCostNs        int64
	DiskCostNs        int64
	ComputeCostNs     int64
}

// LoadDataResult is the typed outcome of loading RDF instance data.
type LoadDataResult struct {
	NodesCreated       int64
	EdgesCreated       int64
	PrefixesRegistered int64
	Prefixes           map[string]string
	Warnings           []string
	// Parser fault-tolerance accounting (server ≥ b37ae12; zero/empty on older servers).
	Parsed            int64
	Failed            int64
	Skipped           int64
	ParserVersionUsed string
	Errors            []ParseError
	TimeCostNs        int64
	DiskCostNs        int64
	ComputeCostNs     int64
}

// ParseError is one non-fatal parse failure surfaced by a load (continue_on_error).
type ParseError struct {
	Line    int64
	Snippet string
	Reason  string
}

// LoadCsvResult is the typed outcome of importing CSV rows.
type LoadCsvResult struct {
	Imported      int64
	Skipped       int64
	IsEdge        bool
	TimeCostNs    int64
	DiskCostNs    int64
	ComputeCostNs int64
}

// LoadPrefixResult is the typed outcome of registering prefixes.
type LoadPrefixResult struct {
	Registered int64
	Updated    int64
	Prefixes   map[string]string
	TimeCostNs int64
}

// LoaderCapabilities reports supported formats and limits.
type LoaderCapabilities struct {
	OntologyFormats     []string
	DataFormats         []string
	MaxUploadBytes      int64
	RemoteSourceEnabled bool
}

// ---- Option types ----

// LoadOntologyOptions configures an ontology load.
type LoadOntologyOptions struct {
	GraphName string // optional; falls back to the session's current graph
	Format    string // OWL|RDFXML|TURTLE|NTRIPLES — required for upload (auto-detected by *File from extension)
	BaseIRI   string // optional: base IRI for resolving relative IRIs
	// Parser fault-tolerance (server ≥ b37ae12; ignored by older servers).
	ValidateOnly    bool   // dry run: parse + validate only, persist nothing
	ContinueOnError bool   // skip bad statements + record them in Errors, don't fail the load
	ParserVersion   string // pin a parser version; "" = server default/stable
}

// LoadDataOptions configures an RDF instance-data load.
type LoadDataOptions struct {
	GraphName string // optional; falls back to the session's current graph
	Format    string // TURTLE|NTRIPLES — required for upload (auto-detected by *File)
	BaseIRI   string
	// Parser fault-tolerance (server ≥ b37ae12; ignored by older servers).
	ValidateOnly    bool
	ContinueOnError bool
	ParserVersion   string
}

// LoadCsvOptions configures a CSV import.
type LoadCsvOptions struct {
	GraphName   string             // optional; falls back to the session's current graph
	Label       string             // required: node label or edge type
	Edge        bool               // import as edges (requires EDGE_ID enabled)
	EdgeFromCol string             // edge import: CSV column holding source node _id
	EdgeToCol   string             // edge import: CSV column holding target node _id
	WithHeader  bool               // first row holds column names
	Delimiter   string             // default ","
	Quote       string             // accepted for compatibility
	Skip        int64              // leading rows to discard
	Mapping     []CsvColumnMapping // explicit property↔column bindings; empty = auto by header
}

// CsvColumnMapping binds a property to a CSV column with an optional type.
type CsvColumnMapping struct {
	Property string
	Column   string
	Type     string // "" | STRING | INT | FLOAT | BOOL | DATE | DATETIME | TIMESTAMP | ZONED_DATETIME | DURATION | DECIMAL | BYTES | POINT | POINT3D | TIME
}

// LoadPrefixOptions configures prefix registration. Use one of: (Name+IRI) for
// a single prefix, AllStandard for the built-in set, or Source to bulk-register
// every prefix declared in the RDF doc at that URL.
type LoadPrefixOptions struct {
	GraphName   string
	Name        string
	IRI         string
	AllStandard bool
	Source      string
}

// =============================================================================
// Ontology
// =============================================================================

// LoadOntology streams an ontology document from r and loads it into the graph.
func (s *LoaderService) LoadOntology(ctx context.Context, r io.Reader, opts LoadOntologyOptions) (*LoadOntologyResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)
	stream, err := s.ctx.LoaderClient.LoadOntology(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(&pb.LoadOntologyRequest{Msg: &pb.LoadOntologyRequest_Header{Header: &pb.LoadOntologyHeader{
		GraphName: opts.GraphName, Format: opts.Format, BaseIri: opts.BaseIRI,
		ValidateOnly: opts.ValidateOnly, ContinueOnError: opts.ContinueOnError, ParserVersion: opts.ParserVersion,
	}}}); err != nil {
		return nil, err
	}
	if err := streamChunks(r, func(b []byte) error {
		return stream.Send(&pb.LoadOntologyRequest{Msg: &pb.LoadOntologyRequest_Chunk{Chunk: b}})
	}); err != nil {
		return nil, err
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}
	return &LoadOntologyResult{
		IRI: resp.GetIri(), Classes: resp.GetClasses(), ObjectProperties: resp.GetObjectProperties(),
		DataProperties: resp.GetDataProperties(), PrefixesRegistered: resp.GetPrefixesRegistered(),
		Prefixes: resp.GetPrefixes(), Warnings: resp.GetWarnings(),
		Parsed: resp.GetParsed(), Failed: resp.GetFailed(), Skipped: resp.GetSkipped(),
		ParserVersionUsed: resp.GetParserVersionUsed(), Errors: convertParseErrors(resp.GetErrors()),
		TimeCostNs: resp.GetTimeCostNs(), DiskCostNs: resp.GetDiskCostNs(), ComputeCostNs: resp.GetComputeCostNs(),
	}, nil
}

// LoadOntologyFile opens path and streams it. Format is auto-detected from the
// extension when opts.Format is empty.
func (s *LoaderService) LoadOntologyFile(ctx context.Context, path string, opts LoadOntologyOptions) (*LoadOntologyResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if opts.Format == "" {
		opts.Format = detectRDFFormat(path)
	}
	return s.LoadOntology(ctx, f, opts)
}

// LoadOntologyFromSource asks the server to load from a server-reachable
// path/URL (no upload), for parity with the GQL LOAD form.
func (s *LoaderService) LoadOntologyFromSource(ctx context.Context, source string, opts LoadOntologyOptions) (*LoadOntologyResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)
	stream, err := s.ctx.LoaderClient.LoadOntology(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(&pb.LoadOntologyRequest{Msg: &pb.LoadOntologyRequest_Header{Header: &pb.LoadOntologyHeader{
		GraphName: opts.GraphName, Format: opts.Format, Source: source, BaseIri: opts.BaseIRI,
		ValidateOnly: opts.ValidateOnly, ContinueOnError: opts.ContinueOnError, ParserVersion: opts.ParserVersion,
	}}}); err != nil {
		return nil, err
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}
	return &LoadOntologyResult{
		IRI: resp.GetIri(), Classes: resp.GetClasses(), ObjectProperties: resp.GetObjectProperties(),
		DataProperties: resp.GetDataProperties(), PrefixesRegistered: resp.GetPrefixesRegistered(),
		Prefixes: resp.GetPrefixes(), Warnings: resp.GetWarnings(),
		Parsed: resp.GetParsed(), Failed: resp.GetFailed(), Skipped: resp.GetSkipped(),
		ParserVersionUsed: resp.GetParserVersionUsed(), Errors: convertParseErrors(resp.GetErrors()),
		TimeCostNs: resp.GetTimeCostNs(), DiskCostNs: resp.GetDiskCostNs(), ComputeCostNs: resp.GetComputeCostNs(),
	}, nil
}

// =============================================================================
// RDF instance data
// =============================================================================

// LoadData streams RDF instance data from r and loads it as nodes/edges.
func (s *LoaderService) LoadData(ctx context.Context, r io.Reader, opts LoadDataOptions) (*LoadDataResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)
	stream, err := s.ctx.LoaderClient.LoadData(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(&pb.LoadDataRequest{Msg: &pb.LoadDataRequest_Header{Header: &pb.LoadDataHeader{
		GraphName: opts.GraphName, Format: opts.Format, BaseIri: opts.BaseIRI,
		ValidateOnly: opts.ValidateOnly, ContinueOnError: opts.ContinueOnError, ParserVersion: opts.ParserVersion,
	}}}); err != nil {
		return nil, err
	}
	if err := streamChunks(r, func(b []byte) error {
		return stream.Send(&pb.LoadDataRequest{Msg: &pb.LoadDataRequest_Chunk{Chunk: b}})
	}); err != nil {
		return nil, err
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}
	return &LoadDataResult{
		NodesCreated: resp.GetNodesCreated(), EdgesCreated: resp.GetEdgesCreated(),
		PrefixesRegistered: resp.GetPrefixesRegistered(), Prefixes: resp.GetPrefixes(), Warnings: resp.GetWarnings(),
		Parsed: resp.GetParsed(), Failed: resp.GetFailed(), Skipped: resp.GetSkipped(),
		ParserVersionUsed: resp.GetParserVersionUsed(), Errors: convertParseErrors(resp.GetErrors()),
		TimeCostNs: resp.GetTimeCostNs(), DiskCostNs: resp.GetDiskCostNs(), ComputeCostNs: resp.GetComputeCostNs(),
	}, nil
}

// LoadDataFile opens path and streams it. Format auto-detected when empty.
func (s *LoaderService) LoadDataFile(ctx context.Context, path string, opts LoadDataOptions) (*LoadDataResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if opts.Format == "" {
		opts.Format = detectRDFFormat(path)
	}
	return s.LoadData(ctx, f, opts)
}

// LoadDataFromSource loads RDF instance data from a server-reachable path/URL.
func (s *LoaderService) LoadDataFromSource(ctx context.Context, source string, opts LoadDataOptions) (*LoadDataResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)
	stream, err := s.ctx.LoaderClient.LoadData(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(&pb.LoadDataRequest{Msg: &pb.LoadDataRequest_Header{Header: &pb.LoadDataHeader{
		GraphName: opts.GraphName, Format: opts.Format, Source: source, BaseIri: opts.BaseIRI,
		ValidateOnly: opts.ValidateOnly, ContinueOnError: opts.ContinueOnError, ParserVersion: opts.ParserVersion,
	}}}); err != nil {
		return nil, err
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}
	return &LoadDataResult{
		NodesCreated: resp.GetNodesCreated(), EdgesCreated: resp.GetEdgesCreated(),
		PrefixesRegistered: resp.GetPrefixesRegistered(), Prefixes: resp.GetPrefixes(), Warnings: resp.GetWarnings(),
		Parsed: resp.GetParsed(), Failed: resp.GetFailed(), Skipped: resp.GetSkipped(),
		ParserVersionUsed: resp.GetParserVersionUsed(), Errors: convertParseErrors(resp.GetErrors()),
		TimeCostNs: resp.GetTimeCostNs(), DiskCostNs: resp.GetDiskCostNs(), ComputeCostNs: resp.GetComputeCostNs(),
	}, nil
}

// =============================================================================
// CSV
// =============================================================================

// LoadCsv streams CSV bytes from r and imports rows as nodes or edges.
func (s *LoaderService) LoadCsv(ctx context.Context, r io.Reader, opts LoadCsvOptions) (*LoadCsvResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)
	stream, err := s.ctx.LoaderClient.LoadCsv(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(&pb.LoadCsvRequest{Msg: &pb.LoadCsvRequest_Header{Header: csvHeader(opts)}}); err != nil {
		return nil, err
	}
	if err := streamChunks(r, func(b []byte) error {
		return stream.Send(&pb.LoadCsvRequest{Msg: &pb.LoadCsvRequest_Chunk{Chunk: b}})
	}); err != nil {
		return nil, err
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}
	return &LoadCsvResult{
		Imported: resp.GetImported(), Skipped: resp.GetSkipped(), IsEdge: resp.GetIsEdge(),
		TimeCostNs: resp.GetTimeCostNs(), DiskCostNs: resp.GetDiskCostNs(), ComputeCostNs: resp.GetComputeCostNs(),
	}, nil
}

// LoadCsvFile opens path and streams it.
func (s *LoaderService) LoadCsvFile(ctx context.Context, path string, opts LoadCsvOptions) (*LoadCsvResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return s.LoadCsv(ctx, f, opts)
}

func csvHeader(opts LoadCsvOptions) *pb.LoadCsvHeader {
	h := &pb.LoadCsvHeader{
		GraphName: opts.GraphName, Label: opts.Label, Edge: opts.Edge,
		EdgeFromCol: opts.EdgeFromCol, EdgeToCol: opts.EdgeToCol, WithHeader: opts.WithHeader,
		Delimiter: opts.Delimiter, Quote: opts.Quote, Skip: opts.Skip,
	}
	for _, m := range opts.Mapping {
		h.Mapping = append(h.Mapping, &pb.CsvColumnMapping{Property: m.Property, Column: m.Column, Type: m.Type})
	}
	return h
}

// =============================================================================
// Prefix (unary) + capabilities
// =============================================================================

// LoadPrefix registers namespace prefixes (single, standard set, or bulk-from-URL).
func (s *LoaderService) LoadPrefix(ctx context.Context, opts LoadPrefixOptions) (*LoadPrefixResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)
	resp, err := s.ctx.LoaderClient.LoadPrefix(ctx, &pb.LoadPrefixRequest{
		GraphName: opts.GraphName, Name: opts.Name, Iri: opts.IRI, AllStandard: opts.AllStandard, Source: opts.Source,
	})
	if err != nil {
		return nil, err
	}
	return &LoadPrefixResult{
		Registered: resp.GetRegistered(), Updated: resp.GetUpdated(),
		Prefixes: resp.GetPrefixes(), TimeCostNs: resp.GetTimeCostNs(),
	}, nil
}

// GetLoaderCapabilities reports supported formats and limits.
func (s *LoaderService) GetLoaderCapabilities(ctx context.Context) (*LoaderCapabilities, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)
	resp, err := s.ctx.LoaderClient.GetLoaderCapabilities(ctx, &pb.GetLoaderCapabilitiesRequest{})
	if err != nil {
		return nil, err
	}
	return &LoaderCapabilities{
		OntologyFormats: resp.GetOntologyFormats(), DataFormats: resp.GetDataFormats(),
		MaxUploadBytes: resp.GetMaxUploadBytes(), RemoteSourceEnabled: resp.GetRemoteSourceEnabled(),
	}, nil
}

// ---- helpers ----

// convertParseErrors maps proto ParseError entries to the public ParseError type.
func convertParseErrors(in []*pb.ParseError) []ParseError {
	if len(in) == 0 {
		return nil
	}
	out := make([]ParseError, 0, len(in))
	for _, e := range in {
		out = append(out, ParseError{Line: e.GetLine(), Snippet: e.GetSnippet(), Reason: e.GetReason()})
	}
	return out
}

// streamChunks reads r in loaderChunkSize blocks and calls send for each.
func streamChunks(r io.Reader, send func([]byte) error) error {
	buf := make([]byte, loaderChunkSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if serr := send(buf[:n]); serr != nil {
				return serr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// detectRDFFormat maps a file extension to the server's format token. Returns
// "" when unknown (caller must then set Format explicitly).
func detectRDFFormat(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttl":
		return "TURTLE"
	case ".nt":
		return "NTRIPLES"
	case ".owl", ".rdf", ".xml":
		return "RDFXML"
	case ".nq":
		return "NQUADS"
	case ".trig":
		return "TRIG"
	case ".jsonld":
		return "JSONLD"
	}
	return ""
}
