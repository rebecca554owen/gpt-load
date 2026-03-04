package jsonutil

import (
	"fmt"
	"maps"

	"github.com/goccy/go-json"
)

func SetField(bodyBytes []byte, field string, value any) ([]byte, error) {
	return SetFields(bodyBytes, map[string]any{field: value})
}

func SetFields(bodyBytes []byte, fields map[string]any) ([]byte, error) {
	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, err
	}

	maps.Copy(data, fields)

	return json.Marshal(data)
}

func GetStringField(bodyBytes []byte, field string) (string, error) {
	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return "", err
	}

	val, ok := data[field]
	if !ok {
		return "", nil
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("field '%s' is not a string", field)
	}

	return str, nil
}
