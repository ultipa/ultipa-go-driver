package api

import (
	"fmt"
	"strconv"
	"time"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/connection"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/session"
)

// Uql, Insert, Export, Download ... API methods

type UltipaAPI struct {
	Pool   *connection.ConnectionPool
	Config *configuration.UltipaConfig
	//Logger *logger.Logger
}

type ClientType int

const (
	ClientTypeGeneral ClientType = 1
	ClientTypeControl ClientType = 2
)

func NewUltipaAPI(conn *connection.ConnectionPool) *UltipaAPI {

	api := &UltipaAPI{
		Pool:   conn,
		Config: conn.Config,
		//Logger: logger.NewLogger(conn.Config.Debug),
	}

	return api
}

func (api *UltipaAPI) GetConn(config *configuration.RequestConfig) (*connection.Connection, *configuration.UltipaConfig, error) {
	var err error
	var conn *connection.Connection

	conf := api.Pool.Config

	if config != nil {
		conf = api.Pool.Config.MergeRequestConfig(config)
		//UqlItem := utils.NewUql(config.Uql)

		// Check if User set Host Address
		if config.Host != "" {
			conn, err = connection.NewConnection(config.Host, conf)
			if err != nil {
				return nil, nil, err
			}
			// if is raft mode, check if contains CUD ops or exec task
		} else {
			conn, err = api.Pool.GetRandomConn(conf)

		}
	}

	if err != nil {
		return nil, conf, err
	}

	return conn, conf, nil
}

func (api *UltipaAPI) GetClient(config *configuration.RequestConfig) (ultipa.UltipaRpcsClient, *configuration.UltipaConfig, error) {

	conn, conf, err := api.GetConn(config)

	if err != nil {
		return nil, conf, err
	}

	client := conn.GetClient()
	//api.Logger.Log(fmt.Sprintf("fetch client,  hit host:[%s], role [%v], graph=[%s]", conn.Host, conn.Role, conf.CurrentGraph))
	return client, conf, nil
}

func (api *UltipaAPI) GetControlClient(config *configuration.RequestConfig) (ultipa.UltipaControlsClient, error) {

	client, _, err := api.GetControlClientAndConfig(config)
	return client, err
}

func (api *UltipaAPI) GetControlClientAndConfig(config *configuration.RequestConfig) (ultipa.UltipaControlsClient, *configuration.UltipaConfig, error) {

	if config == nil {
		config = &configuration.RequestConfig{}
	}

	//config.UseControl = true

	conn, conf, err := api.GetConn(config)

	if err != nil {
		return nil, conf, err
	}
	client := conn.GetControlClient()
	//api.Logger.Log(fmt.Sprintf("fetch control client, hit host:[%s], role [%v], graph=[%s]", conn.Host, conn.Role, conf.CurrentGraph))
	return client, conf, nil
}

// Uql send a uql string to ultipa graph, and return a http Uql Response
// get Alias from Uql Response and convert to any type you need by asNodes, asEdges, asPaths, asTable, as asArray...
// Check DataItem to learn more about Uql Response
func (api *UltipaAPI) Uql(uql string, config *configuration.RequestConfig) (*http.Response, error) {
	return api.query(uql, ultipa.QueryType_UQL, config)
}

func (api *UltipaAPI) query(query string, queryType ultipa.QueryType, config *configuration.RequestConfig) (*http.Response, error) {

	resp, _, hostName, err := api.doExecuteQuery(query, queryType, config)
	//log.Println(query)
	if err != nil {
		return nil, err
	}

	uqlResp, err := http.NewUQLResponse(resp)

	if err != nil {
		return nil, err
	}

	// Set the host that processed this request (for transaction affinity)
	uqlResp.HostName = hostName

	//if uqlResp.Status.Code != ultipa.ErrorCode_SUCCESS {
	//	return nil, errors.New(uqlResp.Status.Message)
	//}

	if config != nil && config.Host != "" {
		return uqlResp, err
	}
	//if uqlResp.NeedRedirect() {
	//    err = api.Pool.RefreshClusterInfo(conf.CurrentGraph)
	//    if err != nil {
	//        return nil, err
	//    }
	//    return api.Uql(query,config)
	//}

	return uqlResp, nil
}

func (api *UltipaAPI) queryStream(query string, queryType ultipa.QueryType, cb func(*http.Response) error, config *configuration.RequestConfig) error {
	resp, _, hostName, err := api.doExecuteQuery(query, queryType, config)
	if err != nil {
		return err
	}

	uqlResp, err := http.NewUQLResponseStream(resp)
	if err != nil {
		return err
	}

	// Set the host that processed this request (for transaction affinity)
	uqlResp.HostName = hostName

	//if uqlResp.Status.Code != ultipa.ErrorCode_SUCCESS {
	//	return nil, errors.New(uqlResp.Status.Message)
	//}

	if config != nil && config.Host != "" {
		return err
	}

	return uqlResp.Recv(cb)
}

func (api *UltipaAPI) UqlStream(uql string, cb func(*http.Response) error, config *configuration.RequestConfig) error {
	return api.queryStream(uql, ultipa.QueryType_UQL, cb, config)
}

func (api *UltipaAPI) doExecuteQuery(query string, queryType ultipa.QueryType, config *configuration.RequestConfig) (ultipa.UltipaRpcs_QueryClient, *configuration.UltipaConfig, string, error) {
	if config == nil {
		config = &configuration.RequestConfig{}
	}

	// If user specified a specific host, don't do failover
	if config.Host != "" {
		return api.doExecuteQueryOnce(query, queryType, config)
	}

	// Try with failover to other hosts
	maxRetries := len(api.Pool.Actives)
	if maxRetries < 1 {
		maxRetries = 1
	}

	var lastErr error
	var conf *configuration.UltipaConfig

	for attempt := 0; attempt < maxRetries; attempt++ {
		conn, c, err := api.GetConn(config)
		conf = c
		if err != nil {
			lastErr = err
			continue
		}

		client := conn.GetClient()
		ctx, cancel, err := api.Pool.NewContext(config)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		uqlRequest := api.buildQueryRequest(query, queryType, config, conf)
		resp, err := client.Query(ctx, uqlRequest)

		if err != nil {
			cancel()
			// Mark current host as unavailable and try next
			api.Pool.MarkHostUnavailable(conn.Host)
			lastErr = err
			continue
		}

		// Success - don't cancel context here as caller needs to use resp
		return resp, conf, conn.Host, nil
	}

	return nil, conf, "", fmt.Errorf("all hosts failed, last error: %v", lastErr)
}

// doExecuteQueryOnce executes query on a specific host without failover
func (api *UltipaAPI) doExecuteQueryOnce(query string, queryType ultipa.QueryType, config *configuration.RequestConfig) (ultipa.UltipaRpcs_QueryClient, *configuration.UltipaConfig, string, error) {
	var err error
	var client ultipa.UltipaRpcsClient
	var conf *configuration.UltipaConfig

	client, conf, err = api.GetClient(config)
	if err != nil {
		return nil, conf, "", err
	}

	ctx, cancel, err := api.Pool.NewContext(config)
	if err != nil {
		defer cancel()
		return nil, conf, "", err
	}

	uqlRequest := api.buildQueryRequest(query, queryType, config, conf)
	resp, err := client.Query(ctx, uqlRequest)
	if err != nil {
		return nil, conf, "", err
	}

	return resp, conf, config.Host, nil
}

// buildQueryRequest build uqlRequest according to requestConfig and configuration
func (api *UltipaAPI) buildQueryRequest(query string, queryType ultipa.QueryType, config *configuration.RequestConfig, conf *configuration.UltipaConfig) *ultipa.QueryRequest {
	graphName := ""
	if config.Graph != "" {
		graphName = config.Graph
	} else if conf.DefaultGraph != "" {
		graphName = conf.DefaultGraph
	}

	uqlRequest := &ultipa.QueryRequest{
		GraphName: graphName,
		Timeout:   uint32(conf.Timeout),
		QueryType: queryType,
		QueryText: query,
	}
	if config.Thread > 0 {
		uqlRequest.ThreadNum = config.Thread
	}
	if config.TimezoneOffset == "" && config.Timezone == "" {
		_, offset := time.Now().Zone()
		uqlRequest.TzOffset = strconv.Itoa(offset)
	} else if config.TimezoneOffset != "" {
		uqlRequest.TzOffset = config.TimezoneOffset
	} else if config.Timezone != "" {
		uqlRequest.Tz = config.Timezone
	}

	// Add session/transaction fields (s5.3 feature)
	uqlRequest.SessionId = config.SessionID
	uqlRequest.TransactionId = config.TransactionID

	// Add transaction config if present
	if config.TransactionConfig != nil {
		uqlRequest.ReadOnly = config.TransactionConfig.ReadOnly
		uqlRequest.IsolationLevel = config.TransactionConfig.IsolationLevel
		uqlRequest.Autocommit = config.TransactionConfig.Autocommit
		uqlRequest.TransactionTimeout = config.TransactionConfig.TransactionTimeout
	}

	return uqlRequest
}

// Test connection test
func (api *UltipaAPI) Test(config *configuration.RequestConfig) (bool, error) {
	conn, err := api.Pool.GetConn(nil)

	if err != nil {
		return false, err
	}
	client := conn.GetClient()
	ctx, cancel, err := api.Pool.NewContext(config)
	if err != nil {
		return false, err
	}
	defer cancel()
	res, err := client.SayHello(ctx, &ultipa.HelloUltipaRequest{
		Name: "Conn Test",
	})

	if err != nil {
		return false, fmt.Errorf("tset error %w", err)
	}

	if res.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
		return false, fmt.Errorf("tset error %v", res.Status.Msg)
	}

	return true, nil
}

//func (api *UltipaAPI) GetActiveClientTest() (bool, *connection.Connection, error) {
//    conn, err := api.Pool.GetConn(nil)
//
//    if err != nil {
//        return false, nil, err
//    }
//    client := conn.GetClient()
//    ctx, cancel, err := api.Pool.NewContext(nil)
//    if err != nil {
//        return false, nil, err
//    }
//    defer cancel()
//    resp, err := client.SayHello(ctx, &ultipa.HelloUltipaRequest{
//        Name: "Pool Test",
//    })
//
//    if err != nil || resp.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
//        return false, nil, err
//    }
//
//    return true, conn, err
//}

//func (api *UltipaAPI) SetCurrentGraph(graphName string) error {
//    api.Config.CurrentGraph = graphName
//    return nil
//}

func (api *UltipaAPI) Close() error {
	return api.Pool.Close()
}

func (api *UltipaAPI) SafelyClose() error {
	if api != nil && api.Pool != nil {
		return api.Pool.Close()
	}
	return nil
}

// Session creates a new database session with auto-generated ID
// The session provides session-scoped UQL/GQL execution and transaction support
func (api *UltipaAPI) Session(config *configuration.SessionConfig) (*session.Session, error) {
	// Auto-populate graph from DefaultGraph if not set
	if config == nil {
		config = &configuration.SessionConfig{}
	}
	if config.Graph == "" && api.Pool.Config.DefaultGraph != "" {
		config.Graph = api.Pool.Config.DefaultGraph
	}
	return session.NewSession(api, config)
}

// SessionWithID creates a session with a specified ID
// Use this when you need to maintain a specific session ID across requests
func (api *UltipaAPI) SessionWithID(sessionID uint64, config *configuration.SessionConfig) *session.Session {
	// Auto-populate graph from DefaultGraph if not set
	if config == nil {
		config = &configuration.SessionConfig{}
	}
	if config.Graph == "" && api.Pool.Config.DefaultGraph != "" {
		config.Graph = api.Pool.Config.DefaultGraph
	}
	return session.NewSessionWithID(api, sessionID, config)
}
