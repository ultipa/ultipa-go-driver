/**
 * @Author: zhaohaichao
 * @Description:
 * @File:  index_test
 * @Date: 2022/8/4 3:41 pm
 */

package test

import (
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"log"
	"testing"

	"github.com/ultipa/ultipa-go-driver/v5/utils"
)

func TestCreateIndex(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	//client.SetCurrentGraph("go_sdk_test")
	//resp, err := client.CreateIndex(ultipa.DBType_DBNODE, "t1", "name(10)", "name3", nil)
	//if err != nil {
	//    t.Fatal(err)
	//}

	resp, err := client.CreateNodeIndex("@`account`.`industry`(10)", "name1", nil)

	if err != nil {
		t.Error(err)
	}

	resp, err = client.CreateEdgeIndex("@review.`中文名`(23)", "edgeIndex_name_age", nil)
	if err != nil {
		t.Error(err)
	}

	_, err = client.DropIndex(ultipa.DBType_DBNODE, "name1", nil)
	if err != nil {
		t.Error(err)
	}

	log.Printf(utils.JSONString(resp))
}

func TestCreateFullIndex(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	//client.SetCurrentGraph("go_sdk_test")
	//resp, err := client.CreateIndex(ultipa.DBType_DBNODE, "t1", "name(10)", "name3", nil)
	//if err != nil {
	//    t.Fatal(err)
	//}

	resp, err := client.CreateNodeFullText("t1", "name", "full_name1", nil)
	if err != nil {
		t.Error(err)
	}

	resp, err = client.CreateEdgeFullText("e1", "name", "full_name2", nil)
	if err != nil {
		t.Error(err)
	}

	log.Printf(utils.JSONString(resp))
}

func TestListIndex(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	indexes, err := client.ShowIndex(nil)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf(utils.JSONString(indexes))
}

func TestListNodeIndex(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	indexes, err := client.ShowNodeIndex(nil)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf(utils.JSONString(indexes))
}

func TestListEdgeIndex(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	indexes, err := client.ShowEdgeIndex(nil)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf(utils.JSONString(indexes))
}

func TestListFullText(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	indexes, err := client.ShowFullText(nil)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf(utils.JSONString(indexes))
}

func TestListNodeFullText(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	indexes, err := client.ShowNodeFullText(nil)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf(utils.JSONString(indexes))
}

func TestListEdgeFullText(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	indexes, err := client.ShowEdgeFullText(nil)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf(utils.JSONString(indexes))
}

func TestCreateFullText(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	jresp, err := client.CreateFullText(ultipa.DBType_DBNODE, "account", "中文名", "full", nil)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf(jresp.Status.Code.String())

	resp, err := client.DropFullText("full", ultipa.DBType_DBNODE, nil)
	resp, err = client.DropFullText("full", ultipa.DBType_DBNODE, nil)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf(resp.Status.Code.String())

}
