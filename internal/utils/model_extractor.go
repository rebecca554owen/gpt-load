package utils

import (
	"github.com/goccy/go-json"
)

// ExtractModelFromBody 从请求体中提取模型名称
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
