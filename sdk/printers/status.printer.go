package printers

import (
	"fmt"

	"github.com/ultipa/ultipa-go-driver/sdk/http"
)

func PrintStatistics(stats *http.Statistic) {

	tc := stats.TotalCost
	ec := stats.EngineCost
	ne := stats.NodeAffected
	ee := stats.EdgeAffected

	if ne != 0 {
		fmt.Printf("Effect Nodes: %v \n", ne)
	}

	if ee != 0 {
		fmt.Printf("Effect Edges: %v \n", ee)
	}

	fmt.Printf("Total Cost : %vs | Engine Cost : %vs \n", float64(tc)/1000, float64(ec)/1000)
}
