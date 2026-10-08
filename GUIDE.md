# Ultipa Go Driver Guide

## Installation

```bash
go get github.com/ultipa/ultipa-go-driver/v6
```

## Configuration

```go
import gqldb "github.com/ultipa/ultipa-go-driver/v6"

// Default config
config := gqldb.DefaultConfig()

// Custom config with builder
config := gqldb.NewConfigBuilder().
    Hosts("host1:60061", "host2:60061").
    Username("admin").
    Password("password").
    DefaultGraph("myGraph").
    Timeout(60 * time.Second).
    MaxRecvSize(128 * 1024 * 1024).
    PoolSize(20).
    RetryCount(5).
    RetryDelay(200 * time.Millisecond).
    Build()

client, err := gqldb.NewClient(config)
```

### Options

| Option | Description | Default |
|--------|-------------|---------|
| Hosts | Server addresses | localhost:60061 |
| Timeout | Query timeout | 30s |
| MaxRecvSize | Max message size | 64MB |
| PoolSize | Connection pool size | 10 |
| RetryCount | How many times a read-only query is sent again while a fulltext index loads (code 5020); 0 turns it off | 3 |
| RetryDelay | The wait before the first such retry; doubles each time, capped at 2 s | 100ms |
| DisableUseGraph | Reject caller GQL that selects/replaces a graph | false |

### Multi-tenant graph pinning

`QueryConfig` graph name is a *per-request override*, not a boundary: a `USE`
inside caller-supplied GQL beats it and reads another tenant's graph. Enable
``DisableUseGraph`` to reject query text whose leading keyword is `USE` — before any RPC:

```go
cfg := gqldb.DefaultConfig()
cfg.DisableUseGraph = true

_, err := client.Gql(ctx, "USE other_tenant\nMATCH (n) RETURN n",
    &gqldb.QueryConfig{GraphName: "tenant_a"})
if errors.Is(err, gqldb.ErrGraphSwitchRejected) { /* ... */ }
```

Rejection raises `*GraphSwitchRejectedError`, matchable with `errors.Is(err, ErrGraphSwitchRejected)`. GQL the driver builds itself (convenience DDL,
loaders, `useGraph()`) is unaffected.

> **Defense-in-depth, not a security boundary.** It inspects query text and
> cannot constrain what the connected account may touch; `SHOW GRAPHS` still
> enumerates every graph. Use a separate database user per tenant with a
> graph-scoped role for an actual boundary, and pair the flag with read-only
> requests for user-supplied queries. See the root `GUIDE.md` for the full
> rationale and the bypass chains this does and does not cover.
>
> **Graph-lifecycle DDL is not blocked** — `DROP GRAPH` + `CREATE GRAPH …
> AS COPY OF …` reaches another tenant with no `USE`. Pair the flag with
> read-only requests, which the server rejects any write under.
>
> **Interim measure.** It covers the forms known when it shipped and
> silently stops covering any graph-selection syntax the server adds
> later. Once the server can enforce this, the flag is deprecated and
> removed at the next major version — plan the move to per-tenant
> database users alongside enabling it, not after.

## Queries

### Basic Query

```go
ctx := context.Background()
resp, err := client.Gql(ctx, "MATCH (n:Person) RETURN n.name LIMIT 10", nil)
if err != nil {
    return err
}

for _, row := range resp.Rows {
    name := row.Get(0)
    fmt.Println(name)
}
```

### Query with Config

```go
config := &gqldb.QueryConfig{
    GraphName:      "myGraph",
    Timeout:        60,
    ReadOnly:       true,
    MaxPathResults: 100,
    Parameters: map[string]interface{}{
        "name": "Alice",
    },
}

resp, err := client.Gql(ctx, "MATCH (n:Person {name: $name}) RETURN n", config)
```

### Streaming Query

```go
err := client.GqlStream(ctx, "MATCH (n) RETURN n", nil, func(resp *gqldb.Response) {
    fmt.Printf("Batch: %d rows\n", len(resp.Rows))
})
```

### Explain & Profile

```go
plan, err := client.Explain(ctx, "MATCH (n) RETURN n", nil)
fmt.Println(plan)

profile, err := client.Profile(ctx, "MATCH (n) RETURN n", nil)
fmt.Println(profile)
```

## Transactions

```go
// Begin transaction
txId, err := client.Begin(ctx, "myGraph", false)
if err != nil {
    return err
}

// Execute within transaction
config := &gqldb.QueryConfig{TransactionID: txId}

_, err = client.Gql(ctx, "INSERT (:Person {name: 'Alice'})", config)
if err != nil {
    client.Rollback(ctx, txId)
    return err
}

_, err = client.Gql(ctx, "INSERT (:Person {name: 'Bob'})", config)
if err != nil {
    client.Rollback(ctx, txId)
    return err
}

// Commit
err = client.Commit(ctx, txId)
```

### Write conflicts

When two transactions change the same element, the server refuses the later
one with a write conflict (engine code 3011, reason `WRITE_CONFLICT`): its
transaction is over and none of its changes are stored. `Gql`, `GqlStream` and
`Commit` return it as a `*gqldb.WriteConflictError`: `gqldb.IsWriteConflict(err)`
and `errors.Is(err, gqldb.ErrWriteConflict)` are true, and its `Cause` is the
`*GqldbError`, so `gqldb.EngineCode(err)` is 3011. A statement can get it, not
only `Commit` (a `MERGE` that needs a key another transaction holds).

**The driver never sends a request again after a write conflict**, with or
without a `TransactionID`. The server already runs a conflicting auto-commit
statement again by itself, and a request without a `TransactionID` may belong to
a transaction opened with GQL text (`START TRANSACTION` ... `COMMIT`), which a
resend would run outside that transaction. Run the whole transaction again,
its reads included. `WithTransactionRetry` does that for you; it is off unless
you give it a count:

```go
err := client.WithTransactionRetry(ctx, "myGraph", false, 3, func(txID uint64) error {
    cfg := &gqldb.QueryConfig{TransactionID: txID}
    resp, err := client.Gql(ctx, "MATCH (n:Account WHERE n.id='a') RETURN n.v", cfg)
    if err != nil {
        return err
    }
    v, _ := resp.Rows[0].GetInt(0)
    _, err = client.Gql(ctx, fmt.Sprintf("MATCH (n:Account WHERE n.id='a') SET n.v = %d", v+1), cfg)
    return err
})
```

Each attempt is a new transaction: begin, the whole function, commit. A new
attempt starts only after a `*WriteConflictError` from a statement or the
commit of the attempt's own transaction, after a random wait of up to 5 ms,
doubling up to 200 ms; every other error is returned at once. Anything the
function does outside the transaction happens again on each attempt.

**An older server** (`gqldb-grpc` before the error detail) answers a conflict
with gRPC status `Aborted` and a message that starts with `[3011]`, and no
detail. The driver returns a `*WriteConflictError` for that only for a call
that carries a transaction id (a `Gql` / `GqlStream` with `TransactionID`, or
`Commit`), so `WithTransactionRetry` works there too. Any other request's
conflict from such a server is a plain `*GqldbError` (`Code` 0). See the root
[GUIDE.md](../GUIDE.md#write-conflicts) for all five drivers.

## Bulk Import

```go
// Start session
sessionId, err := client.StartBulkImport(ctx, "myGraph", nil)
if err != nil {
    return err
}

// Insert nodes
nodes := []*gqldb.NodeData{
    {
        ID:     "1",
        Labels: []string{"Person"},
        Properties: map[string]interface{}{
            "name": "Alice",
            "age":  30,
        },
    },
    {
        ID:     "2",
        Labels: []string{"Person"},
        Properties: map[string]interface{}{
            "name": "Bob",
            "age":  25,
        },
    },
}
_, err = client.InsertNodes(ctx, "myGraph", nodes, &gqldb.InsertNodesConfig{BulkImportSessionID: sessionId})

// Insert edges
edges := []*gqldb.EdgeData{
    {
        Label:      "KNOWS",
        FromNodeID: "1",
        ToNodeID:   "2",
        Properties: map[string]interface{}{
            "since": 2020,
        },
    },
}
_, err = client.InsertEdges(ctx, "myGraph", edges, &gqldb.InsertEdgesConfig{BulkImportSessionID: sessionId})

// End session
_, err = client.EndBulkImport(ctx, sessionId)
```

`EndBulkImport` and `AbortBulkImport` (which discards what the session wrote) wait
until the server reports a final state, sending the call again while the server
answers that it still runs; a repeated End or Abort is safe. Each request lasts at
most 30 s. There is no total limit by default: they wait until the session reaches a
final state. Set `Config.BulkImportWaitTimeout` or `BulkImportWaitOptions.TotalTimeout`
(0: no limit), or give the context a deadline, to bound the wait; past it they return
a `*BulkImportInProgressError`: the work still runs and has not failed.

```go
res, err := client.EndBulkImport(ctx, sessionId, gqldb.BulkImportWaitOptions{
    TotalTimeout: 2 * time.Hour,
    OnProgress:   func(p gqldb.BulkImportProgress) { log.Println(p.State, p.Progress, p.ProgressTotal) },
})
var inProgress *gqldb.BulkImportInProgressError
if errors.As(err, &inProgress) {
    // inProgress.State ENDING: still running on the server, not failed: call EndBulkImport again.
    // inProgress.State ACTIVE: the End may not have reached the server: call EndBulkImport again
    // to keep the data (an open session is discarded by the server's idle cleanup).
}
_ = res // res.State == gqldb.BulkImportStateEnded
```

- `ProgressInterval` is at least 100 ms (a smaller value is raised to it).
- A panic in `OnProgress` does not end the wait: the driver recovers it, logs the
  first one of the wait with the standard `log` package, and goes on waiting.
- Cancelling the context ends the wait with `*BulkImportInProgressError` whose
  `Cause` is the context's error. If the End had reached the server it goes on there.
- A server older than the states (push 8 and before) gets one blocking End or Abort:
  `TotalTimeout`, `ProgressInterval` and `OnProgress` have no effect there; only the
  context ends it sooner.

**The options argument and interfaces or mocks.** `EndBulkImport` and
`AbortBulkImport` gained a variadic `opts ...gqldb.BulkImportWaitOptions` argument.
Existing calls compile unchanged, but the method's signature changed: an interface
you declared with the old two-argument signature, such as

```go
type bulkEnder interface {
    EndBulkImport(ctx context.Context, sessionID string) (*gqldb.EndBulkImportResult, error)
}
```

no longer matches `*gqldb.Client`, and a mock or wrapper that implements the old
signature no longer satisfies an interface declared with the new one. Add
`opts ...gqldb.BulkImportWaitOptions` to the interface, the mock and the wrapper,
and pass `opts...` through.

**`Config.SessionID`.** An id you set is never changed by the driver. When the
sign-in expires while a transaction is open, the driver signs in again and returns
`*TransactionSignInExpiredError` for that transaction, but sends nothing under your id
to end it, because other clients may share it. A generated id (the default) is
replaced after such a re-login instead. See the root GUIDE, "When the sign-in expires
during a transaction".

See the root [GUIDE.md](../GUIDE.md#ending-and-aborting-a-bulk-import).

## Graph Management

```go
// Create graph
err := client.CreateGraph(ctx, "newGraph", gqldb.GraphTypeOpen, "Description")

// Use graph
err := client.UseGraph(ctx, "myGraph")

// List graphs
graphs, err := client.ListGraphs(ctx)
for _, g := range graphs {
    fmt.Printf("%s: %d nodes, %d edges\n", g.Name, g.NodeCount, g.EdgeCount)
}

// Drop graph
err := client.DropGraph(ctx, "oldGraph", true) // ifExists=true
```

## Error Handling

```go
resp, err := client.Gql(ctx, query, nil)
if err != nil {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        // Timeout
    case errors.Is(err, gqldb.ErrNotLoggedIn):
        // Not authenticated
    default:
        // Other error
    }
    return err
}
```

### Engine error codes

Every error from the server carries the engine's error code and a stable
reason, sent beside the message (not inside it). The driver puts them on
`*gqldb.GqldbError` as `Code` and `Reason`, and `gqldb.EngineCode(err)` /
`gqldb.ErrorReason(err)` read them from any error, however it is wrapped.
Decide what an error means from these, never from the message text.

```go
_, err := client.Gql(ctx, query, nil)
switch gqldb.EngineCode(err) {
case gqldb.CodeFulltextIndexLoading: // 5020: the fulltext index is still loading; wait and ask again
case gqldb.CodeWritesCommitted:      // 5024: the changes are stored; do not run the statement again
case gqldb.CodeWriteConflict:        // 3011: run the whole transaction again
case gqldb.CodeReadOnly, gqldb.CodeLicenseReadOnly: // 4016, 6020: writes are refused
}
```

The code is 0 and the reason empty for an error the driver raises itself, and
for a server that predates the error detail.

**Automatic retry, for reads only.** A `Gql` or `GqlStream` call whose
`QueryConfig.ReadOnly` is true, outside a transaction, that fails with 5020
(a fulltext index still loading) is sent again up to `Config.RetryCount` times
(default 3), waiting `Config.RetryDelay` (default 100 ms) and doubling the wait
each time, up to 2 seconds and never past the context's deadline. A stream is
sent again only if no rows had reached the callback. No other code is retried,
a write conflict (3011) included, and nothing inside a transaction or without
`ReadOnly`. Set `RetryCount` to 0 to turn the retry off. The one thing that
runs again after a write conflict is a whole transaction, through
`WithTransactionRetry` (see [Write conflicts](#write-conflicts)).

**Other times a request is sent again.** Two other paths can send a failed
request again, under one rule: only when the server refused it before any of
it ran (`gqldb.RefusedBeforeRun(err)` is true), or the request has
`ReadOnly: true` and no transaction id (the server refuses any write in it);
and a stream only while no row has reached the callback. **A request that may
have written is never sent again**: you get its error. Nor is one whose answer
says part of it is stored (`gqldb.PartlyStored(err)`, the detail's
`partly_stored=true`: a statement failed after earlier ones of the request
committed, or a [5024] answer), on any path, a 5020 included: never run it
again as it is.

- *Signing in again.* When the server answers UNAUTHENTICATED (the session
  expired or is unknown), the driver signs in again with the stored
  credentials, so the next call works, and sends the failed `Gql`,
  `GqlStream`, `Explain` or `Profile` call again only under the rule above.
  A call that carries a `TransactionID` is never sent again: the transaction
  belonged to the expired sign-in, and you get `*TransactionSignInExpiredError`,
  also when signing in again failed at that moment (the message then says so).
  `ListTransactions` is always sent again: it reads and writes nothing, so a
  listing left idle past the sign-in TTL answers instead of failing.
- *A result too large for `Gql`.* When a result passes the server's 10,000-row
  limit, `Gql` fetches it through `GqlStream`, again only under the rule above.
  A request without `ReadOnly` gets the error instead (`RESOURCE_EXHAUSTED`,
  reason `RESULT_ROW_LIMIT`): set `ReadOnly`, or use `GqlStream`.

With a server too old to send the error detail, only read-only requests are
sent again. A write conflict is never sent again, from any server. When the connection drops, the driver rebuilds its connections for
the next call and does not send the failed one again.

**HA leader change.** The driver moves a write to the new leader when the error
has the reason `LEADER_CHANGED`: a follower refuses a write before running it.
Once the write reaches the leader, the leader's answer is final: any other
error ends the call, and the write is not sent to the leader again or to
another server, unless the rule above allows it. A stream that has handed
rows to the callback is never moved, a `LEADER_CHANGED` included: its error is
returned. A server that could not be signed in to was sent nothing, and the
search moves on; the waits between servers end when the context ends.

**Follower reads.** A `Gql` call with `ReadPreference: ReadPreferenceFollower`
is tried on a fresh-enough follower first. If the follower fails it, the call
goes on to the leader only under the rule above (`ReadOnly` with no
transaction id, `executed=false`, or `LEADER_CHANGED`); otherwise the
follower's error is returned, since a follower that ran a write may have
stored it. With no eligible follower the call goes straight to the leader.

These decisions come from the gRPC status, the error's detail and the
request's `ReadOnly` flag. The message text is read only for two answers of a
server too old to send the detail: the row-limit answer ("use streaming API"),
and then only for a read-only request; and the exact leader-change marker
(`LEADER_CHANGED`, or `LEADER_CHANGED leader=<address>`). The word anywhere
else in a message, such as an index name, does not count.

## Context & Cancellation

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

resp, err := client.Gql(ctx, query, nil)

// With cancellation
ctx, cancel := context.WithCancel(context.Background())
go func() {
    time.Sleep(5 * time.Second)
    cancel() // Cancel after 5s
}()

resp, err := client.Gql(ctx, query, nil)
```
