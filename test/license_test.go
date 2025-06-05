package test

import (
	"github.com/ultipa/ultipa-go-driver/v5/sdk/printers"
	"testing"
)

func TestShowLicense(t *testing.T) {
	license, err := client.ShowLicense(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", license)
}

func TestShowBackup(t *testing.T) {
	license, err := client.ShowBacukup(nil)
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintBackupInfoList(license)
}
