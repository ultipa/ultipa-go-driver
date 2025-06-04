package test

import (
	"fmt"
	"log"
	"testing"

	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func TestUser(t *testing.T) {
	users, err := client.ShowUser(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		fmt.Println(user)
	}

	user, err := client.GetUser("root", nil)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(user)

}

func TestUserUql(t *testing.T) {
	//gp := structs.GraphPrivileges{
	//	"default": []string{"UPDATE", "DELETE"},
	//	"amz":     []string{"UPDATE"},
	//}
	//pp := structs.PropertyPrivileges{
	//    "node": {
	//        "read":  {},
	//        "write": {{"default", "*", "*"}, {"amz", "nodx", "age"}},
	//        "deny":  {},
	//    },
	//    "edge": {
	//        "read":  {},
	//        "write": {{"default", "*", "*"}},
	//        "deny":  {},
	//    },
	//}

	p := structs.User{
		UserName: "gosdk",
		PassWord: "gosdk1212",
		//GraphPrivileges:  structs.GraphPrivileges{},
		//SystemPrivileges: []string{},
		//PropertyPrivileges: pp,
		//AsPolicies:           []string{"yu"},
		Policies: []string{"12"},
	}

	fmt.Println(p.ToCreateUserUql())

	p1 := p
	fmt.Println(p1.ToAlterUserUql())

	_, err := client.DropUser(p.UserName, nil)
	if err != nil {
		log.Println(err)
	}

	_, err = client.CreateUser(&p, nil)
	if err != nil {
		log.Println(err)
	}

	p1.PassWord = ""
	p1.Policies = []string{}
	p1.SystemPrivileges = nil
	fmt.Println(p1.ToAlterUserUql())

	_, err = client.AlterUser(&p1, nil)
	if err != nil {
		log.Println(err)
	}

}
