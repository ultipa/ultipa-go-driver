package test

import (
	"fmt"
	"testing"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/printers"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/types"
)

// TestInsertRecordPropertyWithGQL tests creating schema with RECORD property and verifying data with GQL
func TestInsertRecordPropertyWithGQL(t *testing.T) {
	// Skip if client is not initialized
	if client == nil {
		t.Skip("Client not initialized, skipping integration test")
	}

	schemaName := "RecordTestNode"
	propName := "metadata"

	// Step 1: Create node schema using SDK API
	schema := &structs.Schema{
		Name:   schemaName,
		DBType: ultipa.DBType_DBNODE,
	}

	_, err := client.CreateSchemaIfNotExist(schema, false, nil)
	if err != nil {
		t.Logf("Schema creation warning (may already exist): %v", err)
	} else {
		t.Logf("✅ Schema '%s' created/verified", schemaName)
	}

	// Step 2: Create RECORD property using GQL
	createPropertyGQL := fmt.Sprintf(`ALTER NODE %s ADD PROPERTY {
    %s RECORD
}`, schemaName, propName)

	propResp, err := client.Gql(createPropertyGQL, nil)
	if err != nil {
		t.Logf("Property creation warning (may already exist): %v", err)
	} else if propResp.Status.Code != ultipa.ErrorCode_SUCCESS {
		t.Logf("Property creation status: %v, message: %v", propResp.Status.Code, propResp.Status.Message)
	} else {
		t.Logf("✅ Property '%s' (RECORD) created with GQL", propName)
	}

	// Step 3: Retrieve and verify the property
	retrievedProp, err := client.GetNodeProperty(schemaName, propName, nil)
	if err != nil {
		t.Fatalf("Failed to retrieve property after creation: %v", err)
	}

	if retrievedProp.Type != ultipa.PropertyType_RECORD {
		t.Fatalf("Property type mismatch: expected RECORD, got %v", retrievedProp.Type)
	}
	t.Logf("✅ Property '%s' verified as RECORD type", propName)

	// Step 4: Insert nodes with RECORD values using SDK API
	testSchema := structs.NewSchema(schemaName)
	testSchema.Properties = append(testSchema.Properties, retrievedProp)

	var nodes []*structs.Node

	// Node 1: Using Record type
	node1 := structs.NewNode()
	node1.Set("_id", "record_node1")
	record1 := types.NewRecord(map[string]interface{}{
		"name":  "Alice",
		"age":   30,
		"email": "alice@example.com",
	})
	node1.Set(propName, record1)

	// Node 2: Using map[string]interface{} directly
	node2 := structs.NewNode()
	node2.Set("_id", "record_node2")
	record2 := map[string]interface{}{
		"user": map[string]interface{}{
			"id":   12345,
			"name": "Bob",
		},
		"settings": map[string]interface{}{
			"theme": "dark",
			"notifications": map[string]bool{
				"email": true,
				"sms":   false,
			},
		},
	}
	node2.Set(propName, record2)

	// Node 3: Using JSON string
	node3 := structs.NewNode()
	node3.Set("_id", "record_node3")
	node3.Set(propName, `{"items":["item1","item2","item3"],"count":3}`)

	// Node 4: Empty Record
	node4 := structs.NewNode()
	node4.Set("_id", "record_node4")
	node4.Set(propName, types.NewRecord(map[string]interface{}{}))

	// Node 5: Using Record with various types
	node5 := structs.NewNode()
	node5.Set("_id", "record_node5")
	record5 := types.NewRecord(map[string]interface{}{
		"count":   42,
		"price":   19.99,
		"active":  true,
		"deleted": false,
	})
	node5.Set(propName, record5)

	nodes = append(nodes, node1, node2, node3, node4, node5)

	// Insert nodes using SDK API
	insertResp, err := client.InsertNodesBatchBySchema(testSchema, nodes, &configuration.InsertRequestConfig{
		InsertType: ultipa.InsertType_OVERWRITE,
	})

	if err != nil {
		t.Fatalf("Failed to insert nodes: %v", err)
	}

	if insertResp.Status.Code != ultipa.ErrorCode_SUCCESS {
		t.Fatalf("Insert failed with status: %v, message: %v", insertResp.Status.Code, insertResp.Status.Message)
	}

	t.Logf("✅ Successfully inserted %d nodes using SDK API (engine cost: %dms, total cost: %dms)",
		len(nodes), insertResp.Statistic.EngineCost, insertResp.Statistic.TotalCost)

	// Step 5: Verify data using GQL query
	t.Log("📊 Verifying data using GQL query...")
	queryGQL := fmt.Sprintf(`MATCH (n:%s) RETURN n LIMIT 100`, schemaName)
	queryResp, err := client.Gql(queryGQL, nil)

	if err != nil {
		t.Fatalf("Failed to query nodes with GQL: %v", err)
	}

	if queryResp.Status.Code != ultipa.ErrorCode_SUCCESS {
		t.Fatalf("GQL query failed with status: %v, message: %v", queryResp.Status.Code, queryResp.Status.Message)
	}

	// Get nodes from response
	readNodes, schemas, err := queryResp.Alias("n").AsNodes()
	if err != nil {
		// Debug: print the raw response to understand the issue
		t.Logf("DEBUG: Error parsing nodes: %v", err)
		t.Logf("DEBUG: Response status: %v", queryResp.Status)
		t.Fatalf("Failed to parse nodes from GQL response: %v", err)
	}

	if len(readNodes) < 5 {
		t.Fatalf("Expected at least 5 nodes, got %d", len(readNodes))
	}

	t.Logf("✅ GQL query returned %d nodes", len(readNodes))

	// Print the results
	t.Log("📋 Query results:")
	printers.PrintNodes(readNodes, schemas)

	// Verify RECORD values
	verificationCount := 0

	for _, node := range readNodes {
		// Use node.ID as identifier
		nodeId := node.ID
		if nodeId == "" {
			nodeId = fmt.Sprintf("UUID_%d", node.UUID)
		}

		recordVal := node.Get(propName)
		if recordVal == nil {
			continue
		}

		// RECORD should be returned as *types.Record
		record, ok := recordVal.(*types.Record)
		if !ok {
			t.Logf("Node %s: RECORD value has unexpected type: %T (value: %v)", nodeId, recordVal, recordVal)
			continue
		}

		// Verify it's a valid Record
		if record == nil {
			t.Logf("Node %s: RECORD value is nil", nodeId)
			continue
		}

		// Log the record for debugging
		t.Logf("✓ Node %s has Record: %v", nodeId, record.String())

		// Successfully parsed RECORD
		verificationCount++
	}

	if verificationCount == 0 {
		t.Error("❌ No RECORD values were successfully verified")
	} else {
		t.Logf("✅ Successfully verified %d RECORD values from GQL query", verificationCount)
	}

	t.Log("✅ RECORD property test completed successfully!")
}
