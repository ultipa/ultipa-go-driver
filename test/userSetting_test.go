package test

import (
	"fmt"
	"testing"

	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func TestUserSetting(t *testing.T) {
	key := "yu_tst"
	data := "test data"
	resp, err := client.SetUserSetting(&structs.SetUserSetting{
		UserName: key,
		Type:     "int",
		Data:     data,
	}, nil)
	fmt.Println(resp)
	fmt.Println(err)

	resp2, err := client.GetUserSetting(&structs.GetUserSetting{
		UserName: key,
		//DBType:     "int",
	}, nil)
	fmt.Println(err)

	fmt.Println(resp2)
}
