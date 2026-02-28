package db

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// V1_4_5_AddOriginalModelColumn 向 request_logs 表添加 original_model 列
func V1_4_5_AddOriginalModelColumn(db *gorm.DB) error {
	logrus.Info("Running v1.4.5 migration: Adding original_model column to request_logs table")

	// Check if column already exists
	if db.Migrator().HasColumn(&requestLogTable{}, "original_model") {
		logrus.Info("original_model column already exists, skipping v1.4.5 migration")
		return nil
	}

	// Add column
	if err := db.Migrator().AddColumn(&requestLogTable{}, "original_model"); err != nil {
		logrus.WithError(err).Error("Failed to add original_model column")
		return err
	}

	logrus.Info("Migration v1.4.5 completed successfully")
	return nil
}
