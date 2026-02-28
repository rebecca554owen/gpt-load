package utils

import "encoding/json"

// ModifyJSONField 修改 JSON 对象中的字段并返回新的 JSON 字节。
// 如果解析或序列化失败，返回原始字节不变。
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

// ModifyJSONFields 修改 JSON 对象中的多个字段并返回新的 JSON 字节。
// 如果解析或序列化失败，返回原始字节不变。
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
