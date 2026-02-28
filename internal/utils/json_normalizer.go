package utils

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	hjson "github.com/hjson/hjson-go/v4"
	"gopkg.in/yaml.v3"
)

var (
	jsonKeyPattern           = regexp.MustCompile(`([{,]\s*)([A-Za-z_][A-Za-z0-9_-]*)(\s*:)`)
	barewordValuePattern     = regexp.MustCompile(`(:\s*)([A-Za-z_./:@?&=%+~\-][A-Za-z0-9_./:@?&=%+~\-]*)(\s*[,}\]])`)
	barewordArrayItemPattern = regexp.MustCompile(`([\[,]\s*)([A-Za-z_./:@?&=%+~\-][A-Za-z0-9_./:@?&=%+~\-]*)(\s*[\],])`)
	strictJSONNumberPattern  = regexp.MustCompile(`^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?$`)
)

// NormalizeJSONRequestBody 尝试将非标准 JSON 类体（例如未引号的对象键）
// 规范化为严格 JSON 字节。
// 规范化成功时返回（normalizedBytes, true）。
func NormalizeJSONRequestBody(body []byte, contentType string) ([]byte, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return body, false
	}

	if !shouldAttemptJSONNormalization(trimmed, contentType) {
		return body, false
	}

	if json.Valid(trimmed) {
		return body, false
	}

	if normalized, ok := normalizeByLooseTokenRepair(trimmed); ok {
		return normalized, true
	}

	var parsed any
	if err := hjson.Unmarshal(trimmed, &parsed); err != nil {
		if err := yaml.Unmarshal(trimmed, &parsed); err != nil {
			return body, false
		}
	}

	normalized, ok := toJSONCompatible(parsed)
	if !ok {
		return body, false
	}

	normalizedBytes, err := json.Marshal(normalized)
	if err != nil {
		return body, false
	}

	return normalizedBytes, true
}

func normalizeByLooseTokenRepair(trimmed []byte) ([]byte, bool) {
	repaired := jsonKeyPattern.ReplaceAll(trimmed, []byte(`${1}"${2}"${3}`))
	repaired = quoteBarewordTokens(repaired, barewordValuePattern)
	repaired = quoteBarewordTokens(repaired, barewordArrayItemPattern)
	if !json.Valid(repaired) {
		return nil, false
	}
	return repaired, true
}

func quoteBarewordTokens(input []byte, pattern *regexp.Regexp) []byte {
	indices := pattern.FindAllSubmatchIndex(input, -1)
	if len(indices) == 0 {
		return input
	}

	var out bytes.Buffer
	last := 0
	for _, idx := range indices {
		fullStart, fullEnd := idx[0], idx[1]
		prefixStart, prefixEnd := idx[2], idx[3]
		tokenStart, tokenEnd := idx[4], idx[5]
		suffixStart, suffixEnd := idx[6], idx[7]

		token := string(input[tokenStart:tokenEnd])
		out.Write(input[last:fullStart])
		out.Write(input[prefixStart:prefixEnd])
		if shouldQuoteBarewordToken(token) {
			out.WriteByte('"')
			out.Write(input[tokenStart:tokenEnd])
			out.WriteByte('"')
		} else {
			out.Write(input[tokenStart:tokenEnd])
		}
		out.Write(input[suffixStart:suffixEnd])
		last = fullEnd
	}
	out.Write(input[last:])
	return out.Bytes()
}

func shouldQuoteBarewordToken(token string) bool {
	if token == "true" || token == "false" || token == "null" {
		return false
	}
	return !strictJSONNumberPattern.MatchString(token)
}

func shouldAttemptJSONNormalization(trimmed []byte, contentType string) bool {
	first := trimmed[0]
	if first != '{' && first != '[' {
		return false
	}

	ct := strings.ToLower(strings.TrimSpace(contentType))
	return ct == "" || strings.Contains(ct, "application/json") || strings.Contains(ct, "+json")
}

func toJSONCompatible(v any) (any, bool) {
	switch value := v.(type) {
	case map[string]any:
		return convertMapString(value), true
	case map[any]any:
		return convertMapAny(value)
	case []any:
		return convertSlice(value), true
	case nil, bool, string,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, json.Number:
		return value, true
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, false
		}
		var out any
		if err := json.Unmarshal(encoded, &out); err != nil {
			return nil, false
		}
		return out, true
	}
}

func convertMapString(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, raw := range m {
		converted, _ := toJSONCompatible(raw)
		out[k] = converted
	}
	return out
}

func convertMapAny(m map[any]any) (map[string]any, bool) {
	out := make(map[string]any, len(m))
	for k, raw := range m {
		key, ok := k.(string)
		if !ok {
			return nil, false
		}
		converted, ok := toJSONCompatible(raw)
		if !ok {
			return nil, false
		}
		out[key] = converted
	}
	return out, true
}

func convertSlice(s []any) []any {
	out := make([]any, len(s))
	for i := range s {
		converted, _ := toJSONCompatible(s[i])
		out[i] = converted
	}
	return out
}
