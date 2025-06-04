# Ultipa GO SDK

## Installation

```shell

go get  github.com/ultipa/ultipa-go-driver
go mod tidy

```

## Document

Read Ultipa Go SDK document  [ultipa.com](https://www.ultipa.com/document/ultipa-drivers/go-installation/v5.0)

## Create Client

```go

import (
    "github.com/ultipa/ultipa-go-driver/sdk"
    "github.com/ultipa/ultipa-go-driver/sdk/api"
    "github.com/ultipa/ultipa-go-driver/sdk/configuration"
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

## UQLResponse Interface

- Alias(alias string) : *DataItem
- Alias(index int) : *DataItem


## DataItem Interface

- asNodes() : (nodes []*structs.Node, schemas map[string]*structs.Schema, err error)
- asEdges() : (edges []*structs.Edge, schemas map[string]*structs.Schema, err error)
- asPaths() : (paths []*structs.Path, err error)
- asTable() : (table *structs.Table, err error)
- asArray() : (arr *structs.Array, err error)
- asAttr()  : (arr *structs.Array, err error)
- AsGraphSets() (graphs []*structs.GraphSet, err error)
- asSchemas() : (schemas []*structs.Schema, err error)
- AsProperties() : (properties []*structs.Property, err error)
- AsIndexes() : (indexes []*structs.Index, err error)
- AsFullTexts() : (fullTextIndexes []*structs.Index, err error)
- asAlgos() : ([]*structs.Algo, error)
- AsGraph() : (graph *structs.Graph, err error)
- asAny() : (interface{}, error)
- AsPolicies() : (policies []*structs.Policy, err error)
- AsExtas() : (extas []*structs.Exta, err error)
- AsTasks() : (tasks []*structs.Task, err error)
- AsTops() : (tops []*structs.Top, err error)
- AsStats() : (stat *structs.Stat, err error)
- AsPrivileges() : (privileges []*structs.Privilege, err error)
- AsUsers() : (users []*structs.User, err error)

## Batch Insert

Check Go Transporter For Details

---