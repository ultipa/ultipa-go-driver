package test

import (
	"testing"

	"github.com/ultipa/ultipa-go-driver/sdk/utils"
)

func TestGetCertificate(t *testing.T) {

	certificate := utils.GetCertificate("kinqhpwws.us-east-2.uct.ultipa-inc.org:60010")

	//log.Println(certificate)
	if certificate == nil {
		t.Error("Get Certificate error")
	}
}
