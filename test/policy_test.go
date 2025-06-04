package test

import (
	"fmt"
	"testing"

	"github.com/ultipa/ultipa-go-driver/sdk/structs"
	"github.com/ultipa/ultipa-go-driver/utils"
)

func TestShowPolicy(t *testing.T) {
	all, err := client.ShowPolicy(nil)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(utils.JSONString(all))

	policy, err := client.GetPolicy("test_asP", nil)
	if err != nil {
		t.Log(err)
	} else if policy != nil {
		fmt.Println(utils.JSONString(policy))
	}

	if policy == nil {
		return
	}
	policy.Name = "yu_new"
	_, err = client.CreatePolicy(policy, nil)
	if err != nil {
		t.Fatal(err)
	}

	policy.Policies = []string{"test_policy"}
	_, err = client.AlterPolicy(policy, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DropPolicy(policy.Name, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPolicyUql(t *testing.T) {
	//gp := structs.GraphPrivileges{
	//	"default": []string{"UPDATE", "DELETE"},
	//	"amz":     []string{"UPDATE"},
	//}
	pp := structs.PropertyPrivileges{
		Node: structs.PropertyPrivilegeElement{
			Read:  [][]string{},
			Write: [][]string{{"default", "*", "*"}, {"amz", "nodx", "age"}},
			Deny:  [][]string{},
		},
		Edge: structs.PropertyPrivilegeElement{
			Read:  [][]string{},
			Write: [][]string{{"default", "*", "*"}},
			Deny:  [][]string{},
		},
	}

	p := structs.Policy{
		Name: "yu",
		//GraphPrivileges: gp,
		//SystemPrivileges: []string{"STAT"},
		PropertyPrivileges: pp,
		//AsPolicies:           []string{"yu"},
	}

	//log.Println(p.ToCreatePolicyUql())
	//log.Println(p.ToAlterPolicyUql())

	_, err := client.Uql(p.ToCreatePolicyUql(), nil)

	if err != nil {
		t.Error(err)
	}

	_, err = client.Uql(p.ToAlterPolicyUql(), nil)
	if err != nil {
		t.Error(err)
	}

}
