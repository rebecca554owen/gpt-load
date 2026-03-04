package services

import (
	"context"
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	// Default configuration values
	DefaultMinInterval  = 5 * time.Minute  // Minimum cleanup interval
	DefaultMaxInterval  = 30 * time.Minute // Maximum cleanup interval
	DefaultBaseInterval = 15 * time.Minute // Base cleanup interval
	DefaultDeleteWindow = 5 * time.Minute
	DefaultMaxBatchSize = 1000
	DefaultMinBatchSize = 100
	DefaultMaxOperationTime = 2 * time.Second

	// Adaptive adjustment constants
	EmptyDeleteThreshold = 3 // Consecutive empty deletes before extending interval
)

// CleanupConfig 日志清理配置
type CleanupConfig struct {
	RetentionDays      int           `json:"retention_days"`       // Retention days
	BaseInterval       time.Duration `json:"base_interval"`        // Base interval
	MinInterval        time.Duration `json:"min_interval"`         // Minimum interval (1 minute)
	MaxInterval        time.Duration `json:"max_interval"`         // Maximum interval (30 minutes)
	DeleteWindow       time.Duration `json:"delete_window"`        // Deletion time window
	MaxBatchSize       int           `json:"max_batch_size"`       // Maximum batch size
	MinBatchSize       int           `json:"min_batch_size"`       // Minimum batch size
	MaxOperationTime   time.Duration `json:"max_operation_time"`   // Single operation max duration
	EnableAdaptive     bool          `json:"enable_adaptive"`      // Enable adaptive adjustment
}

// CleanupMetrics 清理指标
type CleanupMetrics struct {
	LastDeleteTime         time.Time     `json:"last_delete_time"`
	DeletedCountTotal      int64         `json:"deleted_count_total"`
	LastDeleteCount        int           `json:"last_delete_count"`
	CurrentInterval        time.Duration `json:"current_interval"`
	DeleteLatency          time.Duration `json:"delete_latency"`
	DataAgeHours           int           `json:"data_age_hours"`
	ConsecutiveEmptyDeletes int          `json:"consecutive_empty_deletes"` // 连续空删除次数
}

// LogCleanupService 负责清理过期的请求日志
type LogCleanupService struct {
	db              *gorm.DB
	settingsManager *config.SystemSettingsManager
	config          CleanupConfig
	metrics         CleanupMetrics
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// NewLogCleanupService 创建新的日志清理服务
func NewLogCleanupService(db *gorm.DB, settingsManager *config.SystemSettingsManager) *LogCleanupService {
	return &LogCleanupService{
		db:              db,
		settingsManager: settingsManager,
		config: CleanupConfig{
			MinInterval:      DefaultMinInterval,
			MaxInterval:      DefaultMaxInterval,
			DeleteWindow:     DefaultDeleteWindow,
			MaxBatchSize:     DefaultMaxBatchSize,
			MinBatchSize:     DefaultMinBatchSize,
			MaxOperationTime: DefaultMaxOperationTime,
			EnableAdaptive:   true,
		},
		stopCh: make(chan struct{}),
	}
}

// Start 启动日志清理服务
func (s *LogCleanupService) Start() {
	s.wg.Add(1)
	go s.run()
	logrus.Debug("Log cleanup service started with dynamic interval")
}

// Stop 停止日志清理服务
func (s *LogCleanupService) Stop(ctx context.Context) {
	close(s.stopCh)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logrus.Info("LogCleanupService stopped gracefully.")
	case <-ctx.Done():
		logrus.Warn("LogCleanupService stop timed out.")
	}
}

// run 运行日志清理主循环
func (s *LogCleanupService) run() {
	defer s.wg.Done()

	// Execute cleanup once at startup
	s.cleanupExpiredLogs()

	for {
		interval := s.calculateInterval()
		s.metrics.CurrentInterval = interval

		ticker := time.NewTicker(interval)

		select {
		case <-ticker.C:
			s.cleanupExpiredLogs()
			ticker.Stop()
		case <-s.stopCh:
			ticker.Stop()
			return
		}
	}
}

// calculateInterval 计算动态删除间隔
// 基于实际删除结果自适应调整：
// - 基础间隔15分钟
// - 连续3次删除0条 → 延长到30分钟
// - 删除满批次 → 缩短到5分钟
func (s *LogCleanupService) calculateInterval() time.Duration {
	settings := s.settingsManager.GetSettings()
	retentionDays := settings.RequestLogRetentionDays

	if retentionDays <= 0 {
		return s.config.MaxInterval
	}

	interval := DefaultBaseInterval

	// 根据删除结果动态调整
	if s.config.EnableAdaptive {
		lastCount := s.metrics.LastDeleteCount
		consecutiveEmpty := s.metrics.ConsecutiveEmptyDeletes

		switch {
		// 连续空删除，延长间隔
		case consecutiveEmpty >= EmptyDeleteThreshold:
			interval = s.config.MaxInterval
			logrus.WithField("consecutive_empty", consecutiveEmpty).
				Debug("Extending cleanup interval due to consecutive empty deletes")

		// 删除满批次，缩短间隔
		case lastCount >= s.config.MaxBatchSize:
			interval = s.config.MinInterval
			logrus.WithField("deleted_count", lastCount).
				Debug("Shortening cleanup interval due to full batch deletion")

		// 删除少量数据，保持基础间隔
		case lastCount > 0:
			interval = DefaultBaseInterval
		}
	}

	logrus.WithFields(logrus.Fields{
		"interval_minutes":       interval.Minutes(),
		"last_delete_count":      s.metrics.LastDeleteCount,
		"consecutive_empty":      s.metrics.ConsecutiveEmptyDeletes,
	}).Debug("Calculated cleanup interval")

	return interval
}

// getOldestLogAge 获取最旧日志的年龄（以小时为单位）
func (s *LogCleanupService) getOldestLogAge() int {
	var oldestTime time.Time
	err := s.db.Model(&models.RequestLog{}).
		Order("timestamp ASC").
		Limit(1).
		Select("timestamp").
		Scan(&oldestTime).Error

	if err != nil {
		logrus.WithError(err).Debug("Failed to get oldest log age")
		return 0
	}

	if oldestTime.IsZero() {
		return 0
	}

	return int(time.Since(oldestTime).Hours())
}

// cleanupExpiredLogs 清理过期的请求日志（批量删除）
func (s *LogCleanupService) cleanupExpiredLogs() {
	start := time.Now()

	// Get log retention days configuration
	settings := s.settingsManager.GetSettings()
	retentionDays := settings.RequestLogRetentionDays

	if retentionDays <= 0 {
		logrus.Debug("Log retention is disabled (retention_days <= 0)")
		return
	}

	// Calculate expiration time point and deletion window
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays).UTC()

	// Batch deletion
	totalDeleted := int64(0)
	batchCount := 0

	for {
		batchSize := s.config.MaxBatchSize

		// Execute batch deletion
		result := s.db.Where("timestamp < ?", cutoffTime).
			Limit(batchSize).
			Delete(&models.RequestLog{})

		if result.Error != nil {
			logrus.WithError(result.Error).Error("Failed to cleanup expired request logs batch")
			return
		}

		deletedCount := result.RowsAffected
		if deletedCount == 0 {
			break // No more data to delete
		}

		totalDeleted += deletedCount
		batchCount++

		logrus.WithFields(logrus.Fields{
			"batch_number":   batchCount,
			"deleted_count":  deletedCount,
			"batch_size":     batchSize,
			"cutoff_time":    cutoffTime.Format(time.RFC3339),
		}).Debug("Deleted batch of expired request logs")
	}

	// Update metrics
	s.metrics.LastDeleteTime = time.Now()
	s.metrics.DeletedCountTotal += totalDeleted
	s.metrics.LastDeleteCount = int(totalDeleted)
	s.metrics.DeleteLatency = time.Since(start)

	// 更新连续空删除计数
	if totalDeleted == 0 {
		s.metrics.ConsecutiveEmptyDeletes++
	} else {
		s.metrics.ConsecutiveEmptyDeletes = 0
	}

	if totalDeleted > 0 {
		s.metrics.DataAgeHours = s.getOldestLogAge()
		logrus.WithFields(logrus.Fields{
			"total_deleted":     totalDeleted,
			"batch_count":       batchCount,
			"operation_time":    s.metrics.DeleteLatency.Milliseconds(),
			"cutoff_time":       cutoffTime.Format(time.RFC3339),
			"retention_days":    retentionDays,
			"current_interval":  s.metrics.CurrentInterval.Seconds(),
		}).Info("Successfully cleaned up expired request logs")
	} else {
		logrus.WithFields(logrus.Fields{
			"consecutive_empty": s.metrics.ConsecutiveEmptyDeletes,
		}).Debug("No expired request logs found to cleanup")
	}
}

// GetMetrics 获取清理指标
func (s *LogCleanupService) GetMetrics() CleanupMetrics {
	return s.metrics
}
