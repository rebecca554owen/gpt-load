package commands

import (
	"flag"
	"fmt"
	"gpt-load/internal/container"
	db "gpt-load/internal/db/migrations"
	"gpt-load/internal/encryption"
	"gpt-load/internal/models"
	"gpt-load/internal/store"
	"gpt-load/internal/types"
	"gpt-load/internal/utils"
	"os"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// RunMigrateKeys 处理 migrate-keys 命令入口点
func RunMigrateKeys(args []string) {
	// 解析 migrate-keys 子命令参数
	migrateCmd := flag.NewFlagSet("migrate-keys", flag.ExitOnError)
	fromKey := migrateCmd.String("from", "", "Source encryption key (for decrypting existing data)")
	toKey := migrateCmd.String("to", "", "Target encryption key (for encrypting new data)")

	// 设置自定义使用消息
	migrateCmd.Usage = func() {
		fmt.Println("GPT-Load Key Migration Tool")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  Enable encryption: gpt-load migrate-keys --to new-key")
		fmt.Println("  Disable encryption: gpt-load migrate-keys --from old-key")
		fmt.Println("  Change key: gpt-load migrate-keys --from old-key --to new-key")
		fmt.Println()
		fmt.Println("Arguments:")
		migrateCmd.PrintDefaults()
		fmt.Println()
		fmt.Println("⚠️  Important Notes:")
		fmt.Println("  1. Always backup database before migration")
		fmt.Println("  2. Stop service during migration")
		fmt.Println("  3. Restart service after migration completes")
	}

	// 解析参数
	if err := migrateCmd.Parse(args); err != nil {
		logrus.Fatalf("Parameter parsing failed: %v", err)
	}

	// 检查是否应显示帮助
	if len(args) == 0 || (*fromKey == "" && *toKey == "") {
		migrateCmd.Usage()
		os.Exit(0)
	}

	// 构建依赖注入容器
	cont, err := container.BuildContainer()
	if err != nil {
		logrus.Fatalf("Failed to build container: %v", err)
	}

	// 初始化全局日志器
	if err := cont.Invoke(func(configManager types.ConfigManager) {
		utils.SetupLogger(configManager)
	}); err != nil {
		logrus.Fatalf("Failed to setup logger: %v", err)
	}

	// 执行迁移命令
	if err := cont.Invoke(func(db *gorm.DB, configManager types.ConfigManager, cacheStore store.Store) {
		migrateKeysCmd := NewMigrateKeysCommand(db, configManager, cacheStore, *fromKey, *toKey)
		if err := migrateKeysCmd.Execute(); err != nil {
			logrus.Fatalf("Key migration failed: %v", err)
		}
	}); err != nil {
		logrus.Fatalf("Failed to execute migration: %v", err)
	}

	logrus.Info("Key migration command completed")
}

// 迁移批大小配置
const migrationBatchSize = 1000

// MigrateKeysCommand 处理加密密钥迁移
type MigrateKeysCommand struct {
	db            *gorm.DB
	configManager types.ConfigManager
	cacheStore    store.Store
	fromKey       string
	toKey         string
}

// NewMigrateKeysCommand 创建新的迁移命令
func NewMigrateKeysCommand(db *gorm.DB, configManager types.ConfigManager, cacheStore store.Store, fromKey, toKey string) *MigrateKeysCommand {
	return &MigrateKeysCommand{
		db:            db,
		configManager: configManager,
		cacheStore:    cacheStore,
		fromKey:       fromKey,
		toKey:         toKey,
	}
}

// Execute 执行密钥迁移
func (cmd *MigrateKeysCommand) Execute() error {
	db.HandleLegacyIndexes(cmd.db)
	// 预先处理。数据库迁移和修复
	if err := cmd.db.AutoMigrate(&models.APIKey{}); err != nil {
		return fmt.Errorf("database auto-migration failed: %w", err)
	}

	// 1. 验证参数并获取场景
	scenario, err := cmd.validateAndGetScenario()
	if err != nil {
		return fmt.Errorf("parameter validation failed: %w", err)
	}

	logrus.Infof("Starting key migration, scenario: %s", scenario)

	// 2. 预检查 - 验证当前密钥可以解密所有数据
	if err := cmd.preCheck(); err != nil {
		return fmt.Errorf("pre-check failed: %w", err)
	}

	// 3. 将数据迁移到临时列
	if err := cmd.createBackupTableAndMigrate(); err != nil {
		return fmt.Errorf("data migration failed: %w", err)
	}

	// 4. 验证临时列数据完整性
	if err := cmd.verifyTempColumns(); err != nil {
		logrus.Errorf("Data verification failed: %v", err)
		return fmt.Errorf("data verification failed: %w", err)
	}

	// 5. 原子性切换列
	if err := cmd.switchColumns(); err != nil {
		logrus.Errorf("Column switch failed: %v", err)
		return fmt.Errorf("column switch failed: %w", err)
	}

	// 6. 清除缓存
	if err := cmd.clearCache(); err != nil {
		logrus.Warnf("Cache cleanup failed, recommend manual service restart: %v", err)
	}

	// 7. 清理临时表
	if err := cmd.dropTempTable(); err != nil {
		logrus.Warnf("Temporary table cleanup failed, can manually drop temp_migration table: %v", err)
	}

	logrus.Info("Key migration completed successfully!")
	logrus.Info("Recommend restarting service to ensure all cached data is loaded correctly")

	return nil
}

// validateAndGetScenario 验证参数并返回迁移场景
func (cmd *MigrateKeysCommand) validateAndGetScenario() (string, error) {
	hasFrom := cmd.fromKey != ""
	hasTo := cmd.toKey != ""

	switch {
	case !hasFrom && hasTo:
		// 启用加密
		utils.ValidatePasswordStrength(cmd.toKey, "new encryption key")
		return "enable encryption", nil
	case hasFrom && !hasTo:
		// 禁用加密
		return "disable encryption", nil
	case hasFrom && hasTo:
		// 更改加密密钥
		if cmd.fromKey == cmd.toKey {
			return "", fmt.Errorf("new and old keys cannot be the same")
		}
		utils.ValidatePasswordStrength(cmd.toKey, "new encryption key")
		return "change encryption key", nil
	default:
		return "", fmt.Errorf("must specify --from or --to parameter, or both")
	}
}

// preCheck 验证当前数据是否能被正确处理
func (cmd *MigrateKeysCommand) preCheck() error {
	logrus.Info("Executing pre-check...")

	// 关键检查：如果启用加密（fromKey 为空），确保数据尚未加密
	if cmd.fromKey == "" && cmd.toKey != "" {
		if err := cmd.detectIfAlreadyEncrypted(); err != nil {
			return err
		}
	}

	// 仅根据参数获取当前加密服务
	var currentService encryption.Service
	var err error

	if cmd.fromKey != "" {
		// 使用 fromKey 创建加密服务进行验证
		currentService, err = encryption.NewService(cmd.fromKey)
	} else {
		// 启用加密场景：数据应该是未加密的
		// 使用无操作服务验证数据未加密
		currentService, err = encryption.NewService("")
	}

	if err != nil {
		return fmt.Errorf("failed to create current encryption service: %w", err)
	}

	// 检查数据库中的密钥数量
	var totalCount int64
	if err := cmd.db.Model(&models.APIKey{}).Count(&totalCount).Error; err != nil {
		return fmt.Errorf("failed to get total key count: %w", err)
	}

	if totalCount == 0 {
		logrus.Info("No key data in database, skipping pre-check")
		return nil
	}

	logrus.Infof("Starting validation of %d keys...", totalCount)

	// 批量验证所有密钥可以正确解密
	offset := 0
	failedCount := 0

	for {
		var keys []models.APIKey
		if err := cmd.db.Order("id").Offset(offset).Limit(migrationBatchSize).Find(&keys).Error; err != nil {
			return fmt.Errorf("failed to get key data: %w", err)
		}

		if len(keys) == 0 {
			break
		}

		for _, key := range keys {
			_, err := currentService.Decrypt(key.KeyValue)
			if err != nil {
				logrus.Errorf("Key ID %d decryption failed: %v", key.ID, err)
				failedCount++
			}
		}

		offset += migrationBatchSize
		// 确保我们显示的数量不超过总数
		actualVerified := offset
		if int64(offset) > totalCount {
			actualVerified = int(totalCount)
		}
		logrus.Infof("Verified %d/%d keys", actualVerified, totalCount)
	}

	if failedCount > 0 {
		return fmt.Errorf("found %d keys that cannot be decrypted, please check the --from parameter", failedCount)
	}

	logrus.Info("Pre-check passed, all keys verified successfully")
	return nil
}

// detectIfAlreadyEncrypted 检查数据是否已加密以防止双重加密
func (cmd *MigrateKeysCommand) detectIfAlreadyEncrypted() error {
	logrus.Info("Detecting if data is already encrypted...")

	// 采样检查
	var sampleKeys []models.APIKey
	if err := cmd.db.Limit(20).Where("key_hash IS NOT NULL AND key_hash != ''").Find(&sampleKeys).Error; err != nil {
		return fmt.Errorf("failed to fetch sample keys: %w", err)
	}

	if len(sampleKeys) == 0 {
		logrus.Info("No keys found in database, safe to proceed")
		return nil
	}

	// 1. 哈希一致性检查
	// 如果数据未加密，key_hash 应该等于 SHA256(key_value)
	hashConsistentCount := 0
	noopService, err := encryption.NewService("") // SHA256 service for unencrypted data
	if err != nil {
		return fmt.Errorf("failed to create noop service: %w", err)
	}

	for _, key := range sampleKeys {
		// 对于未加密数据：key_hash 应该匹配 SHA256(key_value)
		expectedHash := noopService.Hash(key.KeyValue)
		if expectedHash == key.KeyHash {
			hashConsistentCount++
		}
	}

	// 2. 分析结果
	if hashConsistentCount == len(sampleKeys) {
		// 所有哈希匹配 SHA256(key_value) - 数据未加密
		logrus.Info("Hash check passed: Data appears to be unencrypted (SHA256 hashes match)")
		return nil // 可以继续进行加密
	}

	if hashConsistentCount == 0 {
		// 没有哈希匹配 SHA256(key_value) - 数据已经加密！

		// 3. 进一步检查：我们能用目标密钥解密吗？
		if cmd.toKey != "" {
			targetService, err := encryption.NewService(cmd.toKey)
			if err != nil {
				return fmt.Errorf("failed to create target encryption service: %w", err)
			}

			canDecryptCount := 0
			for _, key := range sampleKeys {
				decrypted, err := targetService.Decrypt(key.KeyValue)
				if err == nil {
					// 验证哈希匹配
					expectedHash := targetService.Hash(decrypted)
					if expectedHash == key.KeyHash {
						canDecryptCount++
					}
				}
			}

			if canDecryptCount > 0 {
				return fmt.Errorf(
					"CRITICAL: Data is already encrypted with the target key! %d/%d keys can be decrypted with target key",
					canDecryptCount,
					len(sampleKeys),
				)
			}
		}

		return fmt.Errorf(
			"CRITICAL: Data appears to be already encrypted! 0/%d keys have matching SHA256 hashes (expected for unencrypted data)",
			len(sampleKeys),
		)
	}

	// 部分匹配 - 不一致的数据状态
	return fmt.Errorf(
		"WARNING: Inconsistent data state detected! %d/%d keys appear unencrypted (SHA256 hash matches), %d/%d keys appear encrypted (SHA256 hash doesn't match)",
		hashConsistentCount,
		len(sampleKeys),
		len(sampleKeys)-hashConsistentCount,
		len(sampleKeys),
	)
}

// createBackupTableAndMigrate 使用临时表执行迁移
func (cmd *MigrateKeysCommand) createBackupTableAndMigrate() error {
	logrus.Info("Starting key migration using temporary table...")

	// 1. 创建临时表
	if err := cmd.createTempTable(); err != nil {
		return fmt.Errorf("failed to create temporary table: %w", err)
	}

	// 2. 创建旧加密服务和新加密服务
	oldService, newService, err := cmd.createMigrationServices()
	if err != nil {
		return err
	}

	// 3. 获取要迁移的总数
	var totalCount int64
	if err := cmd.db.Model(&models.APIKey{}).Count(&totalCount).Error; err != nil {
		return fmt.Errorf("failed to get key count: %w", err)
	}

	if totalCount == 0 {
		logrus.Info("No keys to migrate")
		return nil
	}

	logrus.Infof("Starting migration of %d keys...", totalCount)

	// 4. 批量处理迁移
	processedCount := 0
	lastID := uint(0)

	for {
		var keys []models.APIKey
		// 使用基于 ID 的分页以获得稳定的结果
		if err := cmd.db.Where("id > ?", lastID).Order("id").Limit(migrationBatchSize).Find(&keys).Error; err != nil {
			return fmt.Errorf("failed to get key data: %w", err)
		}

		if len(keys) == 0 {
			break
		}

		// 处理当前批次到临时表
		if err := cmd.processBatchToTempTable(keys, oldService, newService); err != nil {
			return fmt.Errorf("failed to process batch data: %w", err)
		}

		processedCount += len(keys)
		lastID = keys[len(keys)-1].ID
		logrus.Infof("Processed %d/%d keys", processedCount, totalCount)
	}

	logrus.Info("Data migration to temporary table completed")
	return nil
}

// createTempTable 创建用于迁移的临时表
func (cmd *MigrateKeysCommand) createTempTable() error {
	logrus.Info("Creating temporary migration table...")

	// 如果存在则删除现有临时表
	if err := cmd.db.Exec("DROP TABLE IF EXISTS temp_migration").Error; err != nil {
		logrus.WithError(err).Warn("Failed to drop existing temp table, continuing anyway")
	}

	dbType := cmd.db.Dialector.Name()
	var createTableSQL string

	// 使用数据库特定语法以获得更好的兼容性
	switch dbType {
	case "mysql":
		createTableSQL = `
			CREATE TABLE temp_migration (
				id BIGINT PRIMARY KEY,
				key_value_new TEXT,
				key_hash_new VARCHAR(255)
			)
		`
	case "postgres":
		createTableSQL = `
			CREATE TABLE temp_migration (
				id BIGINT PRIMARY KEY,
				key_value_new TEXT,
				key_hash_new VARCHAR(255)
			)
		`
	case "sqlite":
		// SQLite 使用 INTEGER 作为主键
		createTableSQL = `
			CREATE TABLE temp_migration (
				id INTEGER PRIMARY KEY,
				key_value_new TEXT,
				key_hash_new VARCHAR(255)
			)
		`
	default:
		// 回退到通用语法
		createTableSQL = `
			CREATE TABLE temp_migration (
				id INTEGER PRIMARY KEY,
				key_value_new TEXT,
				key_hash_new VARCHAR(255)
			)
		`
	}

	// 创建最小结构的临时表
	if err := cmd.db.Exec(createTableSQL).Error; err != nil {
		return fmt.Errorf("failed to create temp_migration table: %w", err)
	}

	// 创建索引以提高 UPDATE 性能（不需要主键但有助于 JOIN）
	// 跳过索引创建，因为 id 已经是主键，会创建隐式索引

	return nil
}

// dropTempTable 删除临时迁移表
func (cmd *MigrateKeysCommand) dropTempTable() error {
	logrus.Info("Dropping temporary migration table...")

	if err := cmd.db.Exec("DROP TABLE IF EXISTS temp_migration").Error; err != nil {
		return fmt.Errorf("failed to drop temp_migration table: %w", err)
	}

	logrus.Info("Temporary table dropped successfully")
	return nil
}

// createMigrationServices 创建用于迁移的旧加密服务和新加密服务
func (cmd *MigrateKeysCommand) createMigrationServices() (oldService, newService encryption.Service, err error) {
	// 创建旧加密服务（用于解密），仅基于参数
	if cmd.fromKey != "" {
		// 使用指定密钥解密
		oldService, err = encryption.NewService(cmd.fromKey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create old encryption service: %w", err)
		}
	} else {
		// 启用加密场景：数据应该是未加密的
		// 使用无操作服务（空密钥意味着不加密）
		oldService, err = encryption.NewService("")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create noop encryption service for source: %w", err)
		}
	}

	// 创建新加密服务（用于加密），仅基于参数
	if cmd.toKey != "" {
		// Encrypt with specified key
		newService, err = encryption.NewService(cmd.toKey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create new encryption service: %w", err)
		}
	} else {
		// 禁用加密场景：数据应该是未加密的
		// Use noop service (empty key means no encryption)
		newService, err = encryption.NewService("")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create noop encryption service for target: %w", err)
		}
	}

	return oldService, newService, nil
}

// processBatchToTempTable 处理一批密钥并写入临时表
func (cmd *MigrateKeysCommand) processBatchToTempTable(keys []models.APIKey, oldService, newService encryption.Service) error {
	// 准备要插入的批次数据
	type TempMigration struct {
		ID          uint   `gorm:"primaryKey"`
		KeyValueNew string `gorm:"column:key_value_new"`
		KeyHashNew  string `gorm:"column:key_hash_new"`
	}

	var tempRecords []TempMigration

	for _, key := range keys {
		// 1. 使用旧服务解密
		decrypted, err := oldService.Decrypt(key.KeyValue)
		if err != nil {
			return fmt.Errorf("key ID %d decryption failed: %w", key.ID, err)
		}

		// 2. 使用新服务加密
		encrypted, err := newService.Encrypt(decrypted)
		if err != nil {
			return fmt.Errorf("key ID %d encryption failed: %w", key.ID, err)
		}

		// 3. 使用新服务生成新哈希
		newHash := newService.Hash(decrypted)

		tempRecords = append(tempRecords, TempMigration{
			ID:          key.ID,
			KeyValueNew: encrypted,
			KeyHashNew:  newHash,
		})
	}

	// 在事务中将批次插入临时表
	return cmd.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("temp_migration").Create(&tempRecords).Error; err != nil {
			return fmt.Errorf("failed to insert batch into temp_migration: %w", err)
		}
		return nil
	})
}

// verifyTempColumns 验证临时表数据完整性
func (cmd *MigrateKeysCommand) verifyTempColumns() error {
	logrus.Info("Verifying temporary table data integrity...")

	// 创建用于验证的新加密服务
	var newService encryption.Service
	var err error

	if cmd.toKey != "" {
		newService, err = encryption.NewService(cmd.toKey)
	} else {
		newService, err = encryption.NewService("")
	}

	if err != nil {
		return fmt.Errorf("failed to create verification encryption service: %w", err)
	}

	// 获取总数
	var totalCount int64
	if err := cmd.db.Model(&models.APIKey{}).Count(&totalCount).Error; err != nil {
		return fmt.Errorf("failed to get key count: %w", err)
	}

	if totalCount == 0 {
		return nil
	}

	// 验证临时表已被填充
	var migratedCount int64
	if err := cmd.db.Table("temp_migration").Count(&migratedCount).Error; err != nil {
		return fmt.Errorf("failed to count migrated keys: %w", err)
	}

	if migratedCount != totalCount {
		return fmt.Errorf("migration incomplete: %d/%d keys migrated", migratedCount, totalCount)
	}

	// 验证部分密钥可以正确解密
	verifiedCount := 0
	for {
		var keys []struct {
			ID          uint
			KeyValueNew string `gorm:"column:key_value_new"`
		}

		if err := cmd.db.Table("temp_migration").Select("id, key_value_new").Order("id").Limit(100).Offset(verifiedCount).Scan(&keys).Error; err != nil {
			return fmt.Errorf("failed to get keys for verification: %w", err)
		}

		if len(keys) == 0 {
			break
		}

		for _, key := range keys {
			_, err := newService.Decrypt(key.KeyValueNew)
			if err != nil {
				return fmt.Errorf("key ID %d verification failed: invalid temporary column data: %w", key.ID, err)
			}
		}

		verifiedCount += len(keys)
		if verifiedCount >= int(totalCount) || verifiedCount >= 1000 { // 最多验证 1000 个密钥以提高性能
			break
		}
	}

	logrus.Infof("Verified %d keys successfully", verifiedCount)
	return nil
}

// switchColumns 从临时表原子性更新到原始表
func (cmd *MigrateKeysCommand) switchColumns() error {
	logrus.Info("Updating original table from temporary table...")

	dbType := cmd.db.Dialector.Name()

	return cmd.db.Transaction(func(tx *gorm.DB) error {
		var updateSQL string

		switch dbType {
		case "mysql":
			// MySQL 使用 JOIN 语法进行跨表 UPDATE
			updateSQL = `
				UPDATE api_keys a
				INNER JOIN temp_migration t ON a.id = t.id
				SET a.key_value = t.key_value_new,
				    a.key_hash = t.key_hash_new
			`

		case "postgres":
			// PostgreSQL 使用 FROM 子句进行跨表 UPDATE
			updateSQL = `
				UPDATE api_keys
				SET key_value = t.key_value_new,
				    key_hash = t.key_hash_new
				FROM temp_migration t
				WHERE api_keys.id = t.id
			`

		case "sqlite":
			// SQLite 使用子查询进行跨表 UPDATE（兼容所有版本）
			updateSQL = `
				UPDATE api_keys
				SET key_value = (SELECT key_value_new FROM temp_migration WHERE temp_migration.id = api_keys.id),
				    key_hash = (SELECT key_hash_new FROM temp_migration WHERE temp_migration.id = api_keys.id)
				WHERE EXISTS (SELECT 1 FROM temp_migration WHERE temp_migration.id = api_keys.id)
			`

		default:
			return fmt.Errorf("unsupported database type: %s", dbType)
		}

		logrus.Infof("正在为 %s 执行跨表 UPDATE...", dbType)
		if err := tx.Exec(updateSQL).Error; err != nil {
			return fmt.Errorf("failed to update api_keys from temp_migration: %w", err)
		}

		logrus.Info("Successfully updated original table with migrated data")
		return nil
	})
}

// clearCache 清除缓存
func (cmd *MigrateKeysCommand) clearCache() error {
	logrus.Info("Starting cache cleanup...")

	if cmd.cacheStore == nil {
		logrus.Info("No cache storage configured, skipping cache cleanup")
		return nil
	}

	logrus.Info("Executing cache cleanup...")
	if err := cmd.cacheStore.Clear(); err != nil {
		return fmt.Errorf("cache cleanup failed: %w", err)
	}

	logrus.Info("Cache cleanup successful")
	return nil
}
