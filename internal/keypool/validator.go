package keypool

import (
	"context"
	"fmt"
	"gpt-load/internal/channel"
	"gpt-load/internal/config"
	"gpt-load/internal/encryption"
	"gpt-load/internal/models"
	"time"

	"github.com/sirupsen/logrus"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

// KeyTestResult 保存单个密钥的验证结果。
type KeyTestResult struct {
	KeyValue string `json:"key_value"`
	IsValid  bool   `json:"is_valid"`
	Error    string `json:"error,omitempty"`
}

// KeyValidator 提供验证 API 密钥的方法。
type KeyValidator struct {
	DB              *gorm.DB
	channelFactory  *channel.Factory
	SettingsManager *config.SystemSettingsManager
	keypoolProvider *KeyProvider
	encryptionSvc   encryption.Service
}

type KeyValidatorParams struct {
	dig.In
	DB              *gorm.DB
	ChannelFactory  *channel.Factory
	SettingsManager *config.SystemSettingsManager
	KeypoolProvider *KeyProvider
	EncryptionSvc   encryption.Service
}

// NewKeyValidator 创建一个新的 KeyValidator。
func NewKeyValidator(params KeyValidatorParams) *KeyValidator {
	return &KeyValidator{
		DB:              params.DB,
		channelFactory:  params.ChannelFactory,
		SettingsManager: params.SettingsManager,
		keypoolProvider: params.KeypoolProvider,
		encryptionSvc:   params.EncryptionSvc,
	}
}

// ValidateSingleKey 对单个 API 密钥执行验证检查。
// model 参数是可选的，如果提供则覆盖通道的 TestModel。
func (s *KeyValidator) ValidateSingleKey(key *models.APIKey, group *models.Group, model string) (bool, error) {
	if group.EffectiveConfig.AppUrl == "" {
		group.EffectiveConfig = s.SettingsManager.GetEffectiveConfig(group.Config)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(group.EffectiveConfig.KeyValidationTimeoutSeconds)*time.Second)
	defer cancel()

	ch, err := s.channelFactory.GetChannel(group)
	if err != nil {
		return false, fmt.Errorf("failed to get channel for group %s: %w", group.Name, err)
	}

	isValid, validationErr := ch.ValidateKey(ctx, key, group, model)

	var errorMsg string
	if !isValid && validationErr != nil {
		errorMsg = validationErr.Error()
	}
	s.keypoolProvider.UpdateStatus(key, group, isValid, errorMsg)

	if !isValid {
		logrus.WithFields(logrus.Fields{
			"error":    validationErr,
			"key_id":   key.ID,
			"group_id": group.ID,
		}).Debug("Key validation failed")
		return false, validationErr
	}

	logrus.WithFields(logrus.Fields{
		"key_id":   key.ID,
		"is_valid": isValid,
	}).Debug("Key validation successful")

	return true, nil
}

// TestMultipleKeys 对特定组中的密钥值列表执行同步验证。
// model 参数是可选的，如果提供则覆盖组的 test_model。
func (s *KeyValidator) TestMultipleKeys(group *models.Group, keyValues []string, model string) ([]KeyTestResult, error) {
	results := make([]KeyTestResult, len(keyValues))

	// 为所有密钥值生成哈希
	var keyHashes []string
	for _, keyValue := range keyValues {
		keyHash := s.encryptionSvc.Hash(keyValue)
		if keyHash == "" {
			continue
		}
		keyHashes = append(keyHashes, keyHash)
	}

	// 查找提供的密钥中哪些实际存在于该组的数据库中
	var existingKeys []models.APIKey
	if len(keyHashes) > 0 {
		if err := s.DB.Where("group_id = ? AND key_hash IN ?", group.ID, keyHashes).Find(&existingKeys).Error; err != nil {
			return nil, fmt.Errorf("failed to query keys from DB: %w", err)
		}
	}

	// 创建 key_hash 到 APIKey 的映射以便快速查找
	existingKeyMap := make(map[string]models.APIKey)
	for _, k := range existingKeys {
		existingKeyMap[k.KeyHash] = k
	}

	for i, kv := range keyValues {
		keyHash := s.encryptionSvc.Hash(kv)
		apiKey, exists := existingKeyMap[keyHash]
		if !exists {
			results[i] = KeyTestResult{
				KeyValue: kv,
				IsValid:  false,
				Error:    "Key does not exist in this group or has been removed.",
			}
			continue
		}

		apiKey.KeyValue = kv

		isValid, validationErr := s.ValidateSingleKey(&apiKey, group, model)

		results[i] = KeyTestResult{
			KeyValue: kv,
			IsValid:  isValid,
			Error:    "",
		}
		if validationErr != nil {
			results[i].Error = validationErr.Error()
		}
	}

	return results, nil
}
