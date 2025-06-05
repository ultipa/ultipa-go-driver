package structs

import (
	"fmt"

	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils"
)

// Policy create().policy(
//
//	  "manager",
//	 {"*": ["UPDATE", "DELETE"]},
//	 ["STAT"],
//	 ["yu", "yu2"],
//	  {
//	  "node": {
//	    "write": [["default","*","*"], ["amz","nodx","age"]]
//	  },
//	  "edge": {
//	    "write": [["default","*","*"]]
//	  }
//	}
//
// )
type Policy struct {
	Name               string             `json:"-"`
	GraphPrivileges    GraphPrivileges    `json:"graph_privileges,omitempty"`
	SystemPrivileges   []string           `json:"system_privileges,omitempty"`
	PropertyPrivileges PropertyPrivileges `json:"property_privileges,omitempty"`
	Policies           []string           `json:"policies,omitempty"`
}

/*
	{
	  "node": {
	    "read": [],
	    "write": [
	      ["default", "*", "*"],
	      ["amz", "nodx", "age"]
	    ],
	    "deny": []
	  },
	  "edge": {
	    "read": [],
	    "write": [
	      ["default", "*", "*"]
	    ],
	    "deny": []
	  }
	}
*/
//type PropertyPrivileges map[string]map[string][][]string
type PropertyPrivileges struct {
	Node PropertyPrivilegeElement `json:"node"`
	Edge PropertyPrivilegeElement `json:"edge"`
}

type PropertyPrivilegeElement struct {
	Read  [][]string `json:"read"`
	Write [][]string `json:"write"`
	Deny  [][]string `json:"deny"`
}

type GraphPrivileges map[string][]string

func (p *Policy) ToCreatePolicyUql() string {
	uql := fmt.Sprintf("create().policy(\"%s\",\n", p.Name)

	s := ""

	if p.GraphPrivileges != nil {
		s = utils.ToJSONString(p.GraphPrivileges) + ",\n"
	} else {
		s = "{},\n"
	}
	uql += s

	if p.SystemPrivileges != nil {
		s = utils.ToJSONString(p.SystemPrivileges) + ",\n"
	} else {
		s = "[],\n"
	}
	uql += s

	if p.Policies != nil {
		s = utils.ToJSONString(p.Policies) + ",\n"
	} else {
		s = "[],\n"
	}
	uql += s

	// PropertyPrivileges nil check
	if p.PropertyPrivileges.Node.Read == nil {
		p.PropertyPrivileges.Node.Read = [][]string{}
	}
	if p.PropertyPrivileges.Node.Write == nil {
		p.PropertyPrivileges.Node.Write = [][]string{}
	}
	if p.PropertyPrivileges.Node.Deny == nil {
		p.PropertyPrivileges.Node.Deny = [][]string{}
	}

	if p.PropertyPrivileges.Edge.Read == nil {
		p.PropertyPrivileges.Edge.Read = [][]string{}
	}
	if p.PropertyPrivileges.Edge.Write == nil {
		p.PropertyPrivileges.Edge.Write = [][]string{}
	}
	if p.PropertyPrivileges.Edge.Deny == nil {
		p.PropertyPrivileges.Edge.Deny = [][]string{}
	}

	s = utils.ToJSONString(p.PropertyPrivileges) + "\n"
	//if p.PropertyPrivileges != nil {
	//	s = utils.ToJSONString(p.PropertyPrivileges) + "\n"
	//} else {
	//	s = "{}\n"
	//}

	uql += s

	uql += ")"

	return uql
	//return fmt.Sprintf("create().policy(\"%s\",\n%s,\n%s,\n%s,\n%s\n)", p.Name, utils.ToJSONString(p.GraphPrivileges), utils.ToJSONString(p.SystemPrivileges), utils.ToJSONString(p.AsPolicies), utils.ToJSONString(p.PropertyPrivileges))
}

func (p *Policy) ToAlterPolicyUql() string {
	if p.PropertyPrivileges.Node.Read == nil {
		p.PropertyPrivileges.Node.Read = [][]string{}
	}
	if p.PropertyPrivileges.Node.Write == nil {
		p.PropertyPrivileges.Node.Write = [][]string{}
	}
	if p.PropertyPrivileges.Node.Deny == nil {
		p.PropertyPrivileges.Node.Deny = [][]string{}
	}

	if p.PropertyPrivileges.Edge.Read == nil {
		p.PropertyPrivileges.Edge.Read = [][]string{}
	}
	if p.PropertyPrivileges.Edge.Write == nil {
		p.PropertyPrivileges.Edge.Write = [][]string{}
	}
	if p.PropertyPrivileges.Edge.Deny == nil {
		p.PropertyPrivileges.Edge.Deny = [][]string{}
	}
	//return fmt.Sprintf("alter().policy(\"%s\").set({\ngraph_privileges: %s,\nsystem_privileges: %s,\npolicies: %s,\nproperty_privileges: %s\n})", p.Name, utils.ToJSONString(p.GraphPrivileges), utils.ToJSONString(p.SystemPrivileges), utils.ToJSONString(p.AsPolicies), utils.ToJSONString(p.PropertyPrivileges))
	return fmt.Sprintf("alter().policy(\"%s\").set(%s)", p.Name, utils.ToJSONString(p))
}
