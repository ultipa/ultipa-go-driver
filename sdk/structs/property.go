package structs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
)

type Property struct {
	Name        string
	Schema      string
	Type        ultipa.PropertyType
	SubTypes    []ultipa.PropertyType
	Lte         bool
	Read        bool
	Write       bool
	Description string
	Encrypt     string
	// extra info for property in json format, e.g. DecimalExtra: precision and scale for decimal type.
	DecimalExtra *DecimalExtra
}

const (
	PropertyType_ID        ultipa.PropertyType = -1
	PropertyType_UUID      ultipa.PropertyType = -2
	PropertyType_FROM      ultipa.PropertyType = -3
	PropertyType_TO        ultipa.PropertyType = -4
	PropertyType_FROM_UUID ultipa.PropertyType = -5
	PropertyType_TO_UUID   ultipa.PropertyType = -6
	PropertyType_IGNORE    ultipa.PropertyType = -7
)

var PropertyMap = map[string]ultipa.PropertyType{
	"_id":        PropertyType_ID,
	"_uuid":      PropertyType_UUID,
	"_from":      PropertyType_FROM,
	"_to":        PropertyType_TO,
	"_from_uuid": PropertyType_FROM_UUID,
	"_to_uuid":   PropertyType_TO_UUID,
	"_ignore":    PropertyType_IGNORE,
	"string":     ultipa.PropertyType_STRING,
	"int32":      ultipa.PropertyType_INT32,
	"int64":      ultipa.PropertyType_INT64,
	"uint32":     ultipa.PropertyType_UINT32,
	"uint64":     ultipa.PropertyType_UINT64,
	"float":      ultipa.PropertyType_FLOAT,
	"double":     ultipa.PropertyType_DOUBLE,
	"datetime":   ultipa.PropertyType_DATETIME,
	"timestamp":  ultipa.PropertyType_TIMESTAMP,
	"text":       ultipa.PropertyType_TEXT,
	"blob":       ultipa.PropertyType_BLOB,
	"point":      ultipa.PropertyType_POINT,
	"decimal":    ultipa.PropertyType_DECIMAL,
	"NULL_":      ultipa.PropertyType_NULL_,
	"bool":       ultipa.PropertyType_BOOL,
	// add in server 5.2.xx
	"local datetime":          ultipa.PropertyType_LOCAL_DATETIME,
	"zoned datetime":          ultipa.PropertyType_ZONED_DATETIME,
	"date":                    ultipa.PropertyType_DATE,
	"zoned time":              ultipa.PropertyType_ZONED_TIME,
	"local time":              ultipa.PropertyType_LOCAL_TIME,
	"duration(year to month)": ultipa.PropertyType_YEAR_TO_MONTH,
	"duration(day to second)": ultipa.PropertyType_DAY_TO_SECOND,
	"json":                    ultipa.PropertyType_JSON,
}

var PropertyReverseMap = map[ultipa.PropertyType]string{
	PropertyType_ID:               "_id",
	PropertyType_UUID:             "_uuid",
	PropertyType_FROM:             "_from",
	PropertyType_TO:               "_to",
	PropertyType_FROM_UUID:        "_from_uuid",
	PropertyType_TO_UUID:          "_to_uuid",
	PropertyType_IGNORE:           "_ignore",
	ultipa.PropertyType_STRING:    "string",
	ultipa.PropertyType_INT32:     "int32",
	ultipa.PropertyType_INT64:     "int64",
	ultipa.PropertyType_UINT32:    "uint32",
	ultipa.PropertyType_UINT64:    "uint64",
	ultipa.PropertyType_FLOAT:     "float",
	ultipa.PropertyType_DOUBLE:    "double",
	ultipa.PropertyType_DATETIME:  "datetime",
	ultipa.PropertyType_TIMESTAMP: "timestamp",
	ultipa.PropertyType_TEXT:      "text",
	ultipa.PropertyType_BLOB:      "blob",
	ultipa.PropertyType_POINT:     "point",
	ultipa.PropertyType_DECIMAL:   "decimal",
	ultipa.PropertyType_NULL_:     "NULL_",
	ultipa.PropertyType_BOOL:      "bool",
	// add in server 5.2.x
	ultipa.PropertyType_LOCAL_DATETIME: "local datetime",
	ultipa.PropertyType_ZONED_DATETIME: "zoned datetime",
	ultipa.PropertyType_DATE:           "date",
	ultipa.PropertyType_ZONED_TIME:     "zoned time",
	ultipa.PropertyType_LOCAL_TIME:     "local time",
	ultipa.PropertyType_YEAR_TO_MONTH:  "duration(year to month)",
	ultipa.PropertyType_DAY_TO_SECOND:  "duration(day to second)",
	ultipa.PropertyType_JSON:           "json",
}

type DecimalExtra struct {
	Precision int `json:"precision"`
	Scale     int `json:"scale"`
}

func MarshalDecimalExtra(d *DecimalExtra) ([]byte, error) {
	if d == nil || (d.Precision == 0 && d.Scale == 0) {
		return []byte(""), nil
	}
	return json.Marshal(d)
}

func UnmarshalDecimalExtra(data []byte, d *DecimalExtra) error {
	if bytes.Equal(data, []byte("")) {
		return nil
	}
	return json.Unmarshal(data, d)
}

func (p *Property) IsIDType() bool {

	idTyps := []ultipa.PropertyType{
		PropertyType_ID,
		PropertyType_UUID,
		PropertyType_FROM,
		PropertyType_TO,
		PropertyType_FROM_UUID,
		PropertyType_TO_UUID,
	}

	for _, t := range idTyps {
		if p.Type == t {
			return true
		}
	}

	return false
}

func (p *Property) IsIgnore() bool {
	return p.Type == PropertyType_IGNORE
}

func (p *Property) SetTypeByString(s string) {
	// set typeStr "set(string)"
	// decimal typeStr "decimal(x,y)"
	// list typeStr "string[]"

	// adjust time : local time.....
	//s = strings.ReplaceAll(s, " ", "")

	// list
	if strings.HasSuffix(s, "[]") {
		p.Type = ultipa.PropertyType_LIST
		p.SubTypes = append(p.SubTypes, GetPropertyTypeByString(strings.TrimSuffix(s, "[]")))
		return
	}

	// set
	reg := regexp.MustCompile(`set\(([^)]+)\)|set<([^>]+)>`)
	if reg.MatchString(s) {
		matches := reg.FindStringSubmatch(s)
		var subType string
		if matches[1] != "" { //  set(...)
			subType = matches[1]
		} else if matches[2] != "" { //  set<...>
			subType = matches[2]
		}

		p.Type = ultipa.PropertyType_SET
		p.SubTypes = append(p.SubTypes, GetPropertyTypeByString(subType))

		// 5.0 server not support set type，set type = UNSET
		//p.DBType = ultipa.PropertyType_UNSET
		return
	}

	// decimal
	re := regexp.MustCompile(`^decimal\((\d+),(\d+)\)$`)
	if re.MatchString(s) {
		p.Type = ultipa.PropertyType_DECIMAL
		matches := re.FindStringSubmatch(s)
		precision, err := strconv.Atoi(matches[1])
		scale, err := strconv.Atoi(matches[2])
		if err != nil {
			return
		}
		extraData := &DecimalExtra{
			Precision: precision,
			Scale:     scale,
		}
		//extraJson, err := json.Marshal(extraData)
		//if err != nil {
		//	return
		//}
		p.DecimalExtra = extraData

		//p.DBType = ultipa.PropertyType_UNSET
		return
	}

	p.Type = GetPropertyTypeByString(s)
}

func (p *Property) GetStringType() (string, error) {
	if p.Type == ultipa.PropertyType_LIST {
		if len(p.SubTypes) == 0 {
			return "", errors.New(fmt.Sprintf("Property [%s] is List but not specified subTypes", p.Name))
		}
		if p.SubTypes[0] == ultipa.PropertyType_DECIMAL {
			return "", errors.New(fmt.Sprintf("Property [%s] is List, subTypes not support decimal", p.Name))
		}
		return GetStringByPropertyType(p.SubTypes[0]) + "[]", nil
	}
	if p.Type == ultipa.PropertyType_SET {
		if len(p.SubTypes) == 0 {
			return "", errors.New(fmt.Sprintf("Property [%s] is Set but not specified subTypes", p.Name))
		}
		return fmt.Sprintf("set(%s)", GetStringByPropertyType(p.SubTypes[0])), nil
	}
	if p.Type == ultipa.PropertyType_DECIMAL {
		//var extraData DecimalExtra
		//err := json.Unmarshal([]byte(p.DecimalExtra), &extraData)
		//if err != nil {
		//	return GetStringByPropertyType(p.Type), nil
		//}
		extraString := fmt.Sprintf("(%d,%d)", p.DecimalExtra.Precision, p.DecimalExtra.Scale)
		return GetStringByPropertyType(p.Type) + extraString, nil
	}
	return GetStringByPropertyType(p.Type), nil
}

func GetPropertyTypeByString(s string) ultipa.PropertyType {
	return PropertyMap[s]
}

func GetStringByPropertyType(t ultipa.PropertyType) string {
	return PropertyReverseMap[t]
}
