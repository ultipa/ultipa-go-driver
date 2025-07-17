package test

import (
	"fmt"
	"github.com/pieterclaerhout/go-waitgroup"
	"github.com/ultipa/ultipa-go-driver/v5/sdk"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils/logger"
	"log"
	"sync"
	"testing"
	"time"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/printers"
)

func TestUQL(t *testing.T) {
	InitCases()

	for _, c := range cases {

		log.Println("Exec : ", c.UQL)

		//resp, err := client.Uql(c.Uql, &configuration.RequestConfig{Graph: "multi_schema_test"})
		resp, err := client.Uql(c.UQL, nil)

		if err != nil {
			t.Fatal(err)
		}

		if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
			log.Println(resp.Status.Message)
			continue
		}

		for _, a := range resp.AliasList {
			dataitem := resp.Alias(a)
			printers.PrintAny(dataitem)
			log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)

		}
	}
}

func TestUQL1(t *testing.T) {
	//uql := `n({@user && _uuid == 1}).e({@relation.relation_type == 'has'}).n({@projects} as project).re({@relation.relation_type == 'has'}).n({@etl} as etl) group by project skip 0 return table(project._id,project._uuid,count(etl)) as t limit 15 order by project.created_at desc`
	//uql := `find().edges(2658) as edges return edges{*}`
	//uql := `find().nodes() as nodes return nodes{*} limit 10`
	uql := `n({@version}).e().n({@docs_tree.status == 1} as book2).e().n({@lang.code == "en"}) with book2 as vbook
n(vbook).e().n({@docs_tree && is_root == "true" && @docs_tree.status == 1} as book1) with book1 as books
n(books).re({@docs_tree}).n(vbook).re({@docs_tree})[:3].n({@docs.status == 1 && @docs.type == "technical"|| @docs_tree} as n100) as path  with path as docs_path, n100 as books2
n({books || vbook || books2}).e({@docs_role || @docs_tree_role}).n({@role.name in ["public"]}) as p with p as role_path
return docs_path{*}, role_path{*},books`
	log.Println("Exec : ", uql)

	//resp, err := client.Uql(c.Uql, &configuration.RequestConfig{Graph: "multi_schema_test"})
	resp, err := client.Uql(uql, nil)

	if err != nil {
		t.Fatal(err)
	}

	log.Println(resp.Statistic.TotalCost)

	//for _, a := range resp.AliasList {
	//	dataitem := resp.Alias(a)
	//	printers.PrintAny(dataitem)
	//	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)
	//
	//}

}

func TestUQL2(t *testing.T) {
	//requestConfig := &configuration.RequestConfig{
	//	//UseMaster: false,
	//	Graph: "ts",
	//}
	//
	//insertRequestConfig := &configuration.InsertRequestConfig{
	//	Silent:        false,
	//	RequestConfig: requestConfig,
	//	InsertType:    ultipa.InsertType_NORMAL,
	//}
	//
	//myDeletion, _ := client.DeleteNodes("{_id == 'test_silent'}", insertRequestConfig)
	//println("Operation succeeds:", myDeletion.Status.IsSuccess(), "message: ", myDeletion.Status.Message)
	//nodes, _, _ := myDeletion.Alias("nodes").AsNodes()
	//for _, node := range nodes {
	//	jsonData, err := json.Marshal(node)
	//	if err != nil {
	//		fmt.Println("Error converting to JSON:", err)
	//		return
	//	}
	//
	//	fmt.Println(string(jsonData))
	//}

	//println("Operation succeeds:", myDeletion.Status.IsSuccess())
	//client, _ := GetClient(hosts, graph)

	//uql := `n({@user && _uuid == 1}).e({@relation.relation_type == 'has'}).n({@projects} as project).re({@relation.relation_type == 'has'}).n({@etl} as etl) group by project skip 0 return table(project._id,project._uuid,count(etl)) as t limit 15 order by project.created_at desc`
	//uql := `find().nodes() return nodes limit 10`
	uql := `find().nodes({uuid in [10009, 1]}) as n return n`
	//uql := `find().nodes({@movie}) as nodes return nodes{*} limit 10`

	log.Println("Exec : ", uql)

	//resp, err := client.Uql(c.Uql, &configuration.RequestConfig{Graph: "multi_schema_test"})
	resp, err := client.Uql(uql, nil)

	if err != nil {
		t.Fatal(err)
	}
	printers.PrintAny(resp.Get(0))
	nodes, _, _ := resp.Get(0).AsNodes()

	log.Println(nodes)

}

func TestUQL3(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := `find().nodes({@test_schema2}) as n1 order by n1._uuid desc limit 1 return n1.test_timestamp`

	log.Println("Exec : ", uql)

	//resp, err := client.Uql(c.Uql, &configuration.RequestConfig{Graph: "multi_schema_test"})
	resp, err := client.Uql(uql, nil)

	if err != nil {
		t.Fatal(err)
	}

	attrs, _ := resp.Get(0).AsAttr()

	for _, row := range attrs.Values {
		log.Println(row)
	}

}

func TestUQL4(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := `ab().src({_uuid == 1}).dest({_uuid == 3}).depth(:2) as paths with pnodes(paths) as nodeArray uncollect nodeArray as node return distinct(node)`

	log.Println("Exec : ", uql)

	//resp, err := client.Uql(c.Uql, &configuration.RequestConfig{Graph: "multi_schema_test"})
	resp, err := client.Uql(uql, nil)

	if err != nil {
		t.Fatal(err)
	}
	printers.PrintAny(resp.Get(0))
}

func TestUQL5(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := `find().nodes({@account.year==1978 && @account.name=="念敏"}) as nodes return nodes{*} limit 1`

	log.Println("Exec : ", uql)

	//resp, err := client.Uql(c.Uql, &configuration.RequestConfig{Graph: "multi_schema_test"})
	resp, err := client.Uql(uql, nil)

	if err != nil {
		t.Fatal(err)
	}
	printers.PrintAny(resp.Get(0))
}

func TestUQL6(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := "find().nodes({@movie}) as nodes return table(nodes.timestamp,nodes.frating) limit 0"
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
		log.Println(resp.Status.Message)
		t.Log(resp.Status.Message)
	}
	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)

	table, err := resp.Alias("table(nodes.timestamp,nodes.frating)").AsTable()
	if err != nil {
		return
	}
	printers.PrintTable(table)
}

func TestUQLAlterGraph(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := "alter().graph('alter_graph_1').set({name:'alter_graph'})" //test123
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)
}

func TestUQLCompactWithNotExistGraph(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := `compact().graph("c1")`
	//ty, leader, follower, global, err := client.GetConnByUQL(uql, "random_test_js_1672976970614")
	//if err != nil {
	//	t.Fatal(err)
	//}
	//t.Logf("uql type:%v", ty)
	//t.Logf("leader:%s", leader.Host)
	//sb := strings.Builder{}
	//for _, f := range follower {
	//	sb.WriteString(f.Host)
	//	sb.WriteString(",")
	//}
	//t.Logf("followers:%s", sb.String())
	//
	//t.Logf("global leader:%s", global.Host)

	_, err := client.Uql(uql, &configuration.RequestConfig{
		Graph: "c1",
	})
	if err != nil {
		t.Fatalf("fail to compact:%v", err)
	}
}

func TestTopUql(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := `top()`

	log.Println("Exec : ", uql)

	//resp, err := client.Uql(c.Uql, &configuration.RequestConfig{Graph: "multi_schema_test"})
	resp, err := client.Uql(uql, nil)

	if err != nil {
		t.Fatal(err)
	}
	printers.PrintAny(resp.Get(0))
}
func TestUQLFindNodesWithList(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := `find().nodes() as nodes return nodes{*}`
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)

	nodes, schemas, err := resp.Alias("nodes").AsNodes()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintNodes(nodes, schemas)
}

func TestUQLFindNodesWithAttrList(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := "find().nodes() as nodes return collect(distinct(nodes)) as arrNode"
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)

	attrs, err := resp.Alias("arrNode").AsAttr()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintAttr(attrs)
}

func TestUQLFindNodesWithAttrListNullValue(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := "find().nodes({@nodeSchemaList}) as n return collect(n.stringList)"
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)

	attrs, err := resp.Alias("collect(n.stringList)").AsAttr()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintAttr(attrs)
}

func TestUQLFindNodesAsAttrList(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := "find().nodes({_uuid < 10}) return collect(nodes)"
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)

	attr, err := resp.Alias("collect(nodes)").AsAttr()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintAttr(attr)
}

func TestUQLFindPathsWithGroupByAttr(t *testing.T) {

	//client, _ := GetClient(hosts, graph)

	uql := "n({_uuid in [4,5,6]} as n1).e().n(as n2) as paths group by n1 return n1{*}, collect(paths)"
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	log.Println(resp.Statistic.EngineCost, "|", resp.Statistic.TotalCost)

	node, schemas, err := resp.Alias("n1").AsNodes()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintNodes(node, schemas)

	attr, err := resp.Alias("collect(paths)").AsAttr()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintAttr(attr)
}

func TestUqlInsertListProperty(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	uql := `insert().nodes({name:["zhangsan","lisi"]}).into(@People)`
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("insert nodes count : %d", resp.Statistic.TotalCost)
}

func TestUQLWithLimit(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	uql := "find().edges({@insertEdge}) as edges return edges{*} limit 40"
	//uql := "find().nodes({@insertNode}) as n return n{typeListString,typeListInt32,typeListInt64,typeListUint32,typeListUint64,typeListFloat,typeListDouble,typeListDatetime,typeListTimestamp,typeListText}"
	//uql := "find().nodes({@insertNode}) as nodes return nodes{*}"
	resp, _ := client.Uql(uql, nil)
	edges, e, _ := resp.Alias("edges").AsEdges()

	printers.PrintEdges(edges, e)
	//var nodes1 []*structs.Node
	//nodes1 = append(nodes1, nodes[0])
	//fmt.Println(nodes1)
}

func TestUqlFindPointProperty(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	uql := `find().nodes([11,12]) as nodes return nodes.typePoint`
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	attr, err := resp.Alias("nodes.typePoint").AsAttr()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintAttr(attr)
}

func TestOnePathAsPaths(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	//client.SetCurrentGraph("miniCircle")

	//var uql = "ab().src(51).dest(103).depth(1) as paths return paths{}"
	//var uql = "n().e()[2].n() as paths return paths{} limit 100"
	var uql = "find().nodes() as p return p{*}"
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := resp.Alias("p").AsGraph()
	//printers.PrintPaths(paths)
	if err != nil {
		t.Log(err)
	}

	t.Log(paths)
}
func TestDateTime(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	var uql = "find().nodes({@account}) as e return table(e.stringList) limit 10"
	resp, _ := client.Uql(uql, nil)
	table, _ := resp.Alias("table(e.stringList)").AsTable()
	printers.PrintTable(table)
}

func TestUqlPoint(t *testing.T) {
	//client, _ := GetClient([]string{"10.132.3.136:62061"}, "test")
	//uql := `find().nodes({@insertNode2}) as nodes return nodes{*} limit 10`

	client, _ := GetClient([]string{"192.168.1.85:61099"}, "test")
	uql := `find().nodes({@default}) as nodes return nodes{*} limit 10`
	resp, err := client.Uql(uql, nil)
	if err != nil {
		log.Fatalln(err)
	}
	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
		t.Fatal(resp.Status.Message)
	}
	nodes, schemas, err := resp.Alias("nodes").AsNodes()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintNodes(nodes, schemas)
}

func TestUqlBool(t *testing.T) {
	//client, _ := GetClient([]string{"10.132.3.136:62061"}, "test")
	//uql := `find().nodes({@insertNode2}) as nodes return nodes{*} limit 10`

	//client, _ := GetClient([]string{"192.168.1.85:61099"}, "sdk_test")
	//uql := `find().nodes({year >2022}) as nodes  RETURN nodes.cPoint LIMIT 8`
	//uql := `find().nodes({@country}) as nodes ORDER by nodes._uuid DESC RETURN nodes.cPoint LIMIT 8`
	uql := `n().e().n() as p limit 1 return collect(p)`
	resp, err := client.Uql(uql, nil)
	if err != nil {
		log.Fatalln(err)
	}
	if resp.Status.Code != ultipa.ErrorCode_SUCCESS {
		t.Fatal(resp.Status.Message)
	}
	nodes, err := resp.Alias("collect(p)").AsAttr()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintAttr(nodes)
}

func TestUqlFindWithDecimalProperty(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	uql := `find().nodes({@default}) as nodes return nodes{*}`
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	nodes, schemas, err := resp.Alias("nodes").AsNodes()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintNodes(nodes, schemas)
}

//func TestUqlAsGraph(t *testing.T) {
//	//client, _ := GetClient(hosts, graph)
//
//	uql := `n( as n1).re(as e).n(as n2) with toGraph(listUnion(collect(n1), collect(n2)), collect(e)) as graph return graph`
//	resp, err := client.Uql(uql, nil)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	graph, err := resp.Alias("graph").AsGraph()
//	if err != nil {
//		t.Fatal(err)
//	}
//	printers.PrintGraph(graph)
//}

func TestFindNodeWithOptionalUql(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	uql := "OPTIONAL find().nodes({@account.year < 1969}) as nodes return nodes{*}"
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	nodes, schemas, err := resp.Alias("nodes").AsNodes()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintNodes(nodes, schemas)
}
func TestFindEdgeWithOptionalUql(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	uql := "OPTIONAL find().edges({@disagree.targetPost <10}) as edges return edges{*}"
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	edges, schemas, err := resp.Alias("edges").AsEdges()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintEdges(edges, schemas)
}

func TestFindPathWithOptionalUql(t *testing.T) {
	//client, _ := GetClient(hosts, graph)
	uql := "OPTIONAL n(1).e().n(103) as paths RETURN paths{*}"
	resp, err := client.Uql(uql, nil)
	if err != nil {
		t.Fatal(err)
	}

	paths, err := resp.Alias("paths").AsPaths()
	if err != nil {
		t.Fatal(err)
	}
	printers.PrintPaths(paths)
}

func TestInsertReturnNodes(t *testing.T) {
	var graphName = "cli_test"

	var uql = "insert().into(@`node_schema_a`).nodes([{typeString:'string',name:'name',typeInt32:12,typeInt64:44,typeUint32:0}]) as node return node{*}"
	res, _ := client.Uql(uql, &configuration.RequestConfig{Graph: graphName})

	_, _, err := res.Alias("node").AsNodes()
	if err != nil {
		t.Fatalf("Insert ReturnNodes error, %v", err)
	}

	//nodes, schema, _ := res.Alias("node").AsNodes()
	//printers.PrintNodes(nodes, schema)
	//fmt.Println(res)
}

func TestUqlKhop(t *testing.T) {
	//var graphName = "cli_test"
	var uql = `khop().src({_id == "2069573"}).depth(1) as n return count(n)`

	totalRequests := 10000
	concurrency := 200

	var wg sync.WaitGroup

	sem := make(chan struct{}, concurrency)
	log.Println("start")
	start := time.Now()

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			sem <- struct{}{}

			_, _ = client.Uql(uql, nil)

			//_, _, err := res.Alias("node").AsNodes()
			//if err != nil {
			//    t.Fatalf("Insert ReturnNodes error, %v", err)
			//}
			//if res.Status != nil{
			//
			//}

			//fmt.Printf("Request #%d completed\n", i)

			<-sem
		}(i)
	}

	wg.Wait()
	log.Println("All requests completed, cost time:", time.Since(start).Seconds())
}

func TestUqlKhop2(t *testing.T) {
	var uql = `khop().src({_id == "2069573"}).depth(1) as n return count(n)`

	totalRequests := 10000
	//concurrency := 200

	var wg sync.WaitGroup

	//sem := make(chan struct{}, concurrency)
	log.Println("start")
	start := time.Now()

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			wg.Done()
			_, _ = client.Uql(uql, nil)

		}()
	}

	wg.Wait()
	log.Println("All requests completed, cost time:", time.Since(start).Seconds())
}

//TIP To run your code, right-click the code and select <b>Run</b>. Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.

func TestKK(t *testing.T) {

	config := &configuration.UltipaConfig{
		Hosts: []string{
			"192.168.1.88:63801",
		},
		Password:     "root",
		Username:     "root",
		DefaultGraph: "ldbc_tiger_sf100_ic_fix_type",
	}

	client, err := sdk.NewUltipaDriver(config)
	if err != nil {
		t.Error("connect failed,", err)
	}

	thread := 200
	wg := waitgroup.NewWaitGroup(thread)

	logger.PrintInfo("started")
	start := time.Now()
	totalTime := 0
	total := 10000
	finished := 0
	//mu := sync.RWMutex{}

	for i := 0; i < total; i++ {
		wg.BlockAdd()
		go func() {
			defer wg.Done()

			res, err := client.Uql(`khop().src({_id == "2069573"}).depth(1) as n return count(n)`, nil)

			if err != nil {
				fmt.Println(err)
			}

			if !res.Status.IsSuccess() {
				logger.PrintWarn(res.Status.Message)
			}
			//mu.Lock()
			finished++
			if finished%2000 == 0 {
				logger.PrintInfo(fmt.Sprintf("finished:%d / %d", finished, total))
			}
			totalTime += res.Statistic.TotalCost
			//mu.Unlock()

		}()
	}
	wg.Wait()

	end := time.Now()
	duration := end.Sub(start)

	fmt.Println("Thread", thread, "finishd", finished, "total time:", duration, "AVG per query(ms)", totalTime/total)
}

//TIP See GoLand help at <a href="https://www.jetbrains.com/help/go/">jetbrains.com/help/go/</a>.
// Also, you can try interactive lessons for GoLand by selecting 'Help | Learn IDE Features' from the main menu.
