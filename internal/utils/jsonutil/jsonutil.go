package jsonutil

import "github.com/goccy/go-json"

func SetField(bodyBytes []byte, field string, value any) ([]byte, error) {
	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, err
	}

	data[field] = value

	return json.Marshal(data)
}

func SetFields(bodyBytes []byte, fields map[string]any) ([]byte, error) {
	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, err
	}

	for field, value := range fields {
		data[field] = value
	}

	return json.Marshal(data)
}

func GetStringField(bodyBytes []byte, field string) (string, error) {
	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return "", err
	}

	if val, ok := data[field]; ok {
		if str, ok := val.(string); ok {
			return str, nil
		}
	}

	return "", nil
}
