//go:build benchmark

// Package benchmark provides performance tests for the GQLDB Go SDK.
// Run with: go test -tags=benchmark -bench=. ./tests/benchmark/...
package benchmark

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

// Test configuration
var (
	host     = getEnv("GQLDB_HOST", "192.168.1.100:60061")
	username = getEnv("GQLDB_USERNAME", "admin")
	password = getEnv("GQLDB_PASSWORD", "root11")
	graph    = getEnv("GQLDB_TEST_GRAPH", "miniCircle")
)

var benchClient *gqldb.Client

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func TestMain(m *testing.M) {
tos.Setenv("NO_PROXY", "192.168.1.100")
	config := gqldb.NewConfigBuilder().
		Hosts(host).
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(60 * time.Second).
		Build()

	var err error
	benchClient, err = gqldb.NewClient(config)
	if err != nil {
		println("Failed to create client:", err.Error())
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_, err = benchClient.Login(ctx, username, password)
	cancel()
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "authentication is not enabled") {
		println("Login failed:", err.Error())
		os.Exit(1)
	}

	code := m.Run()
	benchClient.Close()
	os.Exit(code)
}

// ==================== Latency Benchmarks ====================

// BenchmarkSimpleQuery measures simple query execution latency.
func BenchmarkSimpleQuery(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := benchClient.GQL(ctx, "RETURN 1", nil)
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

// BenchmarkPing measures ping latency.
func BenchmarkPing(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := benchClient.Ping(ctx)
		if err != nil {
			b.Fatalf("Ping failed: %v", err)
		}
	}
}

// BenchmarkHealthCheck measures health check latency.
func BenchmarkHealthCheck(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := benchClient.HealthCheck(ctx)
		if err != nil {
			b.Fatalf("HealthCheck failed: %v", err)
		}
	}
}

// ==================== Throughput Benchmarks ====================

// BenchmarkQueryThroughput measures queries per second.
func BenchmarkQueryThroughput(b *testing.B) {
	ctx := context.Background()
	query := "RETURN 1 AS value"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := benchClient.GQL(ctx, query, nil)
			if err != nil {
				b.Errorf("Query failed: %v", err)
			}
		}
	})
}

// BenchmarkNodeInsertSingle measures single node insertion throughput.
func BenchmarkNodeInsertSingle(b *testing.B) {
	ctx := context.Background()
	graphName := fmt.Sprintf("bench_insert_%d", time.Now().UnixNano())

	// Setup
	_, err := benchClient.GQL(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	if err != nil {
		b.Skipf("Cannot create test graph: %v", err)
		return
	}
	defer benchClient.GQL(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node := gqldb.NewNodeData("BenchNode").WithProperty("id", int64(i))
		_, err := benchClient.InsertNodesBatchAuto(ctx, graphName, "BenchNode", []gqldb.NodeData{node})
		if err != nil {
			b.Errorf("Insert failed: %v", err)
		}
	}
}

// BenchmarkNodeInsertBatch measures batch node insertion throughput.
func BenchmarkNodeInsertBatch(b *testing.B) {
	ctx := context.Background()
	graphName := fmt.Sprintf("bench_batch_%d", time.Now().UnixNano())
	batchSize := 100

	// Setup
	_, err := benchClient.GQL(ctx, fmt.Sprintf("CREATE GRAPH %s", graphName), nil)
	if err != nil {
		b.Skipf("Cannot create test graph: %v", err)
		return
	}
	defer benchClient.GQL(ctx, fmt.Sprintf("DROP GRAPH %s", graphName), nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodes := make([]gqldb.NodeData, batchSize)
		for j := 0; j < batchSize; j++ {
			nodes[j] = gqldb.NewNodeData("BenchNode").WithProperty("id", int64(i*batchSize+j))
		}
		_, err := benchClient.InsertNodesBatchAuto(ctx, graphName, "BenchNode", nodes)
		if err != nil {
			b.Errorf("Batch insert failed: %v", err)
		}
	}
	b.ReportMetric(float64(batchSize), "nodes/op")
}

// ==================== Scalability Benchmarks ====================

// BenchmarkLargeResultSet measures iteration over large result sets.
func BenchmarkLargeResultSet(b *testing.B) {
	ctx := context.Background()

	// First check if we have enough data
	response, err := benchClient.GQL(ctx, "MATCH (n) RETURN count(n) AS cnt", nil)
	if err != nil {
		b.Skipf("Cannot query database: %v", err)
		return
	}
	if response.IsEmpty() {
		b.Skip("No data in database")
		return
	}

	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			query := fmt.Sprintf("MATCH (n) RETURN n LIMIT %d", size)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				response, err := benchClient.GQL(ctx, query, nil)
				if err != nil {
					b.Errorf("Query failed: %v", err)
					continue
				}

				count := 0
				for range response.Rows() {
					count++
				}
			}
		})
	}
}

// BenchmarkConcurrentConnections measures performance with concurrent connections.
func BenchmarkConcurrentConnections(b *testing.B) {
	concurrencyLevels := []int{1, 5, 10, 20}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			var wg sync.WaitGroup
			queries := b.N / concurrency
			if queries < 1 {
				queries = 1
			}

			b.ResetTimer()
			for c := 0; c < concurrency; c++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					ctx := context.Background()
					for i := 0; i < queries; i++ {
						_, err := benchClient.GQL(ctx, "RETURN 1", nil)
						if err != nil {
							b.Errorf("Query failed: %v", err)
						}
					}
				}()
			}
			wg.Wait()
		})
	}
}

// ==================== Memory Benchmarks ====================

// BenchmarkMemoryAllocation tracks memory allocations per operation.
func BenchmarkMemoryAllocation(b *testing.B) {
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		response, err := benchClient.GQL(ctx, "RETURN 1 AS a, 2 AS b, 3 AS c", nil)
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
		// Use first column for AsTable
		ar, err := response.Alias("a")
		if err != nil {
			b.Fatalf("Alias failed: %v", err)
		}
		_, _ = ar.AsTable()
	}
}

// ==================== Transaction Benchmarks ====================

// BenchmarkTransactionOverhead measures transaction begin/commit overhead.
func BenchmarkTransactionOverhead(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, err := benchClient.BeginTransaction(ctx, true) // read-only
		if err != nil {
			b.Fatalf("Begin transaction failed: %v", err)
		}

		_, err = tx.GQL(ctx, "RETURN 1", nil)
		if err != nil {
			b.Errorf("Transaction query failed: %v", err)
		}

		err = tx.Commit(ctx)
		if err != nil {
			b.Errorf("Commit failed: %v", err)
		}
	}
}

// ==================== Connection Benchmarks ====================

// BenchmarkConnectionEstablishment measures connection setup time.
func BenchmarkConnectionEstablishment(b *testing.B) {
	config := gqldb.NewConfigBuilder().
		Hosts(host).
		Username(username).
		Password(password).
		DefaultGraph(graph).
		Timeout(30 * time.Second).
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client, err := gqldb.NewClient(config)
		if err != nil {
			b.Fatalf("Client creation failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = client.Login(ctx, username, password)
		cancel()

		client.Close()
	}
}
