package api

import (
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

func (api *UltipaAPI) SetUserSetting(request *structs.SetUserSetting, config *configuration.RequestConfig) (*http.Response, error) {
	var err error

	client, err := api.GetControlClient(config)
	if err != nil {
		return nil, err
	}

	ctx, cancel, err := api.Pool.NewContext(config)
	if err != nil {
		return nil, err
	}
	defer cancel()

	res, err := client.UserSetting(ctx, &ultipa.UserSettingRequest{
		UserName: request.UserName,
		Opt:      ultipa.UserSettingRequest_OPT_SET,
		Type:     request.Type,
		Data:     request.Data,
	})
	if err != nil {
		return nil, err
	}

	//data := &http.DataItem{
	//	Alias: setUserSetting.UserName,
	//	Data:  res.Data,
	//}

	resp := &http.Response{
		Status: &http.Status{
			Message: res.Status.Msg,
			Code:    res.Status.ErrorCode,
		},
		//DataItemMap: map[string]struct {
		//	DataItem *http.DataItem
		//	Index    int
		//}{setUserSetting.UserName: {DataItem: data}},
	}

	return resp, err
}

func (api *UltipaAPI) GetUserSetting(request *structs.GetUserSetting, config *configuration.RequestConfig) (*http.Response, error) {
	var err error

	client, err := api.GetControlClient(config)
	if err != nil {
		return nil, err
	}

	ctx, cancel, err := api.Pool.NewContext(config)
	if err != nil {
		return nil, err
	}
	defer cancel()

	res, err := client.UserSetting(ctx, &ultipa.UserSettingRequest{
		UserName: request.UserName,
		Opt:      ultipa.UserSettingRequest_OPT_GET,
		Type:     request.Type,
	})
	if err != nil {
		return nil, err
	}

	data := &http.DataItem{
		Alias: "GetUserSetting",
		Data:  res.Data,
	}

	resp := &http.Response{
		Status: &http.Status{
			Message: res.Status.Msg,
			Code:    res.Status.ErrorCode,
		},
		DataItemMap: map[string]struct {
			DataItem *http.DataItem
			Index    int
		}{request.UserName: {DataItem: data}},
	}

	return resp, err
}
