package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ultipa/ultipa-go-driver/utils"
)

type UQLMAKER struct {
	_command  UQLCommand
	_commandP string
	_params   []struct {
		key   string
		value string
	}
}

type UQLCommand string

const (
	UQLCommand_listGraph      UQLCommand = "show().graph"
	UQLCommand_listNodeSchema UQLCommand = "show().node_schema"

	UQLCommand_ab                 UQLCommand = "ab"
	UQLCommand_khop               UQLCommand = "khop"
	UQLCommand_nodes              UQLCommand = "find().nodes"
	UQLCommand_edges              UQLCommand = "find().edges"
	UQLCommand_deleteNodes        UQLCommand = "delete().nodes"
	UQLCommand_deleteEdges        UQLCommand = "delete().edges"
	UQLCommand_updateNodes        UQLCommand = "update().nodes"
	UQLCommand_updateEdges        UQLCommand = "update().edges"
	UQLCommand_template           UQLCommand = "t"
	UQLCommand_autoNet            UQLCommand = "autoNet"
	UQLCommand_autoNetByPart      UQLCommand = "autoNetByPart"
	UQLCommand_nodeSpread         UQLCommand = "spread"
	UQLCommand_insertNode         UQLCommand = "insert().nodes"
	UQLCommand_insertEdge         UQLCommand = "insert().edges"
	UQLCommand_showProperty       UQLCommand = "show().property"
	UQLCommand_showNodeProperty   UQLCommand = "show().node_property"
	UQLCommand_showEdgeProperty   UQLCommand = "show().edge_property"
	UQLCommand_createNodeProperty UQLCommand = "create().node_property"
	UQLCommand_createEdgeProperty UQLCommand = "create().edge_property"
	UQLCommand_alterNodeProperty  UQLCommand = "alter().node_property"
	UQLCommand_alterEdgeProperty  UQLCommand = "alter().edge_property"
	UQLCommand_dropNodeProperty   UQLCommand = "drop().node_property"
	UQLCommand_dropEdgeProperty   UQLCommand = "drop().edge_property"
	UQLCommand_lteNode            UQLCommand = "LTE().node_property"
	UQLCommand_lteEdge            UQLCommand = "LTE().edge_property"
	UQLCommand_ufeNode            UQLCommand = "UFE().node_property"
	UQLCommand_ufeEdge            UQLCommand = "UFE().edge_property"
	UQLCommand_createIndex        UQLCommand = "createIndex"
	UQLCommand_showIndex          UQLCommand = "showIndex"
	UQLCommand_dropIndex          UQLCommand = "dropIndex"
	UQLCommand_stat               UQLCommand = "stat"
	UQLCommand_algo               UQLCommand = "algo"
	UQLCommand_listPrivilege      UQLCommand = "listPrivilege"
	UQLCommand_grant              UQLCommand = "grant"
	UQLCommand_revoke             UQLCommand = "revoke"
	UQLCommand_listUser           UQLCommand = "listUser"
	UQLCommand_getUser            UQLCommand = "getUser"
	UQLCommand_createUser         UQLCommand = "createUser"
	UQLCommand_updateUser         UQLCommand = "updateUser"
	UQLCommand_deleteUser         UQLCommand = "deleteUser"
	UQLCommand_createPolicy       UQLCommand = "createPolicy"
	UQLCommand_updatePolicy       UQLCommand = "updatePolicy"
	UQLCommand_deletePolicy       UQLCommand = "deletePolicy"
	UQLCommand_listPolicy         UQLCommand = "listPolicy"
	UQLCommand_getPolicy          UQLCommand = "getPolicy"
	UQLCommand_showTask           UQLCommand = "show().task"
	UQLCommand_clearTask          UQLCommand = "clear().task"
	UQLCommand_createGraph        UQLCommand = "createGraph"
	UQLCommand_getGraph           UQLCommand = "getGraph"
	UQLCommand_dropGraph          UQLCommand = "dropGraph"
	UQLCommand_updateGraph        UQLCommand = "updateGraph"
)

func replace_doller(str string) string {
	reg := regexp.MustCompile(`"(\$[a-z_A-Z]+)"`)
	return reg.ReplaceAllString(str, "${1}")
}
func (t *UQLMAKER) SetCommand(command UQLCommand) {
	t._command = command
}
func (t *UQLMAKER) SetCommandParams(commandP interface{}) {
	if nil == commandP {
		return
	}
	commandPString, ok := commandP.(string)
	if ok {
		t._commandP = commandPString
		return
	}
	jsonBytes, err := utils.StructToJSONBytes(commandP)
	if nil != err {
		return
	}
	t._commandP = replace_doller(utils.BytesToString(jsonBytes))
}

func (t *UQLMAKER) AddParam(key string, value interface{}, required bool) {
	valueBool, ok := value.(bool)
	if ok {
		if valueBool {
			required = false
		}
		value = ""
	}
	if required {
		if nil == value {
			return
		}
	}
	switch key {
	case "filter":
		t.AddParam("node_filter", value, true)
		t.AddParam("edge_filter", value, true)
		return
	case "select":
	// t.AddParam("select_node_properties", value, true);
	// t.AAddParam("select_edge_properties", value, true);
	// return;
	case "select_node_properties":
	case "select_edge_properties":
	case "privileges":
	case "policies":
		valueArray, ok := value.([]string)
		if ok {
			value = strings.Join(valueArray, ",")
		}
		break
	}
	valueString, ok := value.(string)
	if !ok {
		bytes, err := utils.StructToJSONBytes(value)
		if nil != err {
			return
		}
		valueString = replace_doller(utils.BytesToString(bytes))
	}
	t._params = append(t._params, struct {
		key   string
		value string
	}{key: key, value: valueString})
}
func (t *UQLMAKER) ToString() string {
	uql := ""
	uql = fmt.Sprintf("%s%s(%s)", uql, t._command, t._commandP)
	if len(t._params) > 0 {
		var strs []string
		for _, v := range t._params {
			strs = append(strs, fmt.Sprintf("%s(%s)", v.key, v.value))
		}
		uql = uql + "." + strings.Join(strs, ".")
	}
	return uql
}

type UQL struct {
	Uql          string
	Command      string
	CommandParam string
	Params       map[string]interface{}
}

func (t *UQL) Parse(uqlString string) {
	r, _ := regexp.Compile("([a-zA-Z]*)\\(([^\\(|^\\)]*)\\)")
	var findAll = r.FindAllStringSubmatch(uqlString, -1)

	t.Uql = uqlString
	t.Command = ""
	t.CommandParam = ""
	t.Params = map[string]interface{}{}

	for i, find := range findAll {
		name := find[1]
		value := ""
		if len(find) >= 2 {
			value = find[2]
		}
		value = strings.Trim(value, " '\"")
		if 0 == i {
			t.Command = name
			t.CommandParam = value
		} else {
			t.Params[name] = value
		}
	}
}
