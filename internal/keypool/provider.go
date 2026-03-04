package keypool

import (
	"errors"
	"fmt"
	"gpt-load/internal/config"
	"gpt-load/internal/encryption"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/store"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	// 数据库加载批量大小（与 config.DBLoadBatchSize 同步）
	dbLoadBatchSize = 10000
)

type KeyProvider struct {
	db              *gorm.DB
	store           store.Store
	settingsManager *config.SystemSettingsManager
	encryptionSvc   encryption.Service
}

// NewProvider 创建一个新的 KeyProvider 实例。
func NewProvider(db *gorm.DB, store store.Store, settingsManager *config.SystemSettingsManager, encryptionSvc encryption.Service) *KeyProvider {
	return &KeyProvider{
		db:              db,
		store:           store,
		settingsManager: settingsManager,
		encryptionSvc:   encryptionSvc,
	}
}

// SelectKey 原子性地为指定组选择并轮换一个可用的 APIKey。
func (p *KeyProvider) SelectKey(groupID uint) (*models.APIKey, error) {
	activeKeysListKey := fmt.Sprintf("group:%d:active_keys", groupID)

	// 1. 从列表中原子性地轮换密钥 ID
	keyIDStr, err := p.store.Rotate(activeKeysListKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, app_errors.ErrNoActiveKeys
		}
		return nil, fmt.Errorf("failed to rotate key from store: %w", err)
	}

	keyID, err := strconv.ParseUint(keyIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key ID '%s': %w", keyIDStr, err)
	}

	// 2. 从 HASH 获取密钥详情
	keyHashKey := fmt.Sprintf("key:%d", keyID)
	keyDetails, err := p.store.HGetAll(keyHashKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get key details for key ID %d: %w", keyID, err)
	}

	// 3. 手动将 map 解码为 APIKey 结构体
	failureCount, _ := strconv.ParseInt(keyDetails["failure_count"], 10, 64)
	createdAt, _ := strconv.ParseInt(keyDetails["created_at"], 10, 64)

	encryptedKeyValue := keyDetails["key_string"]
	decryptedKeyValue, err := p.encryptionSvc.Decrypt(encryptedKeyValue)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"keyID": keyID,
			"error": err,
		}).Warn("Failed to decrypt API key, falling back to plaintext")
		decryptedKeyValue = encryptedKeyValue
	}

	apiKey := &models.APIKey{
		ID:           uint(keyID),
		KeyValue:     decryptedKeyValue,
		Status:       keyDetails["status"],
		FailureCount: failureCount,
		GroupID:      groupID,
		CreatedAt:    time.Unix(createdAt, 0),
	}

	return apiKey, nil
}

// UpdateStatus 同步更新密钥状态。
func (p *KeyProvider) UpdateStatus(apiKey *models.APIKey, group *models.Group, isSuccess bool, errorMessage string) {
	keyHashKey := fmt.Sprintf("key:%d", apiKey.ID)
	activeKeysListKey := fmt.Sprintf("group:%d:active_keys", group.ID)

	if isSuccess {
		if err := p.handleSuccess(apiKey.ID, keyHashKey, activeKeysListKey); err != nil {
			logrus.WithFields(logrus.Fields{"keyID": apiKey.ID, "error": err}).Error("Failed to handle key success")
		}
	} else {
		if app_errors.IsUnCounted(errorMessage) {
			logrus.WithFields(logrus.Fields{
				"keyID": apiKey.ID,
				"error": errorMessage,
			}).Debug("Uncounted error, skipping failure handling")
		} else {
			if err := p.handleFailure(apiKey, group, keyHashKey, activeKeysListKey); err != nil {
				logrus.WithFields(logrus.Fields{"keyID": apiKey.ID, "error": err}).Error("Failed to handle key failure")
			}
		}
	}
}

// isDatabaseLockedError 检查错误是否是 SQLite 数据库锁定错误
func isDatabaseLockedError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "database is locked")
}

// executeTransactionWithRetry 使用重试机制包装数据库事务。
func (p *KeyProvider) executeTransactionWithRetry(operation func(tx *gorm.DB) error) error {
	const maxRetries = 3
	const baseDelay = 50 * time.Millisecond
	const maxJitter = 150 * time.Millisecond
	var err error

	for i := range maxRetries {
		err = p.db.Transaction(operation)
		if err == nil {
			return nil
		}

		if isDatabaseLockedError(err) {
			jitter := time.Duration(rand.Intn(int(maxJitter)))
			totalDelay := baseDelay + jitter
			logrus.Debugf("Database is locked, retrying in %v... (attempt %d/%d)", totalDelay, i+1, maxRetries)
			time.Sleep(totalDelay)
			continue
		}

		break
	}

	return err
}

func (p *KeyProvider) handleSuccess(keyID uint, keyHashKey, activeKeysListKey string) error {
	keyDetails, err := p.store.HGetAll(keyHashKey)
	if err != nil {
		return fmt.Errorf("failed to get key details from store: %w", err)
	}

	failureCount, _ := strconv.ParseInt(keyDetails["failure_count"], 10, 64)
	isActive := keyDetails["status"] == models.KeyStatusActive
	keyExistsInStore := len(keyDetails) > 0

	return p.executeTransactionWithRetry(func(tx *gorm.DB) error {
		var key models.APIKey
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&key, keyID).Error; err != nil {
			return fmt.Errorf("failed to lock key %d for update: %w", keyID, err)
		}

		now := time.Now()
		updates := map[string]any{
			"request_count": gorm.Expr("request_count + ?", 1),
			"last_used_at":  now,
		}

		if failureCount > 0 {
			updates["failure_count"] = 0
		}

		if !isActive {
			updates["status"] = models.KeyStatusActive
		}

		if err := tx.Model(&key).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update key in DB: %w", err)
		}

		// 如果密钥不在缓存中或缓存不完整，重新初始化缓存
		if !keyExistsInStore {
			logrus.WithField("keyID", keyID).Debug("Key not found in store, reinitializing cache.")
			key.RequestCount++
			if failureCount > 0 {
				key.FailureCount = 0
			}
			if !isActive {
				key.Status = models.KeyStatusActive
			}
			if err := p.addKeyToStore(&key); err != nil {
				return fmt.Errorf("failed to reinitialize key in store: %w", err)
			}
			if !isActive {
				logrus.WithField("keyID", keyID).Info("Key has recovered and been restored to active pool.")
			}
			return nil
		}

		// 更新存储中的数据
		storeUpdates := map[string]any{
			"request_count": key.RequestCount + 1,
			"last_used_at":  fmt.Sprint(now.Unix()),
		}
		if failureCount > 0 {
			storeUpdates["failure_count"] = 0
		}
		if !isActive {
			storeUpdates["status"] = models.KeyStatusActive
		}

		if err := p.store.HSet(keyHashKey, storeUpdates); err != nil {
			return fmt.Errorf("failed to update key details in store: %w", err)
		}

		if !isActive {
			logrus.WithField("keyID", keyID).Debug("Key has recovered and is being restored to active pool.")
			if err := p.store.LRem(activeKeysListKey, 0, keyID); err != nil {
				return fmt.Errorf("failed to LRem key before LPush on recovery: %w", err)
			}
			if err := p.store.LPush(activeKeysListKey, keyID); err != nil {
				return fmt.Errorf("failed to LPush key back to active list: %w", err)
			}
		}

		return nil
	})
}

func (p *KeyProvider) handleFailure(apiKey *models.APIKey, group *models.Group, keyHashKey, activeKeysListKey string) error {
	keyDetails, err := p.store.HGetAll(keyHashKey)
	if err != nil {
		return fmt.Errorf("failed to get key details from store: %w", err)
	}

	if keyDetails["status"] == models.KeyStatusInvalid {
		return nil
	}

	failureCount, _ := strconv.ParseInt(keyDetails["failure_count"], 10, 64)

	// 获取组的有效配置
	blacklistThreshold := group.EffectiveConfig.BlacklistThreshold

	return p.executeTransactionWithRetry(func(tx *gorm.DB) error {
		var key models.APIKey
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&key, apiKey.ID).Error; err != nil {
			return fmt.Errorf("failed to lock key %d for update: %w", apiKey.ID, err)
		}

		now := time.Now()
		newFailureCount := failureCount + 1

		updates := map[string]any{
			"failure_count":  newFailureCount,
			"request_count": gorm.Expr("request_count + ?", 1),
			"last_used_at":  now,
		}
		shouldBlacklist := blacklistThreshold > 0 && newFailureCount >= int64(blacklistThreshold)
		if shouldBlacklist {
			updates["status"] = models.KeyStatusInvalid
		}

		if err := tx.Model(&key).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update key stats in DB: %w", err)
		}

		if _, err := p.store.HIncrBy(keyHashKey, "failure_count", 1); err != nil {
			return fmt.Errorf("failed to increment failure count in store: %w", err)
		}
		if _, err := p.store.HIncrBy(keyHashKey, "request_count", 1); err != nil {
			return fmt.Errorf("failed to increment request count in store: %w", err)
		}
		if err := p.store.HSet(keyHashKey, map[string]any{"last_used_at": fmt.Sprint(now.Unix())}); err != nil {
			return fmt.Errorf("failed to update last used at in store: %w", err)
		}

		if shouldBlacklist {
			logrus.WithFields(logrus.Fields{"keyID": apiKey.ID, "threshold": blacklistThreshold}).Warn("Key has reached blacklist threshold, disabling.")
			if err := p.store.LRem(activeKeysListKey, 0, apiKey.ID); err != nil {
				return fmt.Errorf("failed to LRem key from active list: %w", err)
			}
			if err := p.store.HSet(keyHashKey, map[string]any{"status": models.KeyStatusInvalid}); err != nil {
				return fmt.Errorf("failed to update key status to invalid in store: %w", err)
			}
		}

		return nil
	})
}

// InitializeFromDatabase 从数据库加载所有组和密钥并填充存储。
func (p *KeyProvider) InitializeFromDatabase() error {
	logrus.Debug("First time startup, loading keys from DB...")

	// 批量从数据库加载并使用 Pipeline 写入 Redis
	allActiveKeyIDs := make(map[uint][]any)
	batchSize := dbLoadBatchSize
	var batchKeys []*models.APIKey

	err := p.db.Model(&models.APIKey{}).FindInBatches(&batchKeys, batchSize, func(tx *gorm.DB, batch int) error {
		logrus.Debugf("Processing batch %d with %d keys...", batch, len(batchKeys))

		var pipeline store.Pipeliner
		if redisStore, ok := p.store.(store.RedisPipeliner); ok {
			pipeline = redisStore.Pipeline()
		}

		for _, key := range batchKeys {
			keyHashKey := fmt.Sprintf("key:%d", key.ID)
			keyDetails := p.apiKeyToMap(key)

			if pipeline != nil {
				pipeline.HSet(keyHashKey, keyDetails)
			} else {
				if err := p.store.HSet(keyHashKey, keyDetails); err != nil {
					logrus.WithFields(logrus.Fields{"keyID": key.ID, "error": err}).Error("Failed to HSet key details")
				}
			}

			if key.Status == models.KeyStatusActive {
				allActiveKeyIDs[key.GroupID] = append(allActiveKeyIDs[key.GroupID], key.ID)
			}
		}

		if pipeline != nil {
			if err := pipeline.Exec(); err != nil {
				return fmt.Errorf("failed to execute pipeline for batch %d: %w", batch, err)
			}
		}
		return nil
	}).Error

	if err != nil {
		return fmt.Errorf("failed during batch processing of keys: %w", err)
	}

	// 2. 更新所有组的 active_keys 列表
	logrus.Info("Updating active key lists for all groups...")
	for groupID, activeIDs := range allActiveKeyIDs {
		if len(activeIDs) > 0 {
			activeKeysListKey := fmt.Sprintf("group:%d:active_keys", groupID)
			p.store.Delete(activeKeysListKey)
			if err := p.store.LPush(activeKeysListKey, activeIDs...); err != nil {
				logrus.WithFields(logrus.Fields{"groupID": groupID, "error": err}).Error("Failed to LPush active keys for group")
			}
		}
	}

	return nil
}

// AddKeys 批量向密钥池和数据库添加新密钥。
func (p *KeyProvider) AddKeys(groupID uint, keys []models.APIKey) error {
	if len(keys) == 0 {
		return nil
	}

	err := p.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&keys).Error; err != nil {
			return err
		}

		// 使用批量方法添加到缓存
		return p.addKeysToCacheBatch(groupID, keys)
	})

	return err
}

// RemoveKeys 批量从密钥池和数据库删除密钥。
func (p *KeyProvider) RemoveKeys(groupID uint, keyValues []string) (int64, error) {
	if len(keyValues) == 0 {
		return 0, nil
	}

	var keysToDelete []models.APIKey
	var deletedCount int64

	err := p.db.Transaction(func(tx *gorm.DB) error {
		var keyHashes []string
		for _, keyValue := range keyValues {
			keyHash := p.encryptionSvc.Hash(keyValue)
			if keyHash != "" {
				keyHashes = append(keyHashes, keyHash)
			}
		}

		if len(keyHashes) == 0 {
			return nil
		}

		if err := tx.Where("group_id = ? AND key_hash IN ?", groupID, keyHashes).Find(&keysToDelete).Error; err != nil {
			return err
		}

		if len(keysToDelete) == 0 {
			return nil
		}

		keyIDsToDelete := pluckIDs(keysToDelete)

		result := tx.Where("id IN ?", keyIDsToDelete).Delete(&models.APIKey{})
		if result.Error != nil {
			return result.Error
		}
		deletedCount = result.RowsAffected

		for _, key := range keysToDelete {
			if err := p.removeKeyFromStore(key.ID, key.GroupID); err != nil {
				logrus.WithFields(logrus.Fields{"keyID": key.ID, "error": err}).Error("Failed to remove key from store after DB deletion, rolling back transaction")
				return err
			}
		}

		return nil
	})

	return deletedCount, err
}

// RestoreKeys 恢复组中所有无效密钥。
func (p *KeyProvider) RestoreKeys(groupID uint) (int64, error) {
	var invalidKeys []models.APIKey
	var restoredCount int64

	err := p.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ? AND status = ?", groupID, models.KeyStatusInvalid).Find(&invalidKeys).Error; err != nil {
			return err
		}

		if len(invalidKeys) == 0 {
			return nil
		}

		updates := map[string]any{
			"status":        models.KeyStatusActive,
			"failure_count": 0,
		}
		result := tx.Model(&models.APIKey{}).Where("group_id = ? AND status = ?", groupID, models.KeyStatusInvalid).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		restoredCount = result.RowsAffected

		for _, key := range invalidKeys {
			key.Status = models.KeyStatusActive
			key.FailureCount = 0
			if err := p.addKeyToStore(&key); err != nil {
				logrus.WithFields(logrus.Fields{"keyID": key.ID, "error": err}).Error("Failed to restore key in store after DB update, rolling back transaction")
				return err
			}
		}
		return nil
	})

	return restoredCount, err
}

// RestoreMultipleKeys 恢复指定的密钥。
func (p *KeyProvider) RestoreMultipleKeys(groupID uint, keyValues []string) (int64, error) {
	if len(keyValues) == 0 {
		return 0, nil
	}

	var keysToRestore []models.APIKey
	var restoredCount int64

	err := p.db.Transaction(func(tx *gorm.DB) error {
		var keyHashes []string
		for _, keyValue := range keyValues {
			keyHash := p.encryptionSvc.Hash(keyValue)
			if keyHash != "" {
				keyHashes = append(keyHashes, keyHash)
			}
		}

		if len(keyHashes) == 0 {
			return nil
		}

		if err := tx.Where("group_id = ? AND key_hash IN ? AND status = ?", groupID, keyHashes, models.KeyStatusInvalid).Find(&keysToRestore).Error; err != nil {
			return err
		}

		if len(keysToRestore) == 0 {
			return nil
		}

		keyIDsToRestore := pluckIDs(keysToRestore)

		updates := map[string]any{
			"status":        models.KeyStatusActive,
			"failure_count": 0,
		}
		result := tx.Model(&models.APIKey{}).Where("id IN ?", keyIDsToRestore).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		restoredCount = result.RowsAffected

		for _, key := range keysToRestore {
			key.Status = models.KeyStatusActive
			key.FailureCount = 0
			if err := p.addKeyToStore(&key); err != nil {
				logrus.WithFields(logrus.Fields{"keyID": key.ID, "error": err}).Error("Failed to restore key in store after DB update")
				return err
			}
		}

		return nil
	})

	return restoredCount, err
}

// RemoveInvalidKeys 删除组中所有无效密钥。
func (p *KeyProvider) RemoveInvalidKeys(groupID uint) (int64, error) {
	return p.removeKeysByStatus(groupID, models.KeyStatusInvalid)
}

// RemoveAllKeys 删除组中所有密钥。
func (p *KeyProvider) RemoveAllKeys(groupID uint) (int64, error) {
	return p.removeKeysByStatus(groupID)
}

// removeKeysByStatus 是一个通用函数，根据状态删除密钥。
// 如果没有提供状态，则删除组中的所有密钥。
func (p *KeyProvider) removeKeysByStatus(groupID uint, status ...string) (int64, error) {
	var keysToRemove []models.APIKey
	var removedCount int64

	err := p.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("group_id = ?", groupID)
		if len(status) > 0 {
			query = query.Where("status IN ?", status)
		}

		if err := query.Find(&keysToRemove).Error; err != nil {
			return err
		}

		if len(keysToRemove) == 0 {
			return nil
		}

		deleteQuery := tx.Where("group_id = ?", groupID)
		if len(status) > 0 {
			deleteQuery = deleteQuery.Where("status IN ?", status)
		}
		result := deleteQuery.Delete(&models.APIKey{})
		if result.Error != nil {
			return result.Error
		}
		removedCount = result.RowsAffected

		for _, key := range keysToRemove {
			if err := p.removeKeyFromStore(key.ID, key.GroupID); err != nil {
				logrus.WithFields(logrus.Fields{"keyID": key.ID, "error": err}).Error("Failed to remove key from store after DB deletion, rolling back transaction")
				return err
			}
		}
		return nil
	})

	return removedCount, err
}

// RemoveKeysFromStore 直接从内存存储中删除指定的密钥，无需数据库操作。
// 此方法适用于数据库已删除但需要清理内存存储的场景。
func (p *KeyProvider) RemoveKeysFromStore(groupID uint, keyIDs []uint) error {
	if len(keyIDs) == 0 {
		return nil
	}

	activeKeysListKey := fmt.Sprintf("group:%d:active_keys", groupID)

	// 第 1 步：删除整个 active_keys 列表
	if err := p.store.Delete(activeKeysListKey); err != nil {
		logrus.WithFields(logrus.Fields{
			"groupID": groupID,
			"error":   err,
		}).Error("Failed to delete active keys list")
		return err
	}

	// 第 2 步：批量删除所有相关的密钥哈希
	for _, keyID := range keyIDs {
		keyHashKey := fmt.Sprintf("key:%d", keyID)
		if err := p.store.Delete(keyHashKey); err != nil {
			logrus.WithFields(logrus.Fields{
				"keyID": keyID,
				"error": err,
			}).Error("Failed to delete key hash")
		}
	}

	logrus.WithFields(logrus.Fields{
		"groupID":  groupID,
		"keyCount": len(keyIDs),
	}).Info("Successfully cleaned up group keys from store")

	return nil
}

// addKeyToStore 是向缓存添加单个密钥的辅助函数。
func (p *KeyProvider) addKeyToStore(key *models.APIKey) error {
	// 1. 在 HASH 中存储密钥详情
	keyHashKey := fmt.Sprintf("key:%d", key.ID)
	keyDetails := p.apiKeyToMap(key)
	if err := p.store.HSet(keyHashKey, keyDetails); err != nil {
		return fmt.Errorf("failed to HSet key details for key %d: %w", key.ID, err)
	}

	// 2. 如果是活跃的，添加到活跃 LIST
	if key.Status == models.KeyStatusActive {
		activeKeysListKey := fmt.Sprintf("group:%d:active_keys", key.GroupID)
		if err := p.store.LRem(activeKeysListKey, 0, key.ID); err != nil {
			return fmt.Errorf("failed to LRem key %d before LPush for group %d: %w", key.ID, key.GroupID, err)
		}
		if err := p.store.LPush(activeKeysListKey, key.ID); err != nil {
			return fmt.Errorf("failed to LPush key %d to group %d: %w", key.ID, key.GroupID, err)
		}
	}
	return nil
}

// addKeysToCacheBatch 批量向缓存添加密钥（用于批量导入场景）
func (p *KeyProvider) addKeysToCacheBatch(groupID uint, keys []models.APIKey) error {
	if len(keys) == 0 {
		return nil
	}

	// 1. 批量 HSet 密钥详情
	if pipeliner, ok := p.store.(store.RedisPipeliner); ok {
		// Redis：使用 Pipeline 进行批量操作
		pipe := pipeliner.Pipeline()
		for i := range keys {
			keyHashKey := fmt.Sprintf("key:%d", keys[i].ID)
			pipe.HSet(keyHashKey, p.apiKeyToMap(&keys[i]))
		}
		if err := pipe.Exec(); err != nil {
			return fmt.Errorf("failed to batch HSet keys: %w", err)
		}
	} else {
		// MemoryStore：回退到单个 HSet
		for i := range keys {
			keyHashKey := fmt.Sprintf("key:%d", keys[i].ID)
			if err := p.store.HSet(keyHashKey, p.apiKeyToMap(&keys[i])); err != nil {
				return fmt.Errorf("failed to HSet key %d: %w", keys[i].ID, err)
			}
		}
	}

	// 2. 收集所有密钥 ID
	activeKeysListKey := fmt.Sprintf("group:%d:active_keys", groupID)
	activeKeyIDs := make([]any, len(keys))
	for i := range keys {
		activeKeyIDs[i] = keys[i].ID
	}

	// 3. 批量 LPush 活跃密钥
	if err := p.store.LPush(activeKeysListKey, activeKeyIDs...); err != nil {
		return fmt.Errorf("failed to batch LPush keys to group %d: %w", groupID, err)
	}

	return nil
}

// removeKeyFromStore 是从缓存删除单个密钥的辅助函数。
func (p *KeyProvider) removeKeyFromStore(keyID, groupID uint) error {
	activeKeysListKey := fmt.Sprintf("group:%d:active_keys", groupID)
	if err := p.store.LRem(activeKeysListKey, 0, keyID); err != nil {
		logrus.WithFields(logrus.Fields{"keyID": keyID, "groupID": groupID, "error": err}).Error("Failed to LRem key from active list")
	}

	keyHashKey := fmt.Sprintf("key:%d", keyID)
	if err := p.store.Delete(keyHashKey); err != nil {
		return fmt.Errorf("failed to delete key HASH for key %d: %w", keyID, err)
	}
	return nil
}

// apiKeyToMap 将 APIKey 模型转换为用于 HSET 的 map。
func (p *KeyProvider) apiKeyToMap(key *models.APIKey) map[string]any {
	return map[string]any{
		"id":            fmt.Sprint(key.ID),
		"key_string":    key.KeyValue,
		"status":        key.Status,
		"failure_count": key.FailureCount,
		"request_count": key.RequestCount,
		"group_id":      key.GroupID,
		"created_at":    fmt.Sprint(key.CreatedAt.Unix()),
	}
}

// pluckIDs 从 APIKey 切片中提取 ID。
func pluckIDs(keys []models.APIKey) []uint {
	ids := make([]uint, len(keys))
	for i, key := range keys {
		ids[i] = key.ID
	}
	return ids
}
