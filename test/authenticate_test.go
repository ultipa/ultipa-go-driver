package test

import (
	"testing"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
)

func TestAuthenticate(t *testing.T) {
	//connection, _ := GetClient(hosts, graph)

	_, err := client.Authenticate(ultipa.AuthenticateType_PERMISSION_TYPE_UQL, "show().graph()", nil)

	if err != nil {
		t.Fatalf("authentication is failed, %v", err)
	}
}
