package dotaccess

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func Get(obj interface{}, prop string) (interface{}, error) {
	// Get the array access
	arr := strings.Split(prop, ".")

	var err error
	for _, key := range arr {
		obj, err = getProperty(obj, key)
		if err != nil {
			return nil, err
		}
		if obj == nil {
			return nil, nil
		}
	}
	return obj, nil
}

// Loop through this to get properties via dot notation
func getProperty(obj interface{}, prop string) (interface{}, error) {

	if reflect.TypeOf(obj).Kind() == reflect.Map {

		val := reflect.ValueOf(obj)

		valueOf := val.MapIndex(reflect.ValueOf(prop))

		if valueOf == reflect.Zero(reflect.ValueOf(prop).Type()) {
			return nil, nil
		}

		idx := val.MapIndex(reflect.ValueOf(prop))

		if !idx.IsValid() {
			return nil, nil
		}
		return idx.Interface(), nil
	}

	prop = strings.Title(prop)

	return getField(obj, prop)
}

func getField(obj interface{}, name string) (interface{}, error) {
	if !hasValidType(obj, []reflect.Kind{reflect.Struct, reflect.Ptr}) {
		return nil, errors.New("cannot use GetField on a non-struct interface")
	}

	objValue := reflectValue(obj)
	field := objValue.FieldByName(name)
	if !field.IsValid() {
		return nil, fmt.Errorf("no such field: %s in obj", name)
	}

	return field.Interface(), nil
}

func hasValidType(obj interface{}, types []reflect.Kind) bool {
	for _, t := range types {
		if reflect.TypeOf(obj).Kind() == t {
			return true
		}
	}
	return false
}

func reflectValue(obj interface{}) reflect.Value {
	var val reflect.Value

	if reflect.TypeOf(obj).Kind() == reflect.Ptr {
		val = reflect.ValueOf(obj).Elem()
	} else {
		val = reflect.ValueOf(obj)
	}

	return val
}
