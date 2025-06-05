package printers

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

type TreeNode struct {
	Explain    *structs.PlanNode
	ChildNodes []*TreeNode
}

var leveledList pterm.LeveledList

func PrintExplain(graphs []*structs.PlanNode) {
	if graphs == nil || len(graphs) == 0 {
		return
	}

	explainChan := make(chan *structs.PlanNode, len(graphs))
	for _, explain := range graphs {
		explainChan <- explain
	}
	close(explainChan)
	tree := buildTree(explainChan)

	traverse(tree, 0)
	//root := putils.TreeFromLeveledList(leveledList)

	//pterm.DefaultTree.WithIndent(3).WithRoot(root).Render()
}

func buildTree(graphs chan *structs.PlanNode) *TreeNode {
	if graphs == nil || len(graphs) == 0 {
		return nil
	}

	record := <-graphs
	tree := &TreeNode{
		Explain:    record,
		ChildNodes: []*TreeNode{},
	}
	for i := 1; i <= int(record.ChildrenNum); i++ {
		node := buildTree(graphs)
		tree.ChildNodes = append(tree.ChildNodes, node)
	}

	return tree
}

func traverse(tree *TreeNode, index int) {

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
