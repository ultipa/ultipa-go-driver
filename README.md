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

    gqldb "github.com/ultipa/ultipa-go-driver/v6"
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
- Engine error codes on every error (`GqldbError.Code` / `.Reason`, `gqldb.EngineCode(err)`), and an automatic, bounded retry of a read-only query while a fulltext index loads (code 5020); see [GUIDE.md](GUIDE.md#engine-error-codes)
- A failed request is sent again (after signing in again, for a result too large for `Gql`, to the HA leader, or from a follower read to the leader) only when the server refused it before it ran or it is read-only outside a transaction, and a stream only before its first row: a request that may have written is never sent again
- An answer marked partly stored (`gqldb.PartlyStored(err)`: a statement failed after earlier statements of the request had committed, or a [5024] answer) is never sent again, a 5020 included; never run such a request again as it is
- A write conflict (3011, `*WriteConflictError`: nothing stored) is never sent again, with or without a `TransactionID`: run the whole transaction again yourself, or let `WithTransactionRetry` do it; see [GUIDE.md](GUIDE.md#write-conflicts)

## Documentation

See [Quick Start](https://www.ultipa.com/docs/drivers/go-quick-start) for detailed usage.

## License

MIT License
