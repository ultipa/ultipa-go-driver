package http

import (
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

type ExplainPlan struct {
	PlanNodes []*structs.PlanNode
}

func ParseExplainPlan(ex *ultipa.ExplainPlan) (*ExplainPlan, error) {
	explainPlan := ExplainPlan{
		PlanNodes: []*structs.PlanNode{},
	}

	if ex == nil {
		return &explainPlan, nil
	}

	for _, planNode := range ex.PlanNodes {
		explain := structs.PlanNode{
			//DBType:        planNode.GetType(),
			Alias:       planNode.GetAlias(),
			ChildrenNum: planNode.GetChildrenNum(),
			Uql:         planNode.GetQueryText(),
			Infos:       planNode.GetInfos(),
		}
		explainPlan.PlanNodes = append(explainPlan.PlanNodes, &explain)
	}

	return &explainPlan, nil
}
