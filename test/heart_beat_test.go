package test

import (
	"log"
	"testing"
	"time"

	"github.com/ultipa/ultipa-go-driver/sdk"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
)

func TestHeartBeat(t *testing.T) {

	var err error
	config := &configuration.UltipaConfig{
		Hosts:     hosts,
		Username:  username,
		Password:  password,
		HeartBeat: 1,
		//Debug:     true,
	}

	client, err = sdk.NewUltipaDriver(config)

	if err != nil {
		t.Fatal(err)
	}

	log.Println("TestHeartBeat - Sleep")
	time.Sleep(10 * time.Second)

}
