package utils

import (
	"path/filepath"
	"strings"
)

// HasWildcard 检查模式是否包含通配符字符
func HasWildcard(pattern string) bool {
	return strings.ContainsAny(pattern, "*?")
}

// MatchWildcard 检查文本是否匹配通配符模式
// 支持 * 和 ? 通配符，不区分大小写
func MatchWildcard(pattern, text string) bool {
	pattern = strings.ToLower(pattern)
	text = strings.ToLower(text)

	matched, err := filepath.Match(pattern, text)
	if err != nil {
		return false
	}
	return matched
}
