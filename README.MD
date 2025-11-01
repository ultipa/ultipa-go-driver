# Ultipa GO Driver

## Installation

```shell

go get  github.com/ultipa/ultipa-go-driver/v5
go mod tidy

```

## Document

Read Ultipa Go Driver document  [ultipa.com](https://www.ultipa.com/document/ultipa-drivers/go-installation/v5.0)

## Create Client

```go

import (
    "github.com/ultipa/ultipa-go-driver/v5/sdk"
    "github.com/ultipa/ultipa-go-driver/v5/sdk/api"
    "github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
)

config := &configuration.UltipaConfig{
    Hosts: []string{"127.0.0.1:60061"},
    Username: "root",
    Password: "root",
    DefaultGraph: "default",
}

client, err = sdk.NewUltipaDriver(config)
```

## Send UQL and get result

```go
resp, err := client.UQL("find().nodes() return nodes limit 1", nil)
nodes := resp.alias("nodes").asNodes()
```

## Response Interface

- Alias(alias string) : *DataItem
- Get(index int) (di *DataItem)


## DataItem Interface

- AsFirstNode() (node *structs.Node, err error)
- AsFirstEdge() (node *structs.Edge, err error)
- AsNodes() (nodes []*structs.Node, schemas map[string]*structs.Schema, err error)
- AsEdges() (edges []*structs.Edge, schemas map[string]*structs.Schema, err error)
- AsTable() (table *structs.Table, err error)
- AsAttr() (*structs.Attr, error)
- AsGraphSets() (graphSets []*structs.GraphSet, err error)
- AsSchemas() (schemas []*structs.Schema, err error)
- AsProperties() (properties []*structs.Property, err error)
- AsIndexes() (indexes []*structs.Index, err error)
- AsFullTexts() (fullTextIndexes []*structs.Index, err error)
- AsAlgos() ([]*structs.Algo, error)
- AsGraph() (graph *structs.Graph, err error)
- AsAny() (interface{}, error)
- AsPolicies() (policies []*structs.Policy, err error)
- AsProcesses() (tops []*structs.Process, err error)
- AsPrivileges() (privileges []*structs.Privilege, err error)
- AsUsers() (users []*structs.User, err error)
- AsJobs() (jobs []*structs.Job, err error)
- AsHDCGraphs() (projections []*structs.HDCGraph, err error)
- AsProjections() (projections []*structs.Projection, err error)

## Batch Insert

Check Go Transporter For Details

## Session and Transaction

### Session

Sessions allow you to group multiple queries with a unique session ID.

```go
import "github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"

// Create a session with default settings
session, err := client.Session(nil)
if err != nil {
    // handle error
}
defer session.Close()

// Execute queries within the session
resp, err := session.Uql("show().graph()", nil)
resp, err := session.Gql("MATCH (n) RETURN n LIMIT 5", nil)

// Get session ID
sessionID := session.SessionID()
```

### Transaction

Transactions provide ACID guarantees for multiple operations.

```go
import "github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"

// Create a session first
session, err := client.Session(nil)
if err != nil {
    // handle error
}
defer session.Close()

// Start a transaction (only supports GQL)
txConfig := &configuration.TransactionConfig{
    Graph:   "default",
    Timeout: 30,
}
tx, err := session.StartTransaction(txConfig)
if err != nil {
    // handle error
}

// Execute GQL queries within the transaction
resp, err := tx.Gql("INSERT (n:Person {name: 'Alice'})", nil)
if err != nil {
    tx.Rollback()
    return
}

// Commit the transaction
err = tx.Commit()
if err != nil {
    // handle error
}
```

**Note**: Sessions and Transactions are NOT thread-safe. Caller must synchronize if used across goroutines.

## Supported Data Types

### POINT3D

The POINT3D type represents a 3-dimensional point in space with X, Y, and Z coordinates.

```go
import "github.com/ultipa/ultipa-go-driver/v5/sdk/types"

// Create a Point3D
point := types.Point3D{X: 1.5, Y: 2.5, Z: 3.5}

// Create from string
point, err := types.Point3DFromStr("(1.5,2.5,3.5)")

// Use in node properties
node := structs.NewNode()
node.Set("location3d", point)
```

### RECORD

The RECORD type stores arbitrary JSON data (objects, arrays, nested structures).

```go
import "github.com/ultipa/ultipa-go-driver/v5/sdk/types"

// Create from map
record := types.NewRecord(map[string]interface{}{
    "name": "Alice",
    "age": 30,
    "email": "alice@example.com",
})

// Create from JSON string
record, err := types.RecordFromJSON(`{"name":"Bob","age":25}`)

// Access data
name := record.Get("name")
record.Set("active", true)
hasEmail := record.Has("email")
keys := record.Keys()

// Convert to JSON string
jsonStr, err := record.ToJSONString()

// Use in node properties
node := structs.NewNode()
node.Set("metadata", record)
```

**Note**: If the database returns a JSON array at the root level, it will be automatically wrapped in an object with a "list" key.

---