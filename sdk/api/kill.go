package api

import (
	"fmt"

	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
)

func (api *UltipaAPI) Kill(processId string, config *configuration.RequestConfig) (resp *http.Response, err error) {
	if processId == "" {
		return nil, fmt.Errorf("processId can not empty")
	}
	uql := fmt.Sprintf(`kill("%s")`, processId)

	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, err
}
