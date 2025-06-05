package connection

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"google.golang.org/grpc/metadata"
	"time"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Connection struct {
	Host   string
	Conn   *grpc.ClientConn
	Client ultipa.UltipaRpcsClient
	Config *configuration.UltipaConfig
	Role   ultipa.FollowerRole // leader, follower, learner, candidate ...
	Active ultipa.ServerStatus
}

func NewConnection(host string, config *configuration.UltipaConfig) (*Connection, error) {
	var err error
	if len(config.Hosts) < 1 {
		return nil, errors.New("hosts can not be empty")
	}

	connection := &Connection{
		Config: config,
		Host:   host,
	}

	// add default mac receive size
	if config.MaxRecvSize == 0 {
		config.MaxRecvSize = configuration.DefaultRecvSize
	}

	// Try to get a certificate
	certificate := utils.GetCertificate(host)
	if config.Crt == nil && certificate != nil {
		cred := credentials.NewTLS(nil)
		connection.Conn, err = grpc.Dial(host, grpc.WithTransportCredentials(cred), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(config.MaxRecvSize), grpc.MaxCallSendMsgSize(config.MaxRecvSize)))
	} else if config.Crt == nil {
		connection.Conn, err = grpc.Dial(host, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(config.MaxRecvSize), grpc.MaxCallSendMsgSize(config.MaxRecvSize)))
	} else {
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(config.Crt)
		cred := credentials.NewTLS(&tls.Config{
			RootCAs: certPool,
		})
		connection.Conn, err = grpc.Dial(host, grpc.WithTransportCredentials(cred), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(config.MaxRecvSize)))
	}

	if err != nil {
		return nil, err
	}

	return connection, err
}

func (conn *Connection) GetClient() ultipa.UltipaRpcsClient {
	return ultipa.NewUltipaRpcsClient(conn.Conn)
}

func (conn *Connection) GetControlClient() ultipa.UltipaControlsClient {
	return ultipa.NewUltipaControlsClient(conn.Conn)
}

//func (conn *Connection) SetRole(role ultipa.FollowerRole) {
//	conn.Role = role
//}

//func (conn *Connection) SetRoleFromInt32(role int32) {
//	conn.Role = ultipa.FollowerRole(role)
//}

//func (conn *Connection) HasRole(role ultipa.FollowerRole) bool {
//	return (conn.Role & role) != 0
//}

func (conn *Connection) Close() error {
	return conn.Conn.Close()
}

func (conn *Connection) NewContext(config *configuration.RequestConfig) (ctx context.Context, cancel context.CancelFunc, err error) {

	if config == nil {
		config = &configuration.RequestConfig{}
	} else if config.Timezone != "" {
		_, err = time.LoadLocation(config.Timezone)
		if err != nil {
			return nil, nil, err
		}
	}

	timeout := config.Timeout

	if timeout == 0 {
		timeout = conn.Config.Timeout
	}

	if timeout == 0 {
		timeout = configuration.DefaultTimeout
	}

	if timeout < 0 {
		parentCtx := context.Background()
		ctx, cancel = context.WithCancel(parentCtx)
	} else {
		if timeout < 10 {
			timeout = 10
		}
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(conn.Config.ToContextKV(config)...))
	return ctx, cancel, nil
}
