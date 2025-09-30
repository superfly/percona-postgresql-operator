package pgbackrestserver

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

func IntoOpts(val reflect.Value) []string {
	args := []string{}

	typ := val.Type()
	for i := range val.NumField() {
		jsonTag := typ.Field(i).Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		field := val.Field(i)
		parts := strings.Split(jsonTag, ",")
		jsonKey := parts[0]
		isOmitEmpty := len(parts) > 1 && parts[1] == "omitempty"

		if isOmitEmpty && field.IsZero() {
			continue
		}

		flag := "--" + strings.ReplaceAll(jsonKey, "_", "-")

		switch field.Kind() {
		case reflect.String:
			if strVal := field.String(); strVal != "" || !isOmitEmpty {
				args = append(args, fmt.Sprintf(" %s=%s", flag, strVal))
			}
		case reflect.Int, reflect.Int64:
			if intVal := field.Int(); intVal != 0 || !isOmitEmpty {
				args = append(args, fmt.Sprintf(" %s=%d", flag, intVal))
			}
		case reflect.Bool:
			if field.Bool() {
				args = append(args, fmt.Sprintf(" %s", flag))
			}
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String {
				for j := range field.Len() {
					args = append(args, fmt.Sprintf(" %s=%s", flag, field.Index(j).String()))
				}
			}
		case reflect.Map:
			// We sort the keys to ensure the output is deterministic (always the same order).
			// This is especially important for reliable testing.
			keys := make([]string, 0, field.Len())
			for _, keyVal := range field.MapKeys() {
				keys = append(keys, keyVal.String())
			}
			sort.Strings(keys)

			for _, key := range keys {
				value := field.MapIndex(reflect.ValueOf(key))
				// Format is --flag=key=value
				arg := fmt.Sprintf("%s=%s=%s", flag, key, value.String())
				args = append(args, arg)
			}
		}
	}

	return args
}
