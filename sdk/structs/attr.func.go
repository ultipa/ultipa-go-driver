package structs

import (
	"errors"
	"fmt"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
)

// ListAttrAsAttrNodes returns AttrNodes, if PropertyType of attr is LIST and inner result type is Node
func (attr *Attr) ListAttrAsAttrNodes() (*AttrNodes, error) {
	if ultipa.PropertyType_LIST != attr.PropertyType {
		return nil, errors.New(fmt.Sprintf("value of this %v is not a LIST type", attr.Name))
	}
	var result *AttrNodes
	if attr.Values == nil {
		return result, nil
	}

	result = NewAttrNodes()
	if len(attr.Values) == 0 {
		result.NodesList = [][]*Node{}
		return result, nil
	}

	if ultipa.ResultType_RESULT_TYPE_NODE != attr.ResultType {
		return nil, errors.New(fmt.Sprintf("value of this %v is not a LIST Node type", attr.Name))
	}
	result.NodesList = [][]*Node{}
	for _, row := range attr.Values {
		attrListData := row.(*AttrListData)
		result.NodesList = append(result.NodesList, attrListData.Nodes)
	}
	return result, nil
}

// ListAttrAsAttrEdges returns AttrEdges, if PropertyType of attr is LIST and inner result type is Edge
func (attr *Attr) ListAttrAsAttrEdges() (*AttrEdges, error) {
	if ultipa.PropertyType_LIST != attr.PropertyType {
		return nil, errors.New(fmt.Sprintf("value of this %v is not a LIST type", attr.Name))
	}
	var result *AttrEdges
	if attr.Values == nil {
		return result, nil
	}

	result = NewAttrEdges()
	if len(attr.Values) == 0 {
		result.EdgesList = [][]*Edge{}
		return result, nil
	}

	if ultipa.ResultType_RESULT_TYPE_EDGE != attr.ResultType {
		return nil, errors.New(fmt.Sprintf("value of this %v is not a LIST Edge type", attr.Name))
	}

	result.EdgesList = [][]*Edge{}
	for _, row := range attr.Values {
		attrListData := row.(*AttrListData)
		result.EdgesList = append(result.EdgesList, attrListData.Edges)
	}
	return result, nil
}

// ListAttrAsAttrPaths returns AttrPaths, if PropertyType of attr is LIST and inner result type is Path
func (attr *Attr) ListAttrAsAttrPaths() (*AttrPaths, error) {
	if ultipa.PropertyType_LIST != attr.PropertyType {
		return nil, errors.New(fmt.Sprintf("value of this %v is not a LIST type", attr.Name))
	}
	var result *AttrPaths
	if attr.Values == nil {
		return result, nil
	}

	result = NewAttrPaths()
	if len(attr.Values) == 0 {
		result.PathsList = [][]*Path{}
		return result, nil
	}

	if ultipa.ResultType_RESULT_TYPE_PATH != attr.ResultType {
		return nil, errors.New(fmt.Sprintf("value of this %v is not a LIST Path type", attr.Name))
	}
	result.PathsList = [][]*Path{}
	result.ResultType = attr.ResultType
	for _, row := range attr.Values {
		attrListData := row.(*AttrListData)
		result.PathsList = append(result.PathsList, attrListData.Paths)
	}
	return result, nil

}

// ListAttrAsAttr returns an attr, if PropertyType of attr is LIST/SET and inner result type is Attr.
// inner result type is attr, then regarded it as basic type, e.g. string,uint64, float64 etc.
func (attr *Attr) ListAttrAsAttr() (*Attr, error) {
	if ultipa.PropertyType_LIST != attr.PropertyType && ultipa.PropertyType_SET != attr.PropertyType {
		return nil, errors.New(fmt.Sprintf("value of this %v is not a LIST or SET type", attr.Name))
	}
	result := NewAttr()
	result.Name = attr.Name
	if attr.Values == nil {
		result.Values = nil
		return result, nil
	}
	if len(attr.Values) == 0 {
		return result, nil
	}
	var newAttr *Attr
	switch attr.ResultType {
	case ultipa.ResultType_RESULT_TYPE_ATTR:
		newAttr = attr
	case ultipa.ResultType_RESULT_TYPE_PATH:
		newAttr = NewAttr()
		newAttr.ResultType = attr.ResultType
		newAttr.Name = attr.Name
		newAttr.PropertyType = attr.PropertyType
		attrPaths, err := attr.ListAttrAsAttrPaths()
		if err != nil {
			return nil, err
		}
		newAttr.Values = append(newAttr.Values, attrPaths)

	case ultipa.ResultType_RESULT_TYPE_NODE:
		newAttr = NewAttr()
		newAttr.ResultType = attr.ResultType
		newAttr.Name = attr.Name
		newAttr.PropertyType = attr.PropertyType
		attrNodes, err := attr.ListAttrAsAttrNodes()
		if err != nil {
			return nil, err
		}
		newAttr.Values = append(newAttr.Values, attrNodes)
	case ultipa.ResultType_RESULT_TYPE_EDGE:
		newAttr = NewAttr()
		newAttr.ResultType = attr.ResultType
		newAttr.Name = attr.Name
		newAttr.PropertyType = attr.PropertyType
		attrEdges, err := attr.ListAttrAsAttrEdges()
		if err != nil {
			return nil, err
		}
		newAttr.Values = append(newAttr.Values, attrEdges)
	}

	return newAttr, nil
}

// @depreacated
func (attr *Attr) parseAttrOfAttrListDataToInterface() []interface{} {
	var result []interface{}
	switch attr.PropertyType {
	case ultipa.PropertyType_LIST:
		var subAttrs []*Attr

		for _, row := range attr.Values {
			eachAttrListData := row.(*AttrListData)
			subAttrs = append(subAttrs, eachAttrListData.Attrs...)
		}
		for _, subAttr := range subAttrs {
			subResult := subAttr.parseAttrOfAttrListDataToInterface()
			result = append(result, subResult...)
		}
	case ultipa.PropertyType_SET:
	case ultipa.PropertyType_MAP:
	case ultipa.PropertyType_NULL_:
		result = append(result, nil)
	default:
		//non-collection type, return List<>
		result = append(result, attr.Values...)
	}
	return result
}

// detectListAttrInnerResultType detects inner result type of Attr with List property type.
func (attr *Attr) detectListAttrInnerResultType() ultipa.ResultType {
	for _, row := range attr.Values {
		toDetectTypeAttrListData := row.(*AttrListData)
		if toDetectTypeAttrListData.ResultType != ultipa.ResultType_RESULT_TYPE_UNSET {
			return toDetectTypeAttrListData.ResultType
		}
	}
	return ultipa.ResultType_RESULT_TYPE_UNSET
}
