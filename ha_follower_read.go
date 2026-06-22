package gqldb

import (
	"context"
	"sync"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
	"google.golang.org/grpc"
)

// followerStatusTTL bounds how long a cached per-host HA status (role + applied index) is reused
// before a fresh HAService.GetStatus sweep. Short enough that staleness decisions track reality,
// long enough that a burst of follower reads doesn't pay a status RPC each.
const followerStatusTTL = time.Second

// hostHAStatus is one node's HA status as last seen via HAService.GetStatus.
type hostHAStatus struct {
	isLeader bool
	applied  uint64 // this node's Raft applied index
	enabled  bool   // false on a single-node / non-HA server
	ok       bool   // the GetStatus call succeeded
}

// followerRouter caches the cluster's per-host HA status so follower-read selection can compute each
// follower's lag (leader.applied - follower.applied) without an RPC per read.
type followerRouter struct {
	mu        sync.Mutex
	statuses  map[string]hostHAStatus // gRPC host -> status
	leaderIdx int                     // index into config.Hosts of the last-known leader (-1 if unknown)
	fetchedAt time.Time
	rr        int // round-robin cursor, spreads reads across eligible followers
}

// followers lazily initializes the per-client follower router.
func (c *Client) followers() *followerRouter {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.followerRouter == nil {
		c.followerRouter = &followerRouter{leaderIdx: -1}
	}
	return c.followerRouter
}

// refreshHAStatusLocked re-polls HAService.GetStatus on every configured host and records each
// node's role + applied index. Caller holds fr.mu. The polls are TTL-gated, so the lock is held
// across network I/O only on an infrequent refresh.
func (c *Client) refreshHAStatusLocked(ctx context.Context, fr *followerRouter) {
	hosts := c.config.Hosts
	statuses := make(map[string]hostHAStatus, len(hosts))
	leaderIdx := -1
	for i, host := range hosts {
		st := hostHAStatus{}
		if conn, err := c.pool.GetConnectionForHost(host); err == nil {
			cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			resp, gerr := pb.NewHAServiceClient(conn).GetStatus(cctx, &pb.HAGetStatusRequest{})
			cancel()
			if gerr == nil && resp.GetStatus() != nil {
				s := resp.GetStatus()
				st.ok = true
				st.enabled = s.GetEnabled()
				st.isLeader = s.GetIsLeader()
				st.applied = s.GetAppliedIndex()
				if st.isLeader {
					leaderIdx = i
				}
			}
		}
		statuses[host] = st
	}
	fr.statuses = statuses
	fr.leaderIdx = leaderIdx
	fr.fetchedAt = time.Now()
}

// selectFollowerConn returns a healthy follower connection within maxStaleness entries of the
// leader, or ok=false when none qualifies (so the caller routes to the leader). maxStaleness == 0
// means any healthy follower is acceptable. It returns ok=false on a single-node / non-HA server.
func (c *Client) selectFollowerConn(ctx context.Context, maxStaleness uint64) (conn *grpc.ClientConn, host string, ok bool) {
	fr := c.followers()
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if fr.statuses == nil || time.Since(fr.fetchedAt) > followerStatusTTL {
		c.refreshHAStatusLocked(ctx, fr)
	}

	hosts := c.config.Hosts
	if fr.leaderIdx < 0 || fr.leaderIdx >= len(hosts) {
		return nil, "", false // no known leader -> leader path
	}
	leaderSt := fr.statuses[hosts[fr.leaderIdx]]
	if !leaderSt.ok || !leaderSt.enabled {
		return nil, "", false // not an HA cluster
	}
	leaderApplied := leaderSt.applied

	n := len(hosts)
	for off := 0; off < n; off++ {
		idx := (fr.rr + off) % n
		host := hosts[idx]
		st := fr.statuses[host]
		if !st.ok || !st.enabled || st.isLeader {
			continue
		}
		// Lag in entries; clamp at 0 in case a follower momentarily reports ahead of the cached leader.
		var lag uint64
		if leaderApplied > st.applied {
			lag = leaderApplied - st.applied
		}
		if maxStaleness != 0 && lag > maxStaleness {
			continue
		}
		conn, err := c.pool.GetConnectionForHost(host)
		if err != nil {
			continue
		}
		fr.rr = (idx + 1) % n
		return conn, host, true
	}
	return nil, "", false
}

// proactiveLeaderIdx force-refreshes the cluster's HA status and returns the index into
// config.Hosts of the current leader, if one is reported (item C7 — proactive routing). Used by
// leader-aware routing to jump straight to the leader on a LEADER_CHANGED instead of rotating
// host-by-host. Returns ok=false when no leader is reported (mid-election) or HAService is
// unavailable (older server), in which case the caller falls back to rotation.
func (c *Client) proactiveLeaderIdx(ctx context.Context) (int, bool) {
	fr := c.followers()
	fr.mu.Lock()
	defer fr.mu.Unlock()
	c.refreshHAStatusLocked(ctx, fr) // force a fresh probe: the leader just changed
	if fr.leaderIdx >= 0 && fr.leaderIdx < len(c.config.Hosts) {
		return fr.leaderIdx, true
	}
	return -1, false
}

// tryFollowerRead attempts to run a read on a fresh-enough follower (design §12). Returns
// (resp, true) on success; (nil, false) when no follower qualifies OR the follower errors — the
// caller then transparently falls back to the leader path. Follower reads are eventually consistent
// with a freshness bound, NOT read-your-writes.
func (c *Client) tryFollowerRead(ctx context.Context, query string, config *QueryConfig) (*Response, bool) {
	conn, _, ok := c.selectFollowerConn(ctx, config.MaxStaleness)
	if !ok {
		return nil, false
	}
	qc := pb.NewQueryServiceClient(conn)
	svcResp, err := c.querySvc.GqlVia(ctx, qc, query, c.convertToServiceQueryConfig(config),
		c.newParameterAdapter, c.sessions.GetDefaultGraph, func() int { return c.config.TimeoutSeconds() })
	if err != nil {
		return nil, false // transparent fallback to the leader path
	}
	return c.convertFromServiceResponse(svcResp), true
}
