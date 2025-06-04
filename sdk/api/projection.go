package api

import (
	"fmt"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func (api *UltipaAPI) ShowProjection(config *configuration.RequestConfig) (projections []*structs.Projection, err error) {
	resp, err := api.Uql("show().projection()", config)

	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	projections, err = resp.Alias(http.RESP_PROJECTION_KEY).AsProjections()

	return projections, err
}
