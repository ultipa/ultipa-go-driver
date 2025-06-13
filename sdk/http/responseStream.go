/**
 * Return a Stream to return Uql results
 */

package http

import (
	"fmt"
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"io"
)

func NewUQLResponseStream(resp ultipa.UltipaRpcs_QueryClient) (response *Response, err error) {

	response = &Response{
		Resp:   resp,
		Status: &Status{},
		DataItemMap: map[string]struct {
			DataItem *DataItem
			Index    int
		}{},
	}

	return response, nil
}

func (r *Response) Recv(cb func(*Response) error) (err error) {
	//if !fetch {
	//	return nil, r.Resp.CloseSend()
	//}
	defer func() {
		_ = r.Resp.CloseSend()
	}()

	for {
		response := &Response{
			Status: &Status{},
			DataItemMap: map[string]struct {
				DataItem *DataItem
				Index    int
			}{},
		}

		record, err := r.Resp.Recv()

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if response.Statistic == nil {
			response.Statistic, err = ParseStatistic(record.Statistics)
			if err != nil {
				return err
			}
		}

		if response.ExplainPlan == nil {
			response.ExplainPlan, err = ParseExplainPlan(record.ExplainPlan)
			if err != nil {
				return err
			}
		}

		response.Reply = record
		if record.Status != nil {
			response.Status.Code = record.Status.ErrorCode
			response.Status.Message = record.Status.Msg
			if response.Status.Code != ultipa.ErrorCode_SUCCESS {
				return fmt.Errorf(response.Status.Message)
			}
		}

		var aliasList []string

		for _, alias := range response.Reply.Alias {
			aliasList = append(aliasList, alias.GetAlias())
		}
		response.AliasList = aliasList

		if err := cb(response); err != nil {
			return err
		}

	}

	return nil
}
