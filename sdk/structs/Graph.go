package structs

import "github.com/ultipa/ultipa-go-driver/sdk/types"

type Graph struct {
	Paths []*Path
	//Name        string
	Nodes map[types.UUID]*Node
	Edges map[types.UUID]*Edge
	//NodeSchemas map[string]*Schema
	//EdgeSchemas map[string]*Schema
}

func NewGraph() *Graph {
	return &Graph{
		Nodes: map[types.UUID]*Node{},
		Edges: map[types.UUID]*Edge{},
	}
}

//func (p *Graph) GetNodes(uuid types.UUID) *Node {
//   return p.Nodes[uuid]
//}

//func (p *Graph) GetLastNode() *Node {
//   return p.Nodes[p.Length()]
//}

//func (p *Graph) GetEdges(uuid types.UUID) *Edge {
//   return p.Edges[uuid]
//}
//
//func (p *Graph) Length() int {
//   return len(p.Edges)
//}

func (p *Graph) GetPath() []*Path {
	return p.Paths
}

func (p *Graph) AddNodes(node *Node) {
	if node == nil {
		return
	}
	p.Nodes[node.UUID] = node
}

func (p *Graph) AddEdges(edge *Edge) {
	if edge == nil {
		return
	}
	p.Edges[edge.UUID] = edge
}
