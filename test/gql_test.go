package test

import (
	"fmt"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"log"
	"testing"
)

func TestGql(t *testing.T) {
	//response, err := client.Uql("find().nodes() as n return n{*}", nil)
	response, err := client.Gql("match (n) return n", nil)
	//response, err := client.Gql("CALL db.schema.visualization()", nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf(response.Status.Code.String())
}

func TestGqlStream(t *testing.T) {
	uql := `match ()-[e]-() return e limit 10000`

	log.Println("Exec : ", uql)

	//resp, err := client.Uql(c.Uql, &configuration.RequestConfig{Graph: "multi_schema_test"})
	cb := func(res *http.Response) error {
		edges, _, err := res.Get(0).AsEdges()
		if err != nil {
			return err
		}
		fmt.Println("edge count ", len(edges))
		for i, edge := range edges {
			if i%100 == 0 {
				fmt.Println(i)
			}
			fmt.Print(edge.UUID, " ")
		}
		return nil
	}

	err := client.GQLStream(uql, cb, nil)

	if err != nil {
		t.Fatal(err)
	}
}
