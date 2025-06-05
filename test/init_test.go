package test

import (
	"github.com/ultipa/ultipa-go-driver/v5/sdk/printers"
	"log"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"github.com/ultipa/ultipa-go-driver/v5/sdk"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/api"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
)

var env map[string]string
var client *api.UltipaAPI
var hosts []string
var username string
var password string
var graph string
var DEBUG bool

func TestMain(m *testing.M) {
	//setup()

	//conn, err := grpc.Dial("192.168.1.85:61299", grpc.WithInsecure())
	//if err != nil {
	//    log.Fatal(err)
	//}
	//
	//client := ultipa.NewUltipaControlsClient(conn)
	//client.SayHello()
	m.Run()

	//teardown()
}

func TestPing(t *testing.T) {
	config := &configuration.UltipaConfig{
		Hosts:        []string{"eedcfcd239f44b878a94a74ded5b35d6s.eu-south-1.cloud.ultipa.com:8443"},
		Username:     "root",
		Password:     "8bde9fa3a6824bab86618788e5ca4fcb",
		DefaultGraph: "retail_test",
	}

	cli, err := sdk.NewUltipaDriver(config)
	if err != nil {
		log.Fatalln("Failed to connect to Ultipa:", err)
	}
	graphs, err := cli.ShowGraph(nil)
	if err != nil {
		log.Fatalln("show graph error:", err)
	}
	printers.PrintGraphSet(graphs)

	//client, _ = GetClient(hosts, graph)
	//ok, err := client.Test(nil)
	//if !ok {
	//	t.Fatal(err)
	//}

}

func GetClient(hosts []string, graphName string) (*api.UltipaAPI, error) {
	var err error
	//DEBUG = true // open if you need
	config := &configuration.UltipaConfig{
		Hosts:        hosts,
		Username:     username,
		Password:     password,
		DefaultGraph: graphName,
		//Debug:        DEBUG,
	}

	client, err = sdk.NewUltipaDriver(config)

	if err != nil {
		panic(err)
	}

	return client, err
}

func setup() {
	log.Println("Setting up the test environment")
	var err error
	env, err = godotenv.Read(".env")

	if err != nil {
		panic("Get env error, " + err.Error())
	}

	hosts = strings.Split(env["hosts"], ",")
	username, password, graph = env["username"], env["password"], env["graph"]

	client, err = GetClient(hosts, graph)

	if err != nil {
		panic("GetClient error, " + err.Error())
	}
}

func teardown() {
	client.Close()

	log.Println("Tearing down the test environment")
}
