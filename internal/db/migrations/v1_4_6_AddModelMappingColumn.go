package db

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// V1_4_6_AddModelMappingColumn 向 groups 表添加 model_mappings 列
func V1_4_6_AddModelMappingColumn(db *gorm.DB) error {
	logrus.Info("Running v1.4.6 migration: Adding model_mappings column to groups table")

	// Check if column already exists
	if db.Migrator().HasColumn(&groupTable{}, "model_mappings") {
		logrus.Info("model_mappings column already exists, skipping v1.4.6 migration")
		return nil
	}

	// Add the column
	if err := db.Migrator().AddColumn(&groupTable{}, "model_mappings"); err != nil {
		logrus.WithError(err).Error("Failed to add model_mappings column")
		return err
	}

	logrus.Info("Migration v1.4.6 completed successfully")
	return nil
}

// groupTable 迁移用的最小模型
type groupTable struct {
	ModelMappings string `gorm:"type:json"`
}

func (groupTable) TableName() string {
	return "groups"
}
