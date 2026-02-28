package db

import (
	"database/sql"
	"fmt"
	"gpt-load/internal/types"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// NewDB 创建并初始化新的数据库连接
func NewDB(configManager types.ConfigManager) (*gorm.DB, error) {
	dbConfig := configManager.GetDatabaseConfig()
	dsn := dbConfig.DSN
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_DSN is not configured")
	}

	var newLogger logger.Interface
	if configManager.GetLogConfig().Level == "debug" {
		newLogger = logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             GetSlowThreshold(),
				LogLevel:                  logger.Info,
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		)
	}

	var dialector gorm.Dialector
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		dialector = postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		})
	} else if strings.Contains(dsn, "@tcp") {
		if !strings.Contains(dsn, "parseTime") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		dialector = mysql.Open(dsn)
	} else {
		if err := os.MkdirAll(filepath.Dir(dsn), 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
		dialector = sqlite.Open(dsn + "?_busy_timeout=5000")
	}

	var err error
	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger:      newLogger,
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Fix MySQL table charset to utf8mb4 if needed
	if strings.Contains(dsn, "@tcp") {
		if err := fixMysqlCharset(DB); err != nil {
			log.Printf("Warning: failed to fix MySQL charset: %v", err)
		}
	}

	// Configure connection pool from environment variables
	maxIdleConns := getEnvInt("DB_MAX_IDLE_CONNS", 50)
	maxOpenConns := getEnvInt("DB_MAX_OPEN_CONNS", 500)
	connMaxLifetime := time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME", 3600)) * time.Second

	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	log.Printf("Database connection pool configured: MaxIdleConns=%d, MaxOpenConns=%d, ConnMaxLifetime=%v",
		maxIdleConns, maxOpenConns, connMaxLifetime)

	return DB, nil
}

// getEnvInt 从环境变量获取整数值，带默认值
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// GetSlowThreshold 从环境变量返回慢 SQL 阈值
func GetSlowThreshold() time.Duration {
	threshold := getEnvInt("DB_SLOW_THRESHOLD", 1000)
	return time.Duration(threshold) * time.Millisecond
}

// GetSQLDB 返回底层 sql.DB 用于高级操作
func GetSQLDB() (*sql.DB, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return DB.DB()
}

// fixMysqlCharset 确保所有表使用 utf8mb4 字符集
func fixMysqlCharset(db *gorm.DB) error {
	tables := []string{
		"request_logs",
		"api_keys",
		"groups",
		"group_sub_groups",
		"system_settings",
		"group_hourly_stats",
	}

	for _, table := range tables {
		var exists int
		err := db.Raw("SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", table).Scan(&exists).Error
		if err != nil || exists == 0 {
			continue // Table doesn't exist yet, will be created by AutoMigrate
		}

		var charset string
		err = db.Raw(`
			SELECT CCSA.CHARACTER_SET_NAME
			FROM information_schema.TABLES T
			JOIN information_schema.COLLATION_CHARACTER_SET_APPLICABILITY CCSA
				ON T.TABLE_COLLATION = CCSA.COLLATION_NAME
			WHERE T.TABLE_SCHEMA = DATABASE() AND T.TABLE_NAME = ?
		`, table).Scan(&charset).Error

		if err != nil {
			continue
		}

		if charset != "utf8mb4" {
			result := db.Exec(fmt.Sprintf("ALTER TABLE %s CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", table))
			if result.Error != nil {
				log.Printf("Warning: failed to convert table %s to utf8mb4: %v", table, result.Error)
			} else {
				log.Printf("Converted table %s from %s to utf8mb4", table, charset)
			}
		}
	}

	return nil
}
