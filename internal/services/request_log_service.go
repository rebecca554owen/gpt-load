package services

import (
	"context"
	"encoding/json"
	"fmt"
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"gpt-load/internal/store"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RequestLogCachePrefix    = "request_log:"
	PendingLogKeysSet        = "pending_log_keys"
	DefaultLogFlushBatchSize = 200
)

// HourlyStat represents aggregated hourly statistics in memory buffer
type HourlyStat struct {
	Time         time.Time
	GroupID      uint
	SuccessCount int64
	FailureCount int64
}

// RequestLogService is responsible for managing request logs.
type RequestLogService struct {
	db              *gorm.DB
	store           store.Store
	settingsManager *config.SystemSettingsManager
	stopChan        chan struct{}
	wg              sync.WaitGroup
	ticker          *time.Ticker
	statsBuffer     map[string]*HourlyStat
	statsBufferMu   sync.Mutex
	statsFlushChan  chan struct{}
}

// NewRequestLogService creates a new RequestLogService instance
func NewRequestLogService(db *gorm.DB, store store.Store, sm *config.SystemSettingsManager) *RequestLogService {
	return &RequestLogService{
		db:              db,
		store:           store,
		settingsManager: sm,
		stopChan:        make(chan struct{}),
		statsBuffer:     make(map[string]*HourlyStat),
		statsFlushChan:  make(chan struct{}, 1),
	}
}

// Start initializes the service and starts the periodic flush routine
func (s *RequestLogService) Start() {
	s.wg.Add(2)
	go s.runLoop()
	go s.runStatsFlusher()
}

func (s *RequestLogService) runLoop() {
	defer s.wg.Done()

	// Initial flush on start
	s.flush()

	interval := time.Duration(s.settingsManager.GetSettings().RequestLogWriteIntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = time.Minute
	}
	s.ticker = time.NewTicker(interval)
	defer s.ticker.Stop()

	for {
		select {
		case <-s.ticker.C:
			newInterval := time.Duration(s.settingsManager.GetSettings().RequestLogWriteIntervalMinutes) * time.Minute
			if newInterval <= 0 {
				newInterval = time.Minute
			}
			if newInterval != interval {
				s.ticker.Reset(newInterval)
				interval = newInterval
				logrus.Debugf("Request log write interval updated to: %v", interval)
			}
			s.flush()
		case <-s.stopChan:
			return
		}
	}
}

// Stop gracefully stops the RequestLogService
func (s *RequestLogService) Stop(ctx context.Context) {
	close(s.stopChan)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.flush()
		s.FlushStats()
		logrus.Info("RequestLogService stopped gracefully.")
	case <-ctx.Done():
		logrus.Warn("RequestLogService stop timed out.")
	}
}

// Record logs a request to the database and cache
func (s *RequestLogService) Record(log *models.RequestLog) error {
	log.ID = uuid.NewString()
	log.Timestamp = time.Now()

	if s.settingsManager.GetSettings().RequestLogWriteIntervalMinutes == 0 {
		go func() {
			if err := s.writeLogsToDB([]*models.RequestLog{log}); err != nil {
				logrus.Errorf("Failed to write request log in sync mode: %v", err)
			}
		}()
		return nil
	}

	cacheKey := RequestLogCachePrefix + log.ID

	logBytes, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal request log: %w", err)
	}

	ttl := time.Duration(s.settingsManager.GetSettings().RequestLogWriteIntervalMinutes*5) * time.Minute
	if err := s.store.Set(cacheKey, logBytes, ttl); err != nil {
		return err
	}

	return s.store.SAdd(PendingLogKeysSet, cacheKey)
}

// flush data from cache to database
func (s *RequestLogService) flush() {
	if s.settingsManager.GetSettings().RequestLogWriteIntervalMinutes == 0 {
		logrus.Debug("Sync mode enabled, skipping scheduled log flush.")
		return
	}

	logrus.Debug("Master starting to flush request logs...")

	for {
		keys, err := s.store.SPopN(PendingLogKeysSet, DefaultLogFlushBatchSize)
		if err != nil {
			logrus.Errorf("Failed to pop pending log keys from store: %v", err)
			return
		}

		if len(keys) == 0 {
			return
		}

		logrus.Debugf("Popped %d request logs to flush.", len(keys))

		var logs []*models.RequestLog
		var processedKeys []string
		for _, key := range keys {
			logBytes, err := s.store.Get(key)
			if err != nil {
				if err == store.ErrNotFound {
					logrus.Warnf("Log key %s found in set but not in store, skipping.", key)
				} else {
					logrus.Warnf("Failed to get log for key %s: %v", key, err)
				}
				continue
			}
			var log models.RequestLog
			if err := json.Unmarshal(logBytes, &log); err != nil {
				logrus.Warnf("Failed to unmarshal log for key %s: %v", key, err)
				continue
			}
			logs = append(logs, &log)
			processedKeys = append(processedKeys, key)
		}

		if len(logs) == 0 {
			continue
		}

		err = s.writeLogsToDB(logs)

		if err != nil {
			logrus.Errorf("Failed to flush request logs batch, will retry next time. Error: %v", err)
			if len(keys) > 0 {
				keysToRetry := make([]any, len(keys))
				for i, k := range keys {
					keysToRetry[i] = k
				}
				if saddErr := s.store.SAdd(PendingLogKeysSet, keysToRetry...); saddErr != nil {
					logrus.Errorf("CRITICAL: Failed to re-add failed log keys to set: %v", saddErr)
				}
			}
			return
		}

		if len(processedKeys) > 0 {
			if err := s.store.Del(processedKeys...); err != nil {
				logrus.Errorf("Failed to delete flushed log bodies from store: %v", err)
			}
		}
		logrus.Infof("Successfully flushed %d request logs.", len(logs))
	}
}

// writeLogsToDB writes a batch of request logs to the database
func (s *RequestLogService) writeLogsToDB(logs []*models.RequestLog) error {
	if len(logs) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.CreateInBatches(logs, len(logs)).Error; err != nil {
			return fmt.Errorf("failed to batch insert request logs: %w", err)
		}

		type keyUsageStat struct {
			Count      int64
			LastUsedAt time.Time
		}

		groupedKeyStats := make(map[uint]map[string]keyUsageStat)
		for _, log := range logs {
			if !log.IsSuccess || log.KeyHash == "" {
				continue
			}

			if _, exists := groupedKeyStats[log.GroupID]; !exists {
				groupedKeyStats[log.GroupID] = make(map[string]keyUsageStat)
			}

			stats := groupedKeyStats[log.GroupID][log.KeyHash]
			stats.Count++
			if stats.LastUsedAt.IsZero() || log.Timestamp.After(stats.LastUsedAt) {
				stats.LastUsedAt = log.Timestamp
			}
			groupedKeyStats[log.GroupID][log.KeyHash] = stats
		}

		if len(groupedKeyStats) > 0 {
			for groupID, keyStats := range groupedKeyStats {
				var requestCountCase strings.Builder
				requestCountCase.WriteString("CASE key_hash ")

				var lastUsedAtCase strings.Builder
				lastUsedAtCase.WriteString("CASE key_hash ")

				requestCountArgs := make([]any, 0, len(keyStats)*2)
				lastUsedAtArgs := make([]any, 0, len(keyStats)*2)
				keyHashes := make([]string, 0, len(keyStats))

				for keyHash, stats := range keyStats {
					requestCountCase.WriteString("WHEN ? THEN request_count + ? ")
					requestCountArgs = append(requestCountArgs, keyHash, stats.Count)

					lastUsedAtCase.WriteString("WHEN ? THEN ? ")
					lastUsedAtArgs = append(lastUsedAtArgs, keyHash, stats.LastUsedAt)

					keyHashes = append(keyHashes, keyHash)
				}

				requestCountCase.WriteString("ELSE request_count END")
				lastUsedAtCase.WriteString("ELSE last_used_at END")

				if err := tx.Model(&models.APIKey{}).
					Where("group_id = ? AND key_hash IN ?", groupID, keyHashes).
					Updates(map[string]any{
						"request_count": gorm.Expr(requestCountCase.String(), requestCountArgs...),
						"last_used_at":  gorm.Expr(lastUsedAtCase.String(), lastUsedAtArgs...),
					}).Error; err != nil {
					return fmt.Errorf("failed to batch update api_key stats: %w", err)
				}
			}
		}

		s.updateStatsBuffer(logs)

		return nil
	})
}

func isDeadlockError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Error 1213") ||
		strings.Contains(err.Error(), "Deadlock found")
}

// runStatsFlusher runs periodic stats flusher in background
func (s *RequestLogService) runStatsFlusher() {
	defer s.wg.Done()

	flushInterval := time.Minute
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	logrus.Info("Stats flusher started with interval: ", flushInterval)

	for {
		select {
		case <-ticker.C:
			s.FlushStats()
		case <-s.statsFlushChan:
			s.FlushStats()
		case <-s.stopChan:
			return
		}
	}
}

// updateStatsBuffer updates the in-memory stats buffer from logs
func (s *RequestLogService) updateStatsBuffer(logs []*models.RequestLog) {
	s.statsBufferMu.Lock()
	defer s.statsBufferMu.Unlock()

	for _, log := range logs {
		if log.RequestType == models.RequestTypeRetry {
			continue
		}

		hourlyTime := log.Timestamp.Truncate(time.Hour)
		key := fmt.Sprintf("%d-%d", hourlyTime.Unix(), log.GroupID)

		if _, exists := s.statsBuffer[key]; !exists {
			s.statsBuffer[key] = &HourlyStat{
				Time:    hourlyTime,
				GroupID: log.GroupID,
			}
		}

		if log.IsSuccess {
			s.statsBuffer[key].SuccessCount++
		} else {
			s.statsBuffer[key].FailureCount++
		}

		if log.ParentGroupID > 0 {
			parentKey := fmt.Sprintf("%d-%d", hourlyTime.Unix(), log.ParentGroupID)
			if _, exists := s.statsBuffer[parentKey]; !exists {
				s.statsBuffer[parentKey] = &HourlyStat{
					Time:    hourlyTime,
					GroupID: log.ParentGroupID,
				}
			}
			if log.IsSuccess {
				s.statsBuffer[parentKey].SuccessCount++
			} else {
				s.statsBuffer[parentKey].FailureCount++
			}
		}
	}
}

// FlushStats flushes buffered stats to database (single-threaded to avoid deadlock)
func (s *RequestLogService) FlushStats() {
	s.statsBufferMu.Lock()
	if len(s.statsBuffer) == 0 {
		s.statsBufferMu.Unlock()
		return
	}

	// Deep copy HourlyStat to avoid race condition with updateStatsBuffer
	statsToFlush := make([]*HourlyStat, 0, len(s.statsBuffer))
	for _, stat := range s.statsBuffer {
		statCopy := &HourlyStat{
			Time:         stat.Time,
			GroupID:      stat.GroupID,
			SuccessCount: stat.SuccessCount,
			FailureCount: stat.FailureCount,
		}
		statsToFlush = append(statsToFlush, statCopy)
	}
	s.statsBuffer = make(map[string]*HourlyStat)
	s.statsBufferMu.Unlock()

	if len(statsToFlush) == 0 {
		return
	}

	var statsToUpsert []models.GroupHourlyStat
	for _, stat := range statsToFlush {
		statsToUpsert = append(statsToUpsert, models.GroupHourlyStat{
			Time:         stat.Time,
			GroupID:      stat.GroupID,
			SuccessCount: stat.SuccessCount,
			FailureCount: stat.FailureCount,
		})
	}

	sort.Slice(statsToUpsert, func(i, j int) bool {
		if statsToUpsert[i].Time.Equal(statsToUpsert[j].Time) {
			return statsToUpsert[i].GroupID < statsToUpsert[j].GroupID
		}
		return statsToUpsert[i].Time.Before(statsToUpsert[j].Time)
	})

	err := s.db.Transaction(func(tx *gorm.DB) error {
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "time"}, {Name: "group_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"success_count": gorm.Expr("group_hourly_stats.success_count + VALUES(success_count)"),
				"failure_count": gorm.Expr("group_hourly_stats.failure_count + VALUES(failure_count)"),
				"updated_at":    time.Now(),
			}),
		}).CreateInBatches(statsToUpsert, 100).Error
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"count": len(statsToUpsert),
			"error": err,
		}).Error("Failed to flush stats to database")

		s.statsBufferMu.Lock()
		for _, stat := range statsToFlush {
			key := fmt.Sprintf("%d-%d", stat.Time.Unix(), stat.GroupID)
			if existing, exists := s.statsBuffer[key]; exists {
				existing.SuccessCount += stat.SuccessCount
				existing.FailureCount += stat.FailureCount
			} else {
				s.statsBuffer[key] = stat
			}
		}
		s.statsBufferMu.Unlock()

		return
	}

	logrus.WithField("count", len(statsToUpsert)).Debug("Successfully flushed stats to database")
}

// GetPendingLogs retrieves logs from Redis cache that haven't been flushed to database yet
// This is used for real-time statistics in dashboard
func (s *RequestLogService) GetPendingLogs() ([]*models.RequestLog, error) {
	keys, err := s.store.SMembers(PendingLogKeysSet)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending log keys: %w", err)
	}

	if len(keys) == 0 {
		return []*models.RequestLog{}, nil
	}

	var logs []*models.RequestLog
	for _, key := range keys {
		logBytes, err := s.store.Get(key)
		if err != nil {
			if err == store.ErrNotFound {
				logrus.Debugf("Log key %s not found in cache, skipping", key)
			} else {
				logrus.Warnf("Failed to get log for key %s: %v", key, err)
			}
			continue
		}

		var log models.RequestLog
		if err := json.Unmarshal(logBytes, &log); err != nil {
			logrus.Warnf("Failed to unmarshal log for key %s: %v", key, err)
			continue
		}
		logs = append(logs, &log)
	}

	return logs, nil
}

// GetPendingStats returns a snapshot of the current stats buffer
// This is used for real-time statistics in dashboard
func (s *RequestLogService) GetPendingStats() map[string]*HourlyStat {
	s.statsBufferMu.Lock()
	defer s.statsBufferMu.Unlock()

	statsCopy := make(map[string]*HourlyStat, len(s.statsBuffer))
	for key, stat := range s.statsBuffer {
		statCopy := &HourlyStat{
			Time:         stat.Time,
			GroupID:      stat.GroupID,
			SuccessCount: stat.SuccessCount,
			FailureCount: stat.FailureCount,
		}
		statsCopy[key] = statCopy
	}

	return statsCopy
}
