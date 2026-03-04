package services

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"gpt-load/internal/encryption"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/keypool"
	"gpt-load/internal/models"

	"github.com/goccy/go-json"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	// Batch processing sizes (synced with config.MaxRequestKeys/config.KeyChunkSize)
	maxRequestKeys = 5000
	chunkSize      = 500
)

// AddKeysResult 存储添加多个密钥的结果。
type AddKeysResult struct {
	AddedCount   int   `json:"added_count"`
	IgnoredCount int   `json:"ignored_count"`
	TotalInGroup int64 `json:"total_in_group"`
}

// DeleteKeysResult 存储删除多个密钥的结果。
type DeleteKeysResult struct {
	DeletedCount int   `json:"deleted_count"`
	IgnoredCount int   `json:"ignored_count"`
	TotalInGroup int64 `json:"total_in_group"`
}

// RestoreKeysResult 存储恢复多个密钥的结果。
type RestoreKeysResult struct {
	RestoredCount int   `json:"restored_count"`
	IgnoredCount  int   `json:"ignored_count"`
	TotalInGroup  int64 `json:"total_in_group"`
}

// KeyService 提供与 API 密钥相关的服务。
type KeyService struct {
	DB            *gorm.DB
	KeyProvider   *keypool.KeyProvider
	KeyValidator  *keypool.KeyValidator
	EncryptionSvc encryption.Service
}

// NewKeyService 创建一个新的 KeyService。
func NewKeyService(db *gorm.DB, keyProvider *keypool.KeyProvider, keyValidator *keypool.KeyValidator, encryptionSvc encryption.Service) *KeyService {
	return &KeyService{
		DB:            db,
		KeyProvider:   keyProvider,
		KeyValidator:  keyValidator,
		EncryptionSvc: encryptionSvc,
	}
}

// AddMultipleKeys 处理从文本块创建新密钥的业务逻辑。
// 已废弃：对于大批量导入请使用 KeyImportService
func (s *KeyService) AddMultipleKeys(groupID uint, keysText string) (*AddKeysResult, error) {
	keys, err := s.parseAndValidateKeys(keysText)
	if err != nil {
		return nil, err
	}

	addedCount, ignoredCount, err := s.processAndCreateKeys(groupID, keys, nil)
	if err != nil {
		return nil, err
	}

	totalInGroup, err := s.countKeysInGroup(groupID)
	if err != nil {
		return nil, err
	}

	return &AddKeysResult{
		AddedCount:   addedCount,
		IgnoredCount: ignoredCount,
		TotalInGroup: totalInGroup,
	}, nil
}

// processAndCreateKeys 是添加密钥的最低级别可复用函数。
func (s *KeyService) processAndCreateKeys(
	groupID uint,
	keys []string,
	progressCallback func(processed int),
) (addedCount int, ignoredCount int, err error) {
	// 1. Get existing key hashes in the group for deduplication
	var existingHashes []string
	if err := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Pluck("key_hash", &existingHashes).Error; err != nil {
		return 0, 0, err
	}
	existingHashMap := make(map[string]bool)
	for _, h := range existingHashes {
		existingHashMap[h] = true
	}

	// 2. Prepare new keys for creation
	var newKeysToCreate []models.APIKey
	uniqueNewKeys := make(map[string]bool)

	for _, keyVal := range keys {
		trimmedKey := strings.TrimSpace(keyVal)
		if trimmedKey == "" || uniqueNewKeys[trimmedKey] || !s.isValidKeyFormat(trimmedKey) {
			continue
		}

		// Generate hash for deduplication check
		keyHash := s.EncryptionSvc.Hash(trimmedKey)
		if existingHashMap[keyHash] {
			continue
		}

		encryptedKey, err := s.EncryptionSvc.Encrypt(trimmedKey)
		if err != nil {
			logrus.WithError(err).WithField("key", trimmedKey).Error("Failed to encrypt key, skipping")
			continue
		}

		uniqueNewKeys[trimmedKey] = true
		newKeysToCreate = append(newKeysToCreate, models.APIKey{
			GroupID:  groupID,
			KeyValue: encryptedKey,
			KeyHash:  keyHash,
			Status:   models.KeyStatusActive,
		})
	}

	if len(newKeysToCreate) == 0 {
		return 0, len(keys), nil
	}

	// 3. Use KeyProvider to add keys in chunks
	for i := 0; i < len(newKeysToCreate); i += chunkSize {
		end := i + chunkSize
		if end > len(newKeysToCreate) {
			end = len(newKeysToCreate)
		}
		chunk := newKeysToCreate[i:end]
		if err := s.KeyProvider.AddKeys(groupID, chunk); err != nil {
			return addedCount, len(keys) - addedCount, err
		}
		addedCount += len(chunk)

		if progressCallback != nil {
			progressCallback(i + len(chunk))
		}
	}

	return addedCount, len(keys) - addedCount, nil
}

// ParseKeysFromText 将各种格式的密钥字符串解析为字符串切片。
// 此函数已导出，可与处理层共享。
func (s *KeyService) ParseKeysFromText(text string) []string {
	var keys []string

	// First, try to parse as a JSON array of strings
	if json.Unmarshal([]byte(text), &keys) == nil && len(keys) > 0 {
		return s.filterValidKeys(keys)
	}

	// General parsing: split text by delimiters, no complex regex
	delimiters := regexp.MustCompile(`[\s,;\n\r\t]+`)
	splitKeys := delimiters.Split(strings.TrimSpace(text), -1)

	for _, key := range splitKeys {
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}

	return s.filterValidKeys(keys)
}

// filterValidKeys 验证并过滤潜在的 API 密钥
func (s *KeyService) filterValidKeys(keys []string) []string {
	var validKeys []string
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if s.isValidKeyFormat(key) {
			validKeys = append(validKeys, key)
		}
	}
	return validKeys
}

// isValidKeyFormat 对密钥格式执行基本验证
func (s *KeyService) isValidKeyFormat(key string) bool {
	return strings.TrimSpace(key) != ""
}

// RestoreMultipleKeys 处理从文本块恢复密钥的业务逻辑。
func (s *KeyService) RestoreMultipleKeys(groupID uint, keysText string) (*RestoreKeysResult, error) {
	keys, err := s.parseAndValidateKeys(keysText)
	if err != nil {
		return nil, err
	}

	totalRestoredCount, err := s.processBatchWithResult(keys, func(chunk []string) (int64, error) {
		return s.KeyProvider.RestoreMultipleKeys(groupID, chunk)
	})
	if err != nil {
		return nil, err
	}

	totalInGroup, err := s.countKeysInGroup(groupID)
	if err != nil {
		return nil, err
	}

	return &RestoreKeysResult{
		RestoredCount: int(totalRestoredCount),
		IgnoredCount:  len(keys) - int(totalRestoredCount),
		TotalInGroup:  totalInGroup,
	}, nil
}

// RestoreAllInvalidKeys 将组内所有 'inactive' 密钥的状态设置为 'active'。
func (s *KeyService) RestoreAllInvalidKeys(groupID uint) (int64, error) {
	return s.KeyProvider.RestoreKeys(groupID)
}

// ClearAllInvalidKeys 从组中删除所有 'inactive' 密钥。
func (s *KeyService) ClearAllInvalidKeys(groupID uint) (int64, error) {
	return s.KeyProvider.RemoveInvalidKeys(groupID)
}

// ClearAllKeys 从组中删除所有密钥。
func (s *KeyService) ClearAllKeys(groupID uint) (int64, error) {
	return s.KeyProvider.RemoveAllKeys(groupID)
}

// DeleteMultipleKeys 处理从文本块删除密钥的业务逻辑。
func (s *KeyService) DeleteMultipleKeys(groupID uint, keysText string) (*DeleteKeysResult, error) {
	keys, err := s.parseAndValidateKeys(keysText)
	if err != nil {
		return nil, err
	}

	totalDeletedCount, err := s.processBatchWithResult(keys, func(chunk []string) (int64, error) {
		return s.KeyProvider.RemoveKeys(groupID, chunk)
	})
	if err != nil {
		return nil, err
	}

	totalInGroup, err := s.countKeysInGroup(groupID)
	if err != nil {
		return nil, err
	}

	return &DeleteKeysResult{
		DeletedCount: int(totalDeletedCount),
		IgnoredCount: len(keys) - int(totalDeletedCount),
		TotalInGroup: totalInGroup,
	}, nil
}

// ListKeysInGroupQuery 构建查询以列出特定组内的所有密钥，按状态过滤。
func (s *KeyService) ListKeysInGroupQuery(groupID uint, statusFilter string, searchHash string) *gorm.DB {
	query := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID)

	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	if searchHash != "" {
		query = query.Where("key_hash = ?", searchHash)
	}

	orderBy := "last_used_at desc, id desc"
	if s.DB.Dialector.Name() == "postgres" {
		orderBy = "last_used_at desc nulls last, id desc"
	}

	query = query.Order(orderBy)

	return query
}

// TestMultipleKeys 处理多个密钥的一次性验证测试。
// model 参数是可选的，如果提供则覆盖组的 test_model。
func (s *KeyService) TestMultipleKeys(group *models.Group, keysText string, model string) ([]keypool.KeyTestResult, error) {
	keys, err := s.parseAndValidateKeys(keysText)
	if err != nil {
		return nil, err
	}

	var allResults []keypool.KeyTestResult
	_, err = s.processBatch(keys, func(chunk []string) error {
		results, err := s.KeyValidator.TestMultipleKeys(group, chunk, model)
		if err != nil {
			return err
		}
		allResults = append(allResults, results...)
		return nil
	})

	return allResults, err
}

// StreamKeysToWriter 从数据库批量获取密钥并将其写入提供的 writer。
func (s *KeyService) StreamKeysToWriter(groupID uint, statusFilter string, writer io.Writer) error {
	query := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Select("id, key_value")

	switch statusFilter {
	case models.KeyStatusActive, models.KeyStatusInvalid:
		query = query.Where("status = ?", statusFilter)
	case "all":
	default:
		return fmt.Errorf("invalid status filter: %s", statusFilter)
	}

	var keys []models.APIKey
	err := query.FindInBatches(&keys, chunkSize, func(tx *gorm.DB, batch int) error {
		for _, key := range keys {
			decryptedKey, err := s.EncryptionSvc.Decrypt(key.KeyValue)
			if err != nil {
				logrus.WithError(err).WithField("key_id", key.ID).Error("Failed to decrypt key for streaming, skipping")
				continue
			}
			if _, err := writer.Write([]byte(decryptedKey + "\n")); err != nil {
				return err
			}
		}
		return nil
	}).Error

	return err
}

// parseAndValidateKeys 从文本解析和验证密钥。
func (s *KeyService) parseAndValidateKeys(keysText string) ([]string, error) {
	keys := s.ParseKeysFromText(keysText)
	if len(keys) > maxRequestKeys {
		return nil, app_errors.NewServiceErrorf(app_errors.ErrBatchSizeExceedsLimit, "batch size exceeds the limit of %d keys, got %d", maxRequestKeys, len(keys))
	}
	if len(keys) == 0 {
		return nil, app_errors.NewServiceError(app_errors.ErrNoValidKeysFound, "no valid keys found in the input text")
	}
	return keys, nil
}

// countKeysInGroup 返回组中密钥的总数。
func (s *KeyService) countKeysInGroup(groupID uint) (int64, error) {
	var totalInGroup int64
	if err := s.DB.Model(&models.APIKey{}).Where("group_id = ?", groupID).Count(&totalInGroup).Error; err != nil {
		return 0, err
	}
	return totalInGroup, nil
}

// batchOperationFunc 定义返回计数的批量操作。
type batchOperationFunc func(chunk []string) (int64, error)

// processBatchWithResult 批量处理项目并返回总计数。
func (s *KeyService) processBatchWithResult(items []string, operation batchOperationFunc) (int64, error) {
	var totalCount int64
	for i := 0; i < len(items); i += chunkSize {
		end := min(i+chunkSize, len(items))
		chunk := items[i:end]
		count, err := operation(chunk)
		if err != nil {
			return totalCount, err
		}
		totalCount += count
	}
	return totalCount, nil
}

// batchFunc 定义无返回值的批量操作。
type batchFunc func(chunk []string) error

// processBatch 批量处理项目。
func (s *KeyService) processBatch(items []string, operation batchFunc) (int, error) {
	for i := 0; i < len(items); i += chunkSize {
		end := min(i+chunkSize, len(items))
		if err := operation(items[i:end]); err != nil {
			return i, err
		}
	}
	return len(items), nil
}
