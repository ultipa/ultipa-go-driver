package http

import ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"

type StatusCode = int

const (
	StatusSUCCESS                    StatusCode = 0
	StatusFAILED                     StatusCode = 1
	StatusParamError                 StatusCode = 2
	StatusBaseDbError                StatusCode = 3
	StatusEngineError                StatusCode = 4
	StatusSystemError                StatusCode = 5
	StatusRaftRedirect               StatusCode = 6
	StatusRaftLeaderNotYetElected    StatusCode = 7
	StatusRaftLogError               StatusCode = 8
	StatusUqlError                   StatusCode = 9
	StatusNotRaftMode                StatusCode = 10
	StatusRaftNoAvailableFollowers   StatusCode = 11
	StatusRaftNoAvailableAlgoServers StatusCode = 12
	StatusPermissionDenied           StatusCode = 13
)

type Status struct {
	Message string
	Code    ultipa.ErrorCode
}

func (t *Status) IsSuccess() bool {
	return t.Code == ultipa.ErrorCode_SUCCESS
}
