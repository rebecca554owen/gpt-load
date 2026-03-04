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
// 支持多种匹配模式（优先级从高到低）：
// 1. 精确匹配：pattern 不包含通配符，直接比较
// 2. 前缀匹配：pattern 格式为 "prefix*"，匹配以 prefix 开头的文本
// 3. 后缀匹配：pattern 格式为 "*suffix"，匹配以 suffix 结尾的文本
// 4. 包含匹配：pattern 格式为 "*substring*"，匹配包含 substring 的文本
// 5. 组合匹配：pattern 格式为 "prefix*suffix"，匹配以 prefix 开头且以 suffix 结尾的文本
// 6. 完全通配：pattern 为 "*"，匹配任意文本
// 不区分大小写
func MatchWildcard(pattern, text string) bool {
	pattern = strings.ToLower(pattern)
	text = strings.ToLower(text)

	if !HasWildcard(pattern) {
		return pattern == text
	}

	if pattern == "*" {
		return true
	}

	// 查找第一个 * 的位置
	firstStar := strings.Index(pattern, "*")
	if firstStar == -1 {
		return pattern == text
	}

	// 检查是否只有一个 *
	if strings.Index(pattern[firstStar+1:], "*") == -1 {
		// 前缀匹配：prefix*
		if firstStar == len(pattern)-1 {
			return strings.HasPrefix(text, pattern[:firstStar])
		}

		// 后缀匹配：*suffix
		if firstStar == 0 {
			return strings.HasSuffix(text, pattern[1:])
		}

		// 组合匹配：prefix*suffix
		return strings.HasPrefix(text, pattern[:firstStar]) && strings.HasSuffix(text, pattern[firstStar+1:])
	}

	// 多个 * 的情况，使用 filepath.Match 作为兜底方案
	matched, err := filepath.Match(pattern, text)
	if err != nil {
		return false
	}
	return matched
}
