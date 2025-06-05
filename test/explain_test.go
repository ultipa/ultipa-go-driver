package test

import (
	"fmt"
	"testing"

	"github.com/pterm/pterm"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/printers"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

func TestExplain(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	//resp, err := client.Uql("explain find().nodes() as nodes limit 1 return nodes limit 10", nil)
	//	resp, err := client.Uql(`explain n({@account} as buyer).e().n({@card}).re({@transaction} as buy).n()
	//with buyer, buy
	//group by buyer
	//with collect(distinct(day_of_week(buy.time))) as array
	//where (1 in array || 7 in array) && 2 nin array && 3 nin array && 4 nin array && 5 nin array && 6 nin array
	//return buyer{*} limit 100`, nil)

	//	resp, err := client.Uql(`explain find().nodes() as n1 limit 1
	// find().nodes() as n2 limit 1
	// find().nodes() as n3 limit 1
	//return n1,n2,n3 limit 10`, nil)

	resp, err := client.Uql(`explain find().nodes() as n1 find().nodes() as n2 find().nodes() as n3 with n1,n2,n3 return n1, n2, n3`, nil)

	if err != nil {
		t.Fatal(err)
	}

	//log.Println(resp)
	explain := resp.ExplainPlan.PlanNodes
	printers.PrintExplain(explain)

}

func TestExplain1(t *testing.T) {

	var Explain []*structs.PlanNode
	Explain = append(Explain, &structs.PlanNode{ChildrenNum: 1, Alias: "1"})
	Explain = append(Explain, &structs.PlanNode{ChildrenNum: 2, Alias: "2"})
	Explain = append(Explain, &structs.PlanNode{ChildrenNum: 2, Alias: "3"})
	Explain = append(Explain, &structs.PlanNode{ChildrenNum: 0, Alias: "4"})
	Explain = append(Explain, &structs.PlanNode{ChildrenNum: 0, Alias: "5"})
	Explain = append(Explain, &structs.PlanNode{ChildrenNum: 0, Alias: "6"})
	//PlanNode = append(PlanNode, &structs.PlanNode{ChildrenNum: 0})

	//root := &printers.TreeNode{
	//	PlanNode: PlanNode[0],
	//}
	//parent = root
	//appendTreeNode(root, PlanNode[1:])
	explainChan := make(chan *structs.PlanNode, len(Explain))
	for _, explain := range Explain {
		explainChan <- explain
	}
	close(explainChan)
	node := buildTreeNode(explainChan)

	traverse(node, 0)
	// FIXME:
	//tree := putils.TreeFromLeveledList(leveledList)

	//pterm.DefaultTree.WithIndent(3).WithRoot(tree).Render()

}

var leveledList = pterm.LeveledList{}
var parent = &printers.TreeNode{}

func appendTreeNode(root *printers.TreeNode, graphs []*structs.PlanNode) {
	if graphs == nil || len(graphs) == 0 {
		return
	}

	record := graphs[0]

	root.ChildNodes = append(root.ChildNodes, &printers.TreeNode{
		Explain:    record,
		ChildNodes: []*printers.TreeNode{},
	})

	explains := graphs[1:]
	if int(record.ChildrenNum) > 0 {

		parent = root
		lastIndex := len(root.ChildNodes) - 1
		root = root.ChildNodes[lastIndex]
		appendTreeNode(root, explains)

	} else {
		var nextExplain *structs.PlanNode
		if len(graphs) > 1 {
			nextExplain = graphs[1]
		}
		if nextExplain != nil {
			if int(nextExplain.ChildrenNum) > 0 {
				appendTreeNode(parent, explains)
			} else {
				appendTreeNode(root, explains)

			}
		}
	}

	return
}

func buildTreeNode(graphs chan *structs.PlanNode) *printers.TreeNode {
	if graphs == nil || len(graphs) == 0 {
		return nil
	}

	record := <-graphs
	tree := &printers.TreeNode{
		Explain:    record,
		ChildNodes: []*printers.TreeNode{},
	}
	for i := 1; i <= int(record.ChildrenNum); i++ {
		node := buildTreeNode(graphs)
		tree.ChildNodes = append(tree.ChildNodes, node)
	}

	return tree
}

func traverse(tree *printers.TreeNode, index int) {
	if tree == nil {
		return
	}

	s := tree.Explain
	//leveledList = append(leveledList, pterm.LeveledListItem{Level: index, Text: "DBType: " + s.DBType.String()})
	leveledList = append(leveledList, pterm.LeveledListItem{Level: index, Text: "Alias: " + s.Alias})
	leveledList = append(leveledList, pterm.LeveledListItem{Level: index, Text: "ChildrenNum: " + fmt.Sprint(s.ChildrenNum)})
	leveledList = append(leveledList, pterm.LeveledListItem{Level: index, Text: "Uql: " + s.Uql})
	leveledList = append(leveledList, pterm.LeveledListItem{Level: index, Text: "Infos: " + s.Infos})
	index++
	if tree == nil || tree.ChildNodes == nil || len(tree.ChildNodes) == 0 {
		return
	}
	for _, node := range tree.ChildNodes {
		traverse(node, index)
	}
}
