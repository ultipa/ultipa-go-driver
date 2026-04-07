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
| RetryCount | Retry attempts | 3 |
| RetryDelay | Retry delay | 100ms |

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
_, err = client.InsertNodes(ctx, "myGraph", nodes, sessionId, nil)

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
_, err = client.InsertEdges(ctx, "myGraph", edges, sessionId, nil)

// End session
_, err = client.EndBulkImport(ctx, sessionId)
```

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
