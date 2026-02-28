package db

import (
	"gorm.io/gorm"
)

// V1_4_7_AddRequestLogCompositeIndex 添加请求日志复合索引
func V1_4_7_AddRequestLogCompositeIndex(db *gorm.DB) error {
	if db.Dialector.Name() == "mysql" {
		// 删除旧索引（如果存在）
		var oldIndexCount int64
		db.Raw(`
			SELECT COUNT(*)
			FROM information_schema.STATISTICS
			WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME = 'request_logs'
			AND INDEX_NAME = 'idx_request_logs_timestamp_request_type'
		`).Count(&oldIndexCount)
		if oldIndexCount > 0 {
			if err := db.Exec("ALTER TABLE request_logs DROP INDEX idx_request_logs_timestamp_request_type").Error; err != nil {
				return err
			}
		}

		// 创建新的覆盖索引
		var newIndexCount int64
		if err := db.Raw(`
			SELECT COUNT(*)
			FROM information_schema.STATISTICS
			WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME = 'request_logs'
			AND INDEX_NAME = 'idx_request_logs_timestamp_type_tokens'
		`).Count(&newIndexCount).Error; err != nil {
			return err
		}

		if newIndexCount == 0 {
			if err := db.Exec("CREATE INDEX idx_request_logs_timestamp_type_tokens ON request_logs(timestamp, request_type, prompt_tokens, completion_tokens, total_tokens, cached_tokens)").Error; err != nil {
				return err
			}
		}
	} else {
		db.Exec("DROP INDEX IF EXISTS idx_request_logs_timestamp_request_type ON request_logs")
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp_type_tokens ON request_logs(timestamp, request_type, prompt_tokens, completion_tokens, total_tokens, cached_tokens)").Error; err != nil {
			return err
		}
	}
	return nil
}
