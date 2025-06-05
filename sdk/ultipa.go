// Package sdk provide Ultipa functions to drive ultipa servers
package sdk

import (
	"github.com/ultipa/ultipa-go-driver/sdk/api"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/connection"
)

// Version represents the current version of the SDK.
const Version = "5.1.1-s5.1"

// NewUltipaDriver Create an Ultipa Client
func NewUltipaDriver(config *configuration.UltipaConfig) (*api.UltipaAPI, error) {

	config.FillDefault()

	encryptedPwd, err := configuration.Encrypt(config.PasswordEncrypt, config.Password)
	if err != nil {
		return nil, err
	}

	config.Password = encryptedPwd

	// set connection pool
	pool, err := connection.NewConnectionPool(config)
	if err != nil {
		return nil, err
	}
	// set heartbeat for Connection Pool
	//pool.RunHeartBeat()
	//
	//if err != nil {
	//    return nil, err
	//}

	return api.NewUltipaAPI(pool), err
}
