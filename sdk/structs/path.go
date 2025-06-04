package structs

import "github.com/ultipa/ultipa-go-driver/sdk/types"

type Path struct {
	//Name        string
	NodeUUIDs []types.UUID
	EdgeUUIDs []types.UUID
	Nodes     map[types.UUID]*Node
	Edges     map[types.UUID]*Edge
	//NodeSchemas map[string]*Schema
	//EdgeSchemas map[string]*Schema
}

//func NewPath() *Path {
//    return &Path{
//        NodeSchemas: map[string]*Schema{},
//        EdgeSchemas: map[string]*Schema{},
//    }
//}

func (p *Path) GetNodes() []types.UUID {
	return p.NodeUUIDs
}

func (p *Path) GetLastNode() types.UUID {
	return p.NodeUUIDs[len(p.NodeUUIDs)-1]
}

func (p *Path) GetEdges() []types.UUID {
	return p.EdgeUUIDs
}

func (p *Path) Length() int {
	return len(p.EdgeUUIDs)
}
