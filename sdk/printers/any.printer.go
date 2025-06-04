package printers

import (
	"fmt"
	"github.com/ultipa/ultipa-go-driver/sdk/utils/logger"
	"log"
	"strings"

	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
)

func PrintAny(dataitem *http.DataItem) {
	if dataitem == nil {
		fmt.Println("No dataItem found.")
		return
	}

	if dataitem.Data == nil {

		fmt.Println(dataitem.Type.String() + ": No Return Data")
		return
	}

	switch dataitem.Type {
	case ultipa.ResultType_RESULT_TYPE_NODE:
		nodes, schemas, _ := dataitem.AsNodes()
		PrintNodes(nodes, schemas)
	case ultipa.ResultType_RESULT_TYPE_EDGE:
		edges, schemas, _ := dataitem.AsEdges()
		PrintEdges(edges, schemas)
	case ultipa.ResultType_RESULT_TYPE_TABLE:
		//handle other table
		res, err := dataitem.AsTable()

		// handle schema table
		if strings.Contains(res.Name, http.RESP_NODE_SCHEMA_KEY) || strings.Contains(res.Name, http.RESP_EDGE_SCHEMA_KEY) {
			schemas, err := dataitem.AsSchemas()

			if err != nil {
				logger.PrintWarn(err.Error())
				//log.Fatalln(err)
			}

			PrintSchema(schemas)
			return
		}

		// handle algo table
		// comment in 5.0 no support algo
		//if strings.Contains(res.Name, http.RESP_ALGOS_KEY) {
		//    algos, err := dataitem.AsAlgos()
		//
		//    if err != nil {
		//        logger.PrintWarn(err.Error())
		//        //log.Fatalln(err)
		//    }
		//
		//    PrintAlgoList(algos)
		//    return
		//
		//}

		if err != nil {
			logger.PrintWarn(err.Error())
			//log.Fatalln(err)
		}

		PrintTable(res)
	case ultipa.ResultType_RESULT_TYPE_PATH:
		paths, err := dataitem.AsPaths()

		if err != nil {
			logger.PrintWarn(err.Error())
			//log.Fatalln(err)
		}

		PrintPaths(paths)
	case ultipa.ResultType_RESULT_TYPE_ATTR:
		attr, err := dataitem.AsAttr()
		if err != nil {
			logger.PrintWarn(err.Error())
			//log.Fatalln(err)
		}

		PrintAttr(attr)

	//case ultipa.ResultType_RESULT_TYPE_ARRAY:
	//	arr, err := dataitem.AsArray()
	//	if err != nil {
	//		log.Fatalln(err)
	//	}
	//
	//	PrintArray(arr)
	default:
		log.Printf("Printer Got UnHandled Alias %v DBType %v \n", dataitem.Alias, dataitem.Type)
	}
}
