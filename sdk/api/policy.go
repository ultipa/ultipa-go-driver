package api

import (
	"fmt"

	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func (api *UltipaAPI) ShowPolicy(config *configuration.RequestConfig) (policies []*structs.Policy, err error) {
	resp, err := api.Uql("show().policy()", config)
	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	policies, err = resp.Alias(http.RESP_POLICY_KEY).AsPolicies()
	if err != nil {
		return nil, err
	}

	return
}

func (api *UltipaAPI) GetPolicy(policyName string, config *configuration.RequestConfig) (policy *structs.Policy, err error) {
	if policyName == "" {
		return nil, fmt.Errorf("policyName is required")
	}

	uql := fmt.Sprintf(`show().policy("%s")`, policyName)
	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}
	if !resp.Status.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	policies, err := resp.Alias(http.RESP_POLICY_KEY).AsPolicies()
	if err != nil {
		return nil, err
	}
	if len(policies) != 0 {
		policy = policies[0]
	}

	return
}

func (api *UltipaAPI) CreatePolicy(policy *structs.Policy, config *configuration.RequestConfig) (resp *http.Response, err error) {
	if policy == nil {
		return nil, fmt.Errorf("policy cant be null")
	}

	if policy.Name == "" {
		return nil, fmt.Errorf("policy name is required")
	}

	uql := policy.ToCreatePolicyUql()
	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (api *UltipaAPI) AlterPolicy(policy *structs.Policy, config *configuration.RequestConfig) (resp *http.Response, err error) {
	uql := policy.ToAlterPolicyUql()
	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (api *UltipaAPI) DropPolicy(policyName string, config *configuration.RequestConfig) (resp *http.Response, err error) {
	uql := fmt.Sprintf(`drop().policy("%s")`, policyName)
	resp, err = api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
