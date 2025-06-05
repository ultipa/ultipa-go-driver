package test

import (
	"testing"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
	"github.com/ultipa/ultipa-go-driver/v5/utils"
)

func TestParseStatistic(t *testing.T) {

	res, err := http.ParseStatistic(&ultipa.Table{
		Headers: []*ultipa.Header{
			{
				PropertyName: "total_time_cost",
				PropertyType: ultipa.PropertyType_STRING,
			},
			{
				PropertyName: "engine_time_cost",
				PropertyType: ultipa.PropertyType_STRING,
			},
		},
		TableRows: []*ultipa.TableRow{
			{
				Values: [][]byte{
					[]byte("10"),
					[]byte("20"),
				},
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	utils.PrintJSON(res)
}
