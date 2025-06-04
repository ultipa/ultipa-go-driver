package structs

import (
	"fmt"

	"github.com/ultipa/ultipa-go-driver/sdk/utils"
)

// PrivilegeToUser
// create().user(
// "user01",
// "pwABC123",
// {"*": ["UPDATE","ALGO","LTE","UFE"]},
// ["STAT","TOP","KILL"],
// [],
// {
// "node": {
// "write": [["*","*","*"]]
// },
// "edge": {
// "write": [["*","*","*"]]
// }
// }
// )
type User struct {
	UserName string `json:"-"`
	PassWord string `json:"password,omitempty"`
	//LastLogin          string             `json:"lastLogin"`
	CreatedTime        string             `json:"-"`
	GraphPrivileges    GraphPrivileges    `json:"graph_privileges,omitempty"`
	SystemPrivileges   []string           `json:"system_privileges,omitempty"`
	Policies           []string           `json:"policies,omitempty"`
	PropertyPrivileges PropertyPrivileges `json:"property_privileges,omitempty"`
}

//type CreateUser struct {
//	UserName         string          `json:"-"`
//	PassWord         string          `json:"password,omitempty"`
//	GraphPrivileges  GraphPrivileges `json:"graph_privileges,omitempty"`
//	SystemPrivileges []string        `json:"system_privileges,omitempty"`
//	Policies         []string        `json:"policies,omitempty"`
//	//PropertyPrivileges PropertyPrivileges `json:"property_privileges,omitempty"`
//}
//
//type AlterUser struct {
//	UserName         string          `json:"-"`
//	PassWord         string          `json:"password,omitempty"`
//	GraphPrivileges  GraphPrivileges `json:"graph_privileges,omitempty"`
//	SystemPrivileges []string        `json:"system_privileges,omitempty"`
//	Policies         []string        `json:"policies,omitempty"`
//	//PropertyPrivileges PropertyPrivileges `json:"property_privileges,omitempty"`
//}

//func (p *User) ToCreateUserUql() string {
//	return fmt.Sprintf("create().user(\"%s\",\"%s\",\n%s,\n%s,\n%s,\n%s\n)", p.UserName, p.PassWord, utils.ToJSONString(p.GraphPrivileges), utils.ToJSONString(p.SystemPrivileges), utils.ToJSONString(p.AsPolicies), utils.ToJSONString(p.PropertyPrivileges))
//}
//
//func (p *User) ToAlterUserUql() string {
//	return fmt.Sprintf("alter().user(\"%s\").set({\npassword: \"%s\",\ngraph_privileges: %s,\nsystem_privileges: %s,\npolicies: %s,\nproperty_privileges: %s\n})", p.UserName, p.PassWord, utils.ToJSONString(p.GraphPrivileges), utils.ToJSONString(p.SystemPrivileges), utils.ToJSONString(p.AsPolicies), utils.ToJSONString(p.PropertyPrivileges))
//}

func (u *User) ToCreateUserUql() string {
	uql := fmt.Sprintf("create().user(\"%s\",\n\"%s\",\n", u.UserName, u.PassWord)

	s := ""

	if u.GraphPrivileges != nil {
		s = utils.ToJSONString(u.GraphPrivileges) + ",\n"
	} else {
		s = "{},\n"
	}
	uql += s

	if u.SystemPrivileges != nil {
		s = utils.ToJSONString(u.SystemPrivileges) + ",\n"
	} else {
		s = "[],\n"
	}
	uql += s

	if u.Policies != nil {
		s = utils.ToJSONString(u.Policies) + "\n"
	} else {
		s = "[]\n"
	}
	uql += s

	//if u.PropertyPrivileges != nil {
	//    s = utils.ToJSONString(u.PropertyPrivileges) + "\n"
	//} else {
	//    s = "{}\n"
	//}

	//uql += s

	uql += ")"

	return uql
	//return fmt.Sprintf("create().user(\"%s\",\n%s,\n%s,\n%s,\n%s\n)", u.Name, utils.ToJSONString(u.GraphPrivileges), utils.ToJSONString(u.SystemPrivileges), utils.ToJSONString(u.AsPolicies), utils.ToJSONString(u.PropertyPrivileges))
}

//func (u *AlterUser) ToAlterUserUql() string {
//	//return fmt.Sprintf("alter().policy(\"%s\").set({\ngraph_privileges: %s,\nsystem_privileges: %s,\npolicies: %s,\nproperty_privileges: %s\n})", p.Name, utils.ToJSONString(p.GraphPrivileges), utils.ToJSONString(p.SystemPrivileges), utils.ToJSONString(p.AsPolicies), utils.ToJSONString(p.PropertyPrivileges))
//	return fmt.Sprintf("alter().user(\"%s\").set(%s)", u.UserName, utils.ToJSONString(u))
//}

func (u *User) ToAlterUserUql() string {
	uql := fmt.Sprintf("alter().user(\"%s\").set({\n", u.UserName)

	s := ""

	if u.PassWord != "" {
		s = fmt.Sprintf("password: \"%s\"", u.PassWord)
		uql += s
	}

	if u.GraphPrivileges != nil {
		if s != "" {
			s = ",\n"
		}
		s += "graph_privileges:" + utils.ToJSONString(u.GraphPrivileges)
		uql += s
	}

	if u.SystemPrivileges != nil {
		if s != "" {
			s = ",\n"
		}
		s += "system_privileges:" + utils.ToJSONString(u.SystemPrivileges)
		uql += s
	}

	if u.Policies != nil {
		if s != "" {
			s = ",\n"
		}
		s += "policies:" + utils.ToJSONString(u.Policies)
		uql += s
	}

	//if u.PropertyPrivileges != nil {
	//    if s != "" {
	//        s = ",\n"
	//    }
	//    s += "property_privileges:" + utils.ToJSONString(u.PropertyPrivileges)
	//    uql += s
	//}

	uql += "\n})"

	return uql
	//return fmt.Sprintf("create().user(\"%s\",\n%s,\n%s,\n%s,\n%s\n)", u.Name, utils.ToJSONString(u.GraphPrivileges), utils.ToJSONString(u.SystemPrivileges), utils.ToJSONString(u.AsPolicies), utils.ToJSONString(u.PropertyPrivileges))
}
