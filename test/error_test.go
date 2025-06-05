package test

import (
	"reflect"
	"testing"

	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils"
)

func TestErrorType(t *testing.T) {
	err := utils.NewLeaderNotYetElectedError("")
	if reflect.TypeOf(err).Elem().String() != "utils.LeaderNotYetElectedError" {
		t.Fatal("not instance of utils.LeaderNotYetElectedError")
	}
}
