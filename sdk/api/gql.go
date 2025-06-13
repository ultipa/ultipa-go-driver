package api

import (
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
)

func (api *UltipaAPI) Gql(gql string, config *configuration.RequestConfig) (*http.Response, error) {
	return api.query(gql, ultipa.QueryType_GQL, config)

}

func (api *UltipaAPI) GqlStream(gql string, cb func(*http.Response) error, config *configuration.RequestConfig) error {
	return api.queryStream(gql, ultipa.QueryType_GQL, cb, config)
}
