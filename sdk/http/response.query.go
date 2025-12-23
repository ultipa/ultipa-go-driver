/**
 * Returns Uql Results by one time
 */

package http

import (
	"io"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils"
)

type Response struct {
	DataItemMap map[string]struct {
		DataItem *DataItem
		Index    int
	}
	Reply         *ultipa.QueryReply
	Status        *Status
	Statistic     *Statistic
	ExplainPlan   *ExplainPlan
	AliasList     []string
	Resp          ultipa.UltipaRpcs_QueryClient
	TransactionID uint64 // Transaction ID returned from START TRANSACTION (0 = no transaction)
}

type ResponseWithExistCheck struct {
	Exist    bool
	Response *Response
}

func NewUQLResponse(resp ultipa.UltipaRpcs_QueryClient) (response *Response, err error) {

	response = &Response{
		Resp:   resp,
		Status: &Status{},
		//Reply:  &ultipa.QueryReply{},
		DataItemMap: map[string]struct {
			DataItem *DataItem
			Index    int
		}{},
	}
	for {

		record, err := resp.Recv()

		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		if response.Statistic == nil {
			response.Statistic, err = ParseStatistic(record.Statistics)
			if err != nil {
				return nil, err
			}
		}

		if response.ExplainPlan == nil {
			response.ExplainPlan, err = ParseExplainPlan(record.ExplainPlan)
			if err != nil {
				return nil, err
			}
		}

		if response.Reply == nil {
			response.Reply = record
			//for _, alias := range record.Alias {
			//	if alias.GetResultType() == ultipa.ResultType_RESULT_TYPE_UNSET {
			//		response.Reply = utils.MergeUQLReply(response.Reply, record)
			//	}
			//}
		} else {
			response.Reply = utils.MergeUQLReply(response.Reply, record)
		}

		if record.Status != nil {
			response.Status.Code = record.Status.ErrorCode
			response.Status.Message = record.Status.Msg
		}

		// Extract transaction ID if present (s5.3 feature)
		if record.TransactionId != 0 {
			response.TransactionID = record.TransactionId
		}

		if response.Status.Code != ultipa.ErrorCode_SUCCESS {
			return response, nil
		}
	}

	var aliasList []string

	for _, alias := range response.Reply.Alias {
		aliasList = append(aliasList, alias.GetAlias())
	}

	response.AliasList = aliasList

	return response, nil
}

func (r *Response) NeedRedirect() bool {
	return r.Status.Code == ultipa.ErrorCode_RAFT_REDIRECT
}

func (r *Response) IsSuccess() bool {
	if r != nil && r.Status != nil {
		return r.Status.Code == ultipa.ErrorCode_SUCCESS
	}
	return false

}

func (r *Response) Get(index int) (di *DataItem) {
	if len(r.AliasList) > index {
		return r.Alias(r.AliasList[index])
	}
	return &DataItem{
		Data: nil,
		Type: ultipa.ResultType_RESULT_TYPE_UNSET,
	}
}

func (r *Response) Alias(alias string) *DataItem {
	data, t := utils.FindAliasDataInReply(r.Reply, alias)
	return &DataItem{
		Alias: alias,
		Data:  data,
		Type:  t,
	}
}

func (r *Response) GetSingleTable() (*structs.Table, error) {
	di := r.Get(0)
	if di != nil {
		t, err := di.AsTable()
		return t, err
	}
	return nil, nil
}

// GetTransactionID returns the transaction ID from the response
func (r *Response) GetTransactionID() uint64 {
	return r.TransactionID
}

// HasTransaction returns true if response contains a transaction ID
func (r *Response) HasTransaction() bool {
	return r.TransactionID != 0
}
