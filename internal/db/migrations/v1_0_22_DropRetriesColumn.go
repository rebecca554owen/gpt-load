package db

import "gorm.io/gorm"

// RequestLog temporary structure for migration
type RequestLog struct {
	Retries int `gorm:"column:retries"`
}

// V1_0_22_DropRetriesColumn drops retries column from request_logs table
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
