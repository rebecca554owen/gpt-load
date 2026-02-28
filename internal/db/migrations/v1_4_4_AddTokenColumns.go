package db

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// V1_4_4_AddTokenColumns 向 request_logs 表添加 token 统计列
func V1_4_4_AddTokenColumns(db *gorm.DB) error {
	logrus.Info("Running v1.4.4 migration: Adding token columns to request_logs table")

	// Check if columns already exist
	if db.Migrator().HasColumn(&requestLogTable{}, "prompt_tokens") {
		logrus.Info("Token columns already exist, skipping v1.4.4 migration")
		return nil
	}

	// Add new columns
	if err := db.Migrator().AddColumn(&requestLogTable{}, "prompt_tokens"); err != nil {
		return err
	}
	if err := db.Migrator().AddColumn(&requestLogTable{}, "completion_tokens"); err != nil {
		return err
	}
	if err := db.Migrator().AddColumn(&requestLogTable{}, "total_tokens"); err != nil {
		return err
	}
	if err := db.Migrator().AddColumn(&requestLogTable{}, "cached_tokens"); err != nil {
		return err
	}

	logrus.Info("Migration v1.4.4 completed successfully")
	return nil
}

// requestLogTable 迁移用的最小模型
type requestLogTable struct {
	PromptTokens     int64  `gorm:"default:0"`
	CompletionTokens int64  `gorm:"default:0"`
	TotalTokens      int64  `gorm:"default:0"`
	CachedTokens     int64  `gorm:"default:0"`
	OriginalModel    string `gorm:"type:varchar(255);index"`
}

func (requestLogTable) TableName() string {
	return "request_logs"
}
