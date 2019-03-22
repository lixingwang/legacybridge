package config

import (
	"github.com/project-flogo/core/app/resource"
	"github.com/project-flogo/core/data"
	"github.com/project-flogo/core/data/schema"
	"github.com/project-flogo/legacybridge"
	"github.com/project-flogo/legacybridge/config/legacyshcema"

	legacyData "github.com/TIBCOSoftware/flogo-lib/core/data"
)

type ConversionContext struct {
	resources []*resource.Config
}

func (cc *ConversionContext) AddResource(res *resource.Config) {
	cc.resources = append(cc.resources, res)
}

func (cc *ConversionContext) AddSchema() {

}

func (cc *ConversionContext) AddImport() {

}

func ConvertLegacyAttr(legacyAttr *legacyData.Attribute) (*data.Attribute, error) {

	newType, _ := legacybridge.ToNewTypeFromLegacy(legacyAttr.Type())
	newVal := legacyAttr.Value()
	var newSchema schema.Schema

	//special handling for ComplexObjects
	if legacyAttr.Type() == legacyData.TypeComplexObject && legacyAttr.Value() != nil {

		if cVal, ok := legacyAttr.Value().(*legacyData.ComplexObject); ok {

			newVal = cVal.Value

			if cVal.Metadata != "" {
				//has schema
				newSchema = legacyshcema.NewLegacySchema(cVal.Metadata)
			}
		}
	}

	return data.NewAttributeWithSchema(legacyAttr.Name(), newType, newVal, newSchema), nil
}
