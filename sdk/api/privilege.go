package api

import (
	"fmt"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

type PrivilegeLevel int32

const (
	GraphLevel  PrivilegeLevel = iota //Graph privilege type
	SystemLevel                       // System privilege type
)

func (api *UltipaAPI) ShowPrivilege(config *configuration.RequestConfig) (privileges []*structs.Privilege, err error) {
	resp, err := api.Uql("show().privilege()", config)

	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	privileges, err = resp.Alias(http.RESP_PRIVILEGE_KEY).AsPrivileges()
	if err != nil {
		return nil, err
	}

	return privileges, nil
}

//func (api *UltipaAPI) GrantPropertyPrivilege(privilegeTypes structs.PrivilegeTypes, graph, schema, property string, targetType structs.PrivilegeTargetType, privilegeName string, config *configuration.RequestConfig) (resp *http.Response, err error) {
//	if len(privilegeTypes) == 0 {
//		return nil, fmt.Errorf("privilegeTypes at least one parameter is required")
//	}
//	uql := fmt.Sprintf(`grant().privilege(%s).on(
// "%s",
// @%s,
// "%s"
//).%s("%s")`, utils.ToJSONString(privilegeTypes), graph, schema, property, targetType, privilegeName)
//
//	resp, err = api.Uql(uql, config)
//
//	if err != nil {
//		return nil, err
//	}
//	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
//		return nil, errors.New(resp.Status.Message)
//	}
//
//	return resp, nil
//}
//
//func (api *UltipaAPI) GrantSystemPrivilege(systemPrivilegeTypes string, targetType structs.PrivilegeTargetType, privilegeName string, config *configuration.RequestConfig) (resp *http.Response, err error) {
//	uql := fmt.Sprintf(`grant().system().privilege(["%s"]).%s("%s")`, systemPrivilegeTypes, targetType, privilegeName)
//
//	resp, err = api.Uql(uql, config)
//
//	if err != nil {
//		return nil, err
//	}
//	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
//		return nil, errors.New(resp.Status.Message)
//	}
//
//	return resp, nil
//}
//
//func (api *UltipaAPI) RevokePropertyPrivilege(privilegeTypes structs.PrivilegeType, graph, schema, property string, targetType structs.PrivilegeTargetType, privilegeName string, config *configuration.RequestConfig) (resp *http.Response, err error) {
//	uql := fmt.Sprintf(`revoke().privilege(["%s"]).on(
//  "%s",
//  @%s,
//  "%s"
//).%s("%s")`, privilegeTypes, graph, schema, property, targetType, privilegeName)
//
//	resp, err = api.Uql(uql, config)
//
//	if err != nil {
//		return nil, err
//	}
//	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
//		return nil, errors.New(resp.Status.Message)
//	}
//
//	return resp, nil
//}
//
//func (api *UltipaAPI) RevokeSystemPrivilege(systemPrivilegeTypes string, targetType structs.PrivilegeTargetType, privilegeName string, config *configuration.RequestConfig) (resp *http.Response, err error) {
//	uql := fmt.Sprintf(`revoke().system().privilege(["%s"]).%s("%s")`, systemPrivilegeTypes, targetType, privilegeName)
//
//	resp, err = api.Uql(uql, config)
//
//	if err != nil {
//		return nil, err
//	}
//	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
//		return nil, errors.New(resp.Status.Message)
//	}
//
//	return resp, nil
//}

//func (api *UltipaAPI) GrantPolicy(userName string, graphPrivileges *structs.GraphPrivileges, systemPrivileges []string, propertyPrivileges *structs.PropertyPrivileges, policies []string, config *configuration.RequestConfig) (resp *http.Response, err error) {
//	uql := fmt.Sprintf(`grant().user("%s").params({
//  graph_privileges: %s,
//  system_privileges: %s,
//  property_privileges: %s,
//  policies: %s
//})`, userName, utils.ToJSONString(graphPrivileges), utils.ToJSONString(systemPrivileges), utils.ToJSONString(propertyPrivileges), utils.ToJSONString(policies))
//
//	resp, err = api.Uql(uql, config)
//
//	if err != nil {
//		return nil, err
//	}
//
//	return resp, nil
//}

//func (api *UltipaAPI) RevokePolicy(userName string, graphPrivileges *structs.GraphPrivileges, systemPrivileges []string, propertyPrivileges *structs.PropertyPrivileges, policies []string, config *configuration.RequestConfig) (resp *http.Response, err error) {
//	uql := fmt.Sprintf(`revoke().user("%s").params({
//  graph_privileges: %s,
//  system_privileges: %s,
//  property_privileges: %s,
//  policies: %s
//})`, userName, utils.ToJSONString(graphPrivileges), utils.ToJSONString(systemPrivileges), utils.ToJSONString(propertyPrivileges), utils.ToJSONString(policies))
//
//	resp, err = api.Uql(uql, config)
//
//	if err != nil {
//		return nil, err
//	}
//
//	return resp, nil
//}
