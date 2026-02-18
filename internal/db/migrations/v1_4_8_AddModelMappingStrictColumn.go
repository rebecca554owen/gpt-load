package db

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// V1_4_8_AddModelMappingStrictColumn adds model_mapping_strict column to groups table
func V1_4_8_AddModelMappingStrictColumn(db *gorm.DB) error {
	logrus.Info("Running v1.4.8 migration: Adding model_mapping_strict column to groups table")

	// Check if column already exists
	if db.Migrator().HasColumn(&groupTableStrict{}, "model_mapping_strict") {
		logrus.Info("model_mapping_strict column already exists, skipping v1.4.8 migration")
		return nil
	}

	// Add the column
	if err := db.Migrator().AddColumn(&groupTableStrict{}, "model_mapping_strict"); err != nil {
		logrus.WithError(err).Error("Failed to add model_mapping_strict column")
		return err
	}

	logrus.Info("Migration v1.4.8 completed successfully")
	return nil
}

// groupTableStrict is a minimal model for migration purposes
type groupTableStrict struct {
	ModelMappingStrict bool `gorm:"default:false"`
}

func (groupTableStrict) TableName() string {
	return "groups"
}
