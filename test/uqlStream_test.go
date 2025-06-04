package test

import (
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/printers"
	"log"
	"testing"
)

func TestUQLStream(t *testing.T) {
	uql := `find().nodes() limit 10000 return nodes{*} `

	log.Println("Exec : ", uql)

	//resp, err := client.Uql(c.Uql, &configuration.RequestConfig{Graph: "multi_schema_test"})
	cb := func(res *http.Response) error {
		nodes, schema, err := res.Get(0).AsNodes()
		if err != nil {
			return err
		}
		printers.PrintNodes(nodes, schema)
		return nil
	}

	err := client.UQLStream(uql, cb, nil)

	if err != nil {
		t.Fatal(err)
	}

}
