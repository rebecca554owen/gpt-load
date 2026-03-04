package db

import (
	"fmt"
	"gpt-load/internal/encryption"
	"gpt-load/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// V1_1_0_AddKeyHashColumn 向 api_keys 和 request_logs 表添加 key_hash 列
func V1_1_0_AddKeyHashColumn(db *gorm.DB) error {
	// 首先检查是否有需要迁移的记录
	var needMigrateCount int64
	db.Model(&models.APIKey{}).
		Where("key_hash IS NULL OR key_hash = ''").
		Count(&needMigrateCount)

	if needMigrateCount == 0 {
		logrus.Info("No api_keys need migration, skipping v1.1.0...")
		return nil
	}

	logrus.Infof("Found %d api_keys need to populate key_hash", needMigrateCount)

	encSvc, err := encryption.NewService("")
	if err != nil {
		return fmt.Errorf("failed to initialize encryption service: %w", err)
	}

	// 分批处理以避免内存问题
	const batchSize = 1000

	for {
		var apiKeys []models.APIKey
		// 仅查询需要迁移的记录
		result := db.Where("key_hash IS NULL OR key_hash = ''").
			Limit(batchSize).
			Find(&apiKeys)

		if result.Error != nil {
			return fmt.Errorf("failed to fetch api_keys: %w", result.Error)
		}

		if len(apiKeys) == 0 {
			break
		}

		// 更新每个密钥的哈希
		for _, key := range apiKeys {
			// 生成哈希
			keyHash := encSvc.Hash(key.KeyValue)

			// 更新记录
			if err := db.Model(&models.APIKey{}).
				Where("id = ?", key.ID).
				Update("key_hash", keyHash).Error; err != nil {
				logrus.WithError(err).Errorf("Failed to update key_hash for api_key ID %d", key.ID)
			}
		}
	}

	logrus.Info("Migration v1.1.0 completed successfully")
	return nil
}
