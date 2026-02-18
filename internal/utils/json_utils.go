package utils

import "encoding/json"

// ModifyJSONField modifies a field in a JSON object and returns the new JSON bytes.
// If parsing or marshaling fails, returns the original bytes unchanged.
func ModifyJSONField(bodyBytes []byte, field string, value any) []byte {
	var requestData map[string]any
	if err := json.Unmarshal(bodyBytes, &requestData); err != nil {
		return bodyBytes
	}

	requestData[field] = value

	newBodyBytes, err := json.Marshal(requestData)
	if err != nil {
		return bodyBytes
	}

	return newBodyBytes
}

// ModifyJSONFields modifies multiple fields in a JSON object and returns the new JSON bytes.
// If parsing or marshaling fails, returns the original bytes unchanged.
func ModifyJSONFields(bodyBytes []byte, fields map[string]any) []byte {
	var requestData map[string]any
	if err := json.Unmarshal(bodyBytes, &requestData); err != nil {
		return bodyBytes
	}

	for field, value := range fields {
		requestData[field] = value
	}

	newBodyBytes, err := json.Marshal(requestData)
	if err != nil {
		return bodyBytes
	}

	return newBodyBytes
}
