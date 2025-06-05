package test

import (
	"testing"

	"github.com/ultipa/ultipa-go-driver/v5/sdk/printers"
)

func TestPrintUQLErr(t *testing.T) {
	c := "[3-5]aaa find\nExported"
	printers.PrintUqlErr(c)
}
