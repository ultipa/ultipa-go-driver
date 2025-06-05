package connection

import (
	"context"
	"errors"
	"fmt"
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils/logger"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/metadata"
	"reflect"
	"strings"
	"sync"
	"time"
)

// //type GraphClusterInfo struct {
// //   Graph         string
// //   Leader        *Connection
// //   Followers     []*Connection
// //   Algos         []*Connection
// //   LastAlgoIndex int // Record the index of the last Task node used
// //}
//
// // handle all connections
type ConnectionPool struct {
	Config          *configuration.UltipaConfig
	Connections     map[string]*Connection // Host : Connection
	RandomTick      int
	Actives         []*Connection
	LastActivesTime time.Time
	muActiveSafely  sync.Mutex
}

func NewConnectionPool(config *configuration.UltipaConfig) (*ConnectionPool, error) {

	if len(config.Hosts) < 1 {
		return nil, errors.New("error Hosts can not by empty")
	}

	pool := &ConnectionPool{
		Config:      config,
		Connections: map[string]*Connection{},
	}

	// Init Cluster Manager
	// Get Connections
	err := pool.CreateConnections()

	if err != nil {
		logger.PrintError(err.Error())
		return nil, err
	}

	// Refresh Actives
	err = pool.RefreshActives()
	if err != nil {
		logger.PrintError(err.Error())
		return nil, err
	}
	// Refresh global Cluster info
	err = pool.RefreshClusterInfo("global")

	if err != nil {
		logger.PrintError(err.Error())
	}

	return pool, err
}

func (pool *ConnectionPool) CreateConnections() error {
	var err error

	for _, host := range pool.Config.Hosts {
		conn, _ := NewConnection(host, pool.Config)
		pool.Connections[host] = conn
	}

	return err
}

func (pool *ConnectionPool) RefreshActivesWithSeconds(seconds int32) error {
	pool.muActiveSafely.Lock()
	defer pool.muActiveSafely.Unlock()
	if time.Now().Sub(pool.LastActivesTime) <= 5*time.Second && len(pool.Connections) == len(pool.Actives) {
		// Avoid frequent refreshing
		return nil
	}
	defer func() {
		pool.LastActivesTime = time.Now()
	}()
	pool.Actives = []*Connection{}
	if seconds <= 0 {
		seconds = 3
	}
	var hosts []string
	connErrors := make([]error, len(pool.Connections))
	var connections []*Connection
	for host, connection := range pool.Connections {
		hosts = append(hosts, host)
		connections = append(connections, connection)
	}

	//var wg sync.WaitGroup
	var eg errgroup.Group
	for idx, conn := range connections {
		//wg.Add(1)
		localConn := conn
		eg.Go(func() error {
			//defer wg.Done()
			ctx, cancel, err := pool.NewContext(&configuration.RequestConfig{
				Timeout: seconds,
			})
			if err != nil {
				logger.PrintWarn(localConn.Host + " failed - " + err.Error())
				localConn.Active = ultipa.ServerStatus_DEAD
				connErrors[idx] = err
				return nil
			}
			defer cancel()

			resp, err := localConn.GetControlClient().SayHello(ctx, &ultipa.HelloUltipaRequest{
				Name: "go sdk refresh",
			})

			if err != nil {
				logger.PrintWarn(localConn.Host + " failed - " + err.Error())
				localConn.Active = ultipa.ServerStatus_DEAD
				connErrors[idx] = err
				// this connection failed, try next, so return nil here to bypass errgroup.
				return nil
			}

			if resp.Status == nil || resp.Status.ErrorCode == ultipa.ErrorCode_SUCCESS {
				localConn.Active = ultipa.ServerStatus_ALIVE
				pool.Actives = append(pool.Actives, localConn)
				connErrors[idx] = nil
			} else if resp.Status.ErrorCode == ultipa.ErrorCode_PERMISSION_DENIED && strings.Contains(resp.Status.Msg, "username does not exist or password is wrong") {
				logger.PrintWarn(localConn.Host + " failed - " + resp.Status.Msg)
				localConn.Active = ultipa.ServerStatus_DEAD
				err = errors.New(resp.Status.Msg)
				connErrors[idx] = err
				// username and password mismatch error, not necessary to try next conn, fail via errgroup
				return err
			} else {
				logger.PrintWarn(conn.Host + " failed - " + resp.Status.Msg)
				localConn.Active = ultipa.ServerStatus_DEAD
				connErrors[idx] = errors.New(resp.Status.Msg)
			}
			return nil
		})
	}
	//wg.Wait()
	if err := eg.Wait(); err != nil {
		return err
	}
	isTcpErr := true
	for idx, connError := range connErrors {
		if connError == nil {
			//any connection success, will pass.
			return nil
		}
		//connection error: desc = "transport: Error while dialing dial tcp xxx.xxx.1.xx:xxxxx: connectex: No connection could be made because the target machine actively refused it."
		if !strings.Contains(connError.Error(), "Error while dialing dial tcp") {
			isTcpErr = false
			logger.PrintError(fmt.Sprintf("failed to connect to host %s: %v", hosts[idx], connError))
		}
	}
	if isTcpErr {
		return errors.New(`transport: Error while dialing dial tcp with all hosts: No connection could be made because the target machine actively refused`)
	} else {
		return errors.New(`failed to connect to all hosts`)
	}
}

// // RefreshActives Update to see which connections are still valid
func (pool *ConnectionPool) RefreshActives() error {
	return pool.RefreshActivesWithSeconds(6)
}

//func (pool *ConnectionPool) ForceRefreshClusterInfo(graphName string) error {
//    pool.GraphMgr.DeleteGraph(graphName)
//    return pool.RefreshClusterInfo(graphName)
//}

// RefreshClusterInfo sync cluster info from server

func (pool *ConnectionPool) RefreshClusterInfo(graphName string) error {
	err := pool.doRefreshClusterInfo(graphName)
	if err != nil && reflect.TypeOf(err).Elem().String() == "utils.LeaderNotYetElectedError" {
		// If the leader has not selected the wrong type, try again
		err = pool.RefreshActives()
		if err != nil {
			return err
		}
		err = pool.doRefreshClusterInfo(graphName)
	}
	return err
}

func (pool *ConnectionPool) doRefreshClusterInfo(graphName string) error {
	//var conn *Connection

	var err error

	activeConns := pool.Actives

	if len(pool.Actives) < 1 {
		return errors.New("no active connection is found")
	}

	allIsNill := true
	for _, activeConn := range activeConns {
		if activeConn == nil {
			continue
		}
		allIsNill = false
		//If the graph is not initialized yet
		//if pool.GraphMgr.GetLeader(graphName) == nil {
		//    conn = activeConn
		//} else {
		//    // After initialization
		//    conn = pool.GraphMgr.GetLeader(graphName)
		//}
		//if pool.Config.Debug && conn != nil {
		//	logger.PrintDebug(fmt.Sprintf("refresh graph [%s] cluster info with connection to host [%s]", graphName, conn.Host))
		//}
		//err = pool.resolveClusterInfo(graphName, conn)
		//if err == nil {
		//    return nil
		//}
	}
	if allIsNill {
		err = errors.New("no active connection exists")
	}
	return err
}

// resolveClusterInfo resolve graphName cluster info with connection conn
// func (pool *ConnectionPool) resolveClusterInfo(graphName string, conn *Connection) error {
//
//	   ctx, cancel, err := pool.NewContext(&configuration.RequestConfig{Graph: graphName})
//	   defer cancel()
//	   if err != nil {
//	       return err
//	   }
//	   client := conn.GetControlClient()
//	   resp, err := client.GetLeader(ctx, &ultipa.GetLeaderRequest{})
//
//	   if err != nil {
//	       return err
//	   }
//
//	   if resp == nil {
//	       return errors.New("no resp when get leader")
//	   }
//
//	   if resp.Status.ErrorCode == ultipa.ErrorCode_NOT_RAFT_MODE {
//	       pool.IsRaft = false
//	       pool.GraphMgr.SetLeader(graphName, conn)
//	       return nil
//	   }
//
//	   if resp.Status.ErrorCode == ultipa.ErrorCode_RAFT_REDIRECT {
//	       pool.IsRaft = true
//	       if pool.Connections[resp.Status.ClusterInfo.Redirect] == nil {
//	           c, err := NewConnection(resp.Status.ClusterInfo.Redirect, pool.Config)
//	           if err != nil {
//	               return err
//	           }
//	           pool.Connections[resp.Status.ClusterInfo.Redirect] = c
//	       }
//	       pool.GraphMgr.SetLeader(graphName, pool.Connections[resp.Status.ClusterInfo.Redirect])
//	       err = pool.RefreshActives()
//	       if err != nil {
//	           return err
//	       }
//	       return nil
//	   }
//
//	   if resp.Status.ErrorCode == ultipa.ErrorCode_RAFT_LEADER_NOT_YET_ELECTED {
//	       pool.IsRaft = true
//	       time.Sleep(time.Millisecond * 300)
//	       return utils.NewLeaderNotYetElectedError("")
//	   }
//
//	   if resp.Status.ErrorCode == ultipa.ErrorCode_SUCCESS {
//	       pool.IsRaft = true
//	       c := pool.Connections[resp.Status.ClusterInfo.LeaderAddress]
//	       pool.GraphMgr.SetLeader(graphName, c)
//	       pool.GraphMgr.ClearFollower(graphName)
//	       for _, follower := range resp.Status.ClusterInfo.Followers {
//	           fconn := pool.Connections[follower.Address]
//	           if fconn == nil {
//	               fconn2, err5 := NewConnection(follower.Address, pool.Config)
//	               if err5 != nil {
//	                   continue
//	               }
//	               fconn = fconn2
//	               pool.Connections[follower.Address] = fconn
//	           }
//	           fconn.Host = follower.Address
//	           fconn.Active = follower.Status
//	           fconn.SetRoleFromInt32(follower.Role)
//	           pool.GraphMgr.AddFollower(graphName, fconn)
//	       }
//	       err = pool.RefreshActives()
//	       if err != nil {
//	           return err
//	       }
//	       return nil
//	   } else {
//	       err = errors.New(fmt.Sprintf("falied to refresh cluster of graph %s: %v,%s", graphName, resp.Status.ErrorCode, resp.Status.Msg))
//	   }
//	   return err
//	}
//
// Get client by global config
func (pool *ConnectionPool) GetConn(config *configuration.UltipaConfig) (*Connection, error) {

	//if pool.Config.Consistency {
	//	return pool.GetMasterConn(config)
	//} else {
	return pool.GetRandomConn(config)
	//}
}

// // Get Master of Global Graph
// //func (pool *ConnectionPool) GetGlobalMasterConn(config *configuration.UltipaConfig) (*Connection, error) {
// //    globalGraph := "global"
// //    if pool.GraphMgr.GetLeader(globalGraph) == nil {
// //        err := pool.RefreshClusterInfo(globalGraph)
// //
// //        if err != nil {
// //            return nil, err
// //        }
// //    }
// //
// //    return pool.GraphMgr.GetLeader(globalGraph), nil
// //}
//
// // Get master client
// //func (pool *ConnectionPool) GetMasterConn(config *configuration.UltipaConfig) (*Connection, error) {
// //
// //    if pool.GraphMgr.GetLeader(config.CurrentGraph) == nil {
// //        err := pool.RefreshClusterInfo(config.CurrentGraph)
// //
// //        if err != nil {
// //            return nil, err
// //        }
// //    }
// //
// //    return pool.GraphMgr.GetLeader(config.CurrentGraph), nil
// //
// //}
//
// // SetMasterConn (graphName , *conn) Set master client
// //func (pool *ConnectionPool) SetMasterConn(graphName string, conn *Connection) {
// //    pool.GraphMgr.SetLeader(graphName, conn)
// //}
//
// // Get random client
func (pool *ConnectionPool) GetRandomConn(config *configuration.UltipaConfig) (*Connection, error) {
	pool.muActiveSafely.Lock()
	defer pool.muActiveSafely.Unlock()
	if len(pool.Actives) < 1 {
		return nil, errors.New("no active connection is found")
	}

	pool.RandomTick++
	conn := pool.Actives[pool.RandomTick%len(pool.Actives)]
	if conn == nil {
		return nil, errors.New("Random Actived Connection is nil")
	}
	return conn, nil
}

// // Get Task/Analytics client
// func (pool *ConnectionPool) GetAnalyticsConn(config *configuration.UltipaConfig) (*Connection, error) {
//
//	gci := pool.GraphMgr.GetGraph(config.CurrentGraph)
//
//	if gci == nil {
//	    err := pool.RefreshClusterInfo(config.CurrentGraph)
//	    if err != nil {
//	        return nil, err
//	    }
//	    gci = pool.GraphMgr.GetGraph(config.CurrentGraph)
//	}
//
//	return gci.GetAnalyticConn()
//
// }
func (pool *ConnectionPool) Close() error {
	for _, conn := range pool.Connections {
		err := conn.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// // set context with timeout and auth info
func (pool *ConnectionPool) NewContext(config *configuration.RequestConfig) (ctx context.Context, cancel context.CancelFunc, err error) {

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
		timeout = pool.Config.Timeout
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
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(pool.Config.ToContextKV(config)...))
	return ctx, cancel, nil
}

//
//// RunHeartBeat used for special network policy for long connection(such like : force disconnection idle socket)
//func (pool *ConnectionPool) RunHeartBeat() {
//
//    if pool.Config.HeartBeat > 0 {
//        go func() {
//            for {
//                //log.Println("Heart Beat Start... ")
//                for _, conn := range pool.Connections {
//
//                    ctx, cancel, err := pool.NewContext(&configuration.RequestConfig{
//                        Timeout: 6,
//                    })
//                    if err != nil {
//                        log.Printf("heart beat failed [%s] with error :%v ", conn.Host, err)
//                        continue
//                    }
//                    defer cancel()
//                    //log.Println("Heart Beat Item", conn.Host)
//                    resp, err := conn.GetControlClient().SayHello(ctx, &ultipa.HelloUltipaRequest{
//                        Name: "go sdk refresh",
//                    })
//
//                    if err != nil || (resp.Status.ErrorCode != ultipa.ErrorCode_SUCCESS) {
//                        log.Printf("heart beat failed : %v\n", conn.Host)
//                        continue
//                    }
//                }
//                //log.Println("Heart Beat End... ")
//                time.Sleep(time.Duration(pool.Config.HeartBeat) * time.Second)
//            }
//        }()
//    }
//}
