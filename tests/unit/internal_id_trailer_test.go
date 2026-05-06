package unit

import (
	"encoding/binary"
	"testing"

	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// Tests the binary wire-format compatibility of the 6.1.147 InternalID
// trailer added by the gqldb-grpc server's encodeNodeBinary /
// encodeEdgeBinary functions. The trailer is 8 bytes little-endian
// uint64 appended after the existing properties block.
//
// The server source for these encoders is at
// gqldb-grpc/server/converter/to_proto.go (encodeNodeBinary,
// encodeEdgeBinary). This test reproduces the same byte layout to
// confirm the driver-side decoders consume the trailer correctly and
// surface it on Node.UUID / Edge.UUID.

func TestNodeBinary_InternalIDTrailer_Roundtrip(t *testing.T) {
	const wantInternalID uint64 = 12345678901234

	// Hand-craft the wire format that gqldb 6.1.147 emits for a node
	// with id="n:1" / labels=["L"] / properties=empty / InternalID=N.
	data := buildNodeBinary(t, "n:1", []string{"L"}, wantInternalID)

	tv := &types.TypedValue{Type: types.PropertyTypeNode, Data: data}
	v, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo: %v", err)
	}
	node, ok := v.(*types.Node)
	if !ok {
		t.Fatalf("expected *types.Node, got %T", v)
	}
	if node.ID != "n:1" {
		t.Errorf("ID: got %q, want %q", node.ID, "n:1")
	}
	if want := "12345678901234"; node.UUID != want {
		t.Errorf("UUID: got %q, want %q", node.UUID, want)
	}
}

func TestNodeBinary_NoTrailer_LeavesUUIDEmpty(t *testing.T) {
	// Pre-6.1.147 wire format: no InternalID trailer.
	data := buildNodeBinary(t, "n:1", []string{"L"}, 0 /*no trailer*/)

	tv := &types.TypedValue{Type: types.PropertyTypeNode, Data: data}
	v, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo: %v", err)
	}
	node, ok := v.(*types.Node)
	if !ok {
		t.Fatalf("expected *types.Node, got %T", v)
	}
	if node.ID != "n:1" {
		t.Errorf("ID: got %q, want %q", node.ID, "n:1")
	}
	if node.UUID != "" {
		t.Errorf("UUID: got %q, want empty (pre-6.1.147 server)", node.UUID)
	}
}

func TestEdgeBinary_InternalIDTrailer_Roundtrip(t *testing.T) {
	const wantInternalID uint64 = 42
	data := buildEdgeBinary(t, "e:42", "rel", "n:1", "n:2", wantInternalID)

	tv := &types.TypedValue{Type: types.PropertyTypeEdge, Data: data}
	v, err := tv.ToGo()
	if err != nil {
		t.Fatalf("ToGo: %v", err)
	}
	edge, ok := v.(*types.Edge)
	if !ok {
		t.Fatalf("expected *types.Edge, got %T", v)
	}
	if edge.ID != "e:42" {
		t.Errorf("ID: got %q, want %q", edge.ID, "e:42")
	}
	if want := "42"; edge.UUID != want {
		t.Errorf("UUID: got %q, want %q", edge.UUID, want)
	}
}

// buildNodeBinary emits the same wire format as gqldb-grpc's
// encodeNodeBinary. trailer=0 means "no trailer" (simulates pre-6.1.147).
func buildNodeBinary(t *testing.T, id string, labels []string, trailer uint64) []byte {
	t.Helper()
	var buf []byte
	buf = appendString(buf, id)
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, uint16(len(labels)))
	buf = append(buf, tmp...)
	for _, l := range labels {
		buf = appendString(buf, l)
	}
	// Empty properties block: count=0
	buf = append(buf, 0x00, 0x00)
	if trailer != 0 {
		t8 := make([]byte, 8)
		binary.LittleEndian.PutUint64(t8, trailer)
		buf = append(buf, t8...)
	}
	return buf
}

func buildEdgeBinary(t *testing.T, id, label, from, to string, trailer uint64) []byte {
	t.Helper()
	var buf []byte
	buf = appendString(buf, id)
	buf = appendString(buf, label)
	buf = appendString(buf, from)
	buf = appendString(buf, to)
	buf = append(buf, 0x00, 0x00) // empty properties
	if trailer != 0 {
		t8 := make([]byte, 8)
		binary.LittleEndian.PutUint64(t8, trailer)
		buf = append(buf, t8...)
	}
	return buf
}

func appendString(buf []byte, s string) []byte {
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, uint16(len(s)))
	buf = append(buf, tmp...)
	return append(buf, []byte(s)...)
}
