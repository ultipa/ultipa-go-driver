package http

import (
	"reflect"
	"strconv"

	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
)

// Statistic Store the returned statistical information
type Statistic struct {
	NodeAffected int `key:"node_affected" type:"int"`
	EdgeAffected int `key:"edge_affected" type:"int"`
	TotalCost    int `key:"total_time_cost" type:"int"`
	EngineCost   int `key:"engine_time_cost" type:"int"`
}

func ParseStatistic(table *ultipa.Table) (*Statistic, error) {

	stat := Statistic{}

	if table == nil {
		return &stat, nil
	}

	kv := map[string]string{}

	for index, header := range table.Headers {
		// fix error: server return table Header more than 4 ,for example: len(header)= 8，TableRows==2
		if index >= 4 {
			break
		}

		key := header.PropertyName

		value := table.TableRows[0].Values[index]
		kv[key] = string(value)
	}

	statReflect := reflect.TypeOf(stat)
	statReflectValues := reflect.ValueOf(&stat)

	for i := 0; i < statReflect.NumField(); i++ {
		field := statReflect.Field(i)
		fieldValue := statReflectValues.Elem().Field(i)

		key := field.Tag.Get("key")
		t := field.Tag.Get("type")

		v := kv[key]

		if v == "" {
			continue
		}

		switch t {
		case "int":
			vv, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return &stat, err
			}
			fieldValue.SetInt(vv)
		}
	}

	return &stat, nil
}
