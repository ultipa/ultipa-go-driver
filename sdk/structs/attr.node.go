package structs

import ultipa "github.com/ultipa/ultipa-go-driver/rpc"

// AttrNodes represents an Attr with Values that is List<List<Node>>
type AttrNodes struct {
	Name       string
	ResultType ultipa.ResultType
	NodesList  [][]*Node
}

func NewAttrNodes() *AttrNodes {
	return &AttrNodes{}
}
