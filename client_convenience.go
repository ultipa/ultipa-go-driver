package gqldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/ultipa/ultipa-go-driver/v6/types"
)

// quoteLabel wraps a name in backticks for safe use in GQL statements.
// This ensures names containing special characters (spaces, hyphens, dots, colons)
// are correctly interpreted by the GQL parser.
// Label names and property names should be quoted; graph names, index names,
// and fulltext names should NOT be wrapped in backticks.
func quoteLabel(name string) string {
	return "`" + name + "`"
}

// quoteLabels wraps each label name in backticks and joins them with ", ".
func quoteLabels(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = quoteLabel(n)
	}
	return strings.Join(quoted, ", ")
}

// =============================================================================
// Convenience API — Graph Operations (6 methods)
// =============================================================================

// CreateOpenGraph creates a new open (schema-less) graph.
func (c *Client) CreateOpenGraph(ctx context.Context, name string) (*Response, error) {
	return c.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s {}", name), nil)
}

// CreateClosedGraph creates a new empty closed (schema-enforced) graph.
// Use CreateNodeLabel/CreateEdgeLabel to add labels after creation.
func (c *Client) CreateClosedGraph(ctx context.Context, name string) (*Response, error) {
	return c.Gql(ctx, fmt.Sprintf("CREATE GRAPH %s {}", name), nil)
}

// CreateGraphIfNotExist creates a graph if it does not already exist.
// Returns (true, nil) if the graph was created, (false, nil) if it already existed.
func (c *Client) CreateGraphIfNotExist(ctx context.Context, name string, graphType GraphType, desc string) (bool, error) {
	err := c.CreateGraph(ctx, name, graphType, desc)
	if err != nil {
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "already exist") || strings.Contains(errMsg, "duplicate") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// HasGraph checks whether a graph with the given name exists.
func (c *Client) HasGraph(ctx context.Context, name string) (bool, error) {
	graphs, err := c.ListGraphs(ctx)
	if err != nil {
		return false, err
	}
	for _, g := range graphs {
		if g.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// AlterGraph renames a graph.
func (c *Client) AlterGraph(ctx context.Context, graphName string, newName string) (*Response, error) {
	gql := fmt.Sprintf("ALTER GRAPH %s RENAME TO %s", graphName, newName)
	return c.Gql(ctx, gql, nil)
}

// Truncate removes all data from a graph while keeping its structure.
func (c *Client) Truncate(ctx context.Context, graphName string) (*Response, error) {
	gql := fmt.Sprintf("TRUNCATE GRAPH %s", graphName)
	return c.Gql(ctx, gql, nil)
}

// =============================================================================
// Convenience API — Label Operations (16 methods)
// =============================================================================

// ShowLabels returns all labels (node and edge) in the current graph.
func (c *Client) ShowLabels(ctx context.Context) ([]types.LabelInfo, error) {
	gql := "SHOW LABELS"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseLabelInfoRows(resp)
}

// ShowNodeLabels returns all node labels in the current graph.
func (c *Client) ShowNodeLabels(ctx context.Context) ([]types.LabelInfo, error) {
	gql := "SHOW NODE LABELS"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseLabelInfoRows(resp)
}

// ShowEdgeLabels returns all edge labels in the current graph.
func (c *Client) ShowEdgeLabels(ctx context.Context) ([]types.LabelInfo, error) {
	gql := "SHOW EDGE LABELS"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseLabelInfoRows(resp)
}

// ShowNodeTypes returns all node types with their properties.
func (c *Client) ShowNodeTypes(ctx context.Context) ([]types.NodeTypeInfo, error) {
	gql := "SHOW NODE TYPES"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseNodeTypeInfoRows(resp)
}

// ShowEdgeTypes returns all edge types with their properties.
func (c *Client) ShowEdgeTypes(ctx context.Context) ([]types.EdgeTypeInfo, error) {
	gql := "SHOW EDGE TYPES"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseEdgeTypeInfoRows(resp)
}

// GetLabel returns a specific label by name from the current graph.
// Returns nil if not found.
func (c *Client) GetLabel(ctx context.Context, name string) (*types.LabelInfo, error) {
	labels, err := c.ShowLabels(ctx)
	if err != nil {
		return nil, err
	}
	for _, l := range labels {
		if l.Name == name {
			return &l, nil
		}
	}
	return nil, nil
}

// GetNodeLabel returns a specific node type by name.
// Returns nil if not found.
func (c *Client) GetNodeLabel(ctx context.Context, name string) (*types.NodeTypeInfo, error) {
	nodeTypes, err := c.ShowNodeTypes(ctx)
	if err != nil {
		return nil, err
	}
	for _, nt := range nodeTypes {
		if nt.Name == name {
			return &nt, nil
		}
	}
	return nil, nil
}

// GetEdgeLabel returns a specific edge type by name.
// Returns nil if not found.
func (c *Client) GetEdgeLabel(ctx context.Context, name string) (*types.EdgeTypeInfo, error) {
	edgeTypes, err := c.ShowEdgeTypes(ctx)
	if err != nil {
		return nil, err
	}
	for _, et := range edgeTypes {
		if et.Name == name {
			return &et, nil
		}
	}
	return nil, nil
}

// CreateNodeLabel creates a node label in the current graph (closed graph).
// GQL: ALTER GRAPH g ADD NODE { Name ({p1 T1, p2 T2}) }
func (c *Client) CreateNodeLabel(ctx context.Context, name string, props []types.PropertyDef) (*Response, error) {
	currentGraph := c.sessions.GetDefaultGraph()
	propStr := buildPropertyDefString(props)
	gql := fmt.Sprintf("ALTER GRAPH %s ADD NODE { %s (%s) }", currentGraph, quoteLabel(name), propStr)
	return c.Gql(ctx, gql, nil)
}

// CreateEdgeLabel creates an edge label in the current graph (closed graph).
// GQL: ALTER GRAPH g ADD EDGE { Name ()-[{p1 T1}]->() }
func (c *Client) CreateEdgeLabel(ctx context.Context, name string, props []types.PropertyDef) (*Response, error) {
	currentGraph := c.sessions.GetDefaultGraph()
	propStr := buildPropertyDefString(props)
	gql := fmt.Sprintf("ALTER GRAPH %s ADD EDGE { %s ()-[%s]->() }", currentGraph, quoteLabel(name), propStr)
	return c.Gql(ctx, gql, nil)
}

// DropNodeLabel drops a node label from the current graph (closed graph).
func (c *Client) DropNodeLabel(ctx context.Context, name string) (*Response, error) {
	currentGraph := c.sessions.GetDefaultGraph()
	gql := fmt.Sprintf("ALTER GRAPH %s DROP NODE %s", currentGraph, quoteLabel(name))
	return c.Gql(ctx, gql, nil)
}

// DropEdgeLabel drops one or more edge labels from the current graph (closed graph).
func (c *Client) DropEdgeLabel(ctx context.Context, names ...string) (*Response, error) {
	currentGraph := c.sessions.GetDefaultGraph()
	gql := fmt.Sprintf("ALTER GRAPH %s DROP EDGE %s", currentGraph, quoteLabels(names))
	return c.Gql(ctx, gql, nil)
}

// CreateLabelIfNotExist creates a label if it does not already exist.
// Returns (true, nil) if created, (false, nil) if it already existed.
func (c *Client) CreateLabelIfNotExist(ctx context.Context, nodeOrEdge types.DBType, name string, props []types.PropertyDef) (bool, error) {
	labels, err := c.ShowLabels(ctx)
	if err != nil {
		return false, err
	}

	targetType := "NODE"
	if nodeOrEdge == types.DBTypeEdge {
		targetType = "EDGE"
	}

	for _, l := range labels {
		if l.Name == name && l.Type == targetType {
			return false, nil
		}
	}

	if nodeOrEdge == types.DBTypeNode {
		_, err = c.CreateNodeLabel(ctx, name, props)
	} else {
		_, err = c.CreateEdgeLabel(ctx, name, props)
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// AlterNodeLabel renames a node label.
func (c *Client) AlterNodeLabel(ctx context.Context, oldName string, newName string) (*Response, error) {
	gql := fmt.Sprintf("ALTER NODE %s RENAME TO %s", quoteLabel(oldName), quoteLabel(newName))
	return c.Gql(ctx, gql, nil)
}

// AlterEdgeLabel renames an edge label.
func (c *Client) AlterEdgeLabel(ctx context.Context, oldName string, newName string) (*Response, error) {
	gql := fmt.Sprintf("ALTER EDGE %s RENAME TO %s", quoteLabel(oldName), quoteLabel(newName))
	return c.Gql(ctx, gql, nil)
}

// ShowAlgos returns all available algorithms.
func (c *Client) ShowAlgos(ctx context.Context) ([]types.AlgoInfo, error) {
	resp, err := c.Gql(ctx, "SHOW ALGOS", nil)
	if err != nil {
		return nil, err
	}
	return parseAlgoInfoRows(resp)
}

// =============================================================================
// Convenience API — Property Operations (13 methods)
// =============================================================================

// ShowProperty returns properties for a label (node or edge) in the current graph.
func (c *Client) ShowProperty(ctx context.Context, nodeOrEdge types.DBType, labelName string) ([]types.PropertyDef, error) {
	if nodeOrEdge == types.DBTypeNode {
		return c.ShowNodeProperty(ctx, labelName)
	}
	return c.ShowEdgeProperty(ctx, labelName)
}

// ShowNodeProperty returns properties for a node label.
func (c *Client) ShowNodeProperty(ctx context.Context, labelName string) ([]types.PropertyDef, error) {
	nt, err := c.GetNodeLabel(ctx, labelName)
	if err != nil {
		return nil, err
	}
	if nt == nil {
		return nil, fmt.Errorf("node label %q not found", labelName)
	}
	return nt.Properties, nil
}

// ShowEdgeProperty returns properties for an edge label.
func (c *Client) ShowEdgeProperty(ctx context.Context, labelName string) ([]types.PropertyDef, error) {
	et, err := c.GetEdgeLabel(ctx, labelName)
	if err != nil {
		return nil, err
	}
	if et == nil {
		return nil, fmt.Errorf("edge label %q not found", labelName)
	}
	return et.Properties, nil
}

// GetProperty returns a specific property definition for a label.
// Returns nil if not found.
func (c *Client) GetProperty(ctx context.Context, nodeOrEdge types.DBType, labelName string, propName string) (*types.PropertyDef, error) {
	props, err := c.ShowProperty(ctx, nodeOrEdge, labelName)
	if err != nil {
		return nil, err
	}
	for _, p := range props {
		if p.Name == propName {
			return &p, nil
		}
	}
	return nil, nil
}

// GetNodeProperty returns a specific property definition for a node label.
// Returns nil if not found.
func (c *Client) GetNodeProperty(ctx context.Context, labelName string, propName string) (*types.PropertyDef, error) {
	return c.GetProperty(ctx, types.DBTypeNode, labelName, propName)
}

// GetEdgeProperty returns a specific property definition for an edge label.
// Returns nil if not found.
func (c *Client) GetEdgeProperty(ctx context.Context, labelName string, propName string) (*types.PropertyDef, error) {
	return c.GetProperty(ctx, types.DBTypeEdge, labelName, propName)
}

// CreateProperty adds properties to a label (node or edge).
// GQL: ALTER NODE/EDGE X ADD PROPERTY {p1 T1, p2 T2}
func (c *Client) CreateProperty(ctx context.Context, nodeOrEdge types.DBType, labelName string, props []types.PropertyDef) (*Response, error) {
	if nodeOrEdge == types.DBTypeNode {
		return c.CreateNodeProperty(ctx, labelName, props)
	}
	return c.CreateEdgeProperty(ctx, labelName, props)
}

// CreateNodeProperty adds properties to a node label.
func (c *Client) CreateNodeProperty(ctx context.Context, labelName string, props []types.PropertyDef) (*Response, error) {
	propStr := buildPropertyDefString(props)
	gql := fmt.Sprintf("ALTER NODE %s ADD PROPERTY %s", quoteLabel(labelName), propStr)
	return c.Gql(ctx, gql, nil)
}

// CreateEdgeProperty adds properties to an edge label.
func (c *Client) CreateEdgeProperty(ctx context.Context, labelName string, props []types.PropertyDef) (*Response, error) {
	propStr := buildPropertyDefString(props)
	gql := fmt.Sprintf("ALTER EDGE %s ADD PROPERTY %s", quoteLabel(labelName), propStr)
	return c.Gql(ctx, gql, nil)
}

// DropProperty drops properties from a label (node or edge).
func (c *Client) DropProperty(ctx context.Context, nodeOrEdge types.DBType, labelName string, propNames ...string) (*Response, error) {
	if nodeOrEdge == types.DBTypeNode {
		return c.DropNodeProperty(ctx, labelName, propNames...)
	}
	return c.DropEdgeProperty(ctx, labelName, propNames...)
}

// DropNodeProperty drops properties from a node label.
func (c *Client) DropNodeProperty(ctx context.Context, labelName string, propNames ...string) (*Response, error) {
	gql := fmt.Sprintf("ALTER NODE %s DROP PROPERTY %s", quoteLabel(labelName), quoteLabels(propNames))
	return c.Gql(ctx, gql, nil)
}

// DropEdgeProperty drops properties from an edge label.
func (c *Client) DropEdgeProperty(ctx context.Context, labelName string, propNames ...string) (*Response, error) {
	gql := fmt.Sprintf("ALTER EDGE %s DROP PROPERTY %s", quoteLabel(labelName), quoteLabels(propNames))
	return c.Gql(ctx, gql, nil)
}

// CreatePropertyIfNotExist creates properties on a label if they do not already exist.
// Returns (true, nil) if any properties were created, (false, nil) if all already existed.
func (c *Client) CreatePropertyIfNotExist(ctx context.Context, nodeOrEdge types.DBType, labelName string, props []types.PropertyDef) (bool, error) {
	existing, err := c.ShowProperty(ctx, nodeOrEdge, labelName)
	if err != nil {
		return false, err
	}

	existingSet := make(map[string]bool)
	for _, p := range existing {
		existingSet[p.Name] = true
	}

	var newProps []types.PropertyDef
	for _, p := range props {
		if !existingSet[p.Name] {
			newProps = append(newProps, p)
		}
	}

	if len(newProps) == 0 {
		return false, nil
	}

	_, err = c.CreateProperty(ctx, nodeOrEdge, labelName, newProps)
	if err != nil {
		return false, err
	}
	return true, nil
}

// =============================================================================
// Convenience API — Constraint Operations (4 methods)
// =============================================================================

// CreateNotNullConstraint creates a NOT NULL constraint on a property.
func (c *Client) CreateNotNullConstraint(ctx context.Context, nodeOrEdge types.DBType, labelName string, propName string) (*Response, error) {
	entityType := dbTypeToKeyword(nodeOrEdge)
	gql := fmt.Sprintf("ALTER %s %s ADD CONSTRAINT NOT NULL ON %s", entityType, quoteLabel(labelName), quoteLabel(propName))
	return c.Gql(ctx, gql, nil)
}

// CreateUniqueConstraint creates a UNIQUE constraint on one or more properties.
func (c *Client) CreateUniqueConstraint(ctx context.Context, nodeOrEdge types.DBType, labelName string, propNames ...string) (*Response, error) {
	entityType := dbTypeToKeyword(nodeOrEdge)
	gql := fmt.Sprintf("ALTER %s %s ADD CONSTRAINT UNIQUE ON %s", entityType, quoteLabel(labelName), quoteLabels(propNames))
	return c.Gql(ctx, gql, nil)
}

// DropNotNullConstraint removes a NOT NULL constraint from a property.
func (c *Client) DropNotNullConstraint(ctx context.Context, nodeOrEdge types.DBType, labelName string, propName string) (*Response, error) {
	entityType := dbTypeToKeyword(nodeOrEdge)
	gql := fmt.Sprintf("ALTER %s %s DROP CONSTRAINT NOT NULL ON %s", entityType, quoteLabel(labelName), quoteLabel(propName))
	return c.Gql(ctx, gql, nil)
}

// DropUniqueConstraint removes a UNIQUE constraint from one or more properties.
func (c *Client) DropUniqueConstraint(ctx context.Context, nodeOrEdge types.DBType, labelName string, propNames ...string) (*Response, error) {
	entityType := dbTypeToKeyword(nodeOrEdge)
	gql := fmt.Sprintf("ALTER %s %s DROP CONSTRAINT UNIQUE ON %s", entityType, quoteLabel(labelName), quoteLabels(propNames))
	return c.Gql(ctx, gql, nil)
}

// =============================================================================
// Convenience API — Index Operations (7 methods)
// =============================================================================

// ShowIndex returns all indexes in the current graph.
func (c *Client) ShowIndex(ctx context.Context) ([]types.IndexInfo, error) {
	gql := "SHOW INDEX"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseIndexInfoRows(resp)
}

// ShowNodeIndex returns all node indexes in the current graph.
func (c *Client) ShowNodeIndex(ctx context.Context) ([]types.IndexInfo, error) {
	gql := "SHOW NODE INDEX"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseIndexInfoRows(resp)
}

// ShowEdgeIndex returns all edge indexes in the current graph.
func (c *Client) ShowEdgeIndex(ctx context.Context) ([]types.IndexInfo, error) {
	gql := "SHOW EDGE INDEX"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseIndexInfoRows(resp)
}

// CreateNodeIndex creates an index on a node label.
// GQL: CREATE INDEX name ON NODE Label (prop1, prop2(prefixLen))
func (c *Client) CreateNodeIndex(ctx context.Context, indexName string, labelName string, props []types.IndexProperty) (*Response, error) {
	propStr := buildIndexPropertyString(props)
	gql := fmt.Sprintf("CREATE INDEX %s ON NODE %s (%s)", indexName, quoteLabel(labelName), propStr)
	return c.Gql(ctx, gql, nil)
}

// CreateEdgeIndex creates an index on an edge label.
func (c *Client) CreateEdgeIndex(ctx context.Context, indexName string, labelName string, props []types.IndexProperty) (*Response, error) {
	propStr := buildIndexPropertyString(props)
	gql := fmt.Sprintf("CREATE INDEX %s ON EDGE %s (%s)", indexName, quoteLabel(labelName), propStr)
	return c.Gql(ctx, gql, nil)
}

// DropNodeIndex drops a node index by name.
func (c *Client) DropNodeIndex(ctx context.Context, indexName string) (*Response, error) {
	gql := fmt.Sprintf("DROP NODE INDEX %s", indexName)
	return c.Gql(ctx, gql, nil)
}

// DropEdgeIndex drops an edge index by name.
func (c *Client) DropEdgeIndex(ctx context.Context, indexName string) (*Response, error) {
	gql := fmt.Sprintf("DROP EDGE INDEX %s", indexName)
	return c.Gql(ctx, gql, nil)
}

// =============================================================================
// Convenience API — Fulltext Operations (7 methods)
// =============================================================================

// ShowFulltext returns all fulltext indexes in the current graph.
func (c *Client) ShowFulltext(ctx context.Context) ([]types.FulltextInfo, error) {
	gql := "SHOW FULLTEXT"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseFulltextInfoRows(resp)
}

// ShowNodeFulltext returns all node fulltext indexes.
func (c *Client) ShowNodeFulltext(ctx context.Context) ([]types.FulltextInfo, error) {
	gql := "SHOW NODE FULLTEXT"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseFulltextInfoRows(resp)
}

// ShowEdgeFulltext returns all edge fulltext indexes.
func (c *Client) ShowEdgeFulltext(ctx context.Context) ([]types.FulltextInfo, error) {
	gql := "SHOW EDGE FULLTEXT"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseFulltextInfoRows(resp)
}

// CreateNodeFulltext creates a fulltext index on a node label.
// GQL: CREATE FULLTEXT name ON NODE Label (prop1, prop2)
func (c *Client) CreateNodeFulltext(ctx context.Context, indexName string, labelName string, props []string) (*Response, error) {
	gql := fmt.Sprintf("CREATE FULLTEXT %s ON NODE %s (%s)", indexName, quoteLabel(labelName), quoteLabels(props))
	return c.Gql(ctx, gql, nil)
}

// CreateEdgeFulltext creates a fulltext index on an edge label.
func (c *Client) CreateEdgeFulltext(ctx context.Context, indexName string, labelName string, props []string) (*Response, error) {
	gql := fmt.Sprintf("CREATE FULLTEXT %s ON EDGE %s (%s)", indexName, quoteLabel(labelName), quoteLabels(props))
	return c.Gql(ctx, gql, nil)
}

// DropNodeFulltext drops a node fulltext index by name.
func (c *Client) DropNodeFulltext(ctx context.Context, indexName string) (*Response, error) {
	gql := fmt.Sprintf("DROP NODE FULLTEXT %s", indexName)
	return c.Gql(ctx, gql, nil)
}

// DropEdgeFulltext drops an edge fulltext index by name.
func (c *Client) DropEdgeFulltext(ctx context.Context, indexName string) (*Response, error) {
	gql := fmt.Sprintf("DROP EDGE FULLTEXT %s", indexName)
	return c.Gql(ctx, gql, nil)
}

// =============================================================================
// Convenience API — Task Operations (3 methods)
// =============================================================================

// ShowTasks returns all tasks in the current graph.
func (c *Client) ShowTasks(ctx context.Context) ([]types.TaskInfo, error) {
	gql := "SHOW TASKS"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseTaskInfoRows(resp)
}

// DeleteTask deletes a task by ID.
func (c *Client) DeleteTask(ctx context.Context, taskId string) (*Response, error) {
	gql := fmt.Sprintf("DELETE TASK %s", taskId)
	return c.Gql(ctx, gql, nil)
}

// StopTask stops a running task by ID.
func (c *Client) StopTask(ctx context.Context, taskId string) (*Response, error) {
	gql := fmt.Sprintf("STOP TASK %s", taskId)
	return c.Gql(ctx, gql, nil)
}

// =============================================================================
// Convenience API — Data Insert Operations (2 methods)
// =============================================================================

// InsertNodes inserts nodes using GQL INSERT syntax with RETURN clause.
// GQL: INSERT (n0:Label {p1: v1}), (n1:Label {p2: v2}) RETURN n0, n1
// When insertType is InsertTypeOverwrite, uses INSERT OVERWRITE syntax.
// If NodeData.ID is set (non-empty), includes it as _id property in the GQL.
// Returns the raw Response containing the inserted nodes in columns n0, n1, etc.
func (c *Client) InsertNodes(ctx context.Context, nodes []types.NodeData, insertType types.InsertType) (*Response, error) {
	if len(nodes) == 0 {
		return &Response{}, nil
	}

	for i, node := range nodes {
		if len(node.Labels) == 0 {
			return nil, fmt.Errorf("node at index %d has no labels: at least one label is required", i)
		}
	}

	var parts []string
	var varNames []string
	for i, node := range nodes {
		varName := fmt.Sprintf("n%d", i)
		varNames = append(varNames, varName)
		// GQL INSERT only supports single label per node
		label := quoteLabel(node.Labels[0])
		// Build properties including _id if set
		props := node.Properties
		if node.ID != "" {
			merged := make(map[string]interface{}, len(props)+1)
			merged["_id"] = node.ID
			for k, v := range props {
				merged[k] = v
			}
			props = merged
		}
		propStr := buildPropertiesValueString(props)
		if propStr != "" {
			parts = append(parts, fmt.Sprintf("(%s:%s %s)", varName, label, propStr))
		} else {
			parts = append(parts, fmt.Sprintf("(%s:%s)", varName, label))
		}
	}

	insertKeyword := "INSERT"
	if insertType == types.InsertTypeOverwrite {
		insertKeyword = "INSERT OVERWRITE"
	}
	gql := insertKeyword + " " + strings.Join(parts, ", ") + " RETURN " + strings.Join(varNames, ", ")
	return c.Gql(ctx, gql, nil)
}

// InsertEdges inserts edges using GQL INSERT syntax with RETURN clause.
// Each edge is inserted individually using MATCH + INSERT to resolve node IDs.
// GQL: MATCH (src WHERE id(src) = 'from'), (dst WHERE id(dst) = 'to') INSERT (src)-[e0:Label {p1: v1}]->(dst) RETURN e0
// When insertType is InsertTypeOverwrite, uses INSERT OVERWRITE syntax.
// Returns a merged Response containing all inserted edges in columns e0, e1, etc.
func (c *Client) InsertEdges(ctx context.Context, edges []types.EdgeData, insertType types.InsertType) (*Response, error) {
	if len(edges) == 0 {
		return &Response{}, nil
	}

	insertKeyword := "INSERT"
	if insertType == types.InsertTypeOverwrite {
		insertKeyword = "INSERT OVERWRITE"
	}

	var allColumns []string
	var allValues []*TypedValue
	var totalAffected int64
	for i, edge := range edges {
		varName := fmt.Sprintf("e%d", i)
		labelPart := ""
		if edge.Label != "" {
			labelPart = ":" + quoteLabel(edge.Label)
		}
		propStr := buildPropertiesValueString(edge.Properties)
		var edgePart string
		if propStr != "" {
			edgePart = fmt.Sprintf("[%s%s %s]", varName, labelPart, propStr)
		} else {
			edgePart = fmt.Sprintf("[%s%s]", varName, labelPart)
		}
		gql := fmt.Sprintf(
			"MATCH (src WHERE id(src) = '%s'), (dst WHERE id(dst) = '%s') %s (src)-%s->(dst) RETURN %s",
			edge.FromNodeID, edge.ToNodeID, insertKeyword, edgePart, varName)
		resp, err := c.Gql(ctx, gql, nil)
		if err != nil {
			return nil, err
		}
		totalAffected += resp.RowsAffected
		allColumns = append(allColumns, varName)
		if len(resp.Rows) > 0 && len(resp.Rows[0].Values) > 0 {
			allValues = append(allValues, resp.Rows[0].Values[0])
		}
	}
	mergedRow := &Row{Values: allValues}
	return &Response{
		Columns:      allColumns,
		Rows:         []*Row{mergedRow},
		RowCount:     1,
		RowsAffected: totalAffected,
	}, nil
}

// =============================================================================
// Convenience API — System & Process Operations (4 methods)
// =============================================================================

// Top returns currently running processes (queries).
func (c *Client) Top(ctx context.Context) ([]types.ProcessInfo, error) {
	resp, err := c.Gql(ctx, "TOP", nil)
	if err != nil {
		return nil, err
	}
	return parseProcessInfoRows(resp)
}

// Kill terminates a running query by its query ID.
func (c *Client) Kill(ctx context.Context, queryId string) (*Response, error) {
	gql := fmt.Sprintf("KILL '%s'", queryId)
	return c.Gql(ctx, gql, nil)
}

// Stats returns statistics for the current graph using db.stats().
func (c *Client) Stats(ctx context.Context) (*types.GraphStats, error) {
	gql := "RETURN db.stats() AS stats"
	resp, err := c.Gql(ctx, gql, nil)
	if err != nil {
		return nil, err
	}
	return parseGraphStats(resp)
}

// Test sends a ping to the server and returns the latency in nanoseconds.
// This is a convenience alias for Ping.
func (c *Client) Test(ctx context.Context) (int64, error) {
	return c.Ping(ctx)
}

// =============================================================================
// Internal Helpers — Parsing Response Rows
// =============================================================================

// findColumnIndex returns the index of a column by name, or -1 if not found.
func findColumnIndex(resp *Response, name string) int {
	for i, col := range resp.Columns {
		if strings.EqualFold(col, name) {
			return i
		}
	}
	return -1
}

// getStringVal safely extracts a string from a row at the given column index.
func getStringVal(row *Row, idx int) string {
	if idx < 0 || idx >= len(row.Values) {
		return ""
	}
	val, err := row.GetString(idx)
	if err != nil {
		return ""
	}
	return val
}

// getInt64Val safely extracts an int64 from a row at the given column index.
func getInt64Val(row *Row, idx int) int64 {
	if idx < 0 || idx >= len(row.Values) {
		return 0
	}
	val, err := row.GetInt(idx)
	if err != nil {
		return 0
	}
	return val
}

func parseLabelInfoRows(resp *Response) ([]types.LabelInfo, error) {
	if resp == nil || len(resp.Rows) == 0 {
		return nil, nil
	}

	labelIdx := findColumnIndex(resp, "label")
	typeIdx := findColumnIndex(resp, "type")

	result := make([]types.LabelInfo, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		info := types.LabelInfo{
			Name: getStringVal(row, labelIdx),
			Type: getStringVal(row, typeIdx),
		}
		result = append(result, info)
	}
	return result, nil
}

func parseNodeTypeInfoRows(resp *Response) ([]types.NodeTypeInfo, error) {
	if resp == nil || len(resp.Rows) == 0 {
		return nil, nil
	}

	typeIdx := findColumnIndex(resp, "type")
	nameIdx := findColumnIndex(resp, "name")
	propsIdx := findColumnIndex(resp, "properties")

	// Use "type" column if "name" is not present
	if nameIdx < 0 {
		nameIdx = typeIdx
	}

	result := make([]types.NodeTypeInfo, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		info := types.NodeTypeInfo{
			Name:       getStringVal(row, nameIdx),
			Properties: types.ParsePropertyString(getStringVal(row, propsIdx)),
		}
		result = append(result, info)
	}
	return result, nil
}

func parseEdgeTypeInfoRows(resp *Response) ([]types.EdgeTypeInfo, error) {
	if resp == nil || len(resp.Rows) == 0 {
		return nil, nil
	}

	typeIdx := findColumnIndex(resp, "type")
	nameIdx := findColumnIndex(resp, "name")
	propsIdx := findColumnIndex(resp, "properties")

	if nameIdx < 0 {
		nameIdx = typeIdx
	}

	result := make([]types.EdgeTypeInfo, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		info := types.EdgeTypeInfo{
			Name:       getStringVal(row, nameIdx),
			Properties: types.ParsePropertyString(getStringVal(row, propsIdx)),
		}
		result = append(result, info)
	}
	return result, nil
}

func parseAlgoInfoRows(resp *Response) ([]types.AlgoInfo, error) {
	if resp == nil || len(resp.Rows) == 0 {
		return nil, nil
	}

	nameIdx := findColumnIndex(resp, "name")
	descIdx := findColumnIndex(resp, "description")
	verIdx := findColumnIndex(resp, "version")
	paramsIdx := findColumnIndex(resp, "parameters")

	result := make([]types.AlgoInfo, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		info := types.AlgoInfo{
			Name:        getStringVal(row, nameIdx),
			Description: getStringVal(row, descIdx),
			Version:     getStringVal(row, verIdx),
			Parameters:  getStringVal(row, paramsIdx),
		}
		result = append(result, info)
	}
	return result, nil
}

func parseIndexInfoRows(resp *Response) ([]types.IndexInfo, error) {
	if resp == nil || len(resp.Rows) == 0 {
		return nil, nil
	}

	idxName := findColumnIndex(resp, "index_name")
	idxEntity := findColumnIndex(resp, "entity_type")
	idxLabel := findColumnIndex(resp, "label")
	idxProp := findColumnIndex(resp, "property")
	idxPrefix := findColumnIndex(resp, "prefix_length")
	idxStatus := findColumnIndex(resp, "status")
	idxProgress := findColumnIndex(resp, "progress")
	idxIndexed := findColumnIndex(resp, "indexed_count")
	idxTotal := findColumnIndex(resp, "total_count")
	idxError := findColumnIndex(resp, "error")

	result := make([]types.IndexInfo, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		info := types.IndexInfo{
			IndexName:    getStringVal(row, idxName),
			EntityType:   getStringVal(row, idxEntity),
			Label:        getStringVal(row, idxLabel),
			Property:     getStringVal(row, idxProp),
			Status:       getStringVal(row, idxStatus),
			Progress:     getStringVal(row, idxProgress),
			IndexedCount: getInt64Val(row, idxIndexed),
			TotalCount:   getInt64Val(row, idxTotal),
			Error:        getStringVal(row, idxError),
		}
		if idxPrefix >= 0 {
			prefixVal := int(getInt64Val(row, idxPrefix))
			info.PrefixLength = &prefixVal
		}
		result = append(result, info)
	}
	return result, nil
}

func parseFulltextInfoRows(resp *Response) ([]types.FulltextInfo, error) {
	if resp == nil || len(resp.Rows) == 0 {
		return nil, nil
	}

	idxName := findColumnIndex(resp, "index_name")
	idxEntity := findColumnIndex(resp, "entity_type")
	idxSchema := findColumnIndex(resp, "schema_name")
	idxProps := findColumnIndex(resp, "properties")
	idxAnalyzer := findColumnIndex(resp, "analyzer")
	idxStatus := findColumnIndex(resp, "status")
	idxDocCount := findColumnIndex(resp, "doc_count")
	idxProgress := findColumnIndex(resp, "progress")

	result := make([]types.FulltextInfo, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		info := types.FulltextInfo{
			IndexName:  getStringVal(row, idxName),
			EntityType: getStringVal(row, idxEntity),
			SchemaName: getStringVal(row, idxSchema),
			Properties: getStringVal(row, idxProps),
			Analyzer:   getStringVal(row, idxAnalyzer),
			Status:     getStringVal(row, idxStatus),
			DocCount:   getInt64Val(row, idxDocCount),
			Progress:   getStringVal(row, idxProgress),
		}
		result = append(result, info)
	}
	return result, nil
}

func parseTaskInfoRows(resp *Response) ([]types.TaskInfo, error) {
	if resp == nil || len(resp.Rows) == 0 {
		return nil, nil
	}

	idxId := findColumnIndex(resp, "task_id")
	idxType := findColumnIndex(resp, "type")
	idxQuery := findColumnIndex(resp, "query")
	idxAlgo := findColumnIndex(resp, "algo_name")
	idxStatus := findColumnIndex(resp, "status")
	idxStarted := findColumnIndex(resp, "started_at")
	idxProgress := findColumnIndex(resp, "progress")
	idxParams := findColumnIndex(resp, "parameters")
	idxNodes := findColumnIndex(resp, "nodes_written")
	idxCompute := findColumnIndex(resp, "compute_time_ms")
	idxWrite := findColumnIndex(resp, "write_time_ms")

	result := make([]types.TaskInfo, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		info := types.TaskInfo{
			TaskId:        getStringVal(row, idxId),
			Type:          getStringVal(row, idxType),
			Query:         getStringVal(row, idxQuery),
			AlgoName:      getStringVal(row, idxAlgo),
			Status:        getStringVal(row, idxStatus),
			StartedAt:     getStringVal(row, idxStarted),
			Progress:      getStringVal(row, idxProgress),
			Parameters:    getStringVal(row, idxParams),
			NodesWritten:  getInt64Val(row, idxNodes),
			ComputeTimeMs: getInt64Val(row, idxCompute),
			WriteTimeMs:   getInt64Val(row, idxWrite),
		}
		result = append(result, info)
	}
	return result, nil
}

func parseProcessInfoRows(resp *Response) ([]types.ProcessInfo, error) {
	if resp == nil || len(resp.Rows) == 0 {
		return nil, nil
	}

	idxId := findColumnIndex(resp, "query_id")
	idxText := findColumnIndex(resp, "query_text")
	idxStart := findColumnIndex(resp, "start_time")
	idxDuration := findColumnIndex(resp, "duration_ms")
	idxStatus := findColumnIndex(resp, "status")

	result := make([]types.ProcessInfo, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		info := types.ProcessInfo{
			QueryId:    getStringVal(row, idxId),
			QueryText:  getStringVal(row, idxText),
			StartTime:  getStringVal(row, idxStart),
			DurationMs: getInt64Val(row, idxDuration),
			Status:     getStringVal(row, idxStatus),
		}
		result = append(result, info)
	}
	return result, nil
}

func parseGraphStats(resp *Response) (*types.GraphStats, error) {
	if resp == nil || len(resp.Rows) == 0 {
		return &types.GraphStats{}, nil
	}

	// db.stats() returns a single row with a stats column that is a map
	statsIdx := findColumnIndex(resp, "stats")
	if statsIdx < 0 {
		// Try first column if "stats" not found
		statsIdx = 0
	}

	if len(resp.Rows) == 0 || statsIdx >= len(resp.Rows[0].Values) {
		return &types.GraphStats{}, nil
	}

	val, err := resp.Rows[0].Get(statsIdx)
	if err != nil {
		return nil, err
	}

	gs := &types.GraphStats{
		LabelCounts:       make(map[string]int64),
		EdgeLabelCounts:   make(map[string]int64),
		NodePropertyStats: make(map[string]map[string]int64),
		EdgePropertyStats: make(map[string]map[string]int64),
	}

	if m, ok := val.(map[string]interface{}); ok {
		if v, ok := m["graph_name"]; ok {
			gs.GraphName = fmt.Sprintf("%v", v)
		}
		if v, ok := m["node_count"]; ok {
			gs.NodeCount = toInt64(v)
		}
		if v, ok := m["edge_count"]; ok {
			gs.EdgeCount = toInt64(v)
		}
	}

	return gs, nil
}

// =============================================================================
// Internal Helpers — GQL String Building
// =============================================================================

// dbTypeToKeyword converts DBType to GQL keyword "NODE" or "EDGE".
func dbTypeToKeyword(dt types.DBType) string {
	if dt == types.DBTypeEdge {
		return "EDGE"
	}
	return "NODE"
}

// buildPropertyDefString builds a GQL property definition like "{age INT64, name STRING}".
func buildPropertyDefString(props []types.PropertyDef) string {
	if len(props) == 0 {
		return "{}"
	}
	parts := make([]string, len(props))
	for i, p := range props {
		parts[i] = fmt.Sprintf("%s %s", quoteLabel(p.Name), types.PropertyTypeToGQL(p.Type))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// buildIndexPropertyString builds the property list for CREATE INDEX, e.g. "prop1, prop2(10)".
func buildIndexPropertyString(props []types.IndexProperty) string {
	parts := make([]string, len(props))
	for i, p := range props {
		if p.PrefixLength > 0 {
			parts[i] = fmt.Sprintf("%s(%d)", quoteLabel(p.Name), p.PrefixLength)
		} else {
			parts[i] = quoteLabel(p.Name)
		}
	}
	return strings.Join(parts, ", ")
}

// buildPropertiesValueString builds a GQL property value map like "{name: 'Alice', age: 30}".
func buildPropertiesValueString(props map[string]interface{}) string {
	if len(props) == 0 {
		return ""
	}
	parts := make([]string, 0, len(props))
	for k, v := range props {
		parts = append(parts, fmt.Sprintf("%s: %s", k, formatGqlValue(v)))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// formatGqlValue formats a Go value as a GQL literal.
func formatGqlValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case string:
		// Escape single quotes in strings
		escaped := strings.ReplaceAll(val, "'", "\\'")
		return "'" + escaped + "'"
	case bool:
		if val {
			return "TRUE"
		}
		return "FALSE"
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32:
		return fmt.Sprintf("%g", val)
	case float64:
		return fmt.Sprintf("%g", val)
	default:
		return fmt.Sprintf("'%v'", v)
	}
}

// toInt64 converts an interface value to int64.
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int32:
		return int64(val)
	case int:
		return int64(val)
	case uint64:
		return int64(val)
	case uint32:
		return int64(val)
	case float64:
		return int64(val)
	case float32:
		return int64(val)
	default:
		return 0
	}
}
