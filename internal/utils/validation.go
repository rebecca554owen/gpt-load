package utils

import (
	"gpt-load/internal/models"
)

// IsValidKeyStatus 检查给定状态是否为有效的密钥状态。
func IsValidKeyStatus(status string) bool {
	return status == "" || status == models.KeyStatusActive || status == models.KeyStatusInvalid
}

// ValidateKeyStatus 如果状态无效则返回错误。
func ValidateKeyStatus(status string) error {
	if !IsValidKeyStatus(status) {
		return &ValidationError{Field: "status", Message: "invalid status value"}
	}
	return nil
}

// ValidationError 表示验证错误。
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
