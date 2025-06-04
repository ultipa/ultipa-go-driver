package api

import (
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
)

func (api *UltipaAPI) Stats(config *configuration.RequestConfig) (stats *http.Response, err error) {
	//return api.license("stats()",config)
	return api.Uql("stats()", config)
}
