package services

import (
	"testing"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
)

// convertDmlStats must return nil when the proto sub-message is absent OR all
// six counts are zero (semantics: nil/absent = "not a data-modifying query /
// pre-DmlStats server", NOT "changed nothing"). A non-zero count yields a
// populated struct with each field copied through 1:1.
func TestConvertDmlStats(t *testing.T) {
	if got := convertDmlStats(nil); got != nil {
		t.Fatalf("nil proto: want nil, got %+v", got)
	}

	if got := convertDmlStats(&pb.DmlStats{}); got != nil {
		t.Fatalf("all-zero proto: want nil, got %+v", got)
	}

	raw := &pb.DmlStats{
		InsertedNodes: 1,
		InsertedEdges: 2,
		DeletedNodes:  3,
		DeletedEdges:  4,
		SetNodes:      5,
		SetEdges:      6,
	}
	got := convertDmlStats(raw)
	if got == nil {
		t.Fatal("populated proto: want non-nil, got nil")
	}
	if got.InsertedNodes != 1 || got.InsertedEdges != 2 ||
		got.DeletedNodes != 3 || got.DeletedEdges != 4 ||
		got.SetNodes != 5 || got.SetEdges != 6 {
		t.Fatalf("field mismatch: %+v", got)
	}

	// A single non-zero category must still surface (not treated as all-zero).
	if got := convertDmlStats(&pb.DmlStats{DeletedEdges: 1}); got == nil ||
		got.DeletedEdges != 1 {
		t.Fatalf("single non-zero: want DeletedEdges=1, got %+v", got)
	}
}
