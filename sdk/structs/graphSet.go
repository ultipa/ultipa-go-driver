package structs

import (
	"errors"
	"strings"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/types"
)

type TruncateParams struct {
	GraphName  string
	SchemaName string
	DBType     *ultipa.DBType
}

type GraphSet struct {
	ID          types.ID
	Name        string
	TotalNodes  uint64
	TotalEdges  uint64
	Description string
	Status      string
	Shards      []string
	SlotNum     int
	//ReplicaNum  string
	PartitionBy string
}

func GetDBTypeByString(str string) (ultipa.DBType, error) {

	switch strings.ToLower(str) {
	case "node":
		return ultipa.DBType_DBNODE, nil
	case "edge":
		return ultipa.DBType_DBEDGE, nil
	}

	return 0, errors.New("DBType is not Exist : " + str)
}

func DBTypeToString(dbType ultipa.DBType) string {

	switch dbType {
	case ultipa.DBType_DBNODE:
		return "node"
	case ultipa.DBType_DBEDGE:
		return "edge"
	}

	return ""
}

//
//func (db ultipa.DBType) ToString(){
//
//}
