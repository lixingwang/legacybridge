package config

import (
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/project-flogo/core/data/expression/script"
	"strconv"
	"strings"
	"text/scanner"
	"unicode"

	"github.com/project-flogo/core/data/coerce"
	"github.com/project-flogo/core/data/expression"
	"github.com/project-flogo/core/data/mapper"
	"github.com/project-flogo/core/data/resolve"

	legacyData "github.com/TIBCOSoftware/flogo-lib/core/data"
)

const (
	MAP_TO_INPUT = "$INPUT"
)

func ConvertLegacyMappings(mappings *legacyData.IOMappings, resolver resolve.CompositeResolver) (input map[string]interface{}, output map[string]interface{}, err error) {
	if mappings == nil {
		return nil, nil, nil
	}

	input = make(map[string]interface{}, len(mappings.Input))
	output = make(map[string]interface{}, len(mappings.Output))

	if mappings.Input != nil {
		input, err = handleMappings(mappings.Input, resolver)
		if err != nil {
			return nil, nil, err
		}
	}

	if mappings.Output != nil {
		output, err = handleMappings(mappings.Output, resolver)
		if err != nil {
			return nil, nil, err
		}
	}

	return input, output, nil
}

func handleMappings(mappings []*legacyData.MappingDef, resolver resolve.CompositeResolver) (map[string]interface{}, error) {

	input := make(map[string]interface{})

	fieldNameMap := make(map[string][]*objectMappings)
	for _, m := range mappings {
		m.MapTo = RemovePrefixInput(m.MapTo)
		//target is single field name
		if strings.Index(m.MapTo, ".") <= 0 && !hasArray(m.MapTo) {
			typ, _ := toString(m.Type)
			val, err := convertMapperValue(m.Value, typ, resolver)
			if err != nil {
				return nil, err
			}
			input[m.MapTo] = val
		} else {
			//Handle multiple mapping to single field value
			field, err := ParseMappingField(m.MapTo)
			if err != nil {
				return nil, err
			}
			mapToFields := field.GetFields()
			fieldName := getFieldName(mapToFields[0])
			objMapping := &objectMappings{fieldName: fieldName, mapping: m}

			if hasArray(mapToFields[0]) {
				//take [index] as first map field if root field is an array
				mapToFields[0] = mapToFields[0][len(fieldName):]
				objMapping.targetFields = mapToFields
			} else {
				if len(mapToFields) > 1 {
					objMapping.targetFields = mapToFields[1:]
				}
			}
			fieldNameMap[fieldName] = append(fieldNameMap[fieldName], objMapping)
		}
	}

	for k, v := range fieldNameMap {
		var obj interface{}
		var isObjectMapping bool
		for _, objMapping := range v {
			typ, _ := toString(objMapping.mapping.Type)
			//If it is array mapping
			if typ == "array" || typ == "5" {
				isObjectMapping = true
			}
			//Convert value to new
			val, err := convertMapperValue(objMapping.mapping.Value, typ, resolver)
			if err != nil {
				return nil, err
			}

			//First field handle, handle attribute name has array
			if obj == nil && len(objMapping.targetFields) > 0 {
				if strings.Index(objMapping.targetFields[0], "[") >= 0 && strings.Index(objMapping.targetFields[0], "]") > 0 {
					obj = make([]interface{}, 1)
				} else {
					obj = make(map[string]interface{})
				}
			}

			if len(objMapping.targetFields) > 0 {
				isObjectMapping = true
				obj, err = constructObjectFromPath(objMapping.targetFields, val, obj)
				if err != nil {
					return nil, err
				}
			} else {
				obj = val
			}

		}

		//Only for object mapping
		if isObjectMapping {
			input[k] = &mapper.ObjectMapping{Mapping: obj}
		} else {
			input[k] = obj
		}

	}

	return input, nil
}

func constructObjectFromPath(fields []string, value interface{}, object interface{}) (interface{}, error) {
	fieldName := getFieldName(fields[0])
	//Has array
	if strings.Index(fields[0], "[") >= 0 && strings.HasSuffix(fields[0], "]") {
		//Make sure the index is integer
		index, err := strconv.Atoi(getNameInsideBracket(fields[0]))
		if err == nil {
			object, err = handlePathArray(fieldName, index, fields, value, object)
			if err != nil {
				return nil, err
			}
		} else {
			if err := handlePathObject(fieldName, fields, value, object); err != nil {
				return nil, err
			}
		}
	} else {
		if err := handlePathObject(fieldName, fields, value, object); err != nil {
			return nil, err
		}
	}

	return object, nil
}

func handlePathObject(fieldName string, fields []string, value interface{}, object interface{}) error {
	//Not an array
	if len(fields) <= 1 {
		object.(map[string]interface{})[fieldName] = value
		return nil
	} else {
		if _, exist := object.(map[string]interface{})[fieldName]; !exist {
			object.(map[string]interface{})[fieldName] = map[string]interface{}{}
		}
	}
	var err error
	object.(map[string]interface{})[fieldName], err = constructObjectFromPath(fields[1:], value, object.(map[string]interface{})[fieldName].(map[string]interface{}))
	if err != nil {
		return err
	}
	return nil
}

func handlePathArray(fieldName string, index int, fields []string, value interface{}, object interface{}) (interface{}, error) {
	//Only root field with array no field name
	var err error
	if fieldName == "" {
		_, ok := object.([]interface{})
		if !ok {
			tmpArray := createArray(index)
			object = tmpArray
		} else {
			if index >= len(object.([]interface{})) {
				object = Insert(object.([]interface{}), index, map[string]interface{}{})
			}
		}
		if len(fields) == 1 {
			object.([]interface{})[index] = value
			return object, nil
		} else {
			if object.([]interface{})[index] == nil {
				object.([]interface{})[index] = make(map[string]interface{})
			}
			object.([]interface{})[index], err = constructObjectFromPath(fields[1:], value, object.([]interface{})[index])
			if err != nil {
				return nil, err
			}
		}
	} else {
		obj := object.(map[string]interface{})
		if array, exist := obj[fieldName]; !exist {
			obj[fieldName] = createArray(index)
			if len(fields) == 1 {
				obj[fieldName].([]interface{})[index] = value
				return obj, nil
			} else {
				obj[fieldName].([]interface{})[index], err = constructObjectFromPath(fields[1:], value, obj[fieldName].([]interface{})[index].(map[string]interface{}))
				if err != nil {
					return nil, err
				}
			}
		} else {
			//exist
			if av, ok := array.([]interface{}); ok {
				if len(fields) <= 1 {
					av = Insert(av, index, value)
					obj[fieldName] = av
					return obj, nil
				} else {
					if index >= len(av) {
						av = Insert(av, index, make(map[string]interface{}))
					} else {
						av[index] = make(map[string]interface{})
					}
					obj[fieldName] = av
					obj[fieldName].([]interface{})[index], err = constructObjectFromPath(fields[1:], value, obj[fieldName].([]interface{})[index].(map[string]interface{}))
					if err != nil {
						return nil, err
					}
				}
			} else {
				return nil, fmt.Errorf("not an array")
			}
		}
	}
	return object, nil
}

func Insert(slice []interface{}, index int, value interface{}) []interface{} {
	if index >= len(slice) {
		// add to the end of slice in case of index >= len(slice)
		tmpArray := make([]interface{}, index+1)
		tmpArray[index] = value
		copy(tmpArray, slice)
		return tmpArray
	}
	slice[index] = value
	return slice
}

func createArray(index int) []interface{} {
	tmpArrray := make([]interface{}, index+1)
	tmpArrray[index] = make(map[string]interface{})
	return tmpArrray
}

func getNameInsideBracket(fieldName string) string {
	if strings.Index(fieldName, "[") >= 0 {
		index := fieldName[strings.Index(fieldName, "[")+1 : strings.Index(fieldName, "]")]
		return index
	}

	return ""
}

type objectMappings struct {
	fieldName    string
	targetFields []string
	mapping      *legacyData.MappingDef
}

func getFieldName(fieldName string) string {
	if strings.Index(fieldName, "[") >= 0 && strings.Index(fieldName, "]") > 0 {
		return fieldName[:strings.Index(fieldName, "[")]
	}
	return fieldName
}

type LegacyArrayMapping struct {
	From   interface{}           `json:"from"`
	To     string                `json:"to"`
	Type   string                `json:"type"`
	Fields []*LegacyArrayMapping `json:"fields,omitempty"`
}

func ParseArrayMapping(arrayData interface{}) (*LegacyArrayMapping, error) {
	amapping := &LegacyArrayMapping{}
	switch t := arrayData.(type) {
	case string:
		err := json.Unmarshal([]byte(t), amapping)
		if err != nil {
			return nil, err
		}
	case interface{}:
		s, err := coerce.ToString(t)
		if err != nil {
			return nil, fmt.Errorf("convert array mapping value to string error, due to [%s]", err.Error())
		}
		err = json.Unmarshal([]byte(s), amapping)
		if err != nil {
			return nil, err
		}
	}
	return amapping, nil
}

func ToNewArray(mapping *LegacyArrayMapping, resolver resolve.CompositeResolver) (interface{}, error) {
	var newMapping interface{}
	var fieldsMapping map[string]interface{}
	if mapping.From == "NEWARRAY" {
		fieldsMappings := make([]interface{}, 1)
		fieldsMappings[0] = make(map[string]interface{})
		fieldsMapping = fieldsMappings[0].(map[string]interface{})
		newMapping = fieldsMappings
	} else {
		newMapping = make(map[string]interface{})
		fieldsMapping = make(map[string]interface{})
		newMapping.(map[string]interface{})[fmt.Sprintf("@foreach(%s)", mapping.From)] = fieldsMapping
	}

	var err error
	for _, field := range mapping.Fields {
		if field.Type == "foreach" {
			//Check to see if it is a new array
			fieldsMapping[ToNewArrayChildMapTo(field.To)], err = ToNewArray(field, resolver)
		} else {
			fieldsMapping[ToNewArrayChildMapTo(field.To)], err = convertMapperValue(field.From, field.Type, resolver)
			if err != nil {
				return nil, err
			}
		}
	}
	return newMapping, nil
}

func ToNewArrayChildMapTo(old string) string {
	old = RemovePrefixInput(old)
	if strings.HasPrefix(old, "$.") || strings.HasPrefix(old, "$$") {
		old = old[2:]
	}

	return RemoveBrackets(old)
}

// ToString coerce a value to a string
func toString(val interface{}) (string, error) {
	switch t := val.(type) {
	case string:
		return t, nil
	case int:
		return strconv.Itoa(t), nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case json.Number:
		return t.String(), nil
	case legacyData.MappingType:
		return strconv.Itoa(int(t)), nil
	default:
		return "", nil
	}
}

func convertMapperValue(value interface{}, typ string, resolver resolve.CompositeResolver) (interface{}, error) {
	switch typ {
	case "assign", "1":
		if v, ok := value.(string); ok {
			if !ResolvableExpr(v, resolver) {
				return v, nil
			}
			return "=" + v, nil
		}
		return value, nil
	case "literal", "2":
		return value, nil
	case "expression", "3":
		expr, ok := value.(string)
		if !ok {
			return value, nil
		}
		return "=" + expr, nil
	case "object", "4":
		return value, nil
	case "array", "5":
		arrayMapping, err := ParseArrayMapping(value)
		if err != nil {
			return nil, err
		}

		return ToNewArray(arrayMapping, resolver)
	case "primitive":
		//This use to handle very old array mapping type
		if priValue, ok := value.(string); ok {
			if !ResolvableExpr(priValue, resolver) {
				//Not an expr, just return is as value
				return priValue, nil
			}
			return "=" + priValue, nil
		}

		return value, nil
	default:
		return 0, errors.New("unsupported mapping type: " + typ)
	}
}

func RemovePrefixInput(str string) string {
	if str != "" && strings.HasPrefix(str, MAP_TO_INPUT) {
		//Remove $INPUT for mapTo
		newMapTo := str[len(MAP_TO_INPUT):]
		if strings.HasPrefix(newMapTo, ".") {
			newMapTo = newMapTo[1:]
		}
		str = newMapTo
	}
	return str
}

func RemoveBrackets(str string) string {
	if len(str) > 0 {
		if (strings.HasPrefix(str, `["`) && strings.HasSuffix(str, `"]`)) || (strings.HasPrefix(str, `['`) && strings.HasSuffix(str, `']`)) {
			str = str[2:]
			str = str[0 : len(str)-2]
		}
	}
	return str
}

func hasArray(field string) bool {
	return strings.Index(field, "[") >= 0 && strings.Index(field, "]") > 0
}

func ResolvableExpr(expr string, resolver resolve.CompositeResolver) bool {
	_, err := expression.NewFactory(resolver).NewExpr(expr)
	if err != nil {
		//Not an expr, just return is as value
		return false
	}
	return true
}

type MappingField struct {
	fields []string
	ref    string
	s      *scanner.Scanner
}

func NewMappingField(fields []string) *MappingField {
	return &MappingField{fields: fields}
}

func ParseMappingField(mRef string) (*MappingField, error) {
	//Remove any . at beginning
	if strings.HasPrefix(mRef, ".") {
		mRef = mRef[1:]
	}
	g := &MappingField{ref: mRef}

	err := g.Start(mRef)
	if err != nil {
		return nil, fmt.Errorf("parse mapping [%s] failed, due to %s", mRef, err.Error())
	}
	return g, nil
}

func (m *MappingField) GetFields() []string {
	return m.fields
}

func (m *MappingField) parseName() error {
	fieldName := ""
	switch ch := m.s.Scan(); ch {
	case '.':
		return m.Parser()
	case '[':
		//Done
		//if fieldName != "" {
		//	m.fields = append(m.fields, fieldName)
		//}
		m.s.Mode = scanner.ScanInts
		nextAfterBracket := m.s.Scan()
		if nextAfterBracket == '"' || nextAfterBracket == '\'' {
			//Special characters
			m.s.Mode = scanner.ScanIdents
			return m.handleSpecialField(nextAfterBracket)
		} else {
			//HandleArray
			if m.fields == nil || len(m.fields) <= 0 {
				m.fields = append(m.fields, "["+m.s.TokenText()+"]")
			} else {
				m.fields[len(m.fields)-1] = m.fields[len(m.fields)-1] + "[" + m.s.TokenText() + "]"
			}
			ch := m.s.Scan()
			if ch != ']' {
				return fmt.Errorf("invalid array format")
			}
			m.s.Mode = scanner.ScanIdents
			return m.Parser()
		}
	case scanner.EOF:
		//if fieldName != "" {
		//	m.fields = append(m.fields, fieldName)
		//}
	default:
		fieldName = fieldName + m.s.TokenText()
		if fieldName != "" {
			m.fields = append(m.fields, fieldName)
		}
		return m.Parser()
	}

	return nil
}

func (m *MappingField) handleSpecialField(startQuotes int32) error {
	fieldName := ""
	run := true

	for run {
		switch ch := m.s.Scan(); ch {
		case startQuotes:
			//Check if end with startQuotes
			nextAfterQuotes := m.s.Scan()
			if nextAfterQuotes == ']' {
				//end specialField, start over
				m.fields = append(m.fields, fieldName)
				run = false
				return m.Parser()
			} else {
				fieldName = fieldName + string(startQuotes)
				fieldName = fieldName + m.s.TokenText()
			}
		default:
			fieldName = fieldName + m.s.TokenText()
		}
	}
	return nil
}

func (m *MappingField) Parser() error {
	switch ch := m.s.Scan(); ch {
	case '.':
		return m.parseName()
	case '[':
		m.s.Mode = scanner.ScanInts
		nextAfterBracket := m.s.Scan()
		if nextAfterBracket == '"' || nextAfterBracket == '\'' {
			//Special characters
			m.s.Mode = scanner.ScanIdents
			return m.handleSpecialField(nextAfterBracket)
		} else {
			//HandleArray
			if m.fields == nil || len(m.fields) <= 0 {
				m.fields = append(m.fields, "["+m.s.TokenText()+"]")
			} else {
				m.fields[len(m.fields)-1] = m.fields[len(m.fields)-1] + "[" + m.s.TokenText() + "]"
			}
			//m.handleArray()
			ch := m.s.Scan()
			if ch != ']' {
				return fmt.Errorf("invalid array format")
			}
			m.s.Mode = scanner.ScanIdents
			return m.Parser()
		}
	case scanner.EOF:
		//Done
		return nil
	default:
		m.fields = append(m.fields, m.s.TokenText())
		return m.parseName()
	}
}

func (m *MappingField) Start(jsonPath string) error {
	m.s = new(scanner.Scanner)
	m.s.IsIdentRune = IsIdentRune
	m.s.Init(strings.NewReader(jsonPath))
	m.s.Mode = scanner.ScanIdents
	//Do not skip space and new line
	m.s.Whitespace ^= 1<<'\t' | 1<<'\n' | 1<<'\r' | 1<<' '
	return m.Parser()
}

func IsIdentRune(ch rune, i int) bool {
	return ch == '$' || ch == '-' || ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch) && i > 0
}
