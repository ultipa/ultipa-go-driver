package api

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ultipa/ultipa-go-driver/sdk/utils"
)

/**
* Utils for SDK Clients.
 */

type UQLType = int

const (
	UQLType_Master = 1
	UQLType_Normal = 2
	UQLType_Global = 3
)

//func (api *UltipaAPI) RefreshClusterInfo(graphName string) error {
//    return api.Pool.RefreshClusterInfo(graphName)
//}

//func (api *UltipaAPI) GetConnByUQL(uql string, graphName string) (uqlType UQLType, leader *connection.Connection, followers []*connection.Connection, global *connection.Connection, err error) {
//
//    graph := api.Pool.GraphMgr.GetGraph(graphName)
//
//    if graph == nil {
//        err = api.Pool.RefreshClusterInfo(graphName)
//        if err != nil {
//            return 0, nil, nil, nil, err
//        }
//        graph = api.Pool.GraphMgr.GetGraph(graphName)
//    }
//
//    // refresh , but not get graph info
//    if graph == nil {
//        return 0, nil, nil, nil, errors.New("unavailable to get graph cluster infos : " + graphName)
//    }
//
//    leader = api.Pool.GraphMgr.GetLeader(graphName)
//    if leader == nil {
//        return 0, nil, nil, nil, errors.New(fmt.Sprintf("no leader found for graph %s", graphName))
//    }
//
//    followers = api.Pool.GraphMgr.GetGraph(graphName).Followers
//    global, err = api.Pool.GetGlobalMasterConn(nil)
//
//    uqlItem := utils.NewUql(uql)
//
//    uqlType = UQLType_Normal
//
//    if uqlItem.HasWrite() {
//        uqlType = UQLType_Master
//    }
//
//    if uqlItem.IsGlobal() {
//        uqlType = UQLType_Global
//    }
//
//    return uqlType, leader, followers, global, err
//
//}

func CheckName(name string) error {
	if len(name) < 2 || len(name) > 127 {
		return errors.New("name bytes length should be between 2 and 127")
	}

	if strings.Contains(name, "`") {
		return errors.New("name can not contain character `")
	}

	if utils.StartWithTilde(name) {
		return errors.New("name can not start with character ~")
	}

	if _, ok := utils.InvalidName[name]; ok {
		return errors.New(fmt.Sprintf("%s is preserved keyword, can NOT use as name", name))
	}
	return nil
}

// CheckReplaceSchemaPropertyName Check if it contains Chinese characters and add `
//
// Used for schemaName and propertyName
func CheckReplaceSchemaPropertyName(name string) (string, error) {
	if name == "" || name == "*" {
		return name, nil
	}

	err := CheckName(name)
	if err != nil {
		return "", err
	}

	// Regular expression to match characters that are not letters, digits, or underscores
	//nonAlphaNumUnderscore := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	//
	//if nonAlphaNumUnderscore.MatchString(name) {
	//	return fmt.Sprintf("`%s`", name), nil
	//}

	return fmt.Sprintf("`%s`", name), nil
	//return name, nil
}

// CheckGraphName Check graph name whether valid
//
// Used for Graph
func CheckGraphName(name string) error {
	err := CheckName(name)
	if err != nil {
		return err
	}

	// Regular expression to match names that must start with a letter and can only contain letters, underscores, and digits (A-Z, a-z, _, 0-9)
	nonAlphaNumUnderscore := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]+$`)
	if !nonAlphaNumUnderscore.MatchString(name) {
		return fmt.Errorf("graph name must start with a letter and can only contain letters, underscores, and numbers (A-Z, a-z, _, 0-9)")
	}

	return nil
}
