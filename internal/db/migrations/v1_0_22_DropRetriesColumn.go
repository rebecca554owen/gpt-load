package db

import "gorm.io/gorm"

// RequestLog 迁移用的临时结构
type RequestLog struct {
	Retries int `gorm:"column:retries"`
}

// V1_0_22_DropRetriesColumn 从 request_logs 表删除 retries 列
func V1_0_22_DropRetriesColumn(db *gorm.DB) error {
	// Check if retries column exists
	if db.Migrator().HasColumn(&RequestLog{}, "retries") {
		// Drop retries column
		if err := db.Migrator().DropColumn(&RequestLog{}, "retries"); err != nil {
			return err
		}
	}
	return nil
}
