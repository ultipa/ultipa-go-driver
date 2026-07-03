//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// p0ScientistTTL is the minimal repro ontology: a defined class authored via
// owl:equivalentClass + a blank-node owl:Restriction. Written to a temp file so
// the test is self-contained (LOAD ONTOLOGY reads it via file:// on the local server).
const p0ScientistTTL = `@prefix ax:   <http://aichax.com/ontology/> .
@prefix owl:  <http://www.w3.org/2002/07/owl#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

ax:Person      a owl:Class .
ax:FieldOfWork a owl:Class .
ax:Science     a owl:Class ; rdfs:subClassOf ax:FieldOfWork .
ax:fieldOfWork a owl:ObjectProperty ; rdfs:domain ax:Person ; rdfs:range ax:FieldOfWork .

ax:Scientist a owl:Class ; rdfs:subClassOf ax:Person .
ax:Scientist owl:equivalentClass
  [ a owl:Class ; owl:intersectionOf
    ( ax:Person [ a owl:Restriction ; owl:onProperty ax:fieldOfWork ; owl:someValuesFrom ax:Science ] ) ] .
`

// Verifies the P0 finding: a defined class authored as owl:equivalentClass +
// blank-node owl:Restriction in TTL is NOT materialized by LOAD ONTOLOGY (TTL
// parse path), whereas the SAME defined class authored via explicit GQL DDL
// (CREATE CLASS ... EQUIVALENT TO ... AND (... SOME ...)) DOES fire — with
// identical instance data. If A=0 and B>=1, the bug is real and isolated to
// the TTL parser.
//   go test -tags integration -run TestP0_DefinedClass_TTLvsDDL -v ./tests/integration/
func TestP0_DefinedClass_TTLvsDDL(t *testing.T) {
	const (
		addr = "127.0.0.1:60063"
		user = "admin"
		pass = "admin11"
	)
	// Write the ontology to a temp file the local server can read via file://.
	tmpFile := filepath.Join(t.TempDir(), "p0_scientist.ttl")
	if err := os.WriteFile(tmpFile, []byte(p0ScientistTTL), 0o644); err != nil {
		t.Fatalf("write ttl: %v", err)
	}
	ttlPath := "file://" + tmpFile
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	sess := pb.NewSessionServiceClient(conn)
	lr, err := sess.Login(ctx, &pb.LoginRequest{Username: user, Password: pass})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	authCtx := metadata.AppendToOutgoingContext(ctx, "session-id", fmt.Sprintf("%d", lr.GetSessionId()))
	q := pb.NewQueryServiceClient(conn)

	run := func(graph, stmt string) (*pb.GqlResponse, error) {
		return q.Gql(authCtx, &pb.GqlRequest{Gql: stmt, GraphName: graph})
	}
	mustRun := func(graph, stmt string) {
		if _, err := run(graph, stmt); err != nil {
			t.Fatalf("[%s] %q\n  -> %v", graph, stmt, err)
		}
	}
	count := func(graph, label, stmt string) int {
		r, err := run(graph, stmt)
		if err != nil {
			t.Logf("    %-22s ERROR: %v", label, err)
			return -1
		}
		t.Logf("    %-22s rows=%d", label, len(r.GetRows()))
		return len(r.GetRows())
	}
	// instance data shared by both graphs: one Person + one Science + fieldOfWork edge.
	insertData := func(graph string) {
		mustRun(graph, "INSERT (n@ax:Person { wikidataId: 'Q937', name: 'Einstein' })")
		mustRun(graph, "INSERT (n@ax:Science { wikidataId: 'QSCI', name: 'Physics' })")
		mustRun(graph, "MATCH (a {wikidataId:'Q937'}),(b {wikidataId:'QSCI'}) INSERT (a)-[@ax:fieldOfWork]->(b)")
	}
	probe := func(graph string) (person, edge, scientist int) {
		person = count(graph, "asserted Person", "MATCH (n@ax:Person) RETURN n LIMIT 50")
		edge = count(graph, "asserted fieldOfWork", "MATCH (a)-[r@ax:fieldOfWork]->(b) RETURN a,b LIMIT 50")
		scientist = count(graph, "defined Scientist", "MATCH (n@ax:Scientist) RETURN n LIMIT 50")
		return
	}

	// ---------- A: TTL parse path (LOAD ONTOLOGY FROM file://) ----------
	const gA = "p0_ttl"
	_, _ = run("", "DROP GRAPH "+gA)
	mustRun("", "CREATE GRAPH "+gA+" WITH ONTOLOGY")
	defer func() { _, _ = run("", "DROP GRAPH "+gA) }()
	if _, err := run(gA, "LOAD ONTOLOGY FROM '"+ttlPath+"'"); err != nil {
		t.Fatalf("[A] LOAD ONTOLOGY: %v", err)
	}
	insertData(gA)
	t.Log("=== A) TTL parse path (owl:equivalentClass + blank-node Restriction) ===")
	_, _, scA := probe(gA)

	// ---------- B: explicit GQL DDL control ----------
	const gB = "p0_gql"
	_, _ = run("", "DROP GRAPH "+gB)
	mustRun("", "CREATE GRAPH "+gB+" WITH ONTOLOGY")
	defer func() { _, _ = run("", "DROP GRAPH "+gB) }()
	mustRun(gB, "LOAD PREFIX ax FROM 'http://aichax.com/ontology/'")
	mustRun(gB, "CREATE CLASS @ax:Person")
	mustRun(gB, "CREATE CLASS @ax:FieldOfWork")
	mustRun(gB, "CREATE CLASS @ax:Science SUBCLASS OF @ax:FieldOfWork")
	mustRun(gB, "CREATE OBJECT PROPERTY @ax:fieldOfWork DOMAIN @ax:Person RANGE @ax:FieldOfWork")
	mustRun(gB, "CREATE CLASS @ax:Scientist EQUIVALENT TO @ax:Person AND (@ax:fieldOfWork SOME @ax:Science)")
	insertData(gB)
	t.Log("=== B) explicit GQL DDL (CREATE CLASS ... EQUIVALENT TO ...) ===")
	_, _, scB := probe(gB)

	// ---------- verdict (regression: P0 fixed) ----------
	// History: on 6.2.97/6.2.100 the TTL path returned 0 (P0 bug — LoadOntology
	// dropped the owl:equivalentClass + blank-node Restriction defined class).
	// After the rdf-loader shared parser landed, both paths must fire.
	t.Logf("VERDICT: TTL-path Scientist=%d  vs  DDL-path Scientist=%d", scA, scB)
	if scB < 1 {
		t.Fatalf("control broken: DDL-path defined-class did not fire (Scientist=%d)", scB)
	}
	if scA < 1 {
		t.Errorf("P0 REGRESSED: TTL-path defined-class not materialized (Scientist=%d); "+
			"LoadOntology dropped owl:equivalentClass+blank-node Restriction again", scA)
	} else {
		t.Logf("✅ P0 FIXED: defined-class fires via both TTL parse (%d) and GQL DDL (%d) on identical data.", scA, scB)
	}
}
