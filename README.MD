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

---