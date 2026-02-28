// Package config 提供应用配置管理
package config

import (
	"fmt"
	"os"
	"strings"

	"gpt-load/internal/errors"
	"gpt-load/internal/types"
	"gpt-load/internal/utils"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// Constants 表示配置常量
type Constants struct {
	MinPort               int
	MaxPort               int
	MinTimeout            int
	DefaultTimeout        int
	DefaultMaxSockets     int
	DefaultMaxFreeSockets int
}

// DefaultConstants 保存默认配置值
var DefaultConstants = Constants{
	MinPort:               1,
	MaxPort:               65535,
	MinTimeout:            1,
	DefaultTimeout:        30,
	DefaultMaxSockets:     50,
	DefaultMaxFreeSockets: 10,
}

// Manager 实现 ConfigManager 接口
type Manager struct {
	config          *Config
	settingsManager *SystemSettingsManager
}

// Config 表示应用配置
type Config struct {
	Server        types.ServerConfig
	Auth          types.AuthConfig
	CORS          types.CORSConfig
	Performance   types.PerformanceConfig
	Log           types.LogConfig
	Database      types.DatabaseConfig
	RedisDSN      string
	EncryptionKey string
}

// NewManager 创建新的配置管理器
func NewManager(settingsManager *SystemSettingsManager) (types.ConfigManager, error) {
	manager := &Manager{
		settingsManager: settingsManager,
	}
	if err := manager.ReloadConfig(); err != nil {
		return nil, err
	}
	return manager, nil
}

// ReloadConfig 从环境变量重新加载配置
func (m *Manager) ReloadConfig() error {
	if err := godotenv.Load(); err != nil {
		logrus.Info("Info: Create .env file to support environment variable configuration")
	}

	config := &Config{
		Server: types.ServerConfig{
			IsMaster:                !utils.ParseBoolean(os.Getenv("IS_SLAVE"), false),
			Port:                    utils.ParseInteger(os.Getenv("PORT"), 3001),
			Host:                    utils.GetEnvOrDefault("HOST", "0.0.0.0"),
			ReadTimeout:             utils.ParseInteger(os.Getenv("SERVER_READ_TIMEOUT"), 60),
			WriteTimeout:            utils.ParseInteger(os.Getenv("SERVER_WRITE_TIMEOUT"), 600),
			IdleTimeout:             utils.ParseInteger(os.Getenv("SERVER_IDLE_TIMEOUT"), 120),
			GracefulShutdownTimeout: utils.ParseInteger(os.Getenv("SERVER_GRACEFUL_SHUTDOWN_TIMEOUT"), 10),
		},
		Auth: types.AuthConfig{
			Key: os.Getenv("AUTH_KEY"),
		},
		CORS: types.CORSConfig{
			Enabled:          utils.ParseBoolean(os.Getenv("ENABLE_CORS"), false),
			AllowedOrigins:   utils.ParseArray(os.Getenv("ALLOWED_ORIGINS"), []string{}),
			AllowedMethods:   utils.ParseArray(os.Getenv("ALLOWED_METHODS"), []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
			AllowedHeaders:   utils.ParseArray(os.Getenv("ALLOWED_HEADERS"), []string{"*"}),
			AllowCredentials: utils.ParseBoolean(os.Getenv("ALLOW_CREDENTIALS"), false),
		},
		Performance: types.PerformanceConfig{
			MaxConcurrentRequests: utils.ParseInteger(os.Getenv("MAX_CONCURRENT_REQUESTS"), 100),
		},
		Log: types.LogConfig{
			Level:      utils.GetEnvOrDefault("LOG_LEVEL", "info"),
			Format:     utils.GetEnvOrDefault("LOG_FORMAT", "text"),
			EnableFile: utils.ParseBoolean(os.Getenv("LOG_ENABLE_FILE"), false),
			FilePath:   utils.GetEnvOrDefault("LOG_FILE_PATH", "./data/logs/app.log"),
		},
		Database: types.DatabaseConfig{
			DSN: utils.GetEnvOrDefault("DATABASE_DSN", "./data/gpt-load.db"),
		},
		RedisDSN:      os.Getenv("REDIS_DSN"),
		EncryptionKey: os.Getenv("ENCRYPTION_KEY"),
	}
	m.config = config

	// Validate configuration
	if err := m.Validate(); err != nil {
		return err
	}

	return nil
}

// IsMaster 返回服务器模式
func (m *Manager) IsMaster() bool {
	return m.config.Server.IsMaster
}

// GetAuthConfig 返回认证配置
func (m *Manager) GetAuthConfig() types.AuthConfig {
	return m.config.Auth
}

// GetCORSConfig 返回 CORS 配置
func (m *Manager) GetCORSConfig() types.CORSConfig {
	return m.config.CORS
}

// GetPerformanceConfig 返回性能配置
func (m *Manager) GetPerformanceConfig() types.PerformanceConfig {
	return m.config.Performance
}

// GetLogConfig 返回日志配置
func (m *Manager) GetLogConfig() types.LogConfig {
	return m.config.Log
}

// GetRedisDSN 返回 Redis DSN 字符串
func (m *Manager) GetRedisDSN() string {
	return m.config.RedisDSN
}

// GetDatabaseConfig 返回数据库配置
func (m *Manager) GetDatabaseConfig() types.DatabaseConfig {
	return m.config.Database
}

// GetEncryptionKey 返回加密密钥
func (m *Manager) GetEncryptionKey() string {
	return m.config.EncryptionKey
}

// GetEffectiveServerConfig 返回与系统设置合并后的服务器配置
func (m *Manager) GetEffectiveServerConfig() types.ServerConfig {
	return m.config.Server
}

// Validate 验证配置
func (m *Manager) Validate() error {
	var validationErrors []string

	// Validate port
	if m.config.Server.Port < DefaultConstants.MinPort || m.config.Server.Port > DefaultConstants.MaxPort {
		validationErrors = append(validationErrors, fmt.Sprintf("port must be between %d-%d", DefaultConstants.MinPort, DefaultConstants.MaxPort))
	}

	if m.config.Performance.MaxConcurrentRequests < 1 {
		validationErrors = append(validationErrors, "max concurrent requests cannot be less than 1")
	}

	// Validate auth key
	if m.config.Auth.Key == "" {
		validationErrors = append(validationErrors, "AUTH_KEY is required and cannot be empty")
	} else {
		utils.ValidatePasswordStrength(m.config.Auth.Key, "AUTH_KEY")
	}

	// Validate GracefulShutdownTimeout and reset if necessary
	if m.config.Server.GracefulShutdownTimeout < 10 {
		logrus.Warnf("SERVER_GRACEFUL_SHUTDOWN_TIMEOUT value %ds is too short, resetting to minimum 10s.", m.config.Server.GracefulShutdownTimeout)
		m.config.Server.GracefulShutdownTimeout = 10
	}

	if m.config.CORS.Enabled {
		if len(m.config.CORS.AllowedOrigins) == 0 {
			validationErrors = append(validationErrors, "CORS is enabled but ALLOWED_ORIGINS is not set. UI will not work from a browser.")
		} else if len(m.config.CORS.AllowedOrigins) == 1 && m.config.CORS.AllowedOrigins[0] == "*" {
			logrus.Warn("CORS is configured with ALLOWED_ORIGINS=*. This is insecure and should only be used for development.")
		}
	}

	if len(validationErrors) > 0 {
		logrus.Error("Configuration validation failed:")
		for _, err := range validationErrors {
			logrus.Errorf("   - %s", err)
		}
		return errors.NewAPIError(errors.ErrValidation, strings.Join(validationErrors, "; "))
	}

	return nil
}

// DisplayServerConfig 显示当前服务器相关配置信息
func (m *Manager) DisplayServerConfig() {
	serverConfig := m.GetEffectiveServerConfig()
	corsConfig := m.GetCORSConfig()
	perfConfig := m.GetPerformanceConfig()
	logConfig := m.GetLogConfig()
	dbConfig := m.GetDatabaseConfig()
	redisDSN := m.GetRedisDSN()
	encryptionKey := m.GetEncryptionKey()

	logrus.Info("")
	logrus.Info("======= Server Configuration =======")
	logrus.Info("  --- Server ---")
	logrus.Infof("    Listen Address: %s:%d", serverConfig.Host, serverConfig.Port)
	logrus.Infof("    Graceful Shutdown Timeout: %d seconds", serverConfig.GracefulShutdownTimeout)
	logrus.Infof("    Read Timeout: %d seconds", serverConfig.ReadTimeout)
	logrus.Infof("    Write Timeout: %d seconds", serverConfig.WriteTimeout)
	logrus.Infof("    Idle Timeout: %d seconds", serverConfig.IdleTimeout)

	logrus.Info("  --- Performance ---")
	logrus.Infof("    Max Concurrent Requests: %d", perfConfig.MaxConcurrentRequests)

	logrus.Info("  --- Security ---")
	logrus.Infof("    Authentication: enabled (key loaded)")
	if encryptionKey != "" {
		logrus.Info("    Encryption: enabled")
	} else {
		logrus.Warn("    Encryption: disabled - WARNING: Sensitive data may be stored unencrypted, which poses security risks including potential key exposure")
	}
	corsStatus := "disabled"
	if corsConfig.Enabled {
		corsStatus = fmt.Sprintf("enabled (Origins: %s)", strings.Join(corsConfig.AllowedOrigins, ", "))
	}
	logrus.Infof("    CORS: %s", corsStatus)

	logrus.Info("  --- Logging ---")
	logrus.Infof("    Log Level: %s", logConfig.Level)
	logrus.Infof("    Log Format: %s", logConfig.Format)
	logrus.Infof("    File Logging: %t", logConfig.EnableFile)
	if logConfig.EnableFile {
		logrus.Infof("    Log File Path: %s", logConfig.FilePath)
	}

	logrus.Info("  --- Dependencies ---")
	if dbConfig.DSN != "" {
		logrus.Info("    Database: configured")
	} else {
		logrus.Info("    Database: not configured")
	}
	if redisDSN != "" {
		logrus.Info("    Redis: configured")
	} else {
		logrus.Info("    Redis: not configured")
	}
	logrus.Info("====================================")
	logrus.Info("")
}
