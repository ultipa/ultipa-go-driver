package api

import (
	"errors"
	"fmt"
	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
	"strings"
)

func (api *UltipaAPI) ShowSchema(config *configuration.RequestConfig) (schemas []*structs.Schema, err error) {
	var resp *http.Response

	resp, err = api.Uql(fmt.Sprintf(`show().schema()`), config)
	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	nodeSchemas, err := resp.Alias(http.RESP_NODE_SCHEMA_KEY).AsSchemas()
	if err != nil {
		return nil, err
	}
	edgesSchemas, err := resp.Alias(http.RESP_EDGE_SCHEMA_KEY).AsSchemas()
	if err != nil {
		return nil, err
	}

	schemas = append(nodeSchemas, edgesSchemas...)

	graphCount, err := resp.Alias(http.RESP_GRAPH_COUNT_KEY).AsGraphCount()
	if err != nil {
		return nil, err
	}
	if graphCount == nil {
		return schemas, nil
	}
	for _, schema := range schemas {
		schema.SetStatsByGraphCount(graphCount)
	}

	if len(schemas) == 0 {
		return nil, fmt.Errorf("no data return")
	}

	return schemas, err
}

func (api *UltipaAPI) ShowNodeSchema(config *configuration.RequestConfig) ([]*structs.Schema, error) {
	var nodeSchemas []*structs.Schema
	schemas, err := api.ShowSchema(config)
	if err != nil {
		return nil, err
	}

	for _, schema := range schemas {
		if schema.DBType == ultipa.DBType_DBNODE {
			nodeSchemas = append(nodeSchemas, schema)
		}
	}

	//if len(nodeSchemas) == 0 {
	//	return nil, fmt.Errorf("no data return")
	//}

	return nodeSchemas, err
}

func (api *UltipaAPI) ShowEdgeSchema(config *configuration.RequestConfig) ([]*structs.Schema, error) {
	var nodeSchemas []*structs.Schema
	schemas, err := api.ShowSchema(config)
	if err != nil {
		return nil, err
	}

	for _, schema := range schemas {
		if schema.DBType == ultipa.DBType_DBEDGE {
			nodeSchemas = append(nodeSchemas, schema)
		}
	}

	//if len(nodeSchemas) == 0 {
	//	return nil, fmt.Errorf("no data return")
	//}

	return nodeSchemas, err
}

func (api *UltipaAPI) GetSchema(schemaName string, dbType ultipa.DBType, config *configuration.RequestConfig) (*structs.Schema, error) {
	if !(dbType == ultipa.DBType_DBNODE || dbType == ultipa.DBType_DBEDGE) {
		return nil, errors.New("DBType must be DBType_DBNODE or DBType_DBEDGE")
	}

	schemas, err := api.ShowSchema(config)
	if err != nil {
		return nil, err
	}

	for _, schema := range schemas {
		if schemaName == schema.Name && schema.DBType == dbType {
			return schema, nil
		}
	}

	//return nil, fmt.Errorf("schema [%s] not exist in db", schemaName)
	return nil, nil
}

func (api *UltipaAPI) GetNodeSchema(schemaName string, config *configuration.RequestConfig) (*structs.Schema, error) {
	return api.GetSchema(schemaName, ultipa.DBType_DBNODE, config)
}

func (api *UltipaAPI) GetEdgeSchema(schemaName string, config *configuration.RequestConfig) (*structs.Schema, error) {
	return api.GetSchema(schemaName, ultipa.DBType_DBEDGE, config)
}

func (api *UltipaAPI) CreateSchema(schema *structs.Schema, isCreateProperties bool, config *configuration.RequestConfig) (*http.Response, error) {
	if schema == nil {
		return nil, fmt.Errorf("CreateSchema: schema can not nil")
	}
	if schema.Name == "" {
		return nil, fmt.Errorf("schemaName can not empty %s", schema.Name)
	}

	schemaName, err := CheckReplaceSchemaPropertyName(schema.Name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, schemaName = %s", err.Error(), schema.Name))
	}

	var resp *http.Response

	//api.Logger.Log("Creating Schema : @" + schemaName)

	uql := ""
	if schema.DBType == ultipa.DBType_DBNODE {
		uql = fmt.Sprintf(`create().node_schema(%v,"%v")`, schemaName, schema.Description)
	} else if schema.DBType == ultipa.DBType_DBEDGE {
		uql = fmt.Sprintf(`create().edge_schema(%v,"%v")`, schemaName, schema.Description)
	} else {
		return nil, errors.New("DBType must be DBType_DBNODE or DBType_DBEDGE")
	}

	resp, err = api.Uql(uql, config)
	if err != nil {
		return nil, err
	}

	//api.Logger.Log("Created Schema : @" + schemaName)
	// create property of schemas
	if isCreateProperties {

		for _, prop := range schema.Properties {

			if prop.IsIDType() || prop.IsIgnore() {
				continue
			}

			prop.Schema = schema.Name
			//if prop.Schema == "" || prop.Schema != schema.Name {
			//    prop.Schema = schema.Name
			//}

			resp, err := api.CreateProperty(schema.DBType, prop, config)

			if err != nil {
				return nil, err
			}

			if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
				return resp, nil
			}
		}

	}

	return resp, err
}

func (api *UltipaAPI) CreateSchemaIfNotExist(schema *structs.Schema, isCreateProperties bool, config *configuration.RequestConfig) (rwc http.ResponseWithExistCheck, err error) {
	s, err := api.GetSchema(schema.Name, schema.DBType, config)
	if err != nil {
		return rwc, fmt.Errorf("GetSchema error, %v", err)
	}

	if s == nil {
		_, err = api.CreateSchema(schema, isCreateProperties, config)
	} else {
		rwc.Exist = true
	}

	return rwc, err
}

func (api *UltipaAPI) DropSchema(schema *structs.Schema, config *configuration.RequestConfig) (*http.JobResponse, error) {
	if schema == nil {
		return nil, errors.New("drop schema: schema can not be nil")
	}

	if schema.Name == "" {
		return nil, errors.New("drop schema: schema name can not be empty")
	}

	schemaName, err := CheckReplaceSchemaPropertyName(schema.Name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, schemaName = %s", err.Error(), schema.Name))
	}

	uql := ""
	switch schema.DBType {
	case ultipa.DBType_DBNODE:
		uql = fmt.Sprintf(`drop().node_schema(@%s)`, schemaName)
	case ultipa.DBType_DBEDGE:
		uql = fmt.Sprintf(`drop().edge_schema(@%s)`, schemaName)
	default:
		return nil, errors.New("DBType must be DBType_DBNODE or DBType_DBEDGE")
	}

	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return http.GetJobResponseFromUqlResponse(resp)
}

func (api *UltipaAPI) AlterSchema(originalSchema, newSchema *structs.Schema, config *configuration.RequestConfig) (*http.Response, error) {
	schemaName, err := CheckReplaceSchemaPropertyName(originalSchema.Name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, schemaName = %s", err.Error(), originalSchema.Name))
	}

	err = CheckName(newSchema.Name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, schemaName = %s", err.Error(), newSchema.Name))
	}

	parms := ""
	switch originalSchema.DBType {
	case ultipa.DBType_DBNODE:
		parms = "node_schema"
	case ultipa.DBType_DBEDGE:
		parms = "edge_schema"
	default:
		return nil, errors.New("DBType must be DBType_DBNODE or DBType_DBEDGE")
	}

	uql := fmt.Sprintf(`alter().%s(@%s).set({name: "%s", description: "%s"})`, parms, schemaName, strings.ReplaceAll(newSchema.Name, `"`, `\"`), newSchema.Description)

	// Only modify the description of the originalSchema
	if newSchema.Name == "" {
		uql = fmt.Sprintf(`alter().%s(@%s).set({description: "%s"})`, parms, schemaName, newSchema.Description)
	}

	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
