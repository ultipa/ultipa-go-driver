package sdk

//
//import (
//	"context"
//	"errors"
//	"fmt"
//	"math"
//	"math/rand"
//	"strings"
//	"sync"
//	"time"
//	ultipa " github.com/ultipa/ultipa-go-driver/rpc"
//	" github.com/ultipa/ultipa-go-driver/utils"
//
//	"google.golang.org/grpc"
//	"google.golang.org/grpc/credentials"
//	"google.golang.org/grpc/metadata"
//)
//
//type ClientInfo struct {
//	conn   *grpc.ClientConn
//	Client ultipa.UltipaRpcsClient
//	Host   string
//}
//
//func (t *ClientInfo) init(conn *grpc.ClientConn, host string) {
//	client := ultipa.NewUltipaRpcsClient(conn)
//	t.Client = client
//	t.conn = conn
//	t.Host = host
//}
//func (t *ClientInfo) Close() {
//	if t.conn != nil {
//		t.conn.Close()
//	}
//}
//
//type ClientType int32
//
//var (
//	ClientType_Default ClientType = 0
//	ClientType_Algo    ClientType = 1
//	ClientType_Update  ClientType = 2
//	ClientType_Leader  ClientType = 3
//)
//var (
//	UQL_Command_Global = []string{
//		"listUser", "getUser", "createUser", "updateUser",
//		"deleteUser", "grant", "revoke", "listPolicy", "getPolicy",
//		"createPolicy", "updatePolicy", "deletePolicy", "listPrivilege",
//		"stat", "listGraph", "getGraph", "createGraph", "dropGraph", "top", "kill",
//	}
//	UQL_Command_Write = []string{
//		"insert", "update", "delete", "drop", "create", "LTE", "UFE",
//		"clearTask", "createUser", "updateUser", "deleteUser", "grant",
//		"revoke", "createPolicy", "updatePolicy", "deletePolicy", "createIndex",
//		"dropIndex", "createGraph", "dropGraph", "stopTask",
//	}
//	UQL_Command_Extra = []string{
//		"top", "kill", "showTask", "stopTask", "clearTask", "show", "stat", "listGraph", "listAlgo", "getGraph",
//		"createPolicy", "deletePolicy", "listPolicy", "getPolicy", "grant", "revoke", "listPrivilege", "getUser",
//		"getSelfInfo", "createUser", "updateUser", "deleteUser", "showIndex",
//	}
//)
//
//
//type HostManager struct {
//	GraphSetName       string
//	LeaderHost         string
//	FollowersPeerInfos []*types.RaftPeerInfo
//	RaftReady          bool
//
//	username string
//	password string
//	crtFile  string
//
//	leaderClientInfo              *ClientInfo
//	algoClientInfos               []*ClientInfo
//	defaultClientInfo             *ClientInfo
//	otherFollowerClientInfos      []*ClientInfo
//	otherUnsetFollowerClientInfos []*ClientInfo
//	nullClientInfo                *ClientInfo
//}
//
//func (t *HostManager) Init(graphSetName string, host string, username string, password string, crtFile string) {
//	t.GraphSetName = graphSetName
//	t.username = username
//	t.password = password
//	t.crtFile = crtFile
//	t.LeaderHost = host
//	t.nullClientInfo, _ = t.createClientInfo("")
//	t.defaultClientInfo, _ = t.createClientInfo(host)
//}
//
//func (t *HostManager) createClientInfo(host string) (*ClientInfo, error) {
//	var opts []grpc.DialOption
//	opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(math.MaxInt32)))
//	opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)))
//	if len(t.crtFile) == 0 {
//		// compatible 2.0
//		opts = append(opts, grpc.WithInsecure())
//		conn, _ := grpc.Dial(host, opts...)
//		clientInfo := ClientInfo{}
//		clientInfo.init(conn, host)
//		return &clientInfo, nil
//	} else {
//		creds, err := credentials.NewClientTLSFromFile(t.crtFile, "ultipa")
//		if err != nil {
//			return nil, err
//		}
//		opts = append(opts, grpc.WithTransportCredentials(creds))
//		conn, _ := grpc.Dial(host, opts...)
//		clientInfo := ClientInfo{}
//		clientInfo.init(conn, host)
//		return &clientInfo, nil
//	}
//}
//
//func (t *HostManager) GetAllHosts() *[]string {
//	hosts := []string{
//		t.LeaderHost,
//	}
//	if t.FollowersPeerInfos != nil && len(t.FollowersPeerInfos) > 0 {
//		for _, info := range t.FollowersPeerInfos {
//			hosts = append(hosts, info.Host)
//		}
//
//	}
//	return &hosts
//
//}
//func (t *HostManager) chooseClientInfo(clientType ClientType, uql string, readModeNonConsistency bool, useHost string) *ClientInfo {
//	if useHost != "" {
//		for _, clientInfo := range t.getAllClientInfos(false, true) {
//			if clientInfo.Host == useHost {
//				return clientInfo
//			}
//		}
//	}
//	if clientType == ClientType_Default && uql != "" {
//		if UqlIsAlgo(uql) {
//			clientType = ClientType_Algo
//		} else if UqlIsWrite(uql) {
//			clientType = ClientType_Update
//		}
//	}
//	if clientType == ClientType_Algo {
//		if t.algoClientInfos != nil && len(t.algoClientInfos) > 0 {
//			return t.algoClientInfos[rand.Intn(len(t.algoClientInfos))]
//		} else {
//			return t.nullClientInfo
//		}
//	}
//	if clientType == ClientType_Update || clientType == ClientType_Leader || readModeNonConsistency == false {
//		if t.leaderClientInfo != nil {
//			return t.leaderClientInfo
//		}
//		return t.defaultClientInfo
//	}
//
//	// Load balancing, random selection
//	all := t.getAllClientInfos(true, false)
//	return all[rand.Intn(len(all))]
//}
//func (t *HostManager) getAllClientInfos(ignoreAlgo bool, needUnset bool) []*ClientInfo {
//	all := []*ClientInfo{
//		t.defaultClientInfo,
//	}
//	if t.leaderClientInfo != nil && t.leaderClientInfo.Host != t.defaultClientInfo.Host {
//		all = append(all, t.leaderClientInfo)
//	}
//	if !ignoreAlgo && t.algoClientInfos != nil {
//		all = append(all, t.algoClientInfos...)
//	}
//	if t.otherFollowerClientInfos != nil {
//		all = append(all, t.otherFollowerClientInfos...)
//	}
//	if needUnset && t.otherUnsetFollowerClientInfos != nil {
//		all = append(all, t.otherUnsetFollowerClientInfos...)
//	}
//	return all
//}
//func (t *HostManager) SetClients(leaderHost string, followersPeerInfos []*types.RaftPeerInfo) {
//	t.LeaderHost = leaderHost
//	if t.defaultClientInfo.Host == leaderHost {
//		t.leaderClientInfo = t.defaultClientInfo
//	} else {
//		t.leaderClientInfo, _ = t.createClientInfo(leaderHost)
//	}
//	t.FollowersPeerInfos = followersPeerInfos
//	t.otherFollowerClientInfos = []*ClientInfo{}
//	t.otherUnsetFollowerClientInfos = []*ClientInfo{}
//	t.algoClientInfos = []*ClientInfo{}
//
//	if t.FollowersPeerInfos != nil && len(t.FollowersPeerInfos) > 0 {
//		for _, info := range t.FollowersPeerInfos {
//			host := info.Host
//			clientInfo, _ := t.createClientInfo(host)
//			if info.IsAlgoExecutable {
//				t.algoClientInfos = append(t.algoClientInfos, clientInfo)
//			}
//			if info.IsFollowerReadable {
//				t.otherFollowerClientInfos = append(t.otherFollowerClientInfos, clientInfo)
//			}
//			if info.IsUnset {
//				t.otherUnsetFollowerClientInfos = append(t.otherUnsetFollowerClientInfos, clientInfo)
//			}
//		}
//
//	} else {
//		t.algoClientInfos = []*ClientInfo{t.leaderClientInfo}
//	}
//
//}
//
//type HostManagerControl struct {
//	InitHost               string
//	username               string
//	password               string
//	crtFile                string
//	ReadModeNonConsistency bool
//	AllHostManager         map[string]*HostManager
//	mutex                  sync.Mutex
//}
//
//func (t *HostManagerControl) Init(initHost string, username string, password string, crtFile string, readModeNonConsistency bool) {
//	t.InitHost = initHost
//	t.username = username
//	t.password = password
//	t.ReadModeNonConsistency = readModeNonConsistency
//	t.AllHostManager = map[string]*HostManager{}
//}
//
//func (t *HostManagerControl) chooseClientInfo(graphSetName string, clientType ClientType, uql string, useHost string) *ClientInfo {
//	hostManager := t.getHostManager(graphSetName)
//	return hostManager.chooseClientInfo(clientType, uql, t.ReadModeNonConsistency, useHost)
//}
//func (t *HostManagerControl) getHostManager(graphSetName string) *HostManager {
//	hostManager := t.AllHostManager[graphSetName]
//	if hostManager == nil {
//		hostManager = t.updateHostManager(graphSetName, t.InitHost)
//	}
//	return hostManager
//}
//func (t *HostManagerControl) updateHostManager(graphSetName string, initHost string) *HostManager {
//	t.mutex.Lock()
//	defer t.mutex.Unlock()
//	hostManager := HostManager{}
//	hostManager.Init(graphSetName, initHost, t.username, t.password, t.crtFile)
//	t.AllHostManager[graphSetName] = &hostManager
//	return &hostManager
//}
//
//const RAFT_GLOBAL = "global"
//
//func (t *HostManagerControl) GetAllHosts() *[]string {
//	hostManager := t.getHostManager(RAFT_GLOBAL)
//	return hostManager.GetAllHosts()
//}
//func (t *HostManagerControl) CloseAll() {
//	for _, hostManager := range t.AllHostManager {
//		if hostManager != nil {
//			saveClose(hostManager.defaultClientInfo)
//			saveClose(hostManager.leaderClientInfo)
//		}
//	}
//}
//
//type DefaultConfig struct {
//	GraphSetName            string
//	TimeoutWithSeconds      uint32
//	ResponseWithRequestInfo bool
//	ReadModeNonConsistency  bool
//	IsMd5                   bool
//}
//
//type Connection struct {
//	HostManagerControl *HostManagerControl
//	metadataKV         *[]string
//	username           string
//	password           string
//	crtFile            string
//	DefaultConfig      *DefaultConfig
//	mutex              sync.Mutex
//}
//
//func GetConnection(host string, username string, password string, crtFile string, defaultConfig *DefaultConfig) (*Connection, error) {
//	connect := Connection{}
//	if defaultConfig.IsMd5 {
//		password = strings.ToUpper(utils.Md5ToString(password))
//	}
//	err := connect.init(host, username, password, crtFile, defaultConfig)
//	if err != nil {
//		return nil, err
//	}
//	return &connect, nil
//}
//func (t *Connection) SetDefaultConfig(defaultConfig *DefaultConfig) {
//	if t.DefaultConfig == nil {
//		t.DefaultConfig = &DefaultConfig{"default", 15, false, false, false}
//	}
//	if defaultConfig != nil {
//		if &defaultConfig.GraphSetName != nil {
//			t.DefaultConfig.GraphSetName = defaultConfig.GraphSetName
//		}
//		if &defaultConfig.TimeoutWithSeconds != nil {
//			t.DefaultConfig.TimeoutWithSeconds = defaultConfig.TimeoutWithSeconds
//		}
//		if &defaultConfig.ResponseWithRequestInfo != nil {
//			t.DefaultConfig.ResponseWithRequestInfo = defaultConfig.ResponseWithRequestInfo
//		}
//		if &defaultConfig.ReadModeNonConsistency != nil {
//			t.DefaultConfig.ReadModeNonConsistency = defaultConfig.ReadModeNonConsistency
//		}
//		if &defaultConfig.IsMd5 != nil {
//			t.DefaultConfig.IsMd5 = defaultConfig.IsMd5
//		}
//	}
//}
//func saveClose(clientInfo *ClientInfo) {
//	if clientInfo != nil {
//		clientInfo.Close()
//	}
//}
//func (t *Connection) CloseAll() {
//	t.HostManagerControl.CloseAll()
//}
//
//func (t *Connection) init(host string, username string, password string, crt string, config *DefaultConfig) error {
//	t.username = username
//	t.password = password
//	t.crtFile = crt
//	t.SetDefaultConfig(config)
//	kv := []string{"user", username, "password", password}
//	t.metadataKV = &kv
//	hmControl := HostManagerControl{}
//	hmControl.Init(host, username, password, crt, t.DefaultConfig.ReadModeNonConsistency)
//	t.HostManagerControl = &hmControl
//	return nil
//}
//
//type GetClientInfoResult struct {
//	ClientInfo   *ClientInfo
//	Context      context.Context
//	CancelFunc   context.CancelFunc
//	Host         string
//	GraphSetName string
//}
//type GetClientInfoParams struct {
//	GraphSetName   string
//	ClientType     ClientType
//	Uql            string
//	IsGlobal       bool
//	IgnoreRaft     bool
//	UseHost        string
//	TimeoutSeconds uint32
//}
//
//func (t *Connection) getClientInfo(params *GetClientInfoParams) *GetClientInfoResult {
//	t.mutex.Lock()
//	defer t.mutex.Unlock()
//	goGraphSetName := t.getGraphSetName(params.GraphSetName, params.Uql, params.IsGlobal)
//	timeout := t.DefaultConfig.TimeoutWithSeconds
//	if params.TimeoutSeconds > 0 {
//		timeout = params.TimeoutSeconds
//	}
//	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout+120)*time.Second)
//	kv := []string{"graph_name", goGraphSetName}
//	kv = append(kv, *t.metadataKV...)
//	ctx = metadata.AppendToOutgoingContext(ctx, kv...)
//	//ctx = metadata.AppendToOutgoingContext(ctx, *t.metadataKV...)
//	//defer cancel()
//
//	if params.IgnoreRaft == false && t.HostManagerControl.getHostManager(goGraphSetName).RaftReady == false {
//		t.RefreshRaftLeader(t.HostManagerControl.InitHost, &types.Request_Common{
//			GraphSetName: goGraphSetName,
//		})
//		t.HostManagerControl.getHostManager(goGraphSetName).RaftReady = true
//	}
//
//	clientInfo := t.HostManagerControl.chooseClientInfo(goGraphSetName, params.ClientType, params.Uql, params.UseHost)
//	return &GetClientInfoResult{
//		ClientInfo:   clientInfo,
//		Context:      ctx,
//		CancelFunc:   cancel,
//		Host:         clientInfo.Host,
//		GraphSetName: goGraphSetName,
//	}
//}
//func (t *Connection) getGraphSetName(currentGraphName string, uql string, isGlobal bool) string {
//	if isGlobal || (uql != "" && UqlIsGlobal(uql)) {
//		return RAFT_GLOBAL
//	}
//	if currentGraphName != "" {
//		return currentGraphName
//	}
//	return t.DefaultConfig.GraphSetName
//}
//func (t *Connection) TestConnect(commonReq *types.Request_Common) (bool, error) {
//	if commonReq == nil {
//		commonReq = &types.Request_Common{}
//	}
//	clientInfo := t.getClientInfo(&GetClientInfoParams{
//		IsGlobal: true,
//		UseHost:  commonReq.UseHost,
//	})
//	defer clientInfo.CancelFunc()
//	name := "MyTest"
//	res, err := clientInfo.ClientInfo.Client.SayHello(clientInfo.Context, &ultipa.HelloUltipaRequest{
//		Name: name,
//	})
//	if err != nil {
//		return false, err
//	}
//	if res.Message != name+" Welcome To Ultipa!" {
//		return false, errors.New("test connect failed!")
//	}
//	return true, nil
//}
//
//type RaftLeaderResSimple struct {
//	Code               ultipa.ErrorCode
//	Message            string
//	LeaderHost         string
//	FollowersPeerInfos []*types.RaftPeerInfo
//}
//
//func (t *Connection) autoGetRaftLeader(host string, commonReq *types.Request_Common, retry int) (*RaftLeaderResSimple, error) {
//	conn, err := GetConnection(host, t.username, t.password, t.crtFile, t.DefaultConfig)
//	// do once ,and close
//	defer conn.CloseAll()
//	if err != nil {
//		return nil, err
//	}
//	res := conn.GetLeaderReuqest(commonReq)
//	errorCode := res.Status.Code
//	switch errorCode {
//	case types.ErrorCode_SUCCESS:
//		followers := res.Status.ClusterInfo.RaftPeers
//		return &RaftLeaderResSimple{
//			Code:               errorCode,
//			LeaderHost:         host,
//			FollowersPeerInfos: utils.RemoveRaftInfos(followers, host),
//		}, nil
//	case types.ErrorCode_NOT_RAFT_MODE:
//		return &RaftLeaderResSimple{
//			Code:               types.ErrorCode_SUCCESS,
//			LeaderHost:         host,
//			FollowersPeerInfos: nil,
//		}, nil
//	case types.ErrorCode_RAFT_REDIRECT,
//		types.ErrorCode_RAFT_LEADER_NOT_YET_ELECTED,
//		types.ErrorCode_RAFT_NO_AVAILABLE_FOLLOWERS,
//		types.ErrorCode_RAFT_NO_AVAILABLE_ALGO_SERVERS:
//		if retry > 2 {
//			return &RaftLeaderResSimple{
//				Code:    errorCode,
//				Message: "raft retry too many times",
//			}, nil
//		}
//		if errorCode != ultipa.ErrorCode_RAFT_REDIRECT {
//			time.Sleep(300 * time.Millisecond)
//		}
//		return t.autoGetRaftLeader(res.Status.ClusterInfo.Redirect, commonReq, retry+1)
//	}
//	return &RaftLeaderResSimple{
//		Code:    errorCode,
//		Message: res.Status.Message,
//	}, nil
//}
//
//func (t *Connection) RefreshRaftLeader(redirectHost string, commonReq *types.Request_Common) error {
//	if commonReq == nil {
//		commonReq = &types.Request_Common{}
//	}
//	var hosts []string
//	if redirectHost != "" {
//		hosts = append(hosts, redirectHost)
//	} else {
//		hosts = append(hosts, *t.HostManagerControl.GetAllHosts()...)
//	}
//	goGraphName := t.getGraphSetName(commonReq.GraphSetName, "", false)
//	for _, host := range hosts {
//		res, err := t.autoGetRaftLeader(host, commonReq, 0)
//		if err != nil {
//			return err
//		}
//		if res.Code == types.ErrorCode_SUCCESS {
//			leaderHost := res.LeaderHost
//			followersPeerInfos := res.FollowersPeerInfos
//			hostManager := t.HostManagerControl.updateHostManager(goGraphName, leaderHost)
//			hostManager.SetClients(leaderHost, followersPeerInfos)
//			return nil
//		}
//		return fmt.Errorf("%v - %v", res.Code.String(), res.Message)
//	}
//	return fmt.Errorf("Unknow error! ")
//
//}
