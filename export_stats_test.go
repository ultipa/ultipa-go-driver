package gqldb

import (
	"context"
	"fmt"
	"net"
	"testing"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc"
)

// Review round 1 of the error-code batch, item 8: the drivers' proto lacked
// ExportStats fields 8-12, so the completeness report the server sends with
// an export (expected and skipped counts, warnings) never reached a caller.

type exportNode struct {
	pb.UnimplementedDataServiceServer
}

func (exportNode) Export(_ *pb.ExportRequest, stream pb.DataService_ExportServer) error {
	if err := stream.Send(&pb.ExportResponse{Data: []byte("{}\n")}); err != nil {
		return err
	}
	return stream.Send(&pb.ExportResponse{IsFinal: true, Stats: &pb.ExportStats{
		NodesExported: 3, EdgesExported: 1, BytesWritten: 99, DurationMs: 5,
		NodesExpected: 4, EdgesExpected: 2, NodesSkipped: 1, EdgesSkipped: 1,
		Warnings: []string{"1 of 4 nodes could not be read", "edge count mismatch"},
	}})
}

func TestExport_CompletenessReachesTheCaller(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterDataServiceServer(srv, exportNode{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	client, err := NewClient(NewConfigBuilder().Hosts(lis.Addr().String()).HealthCheckInterval(0).Build())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	var final *ExportStats
	err = client.Export(context.Background(), &ExportConfig{GraphName: "g", ExportNodes: true, ExportEdges: true},
		func(r *ExportResult) error {
			if r.IsFinal {
				final = r.Stats
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if final == nil {
		t.Fatal("no stats on the final frame")
	}
	got := fmt.Sprint(final.NodesExpected, final.EdgesExpected, final.NodesSkipped, final.EdgesSkipped, final.Warnings)
	if want := "4 2 1 1 [1 of 4 nodes could not be read edge count mismatch]"; got != want {
		t.Errorf("completeness %q, want %q", got, want)
	}
	if final.NodesExported != 3 || final.DurationMs != 5 {
		t.Errorf("the older fields changed: %+v", final)
	}
}
