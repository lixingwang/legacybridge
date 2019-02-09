package legacybridge

import (
	olddata "github.com/TIBCOSoftware/flogo-lib/core/data"
	"reflect"
)

func ToOldComplexObject(value interface{}) *olddata.ComplexObject {
	metadata, _ := reflect.ValueOf(value).Elem().FieldByName("Metadata").Interface().(string)
	return &olddata.ComplexObject{Metadata: metadata, Value: reflect.ValueOf(value).Elem().FieldByName("Value").Interface()}
}
