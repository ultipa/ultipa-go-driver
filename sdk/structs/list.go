package structs

import ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"

type List struct {
	BaseType ultipa.PropertyType
	Values   []*ListValue
}

type ListValue struct {
	Type  ultipa.PropertyType
	Value interface{}
}

type ListData struct {
	Values []interface{}
}
