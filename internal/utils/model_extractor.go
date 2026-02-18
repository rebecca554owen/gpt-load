package utils

import (
	"encoding/json"
)

// ExtractModelFromBody extracts the model name from request body
func ExtractModelFromBody(bodyBytes []byte) string {
	var requestData map[string]any
	if err := json.Unmarshal(bodyBytes, &requestData); err != nil {
		return ""
	}

	modelValue, exists := requestData["model"]
	if !exists {
		return ""
	}

	model, ok := modelValue.(string)
	if !ok {
		return ""
	}

	return model
}
