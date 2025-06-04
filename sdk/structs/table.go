package structs

import ultipa "github.com/ultipa/ultipa-go-driver/rpc"

type Header struct {
	Name         string
	PropertyType ultipa.PropertyType
}

type Table struct {
	Name    string
	Headers []*Header
	Rows    []*Value
}

type Value []interface{}

func NewTable() *Table {
	return &Table{
		Headers: []*Header{},
		Rows:    []*Value{},
	}
}

func (t *Table) GetHeaders() []*Header {
	return t.Headers
}

func (t *Table) GetRows() []*Value {
	return t.Rows
}

func (t *Table) ToKV() []*Values {

	var values []*Values

	for _, row := range t.GetRows() {
		v := NewValues()
		for i, header := range t.GetHeaders() {
			f := (*row)[i]
			v.Set(header.Name, f)
		}

		values = append(values, v)
	}

	return values
}
