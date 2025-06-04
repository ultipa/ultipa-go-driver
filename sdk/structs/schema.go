package structs

import (
	"errors"
	ultipa "github.com/ultipa/ultipa-go-driver/rpc"
	"github.com/ultipa/ultipa-go-driver/sdk/utils"
)

//type Schemas struct {
//	Schemas    []*Schema
//	TotalNodes int
//	TotalEdges int
//}

type Schema struct {
	Name        string
	Description string
	Properties  []*Property
	//Status      string
	//Type   string
	DBType ultipa.DBType
	Total  uint64
	Id     string
	Stats  []*SchemaStat // only for EdgeSchema
}

// SchemaStat only for EdgeSchema
type SchemaStat struct {
	DBType     ultipa.DBType
	Schema     string
	FromSchema string
	ToSchema   string
	Count      uint64
}

type GraphCount struct {
	Type   string
	Schema string
	Stat   *SchemaStat
}

func NewSchema(name string) *Schema {
	return &Schema{Name: name, Properties: []*Property{}}
}

func (s *Schema) GetProperty(name string) *Property {
	prop := utils.Find(s.Properties, func(index int) bool { return s.Properties[index].Name == name })

	if prop != nil {
		return prop.(*Property)
	}

	return nil
}

func NewSchemaMapFromProtoSchema(schemas []*ultipa.Schema, DBType ultipa.DBType) map[string]*Schema {
	m := map[string]*Schema{}

	for _, s := range schemas {
		schema := ConvertProtoSchemaToSdkSchema(s, DBType)
		m[s.SchemaName] = schema
	}

	return m
}

func ConvertProtoSchemaToSdkSchema(pSchema *ultipa.Schema, DBType ultipa.DBType) *Schema {
	s := NewSchema(pSchema.SchemaName)

	s.DBType = DBType
	for _, prop := range pSchema.Properties {
		s.Properties = append(s.Properties, &Property{
			Name:     prop.PropertyName,
			Type:     prop.PropertyType,
			SubTypes: prop.SubTypes,
		})
	}

	return s
}

// compare 2 schema is same, or is able to fit schema1 to schema2
// schema1 is new schema
// schema2 is server side schema
func CompareSchemas(schema1 *Schema, schema2 *Schema, fit bool) (error, []*Property) {

	var NotExistProperties []*Property

	if schema1 == nil {
		return errors.New("schema compare failed, schema1 is required"), nil
	}

	if schema2 == nil {
		if fit {
			return nil, nil
		} else {
			return errors.New("schema compare failed, schema2 is required or set fit to true"), nil
		}

	}

	schema1PropMap := map[string]*Property{}
	schema2PropMap := map[string]*Property{}

	for _, prop := range schema1.Properties {
		schema1PropMap[prop.Name] = prop
	}

	for _, prop := range schema2.Properties {
		schema2PropMap[prop.Name] = prop
	}

	// check one by one
	for name, prop1 := range schema1PropMap {

		if prop1.IsIDType() || prop1.IsIgnore() {
			continue
		}

		prop2 := schema2PropMap[name]

		if fit == true && (prop2 != nil && prop2.Type != prop1.Type) {
			return errors.New("schema compare failed, property : @" + schema1.Name + "." + prop1.Name + " mismatch"), nil
		}

		if fit == false && (prop2 == nil || prop2.Type != prop1.Type) {
			return errors.New("schema compare failed, property : @" + schema1.Name + "." + prop1.Name + " not exist"), nil
		}

		// not exist properties
		if fit == true && prop2 == nil && prop1.IsIDType() == false {
			NotExistProperties = append(NotExistProperties, prop1)
		}
	}

	return nil, NotExistProperties
}

// SetStatsByGraphCount for type = node edge
func (s *Schema) SetStatsByGraphCount(g []*GraphCount) {
	for _, gc := range g {
		dbType, _ := GetDBTypeByString(gc.Type)
		if !(gc.Schema == s.Name && dbType == s.DBType) {
			continue
		}

		s.Stats = append(s.Stats, &SchemaStat{
			Schema:     gc.Schema,
			DBType:     dbType,
			FromSchema: gc.Stat.FromSchema,
			ToSchema:   gc.Stat.ToSchema,
			Count:      gc.Stat.Count,
		})
		s.Total += gc.Stat.Count
	}
}
