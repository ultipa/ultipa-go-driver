package test

import (
	"log"
	"testing"

	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
)

func TestLte(t *testing.T) {
	response, err := client.Lte(ultipa.DBType_DBEDGE, "account", "gender", nil)
	if err != nil {
		return
	}
	log.Println(response)

	//response, err = client.Ufe(ultipa.DBType_DBEDGE, "中文", "中文属性", nil)
	//if err != nil {
	//	return
	//}
	//log.Println(response)
	//
	//response, err = client.Lte(ultipa.DBType_DBEDGE, "*", "中文属性", nil)
	//if err != nil {
	//	return
	//}
	//log.Println(response)
	//
	//response, err = client.Ufe(ultipa.DBType_DBEDGE, "*", "中文属性", nil)
	//if err != nil {
	//	return
	//}
	//log.Println(response)
}
