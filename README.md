# GQLDB Go Driver

Official Go driver for GQLDB graph database.

## Requirements

- Go 1.24+

## Installation

```bash
go get github.com/gqldb/gqldb-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"

    gqldb "github.com/gqldb/gqldb-go"
)

func main() {
    // Create client
    client, err := gqldb.NewClient(gqldb.DefaultConfig())
    if err != nil {
        panic(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Login
    if err := client.Login(ctx, "username", "password"); err != nil {
        panic(err)
    }

    // Execute query
    resp, err := client.Gql(ctx, "MATCH (n) RETURN n LIMIT 10", nil)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Rows: %d, Columns: %v\n", resp.RowCount, resp.Columns)
}
```

## Features

- GQL query execution with parameters
- Streaming results for large datasets
- Transaction support (begin, commit, rollback)
- Graph management (create, drop, list)
- Bulk import for high-throughput loading
- Connection pooling
- Health checks

## Documentation

See [GUIDE.md](GUIDE.md) for detailed usage.

## License

[MIT License](LICENSE)
