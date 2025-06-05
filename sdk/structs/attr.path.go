package structs

import ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"

// AttrPaths represents an Attr with Values that is List<List<Path>>
type AttrPaths struct {
	Name       string
	ResultType ultipa.ResultType
	PathsList  [][]*Path
}

func NewAttrPaths() *AttrPaths {
	return &AttrPaths{}
}
