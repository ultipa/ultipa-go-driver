# Ultipa Go Driver

Official Go driver for Ultipa Graph Database.

> **Note:** This is the v6.x driver for Ultipa Graph Database.
> - For Ultipa v5.x, use `go get github.com/ultipa/ultipa-go-driver@v5.3.0`
> - For Ultipa v4.x, use `go get github.com/ultipa/ultipa-go-sdk@v1.4.5`

## Requirements

- Go 1.24+

## Installation

```bash
go get github.com/ultipa/ultipa-go-driver/v6
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
