package structs

import (
	"encoding/json"
	"fmt"
	"strings"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/types"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils"
)

type Edge struct {
	//Name     string
	UUID     types.UUID
	FromUUID types.UUID
	ToUUID   types.UUID
	From     types.ID
	To       types.ID
	Schema   string
	Values   *Values
}

func NewEdge() *Edge {
	return &Edge{
		Values: NewValues(),
	}
}

func NewEdgeFromMetaData(md *MetaData) *Edge {
	return &Edge{
		From:   md.From,
		To:     md.To,
		UUID:   md.UUID,
		Schema: md.Schema,
		Values: md.Values,
	}
}

func NewEdgeFromEdgeRow(schema *Schema, edgeRow *ultipa.EntityRow) (*Edge, error) {
	newEdge := NewEdge()

	newEdge.UUID = edgeRow.Uuid
	newEdge.From = edgeRow.FromId
	newEdge.To = edgeRow.ToId
	newEdge.FromUUID = edgeRow.FromUuid
	newEdge.ToUUID = edgeRow.ToUuid
	newEdge.Schema = edgeRow.SchemaName

	for index, v := range edgeRow.GetValues() {
		prop := schema.Properties[index]
		value, err := utils.ConvertBytesToInterface(v, prop.Type, prop.SubTypes)
		if err != nil {
			return nil, err
		}
		newEdge.Values.Set(prop.Name, value)
	}

	return newEdge, nil
}

func (edge *Edge) GetUUID() types.UUID {
	return edge.UUID
}

func (edge *Edge) GetFrom() types.ID {
	return edge.From
}

func (edge *Edge) GetTo() types.ID {
	return edge.To
}

func (edge *Edge) GetSchema() string {
	return edge.Schema
}

func (edge *Edge) GetValues() *Values {
	return edge.Values
}

// get a value by key
func (edge *Edge) Get(propName string) interface{} {
	return edge.Values.Get(propName)
}

// get a value by key
func (edge *Edge) GetBytes(key string) ([]byte, error) {
	v := edge.Values.Get(key)
	return utils.ConvertInterfaceToBytes(v)
}

// GetBytesSafe get []byte value by key, if value is nil, then return default value of PropertyType t
func (edge *Edge) GetBytesSafe(key string, t ultipa.PropertyType, subTypes []ultipa.PropertyType, req *configuration.RequestConfig) ([]byte, error) {
	v := edge.Values.Get(key)
	return utils.ConvertInterfaceToBytesSafe(v, t, subTypes, req)
}

// set a value by key
func (edge *Edge) Set(propName string, value interface{}) error {

	edge.Values.Set(propName, value)
	return nil
}

func ConvertStringEdges(schema *Schema, edges []*Edge, req *configuration.RequestConfig) {
	// Obtain the configured time zone information
	// timezoneOffset > timeZone
	location := utils.GetLocationFromConfig(req)

	// For by Schema, not nodes value
	for _, edge := range edges {
		for _, prop := range schema.Properties {
			stri := edge.Values.Get(prop.Name)

			str := ""
			if stri == nil {
				str = utils.GetDefaultNilString(prop.Type)
			} else {
				str = stri.(string)
			}

			v, err := utils.StringAsInterface(str, prop.Type, location)

			if err != nil {
				continue
			}
			edge.Values.Set(prop.Name, v)
		}
	}
}

func GetSchemasOfEdgeList(edges []*Edge) map[string]*Schema {
	var schemaPropertiesMap = make(map[string][]string)
	for _, edge := range edges {
		propertyList, ok := schemaPropertiesMap[edge.Schema]
		if !ok {
			schemaPropertiesMap[edge.Schema] = []string{}
		}
		for property := range edge.Values.Data {
			if !utils.Contains(propertyList, property) {
				propertyList = append(propertyList, property)
				schemaPropertiesMap[edge.Schema] = propertyList
			}
		}
	}
	var schemaMap = make(map[string]*Schema)
	for schemaName, propertyList := range schemaPropertiesMap {
		schema := NewSchema(schemaName)
		schema.DBType = ultipa.DBType_DBEDGE
		for _, propertyName := range propertyList {
			schema.Properties = append(schema.Properties, &Property{
				Name:   propertyName,
				Schema: schemaName,
			})
		}
		schemaMap[schemaName] = schema
	}
	return schemaMap
}

func edgeToString(edge *Edge) string {
	dataMap := make(map[string]interface{}, 10)

	if edge.UUID != 0 {
		dataMap["_uuid"] = edge.UUID
	}
	if edge.FromUUID != 0 {
		dataMap["_from_uuid"] = edge.FromUUID
	}
	if edge.ToUUID != 0 {
		dataMap["_to_uuid"] = edge.ToUUID
	}
	if edge.From != "" {
		dataMap["_from"] = edge.From
	}
	if edge.To != "" {
		dataMap["_to"] = edge.To
	}

	if edge.Values != nil && edge.Values.Data != nil {
		for k, v := range edge.Values.Data {
			dataMap[k] = v
		}
	}

	jsonData, err := json.Marshal(dataMap)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return ""
	}

	return string(jsonData)
}

func EdgesToInsertUql(edges []*Edge) string {
	var edgeStrings []string
	for _, edge := range edges {
		edgeString := edgeToString(edge)
		if edgeString != "" {
			edgeStrings = append(edgeStrings, edgeString)
		}
	}
	return fmt.Sprintf("%s", strings.Join(edgeStrings, ", "))
}
