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

// TestPoint3DCreation tests creating Point3D instances
func TestPoint3DCreation(t *testing.T) {
	// Test with NewPoint3D constructor
	p := types.NewPoint3D(1.0, 2.0, 3.0)
	if p.X != 1.0 || p.Y != 2.0 || p.Z != 3.0 {
		t.Errorf("NewPoint3D failed: expected (1.0, 2.0, 3.0), got (%f, %f, %f)", p.X, p.Y, p.Z)
	}

	// Test with struct literal
	point3d := types.Point3D{
		X: 0.0,
		Y: -1.0,
		Z: 5.5,
	}
	if point3d.X != 0.0 || point3d.Y != -1.0 || point3d.Z != 5.5 {
		t.Errorf("Point3D struct literal failed: expected (0.0, -1.0, 5.5), got (%f, %f, %f)", point3d.X, point3d.Y, point3d.Z)
	}
}

// TestPoint3DString tests Point3D string representation
func TestPoint3DString(t *testing.T) {
	testCases := []struct {
		point3d  *types.Point3D
		expected string
	}{
		{types.NewPoint3D(1.0, 2.0, 3.0), "POINT3D(1.000000 2.000000 3.000000)"},
		{types.NewPoint3D(0.0, 0.0, 0.0), "POINT3D(0.000000 0.000000 0.000000)"},
		{types.NewPoint3D(-10.5, 20.3, -30.7), "POINT3D(-10.500000 20.300000 -30.700000)"},
		{types.NewPoint3D(100.123456, -200.654321, 300.999), "POINT3D(100.123456 -200.654321 300.999000)"},
	}

	for i, tc := range testCases {
		result := tc.point3d.String()
		if result != tc.expected {
			t.Errorf("Test case %d: expected %s, got %s", i, tc.expected, result)
		}
	}
}

// TestPoint3DFromStr tests parsing Point3D from string
func TestPoint3DFromStr(t *testing.T) {
	testCases := []string{
		"POINT3D(1.0 2.0 3.0)",
		"Point3D(9 22.2 -5.5)",
		"Point3D(-10.1 22.2 0.0)",
		"Point3D(-80 -80 -80)",
		"Point3D(0.00 0.00 0.00)",
		"point3d(1.00 -2.00 3.50)",
		"POINT3D(100.123 -200.456 300.789)",
	}

	for i, str := range testCases {
		point3d, err := types.Point3DFromStr(str)
		if err != nil {
			t.Errorf("Test case %d: failed to parse %s: %v", i, str, err)
			continue
		}
		fmt.Printf("Parsed %s -> %v\n", str, point3d)
	}
}

// TestPoint3DFromStrInvalid tests parsing invalid Point3D strings
func TestPoint3DFromStrInvalid(t *testing.T) {
	invalidCases := []string{
		"POINT3D(1.0 2.0)",        // Missing Z coordinate
		"POINT3D(1.0)",            // Only one coordinate
		"POINT(1.0 2.0 3.0)",      // Wrong prefix (POINT instead of POINT3D)
		"POINT3D(a b c)",          // Non-numeric values
		"POINT3D 1.0 2.0 3.0",     // Missing parentheses
		"",                        // Empty string
		"invalid",                 // Invalid format
	}

	for i, str := range invalidCases {
		point3d, err := types.Point3DFromStr(str)
		if err == nil {
			t.Errorf("Test case %d: expected error for invalid string %s, but got point3d: %v", i, str, point3d)
		}
	}
}

// TestPoint3DRoundTrip tests converting Point3D to string and back
func TestPoint3DRoundTrip(t *testing.T) {
	originalPoints := []*types.Point3D{
		types.NewPoint3D(1.0, 2.0, 3.0),
		types.NewPoint3D(-5.5, 10.25, -15.75),
		types.NewPoint3D(0.0, 0.0, 0.0),
		types.NewPoint3D(100.123, -200.456, 300.789),
	}

	for i, original := range originalPoints {
		// Convert to string
		str := original.String()

		// Parse back from string
		parsed, err := types.Point3DFromStr(str)
		if err != nil {
			t.Errorf("Test case %d: failed to parse %s: %v", i, str, err)
			continue
		}

		// Compare coordinates (with tolerance for float precision)
		tolerance := 0.000001
		if !floatEqual(original.X, parsed.X, tolerance) ||
			!floatEqual(original.Y, parsed.Y, tolerance) ||
			!floatEqual(original.Z, parsed.Z, tolerance) {
			t.Errorf("Test case %d: round trip failed. Original: (%f, %f, %f), Parsed: (%f, %f, %f)",
				i, original.X, original.Y, original.Z, parsed.X, parsed.Y, parsed.Z)
		}
	}
}

// floatEqual compares two float64 values with tolerance
func floatEqual(a, b, tolerance float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < tolerance
}

// TestPoint3DVsPoint tests that Point3D and Point are separate types
func TestPoint3DVsPoint(t *testing.T) {
	// Create a 2D point
	point2d := types.NewPoint(10.0, 20.0)
	point2dStr := point2d.String()

	// Create a 3D point
	point3d := types.NewPoint3D(10.0, 20.0, 30.0)
	point3dStr := point3d.String()

	// Verify they have different string representations
	if point2dStr == point3dStr {
		t.Error("Point2D and Point3D should have different string representations")
	}

	// Verify Point2D cannot be parsed as Point3D
	_, err := types.Point3DFromStr(point2dStr)
	if err == nil {
		t.Error("Expected error when parsing Point2D string as Point3D")
	}

	// Verify Point3D cannot be parsed as Point2D
	_, err = types.PointFromStr(point3dStr)
	if err == nil {
		t.Error("Expected error when parsing Point3D string as Point2D")
	}
}

// TestInsertPoint3DPropertyWithGQL tests creating schema with POINT3D property and verifying data with GQL
func TestInsertPoint3DPropertyWithGQL(t *testing.T) {
	// Skip if client is not initialized
	if client == nil {
		t.Skip("Client not initialized, skipping integration test")
	}

	schemaName := "Point3DTestNode"
	propName := "location3d"

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

	// Step 2: Create POINT3D property using GQL
	createPropertyGQL := fmt.Sprintf(`ALTER NODE %s ADD PROPERTY {
    %s POINT3D
}`, schemaName, propName)

	propResp, err := client.Gql(createPropertyGQL, nil)
	if err != nil {
		t.Logf("Property creation warning (may already exist): %v", err)
	} else if propResp.Status.Code != ultipa.ErrorCode_SUCCESS {
		t.Logf("Property creation status: %v, message: %v", propResp.Status.Code, propResp.Status.Message)
	} else {
		t.Logf("✅ Property '%s' (POINT3D) created with GQL", propName)
	}

	// Step 3: Retrieve and verify the property
	retrievedProp, err := client.GetNodeProperty(schemaName, propName, nil)
	if err != nil {
		t.Fatalf("Failed to retrieve property after creation: %v", err)
	}

	if retrievedProp.Type != ultipa.PropertyType_POINT3D {
		t.Fatalf("Property type mismatch: expected POINT3D, got %v", retrievedProp.Type)
	}
	t.Logf("✅ Property '%s' verified as POINT3D type", propName)

	// Step 4: Insert nodes with POINT3D values using SDK API
	testSchema := structs.NewSchema(schemaName)
	testSchema.Properties = append(testSchema.Properties, retrievedProp)

	var nodes []*structs.Node

	// Node 1: Using NewPoint3D pointer
	node1 := structs.NewNode()
	node1.Set("_id", "p3d_node1")
	node1.Set(propName, types.NewPoint3D(1.5, 2.5, 3.5))

	// Node 2: Using Point3D struct value
	node2 := structs.NewNode()
	node2.Set("_id", "p3d_node2")
	node2.Set(propName, types.Point3D{X: 10.123, Y: 20.456, Z: 30.789})

	// Node 3: Using string representation
	node3 := structs.NewNode()
	node3.Set("_id", "p3d_node3")
	node3.Set(propName, "POINT3D(100.0 200.0 300.0)")

	// Node 4: Negative coordinates
	node4 := structs.NewNode()
	node4.Set("_id", "p3d_node4")
	node4.Set(propName, types.NewPoint3D(-50.5, -100.25, -150.75))

	// Node 5: Zero coordinates
	node5 := structs.NewNode()
	node5.Set("_id", "p3d_node5")
	node5.Set(propName, types.NewPoint3D(0.0, 0.0, 0.0))

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
		t.Fatalf("Failed to parse nodes from GQL response: %v", err)
	}

	if len(readNodes) < 5 {
		t.Fatalf("Expected at least 5 nodes, got %d", len(readNodes))
	}

	t.Logf("✅ GQL query returned %d nodes", len(readNodes))

	// Print the results
	t.Log("📋 Query results:")
	printers.PrintNodes(readNodes, schemas)

	// Verify Point3D values
	verificationCount := 0

	for _, node := range readNodes {
		// Use node.ID as identifier
		nodeId := node.ID
		if nodeId == "" {
			nodeId = fmt.Sprintf("UUID_%d", node.UUID)
		}

		point3dVal := node.Get(propName)
		if point3dVal == nil {
			continue
		}

		// Verify the value is a valid Point3D type
		switch v := point3dVal.(type) {
		case *types.Point3D:
			// Valid pointer to Point3D
			verificationCount++
		case types.Point3D:
			// Valid Point3D value
			verificationCount++
		case string:
			// Parse from string to validate
			_, err := types.Point3DFromStr(v)
			if err != nil {
				t.Logf("Node %s: Failed to parse Point3D from string '%v': %v", nodeId, v, err)
				continue
			}
			verificationCount++
		default:
			t.Logf("Node %s: Point3D value has unexpected type: %T (value: %v)", nodeId, point3dVal, point3dVal)
		}
	}

	if verificationCount == 0 {
		t.Error("❌ No Point3D values were successfully verified")
	} else {
		t.Logf("✅ Successfully verified %d Point3D values from GQL query", verificationCount)
	}

	t.Log("✅ POINT3D property test completed successfully!")
}