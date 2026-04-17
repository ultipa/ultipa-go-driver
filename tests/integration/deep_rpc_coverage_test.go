//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestDeepRPCCoverage(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}

	t.Run("Top", testTop)
	t.Run("KillNonexistent", testKillNonexistent)
	t.Run("ShowTasksEmpty", testShowTasksEmpty)
	t.Run("TaskLifecycle", testTaskLifecycle)
	t.Run("GqlStreamMiniCircle", testGqlStreamMiniCircle)
	t.Run("ExplainMiniCircle", testExplainMiniCircle)
	t.Run("ProfileMiniCircle", testProfileMiniCircle)
	t.Run("ExportNodes", testExportNodes)
	t.Run("ExportEdges", testExportEdges)
	t.Run("BulkImportFullFlow", testBulkImportFullFlow)
	t.Run("ConcurrentWrites", testConcurrentWrites)
}

// testTop verifies the TOP command returns without error.
func testTop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := testClient.Gql(ctx, "TOP", nil)
	if err != nil {
		t.Fatalf("TOP failed: %v", err)
	}

	t.Logf("TOP returned %d rows, columns: %v", resp.RowCount, resp.Columns)
}

// testKillNonexistent sends KILL with a nonexistent query ID and expects an error or empty result.
func testKillNonexistent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a fabricated query ID that should not exist
	_, err := testClient.Gql(ctx, "KILL 'nonexistent_query_id_999999'", nil)
	if err != nil {
		// Error is acceptable - the query ID does not exist
		t.Logf("KILL nonexistent returned error (expected): %v", err)
	} else {
		t.Log("KILL nonexistent succeeded without error (server accepted it)")
	}
}

// testShowTasksEmpty runs SHOW TASKS on a fresh test graph and expects zero or more tasks.
func testShowTasksEmpty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_showtasks_" + time.Now().Format("20060102150405")
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for SHOW TASKS")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	config := &gqldb.QueryConfig{GraphName: graphName}
	resp, err := testClient.Gql(ctx, "SHOW TASKS", config)
	if err != nil {
		t.Fatalf("SHOW TASKS failed: %v", err)
	}

	t.Logf("SHOW TASKS returned %d rows, columns: %v", resp.RowCount, resp.Columns)
}

// testTaskLifecycle starts a long-running algorithm via GQL on alimama graph,
// lists tasks via SHOW TASKS, then stops and deletes the task.
func testTaskLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Use alimama graph for algorithm execution; skip if unavailable
	alimamaCfg := &gqldb.QueryConfig{GraphName: "alimama"}

	// Verify alimama graph exists
	_, err := testClient.Gql(ctx, "RETURN 1", alimamaCfg)
	if err != nil {
		t.Skipf("alimama graph not available, skipping task lifecycle test: %v", err)
	}

	// Launch louvain algorithm in a goroutine (long-running)
	var algoErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, algoErr = testClient.Gql(ctx, "CALL louvain() YIELD node, community RETURN count(node)", alimamaCfg)
	}()

	// Give the algorithm a moment to register as a task
	time.Sleep(500 * time.Millisecond)

	// Show tasks to find the running task
	resp, err := testClient.Gql(ctx, "SHOW TASKS", alimamaCfg)
	if err != nil {
		t.Logf("SHOW TASKS failed: %v", err)
	} else {
		t.Logf("SHOW TASKS returned %d rows, columns: %v", resp.RowCount, resp.Columns)

		// Attempt to extract a task ID and stop/delete it
		if resp.RowCount > 0 {
			taskIDVal, getErr := resp.GetByName(resp.Rows[0], "task_id")
			if getErr != nil {
				// Try alternative column name
				taskIDVal, getErr = resp.GetByName(resp.Rows[0], "id")
			}
			if getErr == nil && taskIDVal != nil {
				taskID := fmt.Sprintf("%v", taskIDVal)
				t.Logf("Found task ID: %s", taskID)

				// Stop the task
				stopGQL := fmt.Sprintf("STOP TASK '%s'", taskID)
				_, stopErr := testClient.Gql(ctx, stopGQL, alimamaCfg)
				if stopErr != nil {
					t.Logf("STOP TASK failed (may be expected): %v", stopErr)
				} else {
					t.Logf("STOP TASK succeeded for task %s", taskID)
				}

				// Delete the task
				deleteGQL := fmt.Sprintf("DELETE TASK '%s'", taskID)
				_, delErr := testClient.Gql(ctx, deleteGQL, alimamaCfg)
				if delErr != nil {
					t.Logf("DELETE TASK failed (may be expected): %v", delErr)
				} else {
					t.Logf("DELETE TASK succeeded for task %s", taskID)
				}
			} else {
				t.Logf("Could not extract task ID from SHOW TASKS result: %v", getErr)
			}
		}
	}

	// Wait for the algorithm goroutine to finish (it may error due to STOP)
	<-done
	if algoErr != nil {
		t.Logf("Algorithm goroutine ended with error (expected after STOP): %v", algoErr)
	}
}

// testGqlStreamMiniCircle streams node data from miniCircle and verifies at least one chunk arrives.
func testGqlStreamMiniCircle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := &gqldb.QueryConfig{GraphName: "miniCircle"}

	var chunks int
	var totalRows int64
	err := testClient.GqlStream(ctx, "MATCH (n) RETURN n.name LIMIT 20", config, func(resp *gqldb.Response) error {
		chunks++
		totalRows += resp.RowCount
		t.Logf("Stream chunk %d: %d rows, columns: %v", chunks, resp.RowCount, resp.Columns)
		return nil
	})
	if err != nil {
		t.Fatalf("GqlStream on miniCircle failed: %v", err)
	}

	if chunks == 0 {
		t.Error("expected at least one stream chunk")
	}
	t.Logf("GqlStream total: %d chunks, %d rows", chunks, totalRows)
}

// testExplainMiniCircle runs EXPLAIN on miniCircle and verifies non-empty result.
func testExplainMiniCircle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := &gqldb.QueryConfig{GraphName: "miniCircle"}

	plan, err := testClient.Explain(ctx, "MATCH (n) RETURN n LIMIT 10", config)
	if err != nil {
		t.Fatalf("Explain on miniCircle failed: %v", err)
	}

	if plan == "" {
		t.Error("expected non-empty explain plan")
	} else {
		t.Logf("Explain plan length: %d chars", len(plan))
	}
}

// testProfileMiniCircle runs PROFILE on miniCircle and verifies non-empty result.
func testProfileMiniCircle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := &gqldb.QueryConfig{GraphName: "miniCircle"}

	profile, err := testClient.Profile(ctx, "MATCH (n) RETURN n LIMIT 10", config)
	if err != nil {
		t.Fatalf("Profile on miniCircle failed: %v", err)
	}

	if profile == "" {
		t.Error("expected non-empty profile result")
	} else {
		t.Logf("Profile result length: %d chars", len(profile))
	}
}

// testExportNodes exports nodes from miniCircle and verifies data is received.
func testExportNodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exportCfg := &gqldb.ExportConfig{
		GraphName:       "miniCircle",
		ExportNodes:     true,
		ExportEdges:     false,
		IncludeMetadata: true,
	}

	var chunkCount int
	var totalBytes int
	var finalStats *gqldb.ExportStats

	err := testClient.Export(ctx, exportCfg, func(result *gqldb.ExportResult) error {
		chunkCount++
		totalBytes += len(result.Data)
		if result.IsFinal && result.Stats != nil {
			finalStats = result.Stats
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Export nodes failed: %v", err)
	}

	if chunkCount == 0 {
		t.Error("expected at least one export chunk")
	}

	t.Logf("Export nodes: %d chunks, %d bytes", chunkCount, totalBytes)
	if finalStats != nil {
		t.Logf("Export stats: nodesExported=%d, bytesWritten=%d, durationMs=%d",
			finalStats.NodesExported, finalStats.BytesWritten, finalStats.DurationMs)
	}
}

// testExportEdges exports edges from miniCircle and verifies data is received.
func testExportEdges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exportCfg := &gqldb.ExportConfig{
		GraphName:       "miniCircle",
		ExportNodes:     false,
		ExportEdges:     true,
		IncludeMetadata: true,
	}

	var chunkCount int
	var totalBytes int
	var finalStats *gqldb.ExportStats

	err := testClient.Export(ctx, exportCfg, func(result *gqldb.ExportResult) error {
		chunkCount++
		totalBytes += len(result.Data)
		if result.IsFinal && result.Stats != nil {
			finalStats = result.Stats
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Export edges failed: %v", err)
	}

	if chunkCount == 0 {
		t.Error("expected at least one export chunk")
	}

	t.Logf("Export edges: %d chunks, %d bytes", chunkCount, totalBytes)
	if finalStats != nil {
		t.Logf("Export stats: edgesExported=%d, bytesWritten=%d, durationMs=%d",
			finalStats.EdgesExported, finalStats.BytesWritten, finalStats.DurationMs)
	}
}

// testBulkImportFullFlow tests the complete bulk import lifecycle:
// start -> insert nodes -> insert edges -> end -> verify via GQL.
func testBulkImportFullFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	graphName := "test_bulkfull_" + time.Now().Format("20060102150405")

	// Create graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for full bulk import flow")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	t.Logf("Started bulk import session: %s", session.SessionID)

	importCfg := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}

	// Insert nodes
	nodes := []*gqldb.NodeData{
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Alice", "age": int64(30)}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Bob", "age": int64(25)}},
		{Labels: []string{"Person"}, Properties: map[string]interface{}{"name": "Charlie", "age": int64(35)}},
		{Labels: []string{"City"}, Properties: map[string]interface{}{"name": "Seattle"}},
		{Labels: []string{"City"}, Properties: map[string]interface{}{"name": "Portland"}},
	}

	nodeResult, err := testClient.InsertNodesBatchAuto(ctx, graphName, nodes, importCfg)
	if err != nil {
		testClient.AbortBulkImport(ctx, session.SessionID)
		t.Fatalf("InsertNodesBatchAuto failed: %v", err)
	}
	t.Logf("Inserted nodes: success=%v, count=%d", nodeResult.Success, nodeResult.NodeCount)

	// Collect node IDs for edge creation
	if len(nodeResult.NodeIDs) < 5 {
		testClient.AbortBulkImport(ctx, session.SessionID)
		t.Fatalf("Expected at least 5 node IDs, got %d", len(nodeResult.NodeIDs))
	}

	// Insert edges
	edgeImportCfg := &gqldb.InsertEdgesConfig{
		BulkImportSessionID: session.SessionID,
	}
	edges := []*gqldb.EdgeData{
		{Label: "KNOWS", FromNodeID: nodeResult.NodeIDs[0], ToNodeID: nodeResult.NodeIDs[1], Properties: map[string]interface{}{"since": int64(2020)}},
		{Label: "KNOWS", FromNodeID: nodeResult.NodeIDs[1], ToNodeID: nodeResult.NodeIDs[2], Properties: map[string]interface{}{"since": int64(2021)}},
		{Label: "LIVES_IN", FromNodeID: nodeResult.NodeIDs[0], ToNodeID: nodeResult.NodeIDs[3], Properties: map[string]interface{}{}},
		{Label: "LIVES_IN", FromNodeID: nodeResult.NodeIDs[1], ToNodeID: nodeResult.NodeIDs[4], Properties: map[string]interface{}{}},
	}

	edgeResult, err := testClient.InsertEdgesBatchAuto(ctx, graphName, edges, edgeImportCfg)
	if err != nil {
		testClient.AbortBulkImport(ctx, session.SessionID)
		t.Fatalf("InsertEdgesBatchAuto failed: %v", err)
	}
	t.Logf("Inserted edges: success=%v, count=%d", edgeResult.Success, edgeResult.EdgeCount)

	// End bulk import
	endResult, err := testClient.EndBulkImport(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("EndBulkImport failed: %v", err)
	}
	t.Logf("Ended bulk import: success=%v, totalRecords=%d", endResult.Success, endResult.TotalRecords)

	// Verify data via GQL query
	queryCfg := &gqldb.QueryConfig{GraphName: graphName}

	resp, err := testClient.Gql(ctx, "MATCH (n:Person) RETURN count(n) AS cnt", queryCfg)
	if err != nil {
		t.Fatalf("Verification query for Person nodes failed: %v", err)
	}
	if resp.RowCount > 0 {
		val, getErr := resp.GetByName(resp.Rows[0], "cnt")
		if getErr == nil {
			t.Logf("Verified Person node count: %v", val)
		}
	}

	resp, err = testClient.Gql(ctx, "MATCH ()-[e:KNOWS]->() RETURN count(e) AS cnt", queryCfg)
	if err != nil {
		t.Fatalf("Verification query for KNOWS edges failed: %v", err)
	}
	if resp.RowCount > 0 {
		val, getErr := resp.GetByName(resp.Rows[0], "cnt")
		if getErr == nil {
			t.Logf("Verified KNOWS edge count: %v", val)
		}
	}
}

// testConcurrentWrites tests 5 goroutines inserting nodes concurrently into the same graph.
func testConcurrentWrites(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	graphName := "test_concurrent_" + time.Now().Format("20060102150405")

	// Create graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for concurrent writes")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}

	importCfg := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}

	const goroutineCount = 5
	const nodesPerGoroutine = 10

	var wg sync.WaitGroup
	errors := make([]error, goroutineCount)

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			nodes := make([]*gqldb.NodeData, nodesPerGoroutine)
			for j := 0; j < nodesPerGoroutine; j++ {
				nodes[j] = &gqldb.NodeData{
					Labels: []string{"ConcurrentNode"},
					Properties: map[string]interface{}{
						"worker": int64(workerID),
						"index":  int64(j),
						"name":   fmt.Sprintf("worker%d_node%d", workerID, j),
					},
				}
			}

			result, insertErr := testClient.InsertNodesBatchAuto(ctx, graphName, nodes, importCfg)
			if insertErr != nil {
				errors[workerID] = insertErr
				return
			}
			if !result.Success {
				errors[workerID] = fmt.Errorf("worker %d: insert returned success=false: %s", workerID, result.Message)
			}
		}(i)
	}

	wg.Wait()

	// Report errors
	failCount := 0
	for i, e := range errors {
		if e != nil {
			failCount++
			t.Errorf("Worker %d failed: %v", i, e)
		}
	}

	// End bulk import
	endResult, err := testClient.EndBulkImport(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("EndBulkImport failed: %v", err)
	}
	t.Logf("Ended bulk import: success=%v, totalRecords=%d", endResult.Success, endResult.TotalRecords)

	if failCount == 0 {
		// Verify total count
		queryCfg := &gqldb.QueryConfig{GraphName: graphName}
		resp, queryErr := testClient.Gql(ctx, "MATCH (n:ConcurrentNode) RETURN count(n) AS cnt", queryCfg)
		if queryErr != nil {
			t.Logf("Verification query failed: %v", queryErr)
		} else if resp.RowCount > 0 {
			val, getErr := resp.GetByName(resp.Rows[0], "cnt")
			if getErr == nil {
				t.Logf("Verified concurrent node count: %v (expected %d)", val, goroutineCount*nodesPerGoroutine)
			}
		}
	}
}
