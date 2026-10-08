package services

import (
	"context"
	"testing"
	"time"
)

// The request's timeout field is whole seconds (proto). When it comes from
// the context's deadline, the time left is rounded UP: a field smaller than
// the caller's deadline became the server's own limit, used exactly with no
// margin, and with an 11.003 s deadline the server stopped at 11 s, so the
// "[5024] partly stored" answer arrived after the caller had given up (review
// round 4, item 3). Rounded up, the caller's deadline is the limit that binds,
// and a write gets the margin. The exact deadline itself travels in the gRPC
// deadline header.
func TestBuildGqlRequest_ContextDeadlineRoundsUp(t *testing.T) {
	s := NewQueryService(&ServiceContext{GetSessionID: func() uint64 { return 0 }})
	noGraph := func() string { return "" }
	clientDefault := func() int { return 30 }
	cases := []struct {
		left time.Duration
		want int32
	}{
		{1 * time.Millisecond, 1},
		{300 * time.Millisecond, 1},
		{999 * time.Millisecond, 1},
		{1500 * time.Millisecond, 2},
		{2 * time.Second, 2}, // a little under 2 s by the time it is read
		{11003 * time.Millisecond, 12},
		{30 * time.Second, 30},
	}
	for _, c := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), c.left)
		req, err := s.buildGqlRequest(ctx, "RETURN 1", nil, nil, noGraph, clientDefault)
		cancel()
		if err != nil {
			t.Fatalf("deadline %v: %v", c.left, err)
		}
		if req.Timeout != c.want {
			t.Errorf("deadline %v: request timeout %d s, want %d s (rounded up)", c.left, req.Timeout, c.want)
		}
	}

	// A per-call timeout wins over the context, and no deadline at all falls
	// back to the client's timeout, as before.
	ctx, cancel := context.WithTimeout(context.Background(), 11003*time.Millisecond)
	defer cancel()
	if req, _ := s.buildGqlRequest(ctx, "RETURN 1", &QueryConfig{Timeout: 5}, nil, noGraph, clientDefault); req.Timeout != 5 {
		t.Errorf("per-call timeout: got %d s, want 5 s", req.Timeout)
	}
	if req, _ := s.buildGqlRequest(context.Background(), "RETURN 1", nil, nil, noGraph, clientDefault); req.Timeout != 30 {
		t.Errorf("no deadline: got %d s, want the client's 30 s", req.Timeout)
	}
}
