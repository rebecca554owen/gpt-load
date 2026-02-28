package utils

import (
	"crypto/sha256"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/pbkdf2"
)

// Common weak password patterns for security validation
var WeakPasswordPatterns = []string{
	"password", "sk-123456", "123456", "admin", "secret", "test", "demo",
	"key", "token", "pass", "pwd", "qwerty", "abc", "default",
	"user", "login", "auth", "temp",
}

// ValidatePasswordStrength 验证密码强度，最小长度为 16 个字符
func ValidatePasswordStrength(password, fieldName string) {
	if len(password) < 16 {
		logrus.Warnf("%s is shorter than 16 characters, consider using a longer password", fieldName)
	}

	lower := strings.ToLower(password)
	for _, pattern := range WeakPasswordPatterns {
		if strings.Contains(lower, pattern) {
			logrus.Warnf("%s contains common weak patterns, consider using a stronger password", fieldName)
			break
		}
	}
}

// DeriveAESKey 使用 PBKDF2 从密码派生 32 字节 AES 密钥
func DeriveAESKey(password string) []byte {
	salt := []byte("gpt-load-encryption-v1")
	return pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
}
