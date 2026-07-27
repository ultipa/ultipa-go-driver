//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestStartBulkImport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_bulk_import_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for bulk import")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, &gqldb.BulkImportOptions{})
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}

	if !session.Success {
		t.Errorf("StartBulkImport returned success=false: %s", session.Message)
	}

	if session.SessionID == "" {
		t.Error("StartBulkImport returned empty session ID")
	}

	t.Logf("Started bulk import session: %s", session.SessionID)

	// Abort the session to clean up
	_, err = testClient.AbortBulkImport(ctx, session.SessionID)
	if err != nil {
		t.Logf("AbortBulkImport failed (may be expected): %v", err)
	}
}

func TestBulkImportWorkflow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := "test_bulk_workflow_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for bulk import workflow")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Step 1: Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	t.Logf("Started bulk import session: %s", session.SessionID)

	// Step 2: Insert nodes with bulk import session
	nodes := []*gqldb.NodeData{
		{
			Labels:     []string{"BulkPerson"},
			Properties: map[string]interface{}{"name": "Alice", "idx": int64(1)},
		},
		{
			Labels:     []string{"BulkPerson"},
			Properties: map[string]interface{}{"name": "Bob", "idx": int64(2)},
		},
		{
			Labels:     []string{"BulkPerson"},
			Properties: map[string]interface{}{"name": "Charlie", "idx": int64(3)},
		},
	}

	insertConfig := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	nodeResult, err := testClient.InsertNodesBatchAuto(ctx, graphName, nodes, insertConfig)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}

	if !nodeResult.Success {
		t.Errorf("InsertNodes returned success=false: %s", nodeResult.Message)
	}

	t.Logf("Inserted %d nodes", nodeResult.NodeCount)

	// Step 3: Get status
	status, err := testClient.GetBulkImportStatus(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("GetBulkImportStatus failed: %v", err)
	}

	t.Logf("Bulk import status: active=%v, graph=%s, records=%d",
		status.IsActive, status.GraphName, status.RecordCount)

	// Step 4: End bulk import
	endResult, err := testClient.EndBulkImport(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("EndBulkImport failed: %v", err)
	}

	if !endResult.Success {
		t.Errorf("EndBulkImport returned success=false: %s", endResult.Message)
	}

	// Regression guard (driver_bug_endbulkimport_zero): the server returns the
	// real combined record count and wall-clock time; the driver must surface
	// them, not hardcode 0. Previously TotalRecords/DurationMs were dropped.
	if endResult.TotalRecords <= 0 {
		t.Errorf("EndBulkImport TotalRecords = %d, want > 0 (inserted %d nodes; driver must not drop the server's count)",
			endResult.TotalRecords, len(nodes))
	}
	t.Logf("Ended bulk import: %d total records, %d ms", endResult.TotalRecords, endResult.DurationMs)
}

func TestAbortBulkImport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	graphName := "test_abort_bulk_" + time.Now().Format("20060102150405")

	// Create a test graph
	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for abort bulk import")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}
	t.Logf("Started bulk import session: %s", session.SessionID)

	// Insert some data
	nodes := []*gqldb.NodeData{
		{
			Labels:     []string{"AbortTest"},
			Properties: map[string]interface{}{"name": "TestNode"},
		},
	}

	insertConfig := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, insertConfig)
	if err != nil {
		t.Fatalf("InsertNodes failed: %v", err)
	}

	// Abort the session
	abortResult, err := testClient.AbortBulkImport(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("AbortBulkImport failed: %v", err)
	}

	if !abortResult.Success {
		t.Errorf("AbortBulkImport returned success=false: %s", abortResult.Message)
	}

	t.Logf("Aborted bulk import session successfully")
}

func TestStartBulkImportNonexistentGraph(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.StartBulkImport(ctx, "nonexistent_graph_xyz_999", nil)
	if err == nil {
		t.Fatal("expected error when starting bulk import on nonexistent graph")
	}

	t.Logf("Got expected error for nonexistent graph: %v", err)
}

func TestStartBulkImportEmptyGraph(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.StartBulkImport(ctx, "", nil)
	if err == nil {
		t.Fatal("expected error when starting bulk import with empty graph name")
	}

	t.Logf("Got expected error for empty graph name: %v", err)
}

func TestCheckpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := "test_checkpoint_" + time.Now().Format("20060102150405")

	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for checkpoint")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}

	// Insert some nodes
	nodes := []*gqldb.NodeData{
		{
			Labels:     []string{"CheckpointTest"},
			Properties: map[string]interface{}{"name": "CP1"},
		},
		{
			Labels:     []string{"CheckpointTest"},
			Properties: map[string]interface{}{"name": "CP2"},
		},
	}

	insertConfig := &gqldb.InsertNodesConfig{
		BulkImportSessionID: session.SessionID,
	}
	_, err = testClient.InsertNodesBatchAuto(ctx, graphName, nodes, insertConfig)
	if err != nil {
		// Clean up on failure
		testClient.AbortBulkImport(ctx, session.SessionID)
		t.Fatalf("InsertNodes failed: %v", err)
	}

	// Explicit checkpoint
	cpResult, err := testClient.Checkpoint(ctx, session.SessionID)
	if err != nil {
		testClient.AbortBulkImport(ctx, session.SessionID)
		t.Fatalf("Checkpoint failed: %v", err)
	}

	if !cpResult.Success {
		t.Errorf("Checkpoint returned success=false: %s", cpResult.Message)
	}

	t.Logf("Checkpoint: record_count=%d, last_checkpoint_count=%d",
		cpResult.RecordCount, cpResult.LastCheckpointCount)

	// End the session
	_, err = testClient.EndBulkImport(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("EndBulkImport failed: %v", err)
	}
}

func TestGetBulkImportStatus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	graphName := "test_status_" + time.Now().Format("20060102150405")

	err := testClient.CreateGraph(ctx, graphName, gqldb.GraphTypeOpen, "Test graph for status")
	if err != nil {
		t.Fatalf("CreateGraph failed: %v", err)
	}
	defer dropTestGraph(graphName)

	// Start bulk import session
	session, err := testClient.StartBulkImport(ctx, graphName, nil)
	if err != nil {
		t.Fatalf("StartBulkImport failed: %v", err)
	}

	// Get status
	status, err := testClient.GetBulkImportStatus(ctx, session.SessionID)
	if err != nil {
		testClient.AbortBulkImport(ctx, session.SessionID)
		t.Fatalf("GetBulkImportStatus failed: %v", err)
	}

	t.Logf("Bulk import status: active=%v, graph=%s, records=%d",
		status.IsActive, status.GraphName, status.RecordCount)

	// Abort to clean up
	_, err = testClient.AbortBulkImport(ctx, session.SessionID)
	if err != nil {
		t.Logf("AbortBulkImport failed (may be expected): %v", err)
	}
}

func TestGetBulkImportStatusInvalidSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.GetBulkImportStatus(ctx, "invalid_session_id_xyz")
	if err == nil {
		t.Fatal("expected error when getting status with invalid session ID")
	}

	t.Logf("Got expected error for invalid session status: %v", err)
}

func TestEndBulkImportInvalidSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.EndBulkImport(ctx, "invalid_session_id_xyz")
	if err == nil {
		t.Fatal("expected error when ending invalid session ID")
	}

	t.Logf("Got expected error for invalid end session: %v", err)
}

func TestAbortBulkImportInvalidSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := testClient.AbortBulkImport(ctx, "invalid_session_id_xyz")
	if err == nil {
		t.Fatal("expected error when aborting invalid session ID")
	}

	t.Logf("Got expected error for invalid abort session: %v", err)
}
