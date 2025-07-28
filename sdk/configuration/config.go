package configuration

import (
	"io/ioutil"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/jinzhu/copier"
	"gopkg.in/yaml.v3"
)

type UltipaConfig struct {
	Hosts           []string // hosts with ports
	Username        string   // ultipa graph username
	Password        string   // ultipa graph password
	PasswordEncrypt string   // method of encrypt password, MD5, LDAP, NOTHING
	DefaultGraph    string   `yaml:"default_graph"` // default graph when connection established
	Crt             []byte   // certification file for encrypt messages
	MaxRecvSize     int      `yaml:"max_recv_size"` // grpc max receive size
	//Consistency      bool     // if consistency, reading query will send to master
	//CurrentGraph string `yaml:"current_graph"` // the current graph, used when user what get the connection's current graph name
	//CurrentClusterId string `yaml:"current_cluster_id"` // used for name server only
	Timeout int32 // timeout - seconds
	//Debug            bool   // debug, print more logs
	HeartBeat int `yaml:"heart_beat"` // frequency:second,  if 0 means no heart beat, to make sure the connection is alive
}

// var DefaultTimeout int32 = 1000
var DefaultTimeout int32 = math.MaxInt32

// var DefaultGraph = "default"
var DefaultRecvSize = 1024 * 1024 * 32

// Deprecated: Should use NewUltipa directly
//func NewUltipaConfig(config *UltipaConfig) (*UltipaConfig, error) {
//	config.FillDefault()
//
//	//h := md5.New()
//	//h.Write([]byte(config.Password))
//	encryptedPwd, err := Encrypt(config.PasswordEncrypt, config.Password)
//	if err != nil {
//		return config, err
//	}
//	config.Password = encryptedPwd
//	return config, nil
//}

func (config *UltipaConfig) FillDefault() {
	if config.MaxRecvSize == 0 {
		config.MaxRecvSize = DefaultRecvSize // 10MB
	}

	//if config.DefaultGraph == "" {
	//    config.DefaultGraph = DefaultGraph
	//}
	//
	//if config.CurrentGraph == "" {
	//	config.CurrentGraph = "default"
	//}

	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	if config.PasswordEncrypt == "" {
		config.PasswordEncrypt = "MD5"
	}

}

func (config *UltipaConfig) MergeRequestConfig(rConfig *RequestConfig) *UltipaConfig {

	newConfig := &UltipaConfig{}

	copier.Copy(newConfig, config)

	if rConfig.Timeout > 0 {
		newConfig.Timeout = rConfig.Timeout
	}

	//if rConfig.Graph != "" {
	//	newConfig.CurrentGraph = rConfig.Graph
	//}
	//if rConfig.ClusterId != "" {
	//	newConfig.CurrentClusterId = rConfig.ClusterId
	//}

	return newConfig
}

func (config *UltipaConfig) ToContextKV(rConfig *RequestConfig) []string {
	graphName := config.DefaultGraph

	if rConfig != nil && rConfig.Graph != "" {
		graphName = rConfig.Graph
	}

	headers := []string{
		"user",
		config.Username,
		"password",
		config.Password,
		"graph_name",
		graphName,
		//"cluster_id",
		//config.CurrentClusterId,
	}
	if rConfig == nil || (rConfig.TimezoneOffset == "" && rConfig.Timezone == "") {
		_, offset := time.Now().Zone()
		headers = append(headers, "tz_offset", strconv.Itoa(offset))
	} else if rConfig.TimezoneOffset != "" {
		headers = append(headers, "tz_offset", rConfig.TimezoneOffset)
	} else if rConfig.Timezone != "" {
		headers = append(headers, "tz", rConfig.Timezone)
	}
	//log.Printf("transporter debug UltipaConfig.ToContextKV %v\n", headers)
	return headers
}

func LoadConfigFromYAML(file string) (*UltipaConfig, error) {
	content, err := ioutil.ReadFile(file)

	if err != nil {
		return nil, err
	}
	config := &UltipaConfig{}

	err = yaml.Unmarshal(content, config)

	if err != nil {
		return nil, err
	}

	//config, err = NewUltipaConfig(config)
	//if err != nil {
	//	return nil, err
	//}

	return config, nil
}

func isValidTimezoneOffset(offset string) bool {
	if offset == "" {
		return true // 空字符串合法
	}

	// 支持 ±hh:mm 或 ±hhmm，小时00-23，分钟00-59
	pattern := `^[+-](?:[01][0-9]|2[0-3]):?[0-5][0-9]$`
	re := regexp.MustCompile(pattern)

	return re.MatchString(offset)
}
