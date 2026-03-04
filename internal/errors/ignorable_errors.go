package errors

import (
	"strings"
)

// ignorableErrorSubstrings 包含指示错误可以安全忽略的子字符串列表。
// 这些通常在客户端过早断开连接时发生。
var ignorableErrorSubstrings = []string{
	"context canceled",
	"connection reset by peer",
	"broken pipe",
	"use of closed network connection",
	"request canceled",
}

// IsIgnorableError 检查给定错误是否为常见的、非关键性错误。
// 这些错误可能在客户端断开连接时发生，用于防止记录不必要的错误，
// 并避免因客户端端问题将密钥标记为失败。
func IsIgnorableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	for _, sub := range ignorableErrorSubstrings {
		if strings.Contains(errStr, sub) {
			return true
		}
	}
	return false
}
