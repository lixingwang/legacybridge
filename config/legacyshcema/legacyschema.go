package legacyshcema

import (
	"fmt"
	"github.com/project-flogo/core/data/schema"
)

//To make sure json.marshal can serialize
type legacySchema struct {
	*schema.Def
}

func (s *legacySchema) Type() string {
	return s.Def.Type
}

func (s *legacySchema) Value() string {
	return s.Def.Value
}

func (*legacySchema) Validate(data interface{}) error {
	return nil
}

func NewLegacySchema(metdata string) schema.Schema {
	def := &schema.Def{Type: "json", Value: metdata}
	return &legacySchema{def}
}

func FindAndCreateLegacySchema(schemaRep interface{}) (schema.Schema, error) {
	switch t := schemaRep.(type) {
	case schema.HasSchema:
		return NewLegacySchema(t.Schema().Value()), nil
	case schema.Def:
		return NewLegacySchema(t.Value), nil
	case *schema.Def:
		return NewLegacySchema(t.Value), nil
	case map[string]string:
		var value string
		if sValue, ok := t["value"]; ok {
			value = sValue
		} else {
			return nil, fmt.Errorf("invalid schema definition, value not specified: %+v", t)
		}
		return NewLegacySchema(value), nil
	case map[string]interface{}:
		if sType, ok := t["type"]; ok {
			_, ok = sType.(string)
			if !ok {
				return nil, fmt.Errorf("invalid schema definition, type is not a string specified: %+v", sType)
			}
		} else {
			return nil, fmt.Errorf("invalid schema definition, type not specified: %+v", t)
		}

		var value string
		if sValue, ok := t["value"]; ok {
			value, ok = sValue.(string)
			if !ok {
				return nil, fmt.Errorf("invalid schema definition, value is not a string specified: %+v", sValue)
			}
		} else {
			return nil, fmt.Errorf("invalid schema definition, value not specified: %+v", t)
		}

		return NewLegacySchema(value), nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("invalid schema definition, %v", t)
	}
}
