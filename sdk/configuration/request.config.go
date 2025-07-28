package configuration

import ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"

type RequestConfig struct {
	Graph   string // Graphset Name
	Timeout int32  // timeout (Seconds)
	//ClusterId      string      // Name Server Only
	Host string // set for force host test
	//UseMaster      bool        // Use Master( graphSet master )
	//UseControl     bool        // Use Control Node( global master )
	//Uql            string // for Go Only, used for inner program
	Timezone       string // name of time zone , e.g. Aisa/Shanghai
	TimezoneOffset string // seconds that elapse from UTC, prior to TimeZone
	Thread         uint32 // used for uql request
	//MaxPkgSize     int    // max package size in bytes, for both sending and receiving, if not set, default is 10M
}

type InsertRequestConfig struct {
	*RequestConfig
	InsertType ultipa.InsertType // used for insertBulkNodes/Edges
	//CreateNodeIfNotExist bool              // used for insertBulkEdges
	Silent bool // if returns new ids
}
