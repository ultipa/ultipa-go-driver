/**
 * Returns Uql Results by one time
 */

package http

import (
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/types"
)

type InsertResponse struct {
	UUIDs      []types.UUID
	IDs        []types.ID
	ErrorItems map[int]string // index : error code
	Statistic  *Statistic
	Status     *Status
}

var InsertErrorCodeMsgMap = map[int]string{
	//10001: "uuid and id not match",
	//10002: "id and schema not match",
	//10003: "from_id not exist",
	//10004: "to_id not exist",
	//10005: "id length exceed max length(128 bytes)",
	//10007: "duplicate ids in the data or the ids already exist in the database",
	// update in 2024-11-12
	//10001: "uuid and id not match",
	//10002: "id and schema not match",
	//10003: "from_id not exist",
	//10004: "to_id not exist",
	//10005: "id length exceed max length (128 bytes)",
	//10006: "some required field is null",
	//10007: "duplicate ids in the data or the ids already exist in the database",
	//10008: "id is empty",
	//10009: "from_id is empty",
	//10010: "to_id is empty",
	//10011: "duplicate id found in the data",
	//11001: "operation succeeded but the id already existed",
	//19999: "other error",
	// update in 2025-05-06
	10001: "ID_NOT_MATCH_UUID: id and uuid do not match",
	10002: "ID_UUID_NOT_MATCH_SCHEMA: id and schema do not match",
	10003: "FROM_ID_NOT_EXISTED: fromId does not exist",
	10004: "TO_ID_NOT_EXISTED: toId does not exist",
	10005: "ID_LEN: length of id exceeds the maximum length (128 bytes)",
	10006: "NOT_NULL: violation of the NOT NULL constraint",
	10007: "UNIQUCHECK: violation of the UNIQUE constraint",
	10008: "ID_EMPTY: id cannot be empty",
	10009: "FROM_ID_EMPTY: fromId cannot be empty",
	10010: "TO_ID_EMPTY: toId cannot be empty",
	10011: "DUPLICATE_ID: duplicated id",
	10012: "KEY_CONSTRAINT_VIOLATED: violation of the EDGE KEY constraint",
	10013: "INVALID_FORMAT: invalid format",
	11001: "OK_BUT_ID_EXISTED: id already exists",
	19999: "OTHERS: other error",
}

var InsertErrorCodeMsgMap2 = map[int]string{
	10001: "ID_NOT_MATCH_UUID",
	10002: "ID_UUID_NOT_MATCH_SCHEMA",
	10003: "FROM_ID_NOT_EXISTED",
	10004: "TO_ID_NOT_EXISTED",
	10005: "ID_LEN",
	10006: "NOT_NULL",
	10007: "UNIQUCHECK",
	10008: "ID_EMPTY",
	10009: "FROM_ID_EMPTY",
	10010: "TO_ID_EMPTY",
	10011: "DUPLICATE_ID",
	10012: "KEY_CONSTRAINT_VIOLATED",
	10013: "INVALID_FORMAT",
	11001: "OK_BUT_ID_EXISTED",
	19999: "OTHERS",
}

var InsertErrorCodeMsgMap3 = map[string]int{
	"ID_NOT_MATCH_UUID":        10001,
	"ID_UUID_NOT_MATCH_SCHEMA": 10002,
	"FROM_ID_NOT_EXISTED":      10003,
	"TO_ID_NOT_EXISTED":        10004,
	"ID_LEN":                   10005,
	"NOT_NULL":                 10006,
	"UNIQUCHECK":               10007,
	"ID_EMPTY":                 10008,
	"FROM_ID_EMPTY":            10009,
	"TO_ID_EMPTY":              10010,
	"DUPLICATE_ID":             10011,
	"KEY_CONSTRAINT_VIOLATED":  10012,
	"INVALID_FORMAT":           10013,
	"OK_BUT_ID_EXISTED":        11001,
	"OTHERS":                   19999,
}

func NewNodesInsertResponse(reply *ultipa.InsertNodesReply) (response *InsertResponse, err error) {

	response = &InsertResponse{
		Status: &Status{
			Message: reply.Status.Msg,
			Code:    reply.Status.ErrorCode,
		},
		Statistic: &Statistic{
			TotalCost:  int(reply.TimeCost),
			EngineCost: int(reply.EngineTimeCost),
		},
		UUIDs:      reply.Uuids,
		IDs:        reply.Ids,
		ErrorItems: make(map[int]string),
	}

	if len(reply.IgnoreIndexes) != len(reply.IgnoreErrorCode) {
		return response, nil
	}

	for index := range reply.IgnoreIndexes {
		v := int(reply.IgnoreIndexes[index])
		code := reply.IgnoreErrorCode[index]
		response.ErrorItems[v] = InsertErrorCodeMsgMap2[int(code)]
	}

	return response, nil
}

func NewEdgesInsertResponse(reply *ultipa.InsertEdgesReply) (response *InsertResponse, err error) {

	response = &InsertResponse{
		Status: &Status{
			Message: reply.Status.Msg,
			Code:    reply.Status.ErrorCode,
		},
		Statistic: &Statistic{
			TotalCost:  int(reply.TimeCost),
			EngineCost: int(reply.EngineTimeCost),
		},
		UUIDs:      reply.Uuids,
		ErrorItems: make(map[int]string),
	}

	for index := range reply.IgnoreIndexes {
		v := int(reply.IgnoreIndexes[index])
		code := reply.IgnoreErrorCode[index]
		response.ErrorItems[v] = InsertErrorCodeMsgMap2[int(code)]
	}

	return response, nil
}
