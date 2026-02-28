package db

import (
	"gorm.io/gorm"
)

// MigrateDatabase 运行所有数据库迁移
func MigrateDatabase(db *gorm.DB) error {
	// 运行 v1.0.22 迁移
	if err := V1_0_22_DropRetriesColumn(db); err != nil {
		return err
	}

	// 运行 v1.1.0 迁移
	if err := V1_1_0_AddKeyHashColumn(db); err != nil {
		return err
	}

	// 运行 v1.4.4 迁移
	if err := V1_4_4_AddTokenColumns(db); err != nil {
		return err
	}

	// 运行 v1.4.6 迁移
	if err := V1_4_6_AddModelMappingColumn(db); err != nil {
		return err
	}

	// 运行 v1.4.5 迁移
	if err := V1_4_5_AddOriginalModelColumn(db); err != nil {
		return err
	}

	// 运行 v1.4.7 迁移
	if err := V1_4_7_AddRequestLogCompositeIndex(db); err != nil {
		return err
	}

	return nil
}

// HandleLegacyIndexes 移除旧版本的索引以防止迁移错误
func HandleLegacyIndexes(db *gorm.DB) {
	if db.Dialector.Name() == "mysql" {
		var indexCount int64
		db.Raw(`
				SELECT COUNT(*)
				FROM information_schema.STATISTICS
				WHERE TABLE_SCHEMA = DATABASE()
				AND TABLE_NAME = 'api_keys'
				AND INDEX_NAME = 'idx_group_key'
			`).Count(&indexCount)

		if indexCount > 0 {
			db.Exec("ALTER TABLE api_keys DROP INDEX idx_group_key")
		}
		var idxApiKeysGroupKeyCount int64
		db.Raw(`
				SELECT COUNT(*)
				FROM information_schema.STATISTICS
				WHERE TABLE_SCHEMA = DATABASE()
				AND TABLE_NAME = 'api_keys'
				AND INDEX_NAME = 'idx_api_keys_group_id_key_value'
			`).Count(&idxApiKeysGroupKeyCount)

		if idxApiKeysGroupKeyCount > 0 {
			db.Exec("ALTER TABLE api_keys DROP INDEX idx_api_keys_group_id_key_value")
		}
	} else {
		db.Exec("DROP INDEX IF EXISTS idx_group_key")
		db.Exec("DROP INDEX IF EXISTS idx_api_keys_group_id_key_value")
	}
}
