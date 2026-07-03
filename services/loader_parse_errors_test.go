package services

import (
	"testing"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
)

// convertParseErrors maps the proto fault-tolerance error entries to the public
// ParseError type (line/snippet/reason), and returns nil (not an empty slice)
// when there are no errors.
func TestConvertParseErrors(t *testing.T) {
	if got := convertParseErrors(nil); got != nil {
		t.Errorf("empty input: want nil, got %#v", got)
	}
	in := []*pb.ParseError{
		{Line: 4, Snippet: "@@@ bad", Reason: "syntax error"},
		{Line: 9, Snippet: "", Reason: "unknown prefix"},
	}
	got := convertParseErrors(in)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].Line != 4 || got[0].Snippet != "@@@ bad" || got[0].Reason != "syntax error" {
		t.Errorf("entry[0] mismatch: %#v", got[0])
	}
	if got[1].Line != 9 || got[1].Reason != "unknown prefix" {
		t.Errorf("entry[1] mismatch: %#v", got[1])
	}
}
