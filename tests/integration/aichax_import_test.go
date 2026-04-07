//go:build integration

package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

const (
	aichaxJSONLFile     = "/Volumes/External/gqldb-source-data-scenarios/aichax-export.jsonl"
	aichaxExpectedNodes = 1671919
	aichaxExpectedEdges = 10805920
)

// JSONLRecord represents a record from the JSONL export file
type JSONLRecord struct {
	Type       string         `json:"_type"`
	ID         string         `json:"_id"`
	Labels     []string       `json:"_labels"`
	Label      string         `json:"_label"`
	Properties map[string]any `json:"_properties"`
	From       string         `json:"_from"`
	To         string         `json:"_to"`
}

// TestAichaxImport imports the aichax-export.jsonl file using the Go driver.
// Run with: GQLDB_NOAUTH_HOST=localhost:19000 go test -v -tags=integration -timeout 60m ./tests/integration -run TestAichaxImport
func TestAichaxImport(t *testing.T) {
	// Check if source file exists
	if _, err := os.Stat(aichaxJSONLFile); os.IsNotExist(err) {
		t.Fatalf("Source file not found: %s", aichaxJSONLFile)
	}

	// Use noAuthClient for server without authentication
	client := noAuthClient
	if client == nil {
		t.Fatal("noAuthClient is nil - ensure GQLDB_NOAUTH_HOST env var is set correctly")
	}

	batchSize := 10000
	checkpointEvery := 100000
	progressInterval := 100000

	fmt.Println("=== Aichax Driver Import Test (Custom IDs) ===")
	fmt.Printf("Source: %s\n", aichaxJSONLFile)
	fmt.Printf("Expected: %d nodes, %d edges\n\n", aichaxExpectedNodes, aichaxExpectedEdges)

	ctx := context.Background()

	// Phase 1: Create graph
	fmt.Println("Phase 1: Creating graph...")
	err := client.CreateGraph(ctx, "aichax", gqldb.GraphTypeOpen, "Aichax knowledge graph")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	fmt.Println("  Graph 'aichax' created")

	// Phase 2: Start bulk import session
	fmt.Println("Phase 2: Starting bulk import session...")
	bulkSession, err := client.StartBulkImport(ctx, "aichax", &gqldb.BulkImportOptions{
		EstimatedNodes: int64(aichaxExpectedNodes),
		EstimatedEdges: int64(aichaxExpectedEdges),
	})
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	if !bulkSession.Success {
		t.Fatalf("StartBulkImport not successful: %s", bulkSession.Message)
	}
	sessionID := bulkSession.SessionID
	fmt.Printf("  Bulk session started: %s\n", sessionID)

	// Phase 3: Import data
	fmt.Println("Phase 3: Importing data...")
	importStart := time.Now()

	// Open JSONL file
	file, err := os.Open(aichaxJSONLFile)
	if err != nil {
		t.Fatalf("Failed to open JSONL file: %v", err)
	}
	defer file.Close()

	// Batch buffers - no ID mapping needed since we use custom IDs
	nodeBatch := make([]*gqldb.NodeData, 0, batchSize)
	edgeBatch := make([]*gqldb.EdgeData, 0, batchSize)

	// Counters
	var nodesProcessed, edgesProcessed, recordsSinceCheckpoint int64

	// Helper to flush node batch
	flushNodes := func() error {
		if len(nodeBatch) == 0 {
			return nil
		}
		result, err := client.InsertNodes(ctx, "aichax", nodeBatch, &gqldb.InsertNodesConfig{
			BulkImportSessionID: sessionID,
		})
		if err != nil {
			return fmt.Errorf("InsertNodes failed: %w", err)
		}
		if !result.Success {
			return fmt.Errorf("InsertNodes failed: %s", result.Message)
		}
		nodesProcessed += result.NodeCount
		recordsSinceCheckpoint += result.NodeCount
		nodeBatch = nodeBatch[:0]
		return nil
	}

	// Helper to flush edge batch
	flushEdges := func() error {
		if len(edgeBatch) == 0 {
			return nil
		}
		result, err := client.InsertEdges(ctx, "aichax", edgeBatch, &gqldb.InsertEdgesConfig{
			SkipInvalidNodes:    true,
			BulkImportSessionID: sessionID,
		})
		if err != nil {
			return fmt.Errorf("InsertEdges failed: %w", err)
		}
		if !result.Success {
			return fmt.Errorf("InsertEdges failed: %s", result.Message)
		}
		edgesProcessed += result.EdgeCount
		recordsSinceCheckpoint += result.EdgeCount
		edgeBatch = edgeBatch[:0]
		return nil
	}

	// Checkpoint is now a no-op; kept as placeholder
	doCheckpoint := func() error {
		return nil
	}

	// Pass 1: Import nodes with custom IDs
	fmt.Println("  Pass 1: Importing nodes with custom IDs...")
	nodeStart := time.Now()
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var record JSONLRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}

		if record.Type == "node" {
			// Pass custom ID from JSONL - server will use this as _id
			nodeBatch = append(nodeBatch, &gqldb.NodeData{
				ID:         record.ID,
				Labels:     record.Labels,
				Properties: record.Properties,
			})

			if len(nodeBatch) >= batchSize {
				if err := flushNodes(); err != nil {
					t.Fatalf("Flush nodes failed: %v", err)
				}

				if recordsSinceCheckpoint >= int64(checkpointEvery) {
					if err := doCheckpoint(); err != nil {
						t.Fatalf("Checkpoint failed: %v", err)
					}
				}

				if nodesProcessed%int64(progressInterval) == 0 {
					elapsed := time.Since(nodeStart)
					rate := float64(nodesProcessed) / elapsed.Seconds()
					pct := float64(nodesProcessed) / float64(aichaxExpectedNodes) * 100
					fmt.Printf("    Nodes: %d / %d (%.1f%%) - %.0f nodes/sec\n",
						nodesProcessed, aichaxExpectedNodes, pct, rate)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	// Flush remaining nodes
	if err := flushNodes(); err != nil {
		t.Fatalf("Final flush nodes failed: %v", err)
	}
	if err := doCheckpoint(); err != nil {
		t.Fatalf("Checkpoint after nodes failed: %v", err)
	}

	nodeElapsed := time.Since(nodeStart)
	fmt.Printf("    Nodes complete: %d in %.1fs (%.0f nodes/sec)\n",
		nodesProcessed, nodeElapsed.Seconds(), float64(nodesProcessed)/nodeElapsed.Seconds())

	// Pass 2: Import edges using original _from and _to (custom IDs)
	fmt.Println("  Pass 2: Importing edges using custom node IDs...")
	edgeStart := time.Now()

	file.Close()
	file, err = os.Open(aichaxJSONLFile)
	if err != nil {
		t.Fatalf("Failed to reopen JSONL file: %v", err)
	}

	scanner = bufio.NewScanner(file)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var record JSONLRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}

		if record.Type == "edge" {
			// Use original _from and _to directly - they match our custom node IDs
			edgeBatch = append(edgeBatch, &gqldb.EdgeData{
				Label:      record.Label,
				FromNodeID: record.From,
				ToNodeID:   record.To,
				Properties: record.Properties,
			})

			if len(edgeBatch) >= batchSize {
				if err := flushEdges(); err != nil {
					t.Fatalf("Flush edges failed: %v", err)
				}

				if recordsSinceCheckpoint >= int64(checkpointEvery) {
					if err := doCheckpoint(); err != nil {
						t.Fatalf("Checkpoint failed: %v", err)
					}
				}

				if edgesProcessed%int64(progressInterval) == 0 {
					elapsed := time.Since(edgeStart)
					rate := float64(edgesProcessed) / elapsed.Seconds()
					pct := float64(edgesProcessed) / float64(aichaxExpectedEdges) * 100
					fmt.Printf("    Edges: %d / %d (%.1f%%) - %.0f edges/sec\n",
						edgesProcessed, aichaxExpectedEdges, pct, rate)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	// Flush remaining edges
	if err := flushEdges(); err != nil {
		t.Fatalf("Final flush edges failed: %v", err)
	}

	edgeElapsed := time.Since(edgeStart)
	fmt.Printf("    Edges complete: %d in %.1fs (%.0f edges/sec)\n",
		edgesProcessed, edgeElapsed.Seconds(), float64(edgesProcessed)/edgeElapsed.Seconds())

	// Phase 4: End bulk import
	fmt.Println("\nPhase 4: Ending bulk import session...")
	endResult, err := client.EndBulkImport(ctx, sessionID)
	if err != nil {
		t.Fatalf("EndBulkImport failed: %v", err)
	}
	if !endResult.Success {
		t.Fatalf("EndBulkImport failed: %s", endResult.Message)
	}
	fmt.Printf("  Total records: %d\n", endResult.TotalRecords)

	importElapsed := time.Since(importStart)
	fmt.Printf("\n  Import complete in %.1fs\n", importElapsed.Seconds())
	fmt.Printf("  Nodes: %d, Edges: %d\n", nodesProcessed, edgesProcessed)

	// Phase 5: Verify data using driver queries
	fmt.Println("\nPhase 5: Verifying data...")

	t.Run("VerifyNodeCount", func(t *testing.T) {
		result, err := client.Gql(ctx, "MATCH (n) RETURN count(n) AS cnt", &gqldb.QueryConfig{
			GraphName: "aichax",
		})
		if err != nil {
			t.Fatalf("Node count query failed: %v", err)
		}
		fmt.Printf("  Node count query result: %d rows\n", len(result.Rows))
		if len(result.Rows) > 0 {
			fmt.Printf("  First row: %v\n", result.Rows[0])
		}
	})

	t.Run("VerifyEdgeCount", func(t *testing.T) {
		result, err := client.Gql(ctx, "MATCH ()-[e]->() RETURN count(e) AS cnt", &gqldb.QueryConfig{
			GraphName: "aichax",
		})
		if err != nil {
			t.Fatalf("Edge count query failed: %v", err)
		}
		fmt.Printf("  Edge count query result: %d rows\n", len(result.Rows))
		if len(result.Rows) > 0 {
			fmt.Printf("  First row: %v\n", result.Rows[0])
		}
	})

	t.Run("SampleQuery", func(t *testing.T) {
		result, err := client.Gql(ctx, "MATCH (p:Person) RETURN p.name LIMIT 5", &gqldb.QueryConfig{
			GraphName: "aichax",
		})
		if err != nil {
			t.Fatalf("Sample query failed: %v", err)
		}
		fmt.Printf("  Sample Person query returned %d rows\n", len(result.Rows))
	})

	fmt.Println("\n=== Test Complete ===")
}
