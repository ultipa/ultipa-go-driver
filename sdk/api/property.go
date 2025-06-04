package api

import (
	"errors"
	"fmt"

	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

// CreateProperty property.Schema is required
func (api *UltipaAPI) CreateProperty(dbType ultipa.DBType, property *structs.Property, config *configuration.RequestConfig) (resp *http.Response, err error) {
	if property == nil {
		return nil, errors.New("create property: property can not be nil")
	}

	if property.Schema == "" {
		return nil, fmt.Errorf("property.Schema can not empty")
	}

	if property.Name == "" {
		return nil, fmt.Errorf("property.Name can not empty")
	}

	if property.Type == ultipa.PropertyType_UNSET {
		return nil, fmt.Errorf("property.Type unset")
	}

	params := ""
	switch dbType {
	case ultipa.DBType_DBNODE:
		params = "node_property"
	case ultipa.DBType_DBEDGE:
		params = "edge_property"
	default:
		errStr := fmt.Sprintf("DBType must be DBType_DBNODE or DBType_DBEDGE")
		return nil, errors.New(errStr)
	}

	if property.Type == structs.PropertyType_IGNORE {
		return nil, err
	}

	propertyTypeStr, err := property.GetStringType()
	if err != nil {
		return nil, err
	}

	schemaName, err := CheckReplaceSchemaPropertyName(property.Schema)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, schemaName = %s", err.Error(), schemaName))
	}

	propertyName, err := CheckReplaceSchemaPropertyName(property.Name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, propertyName = %s", err.Error(), property.Name))
	}

	//api.Logger.Log("Creating Property : @" + schemaName + "." + propertyName)
	uql := fmt.Sprintf(`create().%v(@%v,%s,"%v","%v")`, params, schemaName, propertyName, propertyTypeStr, property.Description)

	switch property.Encrypt {
	case "AES128", "AES256", "RSA", "ECC":
		uql = fmt.Sprintf(`%s.encrypt("%s")`, uql, property.Encrypt)
	case "":
	default:
		return nil, errors.New(fmt.Sprintf("invalid property encrypt: %s", property.Encrypt))
	}

	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	//api.Logger.Log("Created Property : @" + schemaName + "." + propertyName)
	return resp, nil
}

func (api *UltipaAPI) CreatePropertyIfNotExist(dbType ultipa.DBType, property *structs.Property, config *configuration.RequestConfig) (rwc http.ResponseWithExistCheck, err error) {
	prop, err := api.GetProperty(dbType, property.Schema, property.Name, config)

	if err != nil {
		return rwc, err
	}

	if prop == nil {
		rwc.Response, err = api.CreateProperty(dbType, property, config)
		if err != nil {
			return rwc, err
		}

		return rwc, nil
	}

	rwc.Exist = true
	return rwc, nil
}

func (api *UltipaAPI) GetProperty(dbType ultipa.DBType, schemaName string, propertyName string, config *configuration.RequestConfig) (property *structs.Property, err error) {
	schema, err := api.GetSchema(schemaName, dbType, config)

	if err != nil {
		return nil, err
	}

	if schema == nil {
		return nil, nil
	}

	for _, prop := range schema.Properties {
		if prop.Name == propertyName {
			return prop, nil
		}
	}

	//return nil, fmt.Errorf("property %s not found in schema %s", propertyName, schemaName)
	return nil, nil
}

func (api *UltipaAPI) ShowProperty(dbType ultipa.DBType, schemaName string, config *configuration.RequestConfig) (nodeProperty, edgeProperty []*structs.Property, err error) {
	//resp, err := api.Uql("show().property()",config)
	////
	//if err != nil {
	//    return nil, nil, err
	//}
	//
	//nodeProperty, err = resp.Alias(http.RESP_NODE_PROPERTY_KEY).AsProperties()
	//if err != nil {
	//    return nil, nil, err
	//}
	//edgeProperty, err = resp.Alias(http.RESP_EDGE_PROPERTY_KEY).AsProperties()
	//if err != nil {
	//    return nil, nil, err
	//}
	//
	//return nodeProperty, edgeProperty, nil

	switch dbType {
	case ultipa.DBType_DBNODE:
		nodeProperty, err = api.ShowNodeProperty(schemaName, config)
	case ultipa.DBType_DBEDGE:
		edgeProperty, err = api.ShowEdgeProperty(schemaName, config)
	default:
		nodeProperty, err = api.ShowNodeProperty(schemaName, config)
		edgeProperty, err = api.ShowEdgeProperty(schemaName, config)
	}
	return
}

func (api *UltipaAPI) ShowNodeProperty(schemaName string, config *configuration.RequestConfig) (property []*structs.Property, err error) {
	schemaName, err = CheckReplaceSchemaPropertyName(schemaName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, schemaName = %s", err.Error(), schemaName))
	}

	if schemaName == "" || schemaName == "*" {
		schemaName = ""
	} else {
		schemaName = "@" + schemaName
	}
	uql := fmt.Sprintf("show().node_property(%s)", schemaName)
	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	property, err = resp.Alias(http.RESP_NODE_PROPERTY_KEY).AsProperties()
	return property, nil
}

func (api *UltipaAPI) ShowEdgeProperty(schemaName string, config *configuration.RequestConfig) (property []*structs.Property, err error) {
	schemaName, err = CheckReplaceSchemaPropertyName(schemaName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, schemaName = %s", err.Error(), schemaName))
	}

	if schemaName == "" || schemaName == "*" {
		schemaName = ""
	} else {
		schemaName = "@" + schemaName
	}
	uql := fmt.Sprintf("show().edge_property(%s)", schemaName)
	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	property, err = resp.Alias(http.RESP_EDGE_PROPERTY_KEY).AsProperties()
	return property, nil
}

func (api *UltipaAPI) GetNodeProperty(schemaName string, propertyName string, config *configuration.RequestConfig) (property *structs.Property, err error) {
	return api.GetProperty(ultipa.DBType_DBNODE, schemaName, propertyName, config)
}

func (api *UltipaAPI) GetEdgeProperty(schemaName string, propertyName string, config *configuration.RequestConfig) (property *structs.Property, err error) {
	return api.GetProperty(ultipa.DBType_DBEDGE, schemaName, propertyName, config)
}

func (api *UltipaAPI) CreateNodeProperty(property *structs.Property, config *configuration.RequestConfig) (resp *http.Response, err error) {
	return api.CreateProperty(ultipa.DBType_DBNODE, property, config)
}

func (api *UltipaAPI) CreateEdgeProperty(property *structs.Property, config *configuration.RequestConfig) (resp *http.Response, err error) {
	return api.CreateProperty(ultipa.DBType_DBEDGE, property, config)
}

func (api *UltipaAPI) AlterNodeProperty(originProp, newProp *structs.Property, config *configuration.RequestConfig) (resp *http.Response, err error) {
	return api.AlterProperty(ultipa.DBType_DBNODE, originProp, newProp, config)
}

func (api *UltipaAPI) AlterEdgeProperty(originProp, newProp *structs.Property, config *configuration.RequestConfig) (resp *http.Response, err error) {
	return api.AlterProperty(ultipa.DBType_DBEDGE, originProp, newProp, config)
}

func (api *UltipaAPI) DropNodeProperty(property *structs.Property, config *configuration.RequestConfig) (resp *http.Response, err error) {
	return api.DropProperty(ultipa.DBType_DBNODE, property, config)
}

func (api *UltipaAPI) DropEdgeProperty(property *structs.Property, config *configuration.RequestConfig) (resp *http.Response, err error) {
	return api.DropProperty(ultipa.DBType_DBEDGE, property, config)
}

func (api *UltipaAPI) DropProperty(dbType ultipa.DBType, property *structs.Property, config *configuration.RequestConfig) (resp *http.Response, err error) {
	if property.Schema == "" {
		return nil, fmt.Errorf("property.Schema can not empty")
	}
	if property.Name == "" {
		return nil, fmt.Errorf("property.Name can not empty")
	}
	schemaName, err := CheckReplaceSchemaPropertyName(property.Schema)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, schemaName = %s", err.Error(), schemaName))
	}

	propertyName, err := CheckReplaceSchemaPropertyName(property.Name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, propertyName = %s", err.Error(), propertyName))
	}

	uql := ""
	switch dbType {
	case ultipa.DBType_DBNODE:
		uql = fmt.Sprintf(`drop().node_property(@%v.%v)`, schemaName, propertyName)
	case ultipa.DBType_DBEDGE:
		uql = fmt.Sprintf(`drop().edge_property(@%v.%v)`, schemaName, propertyName)
	default:
		return nil, errors.New("DBType must be DBType_DBNODE or DBType_DBEDGE")
	}

	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (api *UltipaAPI) AlterProperty(dbType ultipa.DBType, originProp, newProp *structs.Property, config *configuration.RequestConfig) (*http.Response, error) {
	if originProp == nil {
		return nil, errors.New("alter property: originProp can not be nil")
	}
	if newProp == nil {
		return nil, errors.New("alter property: newProp can not be nil")
	}

	if originProp.Schema == "" || originProp.Name == "" {
		return nil, errors.New("alter property: originProp Schema/Name can not be empty")
	}

	if newProp.Name == "" && newProp.Description == "" {
		return nil, errors.New("alter property: newProp Name/Description cannot be empty at the same time")
	}

	params := ""
	switch dbType {
	case ultipa.DBType_DBNODE:
		params = "node_property"
	case ultipa.DBType_DBEDGE:
		params = "edge_property"
	default:
		return nil, errors.New("DBType must be DBType_DBNODE or DBType_DBEDGE")
	}

	schemaName, err := CheckReplaceSchemaPropertyName(originProp.Schema)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, schemaName = %s", err.Error(), originProp.Schema))
	}

	propertyName, err := CheckReplaceSchemaPropertyName(originProp.Name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, propertyName = %s", err.Error(), originProp.Name))
	}

	uql := fmt.Sprintf(`alter().%v(@%v.%v).set({name: "%v", description: "%v"})`, params, schemaName, propertyName, newProp.Name, newProp.Description)

	if newProp.Description == "" {
		uql = fmt.Sprintf(`alter().%v(@%v.%v).set({name: "%v"})`, params, schemaName, propertyName, newProp.Name)
	} else if newProp.Name == "" {
		uql = fmt.Sprintf(`alter().%v(@%v.%v).set({description: "%v"})`, params, schemaName, propertyName, newProp.Description)
	}

	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
