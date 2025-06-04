package api

import (
	"fmt"

	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func (api *UltipaAPI) ShowJob(id string, config *configuration.RequestConfig) (jobs []*structs.Job, err error) {
	uql := fmt.Sprintf("show().job(%v)", id)

	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	jobs, err = resp.Alias(http.RESP_JOB_KEY).AsJobs()

	return jobs, err
}

func (api *UltipaAPI) ClearJob(id string, config *configuration.RequestConfig) (resp *http.Response, err error) {
	if id == "*" {
		id = `"*"`
	}
	uql := fmt.Sprintf("clear().job(%v)", id)

	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (api *UltipaAPI) StopJob(id string, config *configuration.RequestConfig) (resp *http.Response, err error) {
	uql := fmt.Sprintf("stop().job(%v)", id)

	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
