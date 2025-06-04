package api

import (
	"fmt"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func (api *UltipaAPI) Top(config *configuration.RequestConfig) (tops []*structs.Process, err error) {
	resp, err := api.Uql("top()", config)

	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	return resp.Alias(http.RESP_TOP_KEY).AsProcesses()
}
