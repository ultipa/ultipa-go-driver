package api

import (
	"fmt"

	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
	"github.com/ultipa/ultipa-go-driver/sdk/utils"
)

// Deprecated: 5.0 not support, should use ShowJob
func (api *UltipaAPI) ShowTask(algoNameOrId string, status structs.TaskStatus, config *configuration.RequestConfig) (tasks []*structs.Task, err error) {
	uql := ""
	if len(algoNameOrId) == 0 {
		uql = "show().task()"
	} else if utils.IsAllDigits(algoNameOrId) {
		uql = fmt.Sprintf("show().task(%v)", algoNameOrId)
	} else {
		uql = fmt.Sprintf(`show().task("%v","%v")`, algoNameOrId, status.String())
	}

	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	tasks, err = resp.Alias(http.RESP_TASK_KEY).AsTasks()

	return tasks, err
}

func (api *UltipaAPI) ClearTask(algoNameOrId string, status structs.TaskStatus, config *configuration.RequestConfig) (resp *http.Response, err error) {
	uql := ""
	if len(algoNameOrId) == 0 {
		uql = `clear().task("*")`
	} else if utils.IsAllDigits(algoNameOrId) {
		uql = fmt.Sprintf("clear().task(%v)", algoNameOrId)
	} else {
		uql = fmt.Sprintf(`clear().task("%v","%v")`, algoNameOrId, status.String())
	}

	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (api *UltipaAPI) StopTask(id string, config *configuration.RequestConfig) (resp *http.Response, err error) {
	uql := ""
	if len(id) == 0 || id == "*" {
		uql = `stop().task("*")`
	} else if utils.IsAllDigits(id) {
		uql = fmt.Sprintf("stop().task(%v)", id)
	} else {
		uql = fmt.Sprintf(`stop().task("%v")`, id)
	}

	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
