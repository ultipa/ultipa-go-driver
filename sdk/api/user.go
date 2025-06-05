package api

import (
	"fmt"

	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/structs"
)

func (api *UltipaAPI) ShowUser(config *configuration.RequestConfig) (users []*structs.User, err error) {
	resp, err := api.Uql("show().user()", config)

	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	users, err = resp.Alias(http.RESP_USER_KEY).AsUsers()
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (api *UltipaAPI) GetUser(userName string, config *configuration.RequestConfig) (user *structs.User, err error) {
	if userName == "" {
		return nil, fmt.Errorf("userName is required")
	}
	uql := fmt.Sprintf(`show().user("%s")`, userName)
	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	users, err := resp.Alias(http.RESP_USER_KEY).AsUsers()
	if err != nil {
		return nil, err
	}

	// if user no exists， uql will return err : user [userName] does not exist
	user = users[0]

	return user, nil
}

func (api *UltipaAPI) CreateUser(user *structs.User, config *configuration.RequestConfig) (resp *http.Response, err error) {
	uql := user.ToCreateUserUql()
	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (api *UltipaAPI) AlterUser(user *structs.User, config *configuration.RequestConfig) (resp *http.Response, err error) {
	uql := user.ToAlterUserUql()
	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (api *UltipaAPI) DropUser(userName string, config *configuration.RequestConfig) (resp *http.Response, err error) {
	uql := fmt.Sprintf(`drop().user("%s")`, userName)
	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
