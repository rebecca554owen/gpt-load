package services

import (
	"context"
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"math"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
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
	LastDeleteTime       time.Time     `json:"last_delete_time"`
	DeletedCountTotal    int64         `json:"deleted_count_total"`
	LastDeleteCount      int           `json:"last_delete_count"`
	CurrentInterval      time.Duration `json:"current_interval"`
	DeleteLatency        time.Duration `json:"delete_latency"`
	DataAgeHours         int           `json:"data_age_hours"`
	AdaptiveAdjustments  int           `json:"adaptive_adjustments"`
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
			MinInterval:      1 * time.Minute,
			MaxInterval:      30 * time.Minute,
			DeleteWindow:     5 * time.Minute,
			MaxBatchSize:     1000,
			MinBatchSize:     100,
			MaxOperationTime: 2 * time.Second,
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
func (s *LogCleanupService) calculateInterval() time.Duration {
	settings := s.settingsManager.GetSettings()
	retentionDays := settings.RequestLogRetentionDays

	if retentionDays <= 0 {
		return s.config.MaxInterval
	}

	// Base interval calculation: longer retention days, shorter interval
	baseInterval := time.Duration(math.Max(1.0, float64(s.config.MaxInterval.Nanoseconds())/float64(retentionDays))) * time.Nanosecond

	// Limit between min and max interval
	interval := baseInterval
	if interval < s.config.MinInterval {
		interval = s.config.MinInterval
	} else if interval > s.config.MaxInterval {
		interval = s.config.MaxInterval
	}

	// If adaptive adjustment enabled
	if s.config.EnableAdaptive {
		// Check for data accumulation
		dataAge := s.getOldestLogAge()
		if dataAge > retentionDays*24+6 { // If exceeds retention period by 6 hours
			interval = interval / 2 // Increase frequency
			s.metrics.AdaptiveAdjustments++
			logrus.WithField("data_age_hours", dataAge).Info("Increasing delete frequency due to data accumulation")
		}
	}

	logrus.WithFields(logrus.Fields{
		"retention_days":     retentionDays,
		"interval_seconds":   interval.Seconds(),
		"adaptive_enabled":   s.config.EnableAdaptive,
		"adjustments_count":  s.metrics.AdaptiveAdjustments,
	}).Debug("Calculated cleanup interval")

	return interval
}

// getOldestLogAge 获取最旧日志的年龄（以小时为单位）
func (s *LogCleanupService) getOldestLogAge() int {
	var count int64
	err := s.db.Model(&models.RequestLog{}).
		Count(&count).Error

	if err != nil || count == 0 {
		return 0
	}

	var oldestTime time.Time
	err = s.db.Model(&models.RequestLog{}).
		Order("timestamp ASC").
		Limit(1).
		Select("timestamp").
		Scan(&oldestTime).Error

	if err != nil || oldestTime.IsZero() {
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
		if s.config.EnableAdaptive {
			// Adjust batch size based on last operation time
			if s.metrics.DeleteLatency > s.config.MaxOperationTime {
				batchSize = s.config.MinBatchSize
			} else if s.metrics.DeleteLatency < 200*time.Millisecond {
				batchSize = int(float64(batchSize) * 1.2)
				if batchSize > s.config.MaxBatchSize {
					batchSize = s.config.MaxBatchSize
				}
			}
		}

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

		// If batch operation and not last batch, take a short break
		if deletedCount == int64(batchSize) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Update metrics
	s.metrics.LastDeleteTime = time.Now()
	s.metrics.DeletedCountTotal += totalDeleted
	s.metrics.LastDeleteCount = int(totalDeleted)
	s.metrics.DeleteLatency = time.Since(start)
	s.metrics.DataAgeHours = s.getOldestLogAge()

	if totalDeleted > 0 {
		logrus.WithFields(logrus.Fields{
			"total_deleted":     totalDeleted,
			"batch_count":       batchCount,
			"operation_time":    s.metrics.DeleteLatency.Milliseconds(),
			"cutoff_time":       cutoffTime.Format(time.RFC3339),
			"retention_days":    retentionDays,
			"current_interval":  s.metrics.CurrentInterval.Seconds(),
		}).Info("Successfully cleaned up expired request logs")
	} else {
		logrus.Debug("No expired request logs found to cleanup")
	}
}

// GetMetrics 获取清理指标
func (s *LogCleanupService) GetMetrics() CleanupMetrics {
	s.metrics.DataAgeHours = s.getOldestLogAge()
	return s.metrics
}
