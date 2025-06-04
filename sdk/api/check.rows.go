/**
 * @Author: zhaohaichao
 * @Description:
 * @File:  check.rows
 * @Date: 2022/8/2 6:43 pm
 */

package api

import (
	"errors"
	"fmt"

	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func CheckValuesAndProperties(properties []*structs.Property, values *structs.Values, index int) (err error) {
	var props []*structs.Property

	for _, prop := range properties {
		if prop.IsIDType() || prop.IsIgnore() {
			continue
		}
		props = append(props, prop)
	}

	if props != nil {
		if len(props) > 0 && values == nil {
			err = errors.New(fmt.Sprintf("row [%d] error: values are empty but properties size >0.", index))
			return err
		}
		if values != nil {
			if len(values.Data) > len(props) {
				err = errors.New(fmt.Sprintf("row [%d] error: values size larger than properties size.", index))
				return err
			}
			if len(values.Data) < len(props) {
				err = errors.New(fmt.Sprintf("row [%d] error: values size smaller than properties size.", index))
				return err
			}
			for _, prop := range props {
				if _, ok := values.Data[prop.Name]; !ok {
					return errors.New(fmt.Sprintf("row [%d] error: values doesn't contain property [%s].", index, prop.Name))
				}
			}
		}
	} else {
		if values != nil && len(values.Data) > 0 {
			err = errors.New(fmt.Sprintf("row [%d] error: properties are empty but values size >0.", index))
			return err
		}
	}

	return err
}

func CheckEdgeRows(edge *structs.Edge, properties []*structs.Property, index int) (err error) {
	err = CheckValuesAndProperties(properties, edge.Values, index)
	if err != nil {
		return err
	}

	hasFrom := edge.GetFrom() != "" && edge.GetFrom() != "nil" && edge.GetFrom() != "null"
	hasTo := edge.GetTo() != "" && edge.GetTo() != "nil" && edge.GetTo() != "null"
	hasFromUuid := edge.FromUUID != 0
	hasToUuid := edge.ToUUID != 0

	if !hasFrom && !hasTo && !hasFromUuid && !hasToUuid {
		err = errors.New(fmt.Sprintf("row [%d] error: _from/_from_uuid and _to/_to_uuid are null.", index))
		return err
	}
	if hasFrom && !hasTo {
		err = errors.New(fmt.Sprintf("row [%d] error: _from has value [%s] but _to got null", index, edge.GetFrom()))
		return err
	}
	if hasTo && !hasFrom {
		err = errors.New(fmt.Sprintf("row [%d] error: _to has value [%s] but _from got null", index, edge.GetTo()))
		return err
	}
	if hasFromUuid && !hasToUuid {
		err = errors.New(fmt.Sprintf("row [%d] error: _from_uuid has value [%v] but _to_uuid got null", index, edge.FromUUID))
		return err
	}
	if !hasFromUuid && hasToUuid {
		err = errors.New(fmt.Sprintf("row [%d] error: _to_uuid has value [%v] but _from_uuid got null", index, edge.ToUUID))
		return err
	}
	return err
}
