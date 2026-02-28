package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"gpt-load/internal/utils"
	"io"
)

// Service 定义加密接口
type Service interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
	Hash(plaintext string) string
}

// NewService 创建加密服务
func NewService(encryptionKey string) (Service, error) {
	if encryptionKey == "" {
		return new(noopService), nil
	}

	// 从用户输入派生 AES-256 密钥并验证强度
	aesKey := utils.DeriveAESKey(encryptionKey)
	utils.ValidatePasswordStrength(encryptionKey, "ENCRYPTION_KEY")

	// 初始化密码和 GCM 一次以供重用
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &aesService{key: aesKey, gcm: gcm}, nil
}

// aesService 实现 AES-256-GCM 加密
type aesService struct {
	key []byte
	gcm cipher.AEAD
}

func (s *aesService) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := s.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

func (s *aesService) Decrypt(ciphertext string) (string, error) {
	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("invalid hex data: %w", err)
	}

	nonceSize := s.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, encrypted := data[:nonceSize], data[nonceSize:]
	plaintext, err := s.gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// Hash 使用 HMAC-SHA256 生成明文的哈希
func (s *aesService) Hash(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(plaintext))
	return hex.EncodeToString(mac.Sum(nil))
}

// noopService 禁用加密
type noopService struct{}

func (s *noopService) Encrypt(plaintext string) (string, error) {
	return plaintext, nil
}

func (s *noopService) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

// Hash 使用 SHA256 生成明文的哈希（无密钥）
func (s *noopService) Hash(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(hash[:])
}
