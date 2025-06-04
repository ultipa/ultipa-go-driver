package api

import (
	"errors"
	"fmt"

	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
)

func (api *UltipaAPI) Lte(dbType ultipa.DBType, schemaName, propertyName string, config *configuration.RequestConfig) (jobResponse *http.JobResponse, err error) {
	uql := ""

	if schemaName == "" {
		schemaName = "*"
	}
	if propertyName == "" {
		return nil, errors.New("lte: propertyName can not be empty")
	}

	schemaName, err = CheckReplaceSchemaPropertyName(schemaName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, schemaName = %s", err.Error(), schemaName))
	}

	propertyName, err = CheckReplaceSchemaPropertyName(propertyName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, propertyName = %s", err.Error(), propertyName))
	}

	switch dbType {
	case ultipa.DBType_DBNODE:
		uql = fmt.Sprintf(`LTE().node_property(@%v.%v)`, schemaName, propertyName)
	case ultipa.DBType_DBEDGE:
		uql = fmt.Sprintf(`LTE().edge_property(@%v.%v)`, schemaName, propertyName)
	default:
		return nil, errors.New("DBType must be DBType_DBNODE or DBType_DBEDGE")
	}

	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return http.GetJobResponseFromUqlResponse(resp)
}

func (api *UltipaAPI) Ufe(dbType ultipa.DBType, schemaName, propertyName string, config *configuration.RequestConfig) (resp *http.Response, err error) {
	uql := ""
	if schemaName == "" {
		schemaName = "*"
	}
	if propertyName == "" {
		return nil, errors.New("ufe: propertyName can not be empty")
	}

	schemaName, err = CheckReplaceSchemaPropertyName(schemaName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, schemaName = %s", err.Error(), schemaName))
	}

	propertyName, err = CheckReplaceSchemaPropertyName(propertyName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, propertyName = %s", err.Error(), propertyName))
	}

	switch dbType {
	case ultipa.DBType_DBNODE:
		uql = fmt.Sprintf(`UFE().node_property(@%v.%v)`, schemaName, propertyName)
	case ultipa.DBType_DBEDGE:
		uql = fmt.Sprintf(`UFE().edge_property(@%v.%v)`, schemaName, propertyName)
	default:
		return nil, errors.New("DBType must be DBType_DBNODE or DBType_DBEDGE")
	}

	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}
	//if !resp.IsSuccess() {
	//	return nil, fmt.Errorf(resp.Status.Message)
	//}

	return resp, nil
}
