package test

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestNewConn(t *testing.T) {
	var rpcsClient ultipa.UltipaRpcsClient
	h := md5.New()
	username = "root"
	password = "root"
	h.Write([]byte(password))
	pass := hex.EncodeToString(h.Sum(nil))

	conn, err := grpc.Dial("192.168.1.85:61299", grpc.WithInsecure(), grpc.WithDefaultCallOptions())

	for i := 0; i < 100; i++ {
		//conn, err := grpc.Dial("210.13.32.146:60074", grpc.WithInsecure(), grpc.WithDefaultCallOptions())
		//conn, err := grpc.Dial(hosts[0], grpc.WithInsecure(), grpc.WithDefaultCallOptions())
		log.Println("conn state: ", conn.GetState())
		if err != nil {
			t.Fatal(err)
		}

		rpcsClient = ultipa.NewUltipaRpcsClient(conn)

		ctx, _ := context.WithTimeout(context.Background(), time.Second*300)

		//graphName := "go_sdk_test"

		//ctx = metadata.AppendToOutgoingContext(ctx, "user", username, "password", strings.ToUpper(pass), graphName, "multi_schema_test")
		ctx = metadata.AppendToOutgoingContext(ctx, "user", username, "password", strings.ToUpper(pass))

		resp, _ := rpcsClient.SayHello(ctx, &ultipa.HelloUltipaRequest{
			Name: "hello",
		})

		if resp != nil && resp.Status.ErrorCode == ultipa.ErrorCode_SUCCESS {
			log.Println("rpcsClient say hello success")
		} else {
			log.Println("rpcsClient say hello failed")
		}

		resp, err = ultipa.NewUltipaControlsClient(conn).SayHello(ctx, &ultipa.HelloUltipaRequest{
			Name: "hello",
		})

		if resp != nil && resp.Status.ErrorCode == ultipa.ErrorCode_SUCCESS {
			log.Println("ControlsClient say hello success")
		} else {
			log.Println("ControlsClient say hello failed")
		}
		time.Sleep(time.Second * 2)
	}

	ctx2, _ := context.WithTimeout(context.Background(), time.Second*1000)
	ctx2 = metadata.AppendToOutgoingContext(ctx2, "user", username, "password", strings.ToUpper(pass), "graph_name", "go_sdk_test")
	resp2, err := rpcsClient.Query(ctx2, &ultipa.QueryRequest{
		QueryType: ultipa.QueryType_UQL,
		QueryText: "n().e().n() as path return path limit 10;",
		//QueryText: "show().graph()",
	})

	if err != nil {
		t.Fatal(err)
	}

	for {
		record, err := resp2.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		log.Println(record.Alias, record.Paths, err)
	}

	//defer ultipa.Close()
}

func TestUql(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	res, _ := client.Uql("find().nodes({@country}) as nodes ORDER by nodes._uuid DESC RETURN nodes.cPoint LIMIT 8", nil)
	//res, _ := client.Uql("n().e().n() as path return path limit 10;", nil)
	attr, err := res.Alias("nodes.cPoint").AsAttr()
	t.Log(attr)
	if err != nil {
		return
	}
	log.Println(res.AliasList, res.Get(0), res.Status.Code, res.Status.Message)
}

func TestUqlWithSpecialHost(t *testing.T) {
	res, err := client.Uql("show().graph()", &configuration.RequestConfig{
		Host: "localhost:3000",
	})

	target := "transport: Error while dialing: dial tcp 127.0.0.1:3000: connect: connection refused"

	if err == nil || !strings.Contains(err.Error(), target) {
		t.Fatal(err)
	}

	log.Println(res)
}

//func TestRefreshPool(t *testing.T) {
//    //client, _ := GetClient(hosts, graph)
//    for i := 0; i < 10; i++ {
//        err := client.Pool.RefreshActivesWithSeconds(1)
//        if err != nil {
//            t.Error(err)
//        }
//        time.Sleep(time.Millisecond * 500)
//    }
//}

//func TestGetConnByUQL(t *testing.T) {
//
//    //client, _ := GetClient(hosts, graph)
//
//    uql := "show().schema()"
//    _, leader, followers, global, err := client.GetConnByUQL(uql, graph)
//    if err != nil {
//        t.Fatal(err)
//    }
//    if leader == nil {
//        t.Fatal("leader is nill")
//    }
//    if followers == nil {
//        t.Fatal("followers is nill")
//    }
//    if global == nil {
//        t.Fatal("global is nill")
//    }
//}

func TestConnectionSSL(t *testing.T) {

	//if env["ssl_host"] == "" {
	//	t.Skip("no ssl host found")
	//	return
	//}

	var err error
	config := &configuration.UltipaConfig{
		Hosts:        []string{env["ssl_host"]},
		Username:     env["ssl_username"],
		Password:     env["ssl_password"],
		DefaultGraph: env["ssl_graph"],
		//Debug:        true,
	}

	client, err = sdk.NewUltipaDriver(config)

	if err != nil {
		t.Fatal(err)
	}

	uql, err := client.Uql("show().schema()", nil)
	log.Println(uql)
}
