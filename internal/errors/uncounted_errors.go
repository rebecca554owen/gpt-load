package errors

import "strings"

// unCountedSubstrings 包含指示错误的子字符串列表
var unCountedSubstrings = []string{
	"resource has been exhausted",
	"please reduce the length of the messages",
}

// IsUnCounted 检查给定错误消息是否包含子字符串
func IsUnCounted(errorMsg string) bool {
	if errorMsg == "" {
		return false
	}

	errorLower := strings.ToLower(errorMsg)

	for _, pattern := range unCountedSubstrings {
		if strings.Contains(errorLower, pattern) {
			return true
		}
	}

	return false
}
