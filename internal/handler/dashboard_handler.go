package handler

import (
	"fmt"
	"gpt-load/internal/encryption"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/i18n"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"gpt-load/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	rpmCalculationWindowMinutes = 10
	rpmComparisonWindowMinutes  = 20
	groupTypeAggregate          = "aggregate"
)

// boolToInt64 converts bool to int64 (1 for true, 0 for false)
func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// parseDaysParameter converts days string to days integer (for stats API)
func parseDaysParameter(daysStr string) int {
	switch daysStr {
	case "3":
		return 3
	case "7":
		return 7
	default:
		return 1
	}
}

// parseHoursParameter converts hours string to hours integer
// Supports: 1, 5, 24(1 day), 168(1 week), 720(1 month)
func parseHoursParameter(hoursStr string) int {
	switch hoursStr {
	case "1":
		return 1
	case "5":
		return 5
	case "24":
		return 24
	case "168":
		return 168
	case "720":
		return 720
	default:
		return 24 // default to 24 hours (1 day)
	}
}

// parseStatsHoursParameter converts hours string to hours integer for stats API
// Supports: 1, 5, 24, 168, 720 hours
func parseStatsHoursParameter(hoursStr string) int {
	switch hoursStr {
	case "1":
		return 1
	case "5":
		return 5
	case "24":
		return 24
	case "168":
		return 168
	case "720":
		return 720
	default:
		return 1 // default to 1 hour
	}
}

// calculateIntervalMinutes returns the interval in minutes for chart aggregation
// based on the total hours in the time range
func calculateIntervalMinutes(totalHours int) int {
	switch {
	case totalHours <= 1: // 1 hour
		return 10 // 6 points
	case totalHours <= 5: // 1-5 hours
		return 15 // 20 points
	case totalHours <= 24: // 5-24 hours
		return 30 // 48 points for 24h
	case totalHours <= 168: // 24 hours to 1 week
		return 120 // 84 points (every 2 hours)
	default: // > 1 week (1 month = 720 hours)
		return 600 // 72 points (every 10 hours)
	}
}

// getAggregateGroupIDs returns a set of group IDs that are of aggregate type
func (s *Server) getAggregateGroupIDs() (map[uint]bool, error) {
	var ids []uint
	err := s.DB.Model(&models.Group{}).
		Where("group_type = ?", groupTypeAggregate).
		Pluck("id", &ids).Error

	if err != nil {
		return nil, err
	}

	aggregateMap := make(map[uint]bool, len(ids))
	for _, id := range ids {
		aggregateMap[id] = true
	}
	return aggregateMap, nil
}

// isPendingLogIncluded checks if a pending log should be included in statistics
// by verifying it's within time range, is a final request type, and not from an aggregate group
func isPendingLogIncluded(log *models.RequestLog, startTime, endTime time.Time, aggregateGroupIDs map[uint]bool) bool {
	if log.Timestamp.Before(startTime) || log.Timestamp.After(endTime) {
		return false
	}
	if log.RequestType != models.RequestTypeFinal {
		return false
	}
	if log.GroupID > 0 && aggregateGroupIDs[log.GroupID] {
		return false
	}
	return true
}

type trendResult struct {
	value       float64
	isGrowth     bool
}

func calculateTrend(current, previous int64) trendResult {
	if previous > 0 {
		trend := (float64(current-previous) / float64(previous)) * 100
		return trendResult{value: trend, isGrowth: trend >= 0}
	} else if current > 0 {
		return trendResult{value: 100.0, isGrowth: true}
	}
	return trendResult{value: 0.0, isGrowth: true}
}

func calculateErrorRateTrend(currentErrorRate, previousErrorRate float64, hasCurrentData, hasPreviousData bool) trendResult {
	if hasPreviousData {
		trend := currentErrorRate - previousErrorRate
		isGrowth := trend < 0
		return trendResult{value: trend, isGrowth: isGrowth}
	} else if hasCurrentData {
		if currentErrorRate == 0 {
			return trendResult{value: currentErrorRate, isGrowth: true}
		}
		return trendResult{value: currentErrorRate, isGrowth: false}
	}
	return trendResult{value: 0.0, isGrowth: true}
}

// Stats Get dashboard statistics
func (s *Server) Stats(c *gin.Context) {
	// Support both 'hours' and 'days' parameters for backward compatibility
	// Priority: hours > days
	hoursStr := c.Query("hours")
	var hours int
	if hoursStr != "" {
		hours = parseStatsHoursParameter(hoursStr)
	} else {
		days := parseDaysParameter(c.DefaultQuery("days", "1"))
		hours = days * 24
	}

	now := time.Now()
	rpmStats, err := s.getRPMStats(now)
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.rpm_stats_failed")
		return
	}

	// Calculate time ranges based on hours
	currentDuration := time.Duration(hours) * time.Hour
	previousDuration := currentDuration

	currentStart := now.Add(-currentDuration)
	previousStart := now.Add(-currentDuration - previousDuration)
	previousEnd := currentStart

	// Get token consumption statistics
	currentTokenStats, err := s.getDetailedTokenStats(currentStart, now)
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.current_stats_failed")
		return
	}
	previousTokenStats, err := s.getDetailedTokenStats(previousStart, previousEnd)
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.previous_stats_failed")
		return
	}

	currentPeriod, err := s.getHourlyStats(currentStart, now)
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.current_stats_failed")
		return
	}
	previousPeriod, err := s.getHourlyStats(previousStart, previousEnd)
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.previous_stats_failed")
		return
	}

	// Get key count statistics
	currentKeyStats, err := s.getKeyStats(currentStart, now)
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.key_stats_failed")
		return
	}
	previousKeyStats, err := s.getKeyStats(previousStart, previousEnd)
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.previous_stats_failed")
		return
	}

	reqTrendResult := calculateTrend(currentPeriod.TotalRequests, previousPeriod.TotalRequests)
	reqTrend := reqTrendResult.value
	reqTrendIsGrowth := reqTrendResult.isGrowth

	currentErrorRate := 0.0
	if currentPeriod.TotalRequests > 0 {
		currentErrorRate = (float64(currentPeriod.TotalFailures) / float64(currentPeriod.TotalRequests)) * 100
	}

	previousErrorRate := 0.0
	if previousPeriod.TotalRequests > 0 {
		previousErrorRate = (float64(previousPeriod.TotalFailures) / float64(previousPeriod.TotalRequests)) * 100
	}

	errorRateTrendResult := calculateErrorRateTrend(currentErrorRate, previousErrorRate, currentPeriod.TotalRequests > 0, previousPeriod.TotalRequests > 0)
	errorRateTrend := errorRateTrendResult.value
	errorRateTrendIsGrowth := errorRateTrendResult.isGrowth

	tokenTrendResult := calculateTrend(currentTokenStats.TotalTokens, previousTokenStats.TotalTokens)
	tokenTrend := tokenTrendResult.value
	tokenTrendIsGrowth := tokenTrendResult.isGrowth

	completionTrendResult := calculateTrend(currentTokenStats.CompletionTokens, previousTokenStats.CompletionTokens)
	completionTrend := completionTrendResult.value
	completionTrendIsGrowth := completionTrendResult.isGrowth

	cachedTrendResult := calculateTrend(currentTokenStats.CachedTokens, previousTokenStats.CachedTokens)
	cachedTrend := cachedTrendResult.value
	cachedTrendIsGrowth := cachedTrendResult.isGrowth

	// Calculate non-cached prompt tokens (prompt_tokens - cached_tokens)
	currentNonCachedPrompt := currentTokenStats.PromptTokens - currentTokenStats.CachedTokens
	if currentNonCachedPrompt < 0 {
		currentNonCachedPrompt = 0
	}
	previousNonCachedPrompt := previousTokenStats.PromptTokens - previousTokenStats.CachedTokens
	if previousNonCachedPrompt < 0 {
		previousNonCachedPrompt = 0
	}

	nonCachedPromptTrendResult := calculateTrend(currentNonCachedPrompt, previousNonCachedPrompt)
	nonCachedPromptTrend := nonCachedPromptTrendResult.value
	nonCachedPromptTrendIsGrowth := nonCachedPromptTrendResult.isGrowth

	promptTrendResult := calculateTrend(currentTokenStats.PromptTokens, previousTokenStats.PromptTokens)
	promptTrend := promptTrendResult.value
	promptTrendIsGrowth := promptTrendResult.isGrowth

	keyTrendResult := calculateTrend(currentKeyStats.TotalKeys, previousKeyStats.TotalKeys)
	keyTrend := keyTrendResult.value
	keyTrendIsGrowth := keyTrendResult.isGrowth

	// Get security warning information
	securityWarnings := s.getSecurityWarnings(c)

	stats := models.DashboardStatsResponse{
		KeyCount: models.StatCard{
			Value:         float64(currentKeyStats.TotalKeys),
			Trend:         keyTrend,
			TrendIsGrowth: keyTrendIsGrowth,
		},
		TokenConsumption: models.StatCard{
			Value:         float64(currentTokenStats.TotalTokens),
			Trend:         tokenTrend,
			TrendIsGrowth: tokenTrendIsGrowth,
		},
		PromptTokens: models.StatCard{
			Value:         float64(currentTokenStats.PromptTokens),
			Trend:         promptTrend,
			TrendIsGrowth: promptTrendIsGrowth,
		},
		NonCachedPromptTokens: models.StatCard{
			Value:         float64(currentNonCachedPrompt),
			Trend:         nonCachedPromptTrend,
			TrendIsGrowth: nonCachedPromptTrendIsGrowth,
		},
		CachedTokens: models.StatCard{
			Value:         float64(currentTokenStats.CachedTokens),
			Trend:         cachedTrend,
			TrendIsGrowth: cachedTrendIsGrowth,
		},
		CompletionTokens: models.StatCard{
			Value:         float64(currentTokenStats.CompletionTokens),
			Trend:         completionTrend,
			TrendIsGrowth: completionTrendIsGrowth,
		},
		TotalTokens: models.StatCard{
			Value:         float64(currentTokenStats.TotalTokens),
			Trend:         tokenTrend,
			TrendIsGrowth: tokenTrendIsGrowth,
		},
		RPM: rpmStats,
		RequestCount: models.StatCard{
			Value:         float64(currentPeriod.TotalRequests),
			Trend:         reqTrend,
			TrendIsGrowth: reqTrendIsGrowth,
		},
		ErrorRate: models.StatCard{
			Value:         currentErrorRate,
			Trend:         errorRateTrend,
			TrendIsGrowth: errorRateTrendIsGrowth,
		},
		SecurityWarnings: securityWarnings,
	}

	response.Success(c, stats)
}

// Chart Get dashboard chart data
func (s *Server) Chart(c *gin.Context) {
	viewType := c.DefaultQuery("view", "request")
	hours := parseHoursParameter(c.DefaultQuery("hours", "24"))

	now := time.Now()
	endTime := now
	startTime := now.Add(-time.Duration(hours) * time.Hour)

	if viewType == "token" {
		// Token view - get token statistics from request_logs
		s.getTokenChart(c, startTime, endTime)
	} else {
		// Request view - get request statistics from group_hourly_stats
		s.getRequestChart(c, startTime, endTime)
	}
}

// getRequestChart returns request statistics chart data with dynamic granularity
func (s *Server) getRequestChart(c *gin.Context, startTime, endTime time.Time) {
	now := time.Now()

	totalHours := int(now.Sub(startTime).Hours())
	if totalHours < 1 {
		totalHours = 1
	}

	intervalMinutes := calculateIntervalMinutes(totalHours)

	// For hour-level or larger intervals, use group_hourly_stats (more efficient)
	// For minute-level intervals (less than 60 minutes), use request_logs directly
	var labels []string
	var successData, failureData []int64

	// 统一使用 request_logs 表
	type requestResult struct {
		TimeSlot string
		Success  int64
		Failure  int64
	}
	var results []requestResult

	// 构建数据库查询
	dbType := s.DB.Dialector.Name()
	var selectClause, groupClause string

	switch dbType {
	case "mysql":
		selectClause = fmt.Sprintf("DATE_FORMAT(timestamp, '%%Y-%%m-%%d %%H:%%i:00') as time_slot, SUM(CASE WHEN is_success = 1 THEN 1 ELSE 0 END) as success, SUM(CASE WHEN is_success = 0 THEN 1 ELSE 0 END) as failure")
		groupClause = "time_slot"
	case "postgres":
		selectClause = fmt.Sprintf("to_char(DATE_TRUNC('minute', timestamp), 'YYYY-MM-DD HH24:MI:00') as time_slot, SUM(CASE WHEN is_success = true THEN 1 ELSE 0 END) as success, SUM(CASE WHEN is_success = false THEN 1 ELSE 0 END) as failure")
		groupClause = "time_slot"
	default: // sqlite and others
		selectClause = fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:%%M:00', timestamp) as time_slot, SUM(CASE WHEN is_success = 1 THEN 1 ELSE 0 END) as success, SUM(CASE WHEN is_success = 0 THEN 1 ELSE 0 END) as failure")
		groupClause = "time_slot"
	}

	err := s.DB.Model(&models.RequestLog{}).
		Select(selectClause).
		Where("timestamp >= ? AND timestamp < ? AND request_type = ?", startTime, endTime, models.RequestTypeFinal).
		Where("group_id NOT IN (?)",
			s.DB.Table("groups").Select("id").Where("group_type = ?", groupTypeAggregate)).
		Group(groupClause).
		Order("time_slot asc").
		Scan(&results).Error

	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.chart_data_failed")
		return
	}

	// Create a map for easy lookup
	statsBySlot := make(map[time.Time]requestResult)
	for _, result := range results {
		slotTime, _ := time.ParseInLocation("2006-01-02 15:04:05", result.TimeSlot, time.Local)
		slotTime = slotTime.Truncate(time.Minute)
		statsBySlot[slotTime] = result
	}

	// Get pending logs from Redis cache for real-time statistics
	if s.RequestLogService != nil {
		pendingLogs, err := s.RequestLogService.GetPendingLogs()
		if err != nil {
			logrus.Warnf("Failed to get pending logs for real-time stats: %v", err)
		} else {
			aggregateGroupIDs, err := s.getAggregateGroupIDs()
			if err != nil {
				logrus.Warnf("Failed to get aggregate group IDs: %v", err)
				aggregateGroupIDs = make(map[uint]bool)
			}

			for _, log := range pendingLogs {
				if !isPendingLogIncluded(log, startTime, endTime, aggregateGroupIDs) {
					continue
				}

				slotTime := log.Timestamp.Truncate(time.Minute)
				if existing, ok := statsBySlot[slotTime]; ok {
					if log.IsSuccess {
						existing.Success++
					} else {
						existing.Failure++
					}
					statsBySlot[slotTime] = existing
				} else {
					statsBySlot[slotTime] = requestResult{
						TimeSlot: slotTime.Format("2006-01-02 15:04:05"),
						Success:  boolToInt64(log.IsSuccess),
						Failure:  boolToInt64(!log.IsSuccess),
					}
				}
			}
		}
	}

	totalMinutes := totalHours * 60
	intervals := totalMinutes / intervalMinutes
	if intervals < 1 {
		intervals = 1
	}

	startSlot := startTime.Truncate(time.Minute)
	for i := 0; i < intervals; i++ {
		slot := startSlot.Add(time.Duration(i*intervalMinutes) * time.Minute)
		labels = append(labels, slot.Format(time.RFC3339))

		if data, ok := statsBySlot[slot]; ok {
			successData = append(successData, data.Success)
			failureData = append(failureData, data.Failure)
		} else {
			successData = append(successData, 0)
			failureData = append(failureData, 0)
		}
	}

	chartData := models.ChartData{
		Labels: labels,
		Datasets: []models.ChartDataset{
			{
				Label:    i18n.Message(c, "dashboard.success_requests"),
				LabelKey: "dashboard.success_requests",
				Data:     successData,
			},
			{
				Label:    i18n.Message(c, "dashboard.failed_requests"),
				LabelKey: "dashboard.failed_requests",
				Data:     failureData,
			},
		},
	}

	response.Success(c, chartData)
}

// getTokenChart returns token statistics chart data with dynamic granularity
func (s *Server) getTokenChart(c *gin.Context, startTime, endTime time.Time) {
	now := time.Now()
	totalHours := int(now.Sub(startTime).Hours())
	if totalHours < 1 {
		totalHours = 1
	}

	intervalMinutes := calculateIntervalMinutes(totalHours)

	// Get token statistics from request_logs with dynamic aggregation
	type tokenResult struct {
		TimeSlot         string
		PromptTokens     int64
		CompletionTokens int64
		TotalTokens      int64
		CachedTokens     int64
	}

	var results []tokenResult

	// Build database-specific query for dynamic granularity
	dbType := s.DB.Dialector.Name()
	var selectClause, groupClause string

	switch dbType {
	case "mysql":
		selectClause = fmt.Sprintf("DATE_FORMAT(timestamp, '%%Y-%%m-%%d %%H:%%i:00') as time_slot, COALESCE(SUM(prompt_tokens), 0) as prompt_tokens, COALESCE(SUM(completion_tokens), 0) as completion_tokens, COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cached_tokens), 0) as cached_tokens")
		groupClause = "time_slot"
	case "postgres":
		selectClause = fmt.Sprintf("to_char(DATE_TRUNC('minute', timestamp), 'YYYY-MM-DD HH24:MI:00') as time_slot, COALESCE(SUM(prompt_tokens), 0) as prompt_tokens, COALESCE(SUM(completion_tokens), 0) as completion_tokens, COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cached_tokens), 0) as cached_tokens")
		groupClause = "time_slot"
	default: // sqlite and others
		selectClause = fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:%%M:00', timestamp) as time_slot, COALESCE(SUM(prompt_tokens), 0) as prompt_tokens, COALESCE(SUM(completion_tokens), 0) as completion_tokens, COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cached_tokens), 0) as cached_tokens")
		groupClause = "time_slot"
	}

	err := s.DB.Model(&models.RequestLog{}).
		Select(selectClause).
		Where("timestamp >= ? AND timestamp < ? AND request_type = ?", startTime, endTime, models.RequestTypeFinal).
		Where("group_id NOT IN (?)",
			s.DB.Table("groups").Select("id").Where("group_type = ?", groupTypeAggregate)).
		Group(groupClause).
		Order("time_slot asc").
		Scan(&results).Error

	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.chart_data_failed")
		return
	}

	// Create a map for easy lookup
	statsBySlot := make(map[time.Time]tokenResult)
	for _, result := range results {
		slotTime, _ := time.ParseInLocation("2006-01-02 15:04:05", result.TimeSlot, time.Local)
		slotTime = slotTime.Truncate(time.Minute)
		statsBySlot[slotTime] = result
	}

	// Get pending logs from Redis cache for real-time statistics
	if s.RequestLogService != nil {
		pendingLogs, err := s.RequestLogService.GetPendingLogs()
		if err != nil {
			logrus.Warnf("Failed to get pending logs for real-time stats: %v", err)
		} else {
			aggregateGroupIDs, err := s.getAggregateGroupIDs()
			if err != nil {
				logrus.Warnf("Failed to get aggregate group IDs: %v", err)
				aggregateGroupIDs = make(map[uint]bool)
			}

			// Aggregate pending logs by time slot
			pendingBySlot := make(map[time.Time]*tokenResult)
			for _, log := range pendingLogs {
				if !isPendingLogIncluded(log, startTime, endTime, aggregateGroupIDs) {
					continue
				}

				// Truncate to minute level for aggregation
				slotTime := log.Timestamp.Truncate(time.Minute)
				if _, exists := pendingBySlot[slotTime]; !exists {
					pendingBySlot[slotTime] = &tokenResult{
						TimeSlot:         slotTime.Format("2006-01-02 15:04:05"),
						PromptTokens:     0,
						CompletionTokens: 0,
						TotalTokens:      0,
						CachedTokens:     0,
					}
				}
				pendingBySlot[slotTime].PromptTokens += log.PromptTokens
				pendingBySlot[slotTime].CompletionTokens += log.CompletionTokens
				pendingBySlot[slotTime].TotalTokens += log.TotalTokens
				pendingBySlot[slotTime].CachedTokens += log.CachedTokens
			}

			// Merge pending data into statsBySlot
			for slotTime, pendingData := range pendingBySlot {
				if existing, ok := statsBySlot[slotTime]; ok {
					// Add to existing data
					existing.PromptTokens += pendingData.PromptTokens
					existing.CompletionTokens += pendingData.CompletionTokens
					existing.TotalTokens += pendingData.TotalTokens
					existing.CachedTokens += pendingData.CachedTokens
					statsBySlot[slotTime] = existing
				} else {
					// New time slot
					statsBySlot[slotTime] = *pendingData
				}
			}
		}
	}

	var labels []string
	var nonCachedPromptData, cachedData, outputData, totalData []int64

	totalMinutes := totalHours * 60
	intervals := totalMinutes / intervalMinutes
	if intervals < 1 {
		intervals = 1
	}

	startSlot := startTime.Truncate(time.Minute)
	for i := 0; i < intervals; i++ {
		slotStart := startSlot.Add(time.Duration(i*intervalMinutes) * time.Minute)
		slotEnd := slotStart.Add(time.Duration(intervalMinutes) * time.Minute)
		labels = append(labels, slotStart.Format(time.RFC3339))

		var promptSum, cachedSum, outputSum, totalSum int64

		for t := slotStart; t.Before(slotEnd); t = t.Add(time.Minute) {
			if data, ok := statsBySlot[t]; ok {
				promptSum += data.PromptTokens
				cachedSum += data.CachedTokens
				outputSum += data.CompletionTokens
				totalSum += data.TotalTokens
			}
		}

		// Calculate non-cached prompt tokens (prompt - cached)
		nonCachedPromptSum := promptSum - cachedSum
		if nonCachedPromptSum < 0 {
			nonCachedPromptSum = 0
		}

		nonCachedPromptData = append(nonCachedPromptData, nonCachedPromptSum)
		cachedData = append(cachedData, cachedSum)
		outputData = append(outputData, outputSum)
		totalData = append(totalData, totalSum)
	}

	chartData := models.ChartData{
		Labels: labels,
		Datasets: []models.ChartDataset{
			{
				Label:    i18n.Message(c, "dashboard.non_cached_prompt_tokens"),
				LabelKey: "dashboard.non_cached_prompt_tokens",
				Data:     nonCachedPromptData,
			},
			{
				Label:    i18n.Message(c, "dashboard.cached_tokens"),
				LabelKey: "dashboard.cached_tokens",
				Data:     cachedData,
			},
			{
				Label:    i18n.Message(c, "dashboard.completion_tokens"),
				LabelKey: "dashboard.completion_tokens",
				Data:     outputData,
			},
			{
				Label:    i18n.Message(c, "dashboard.total_tokens"),
				LabelKey: "dashboard.total_tokens",
				Data:     totalData,
			},
		},
	}

	response.Success(c, chartData)
}

type hourlyStatResult struct {
	TotalRequests int64
	TotalFailures int64
}

func (s *Server) getHourlyStats(startTime, endTime time.Time) (hourlyStatResult, error) {
	var result hourlyStatResult

	err := s.DB.Model(&models.RequestLog{}).
		Where("timestamp >= ? AND timestamp < ? AND request_type = ?", startTime, endTime, models.RequestTypeFinal).
		Where("group_id NOT IN (?)",
			s.DB.Table("groups").Select("id").Where("group_type = ?", groupTypeAggregate)).
		Select("COUNT(*) as total_requests, SUM(CASE WHEN is_success = 0 THEN 1 ELSE 0 END) as total_failures").
		Scan(&result).Error
	if err != nil {
		return result, err
	}

	if s.RequestLogService != nil {
		pendingLogs, err := s.RequestLogService.GetPendingLogs()
		if err != nil {
			logrus.Warnf("Failed to get pending logs: %v", err)
		} else {
			aggregateGroupIDs, err := s.getAggregateGroupIDs()
			if err != nil {
				logrus.Warnf("Failed to get aggregate group IDs: %v", err)
				aggregateGroupIDs = make(map[uint]bool)
			}

			for _, log := range pendingLogs {
				if !isPendingLogIncluded(log, startTime, endTime, aggregateGroupIDs) {
					continue
				}

				result.TotalRequests++
				if !log.IsSuccess {
					result.TotalFailures++
				}
			}
		}
	}

	return result, nil
}

type rpmStatResult struct {
	CurrentRequests  int64
	PreviousRequests int64
}

func (s *Server) getRPMStats(now time.Time) (models.StatCard, error) {
	tenMinutesAgo := now.Add(-time.Duration(rpmCalculationWindowMinutes) * time.Minute)
	twentyMinutesAgo := now.Add(-time.Duration(rpmComparisonWindowMinutes) * time.Minute)

	var result rpmStatResult
	err := s.DB.Model(&models.RequestLog{}).
		Select("count(case when timestamp >= ? then 1 end) as current_requests, count(case when timestamp >= ? and timestamp < ? then 1 end) as previous_requests", tenMinutesAgo, twentyMinutesAgo, tenMinutesAgo).
		Where("timestamp >= ? AND request_type = ?", twentyMinutesAgo, models.RequestTypeFinal).
		Scan(&result).Error

	if err != nil {
		return models.StatCard{}, err
	}

	currentRPM := float64(result.CurrentRequests) / float64(rpmCalculationWindowMinutes)
	previousRPM := float64(result.PreviousRequests) / float64(rpmCalculationWindowMinutes)

	rpmTrend := 0.0
	rpmTrendIsGrowth := true
	if previousRPM > 0 {
		rpmTrend = (currentRPM - previousRPM) / previousRPM * 100
		rpmTrendIsGrowth = rpmTrend >= 0
	} else if currentRPM > 0 {
		rpmTrend = 100.0
		rpmTrendIsGrowth = true
	}

	return models.StatCard{
		Value:         currentRPM,
		Trend:         rpmTrend,
		TrendIsGrowth: rpmTrendIsGrowth,
	}, nil
}

// getSecurityWarnings checks security configuration and returns warning messages
func (s *Server) getSecurityWarnings(c *gin.Context) []models.SecurityWarning {
	var warnings []models.SecurityWarning

	// Get AUTH_KEY and ENCRYPTION_KEY
	authConfig := s.config.GetAuthConfig()
	encryptionKey := s.config.GetEncryptionKey()

	// Check AUTH_KEY
	if authConfig.Key == "" {
		warnings = append(warnings, models.SecurityWarning{
			Type:       "AUTH_KEY",
			Message:    i18n.Message(c, "dashboard.auth_key_missing"),
			Severity:   "high",
			Suggestion: i18n.Message(c, "dashboard.auth_key_required"),
		})
	} else {
		authWarnings := checkPasswordSecurity(c, authConfig.Key, "AUTH_KEY")
		warnings = append(warnings, authWarnings...)
	}

	// Check ENCRYPTION_KEY
	if encryptionKey == "" {
		warnings = append(warnings, models.SecurityWarning{
			Type:       "ENCRYPTION_KEY",
			Message:    i18n.Message(c, "dashboard.encryption_key_missing"),
			Severity:   "high",
			Suggestion: i18n.Message(c, "dashboard.encryption_key_recommended"),
		})
	} else {
		encryptionWarnings := checkPasswordSecurity(c, encryptionKey, "ENCRYPTION_KEY")
		warnings = append(warnings, encryptionWarnings...)
	}

	// Check system-level proxy keys
	systemSettings := s.SettingsManager.GetSettings()
	if systemSettings.ProxyKeys != "" {
		proxyKeys := strings.Split(systemSettings.ProxyKeys, ",")
		for i, key := range proxyKeys {
			key = strings.TrimSpace(key)
			if key != "" {
				keyName := fmt.Sprintf("%s #%d", i18n.Message(c, "dashboard.global_proxy_key"), i+1)
				proxyWarnings := checkPasswordSecurity(c, key, keyName)
				warnings = append(warnings, proxyWarnings...)
			}
		}
	}

	// Check group-level proxy keys
	var groups []models.Group
	if err := s.DB.Where("proxy_keys IS NOT NULL AND proxy_keys != ''").Find(&groups).Error; err == nil {
		for _, group := range groups {
			if group.ProxyKeys != "" {
				proxyKeys := strings.Split(group.ProxyKeys, ",")
				for i, key := range proxyKeys {
					key = strings.TrimSpace(key)
					if key != "" {
						keyName := fmt.Sprintf("%s [%s] #%d", i18n.Message(c, "dashboard.group_proxy_key"), group.Name, i+1)
						proxyWarnings := checkPasswordSecurity(c, key, keyName)
						warnings = append(warnings, proxyWarnings...)
					}
				}
			}
		}
	}

	return warnings
}

// checkPasswordSecurity comprehensively checks password security
func checkPasswordSecurity(c *gin.Context, password, keyType string) []models.SecurityWarning {
	var warnings []models.SecurityWarning

	// 1. Length check
	if len(password) < 16 {
		warnings = append(warnings, models.SecurityWarning{
			Type:       keyType,
			Message:    i18n.Message(c, "security.password_too_short", map[string]any{"keyType": keyType, "length": len(password)}),
			Severity:   "high", // Insufficient length is high risk
			Suggestion: i18n.Message(c, "security.password_recommendation_16"),
		})
	} else if len(password) < 32 {
		warnings = append(warnings, models.SecurityWarning{
			Type:       keyType,
			Message:    i18n.Message(c, "security.password_short", map[string]any{"keyType": keyType, "length": len(password)}),
			Severity:   "medium",
			Suggestion: i18n.Message(c, "security.password_recommendation_32"),
		})
	}

	// 2. Common weak password check
	lower := strings.ToLower(password)
	weakPatterns := utils.WeakPasswordPatterns

	for _, pattern := range weakPatterns {
		if strings.Contains(lower, pattern) {
			warnings = append(warnings, models.SecurityWarning{
				Type:       keyType,
				Message:    i18n.Message(c, "security.password_weak_pattern", map[string]any{"keyType": keyType, "pattern": pattern}),
				Severity:   "high",
				Suggestion: i18n.Message(c, "security.password_avoid_common"),
			})
			break
		}
	}

	// 3. Complexity check (only when length is sufficient)
	if len(password) >= 16 && !hasGoodComplexity(password) {
		warnings = append(warnings, models.SecurityWarning{
			Type:       keyType,
			Message:    i18n.Message(c, "security.password_low_complexity", map[string]any{"keyType": keyType}),
			Severity:   "medium",
			Suggestion: i18n.Message(c, "security.password_complexity"),
		})
	}

	return warnings
}

// hasGoodComplexity checks password complexity
func hasGoodComplexity(password string) bool {
	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasDigit = true
		case !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')):
			hasSpecial = true
		}
	}

	// At least 3 types of characters required
	count := 0
	if hasUpper {
		count++
	}
	if hasLower {
		count++
	}
	if hasDigit {
		count++
	}
	if hasSpecial {
		count++
	}

	return count >= 3
}

// Encryption scenario types
const (
	ScenarioNone             = ""
	ScenarioDataNotEncrypted = "data_not_encrypted"
	ScenarioKeyNotConfigured = "key_not_configured"
	ScenarioKeyMismatch      = "key_mismatch"
)

// EncryptionStatus checks if ENCRYPTION_KEY is configured but keys are not encrypted
func (s *Server) EncryptionStatus(c *gin.Context) {
	hasMismatch, scenarioType, message, suggestion := s.checkEncryptionMismatch(c)

	response.Success(c, gin.H{
		"has_mismatch":  hasMismatch,
		"scenario_type": scenarioType,
		"message":       message,
		"suggestion":    suggestion,
	})
}

// checkEncryptionMismatch detects encryption configuration mismatches
func (s *Server) checkEncryptionMismatch(c *gin.Context) (bool, string, string, string) {
	encryptionKey := s.config.GetEncryptionKey()

	// Sample check API keys
	var sampleKeys []models.APIKey
	if err := s.DB.Limit(20).Where("key_hash IS NOT NULL AND key_hash != ''").Find(&sampleKeys).Error; err != nil {
		logrus.WithError(err).Error("Failed to fetch sample keys for encryption check")
		return false, ScenarioNone, "", ""
	}

	if len(sampleKeys) == 0 {
		// No keys in database, no mismatch
		return false, ScenarioNone, "", ""
	}

	// Check hash consistency with unencrypted data
	noopService, err := encryption.NewService("")
	if err != nil {
		logrus.WithError(err).Error("Failed to create noop encryption service")
		return false, ScenarioNone, "", ""
	}

	unencryptedHashMatchCount := 0
	for _, key := range sampleKeys {
		// For unencrypted data: key_hash should match SHA256(key_value)
		expectedHash := noopService.Hash(key.KeyValue)
		if expectedHash == key.KeyHash {
			unencryptedHashMatchCount++
		}
	}

	unencryptedConsistencyRate := float64(unencryptedHashMatchCount) / float64(len(sampleKeys))

	// If ENCRYPTION_KEY is configured, also check if current key can decrypt the data
	var currentKeyHashMatchCount int
	if encryptionKey != "" {
		currentService, err := encryption.NewService(encryptionKey)
		if err == nil {
			for _, key := range sampleKeys {
				// Try to decrypt and re-hash to check if current key matches
				decrypted, err := currentService.Decrypt(key.KeyValue)
				if err == nil {
					// Successfully decrypted, check if hash matches
					expectedHash := currentService.Hash(decrypted)
					if expectedHash == key.KeyHash {
						currentKeyHashMatchCount++
					}
				}
			}
		}
	}
	currentKeyConsistencyRate := float64(currentKeyHashMatchCount) / float64(len(sampleKeys))

	// Scenario A: ENCRYPTION_KEY configured but data not encrypted
	if encryptionKey != "" && unencryptedConsistencyRate > 0.8 {
		return true,
			ScenarioDataNotEncrypted,
			i18n.Message(c, "dashboard.encryption_key_configured_but_data_not_encrypted"),
			i18n.Message(c, "dashboard.encryption_key_migration_required")
	}

	// Scenario B: ENCRYPTION_KEY not configured but data is encrypted
	if encryptionKey == "" && unencryptedConsistencyRate < 0.2 {
		return true,
			ScenarioKeyNotConfigured,
			i18n.Message(c, "dashboard.data_encrypted_but_key_not_configured"),
			i18n.Message(c, "dashboard.configure_same_encryption_key")
	}

	// Scenario C: ENCRYPTION_KEY configured but doesn't match encrypted data
	if encryptionKey != "" && unencryptedConsistencyRate < 0.2 && currentKeyConsistencyRate < 0.2 {
		return true,
			ScenarioKeyMismatch,
			i18n.Message(c, "dashboard.encryption_key_mismatch"),
			i18n.Message(c, "dashboard.use_correct_encryption_key")
	}

	return false, ScenarioNone, "", ""
}

// getTokenStats gets token usage statistics for a time period
type tokenStatsResult struct {
	TotalTokens int64
}

func (s *Server) getTokenStats(startTime, endTime time.Time) (tokenStatsResult, error) {
	var result tokenStatsResult
	err := s.DB.Model(&models.RequestLog{}).
		Where("timestamp >= ? AND timestamp < ? AND request_type = ?", startTime, endTime, models.RequestTypeFinal).
		Select("COALESCE(SUM(total_tokens), 0) as total_tokens").
		Scan(&result).Error
	return result, err
}

// detailedTokenStatsResult represents detailed token statistics
type detailedTokenStatsResult struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	CachedTokens     int64
}

// getDetailedTokenStats gets detailed token usage statistics for a time period
func (s *Server) getDetailedTokenStats(startTime, endTime time.Time) (detailedTokenStatsResult, error) {
	var result detailedTokenStatsResult
	err := s.DB.Model(&models.RequestLog{}).
		Where("timestamp >= ? AND timestamp < ? AND request_type = ?", startTime, endTime, models.RequestTypeFinal).
		Select("COALESCE(SUM(prompt_tokens), 0) as prompt_tokens, COALESCE(SUM(completion_tokens), 0) as completion_tokens, COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cached_tokens), 0) as cached_tokens").
		Scan(&result).Error
	return result, err
}

// keyStatsResult represents key count statistics
type keyStatsResult struct {
	TotalKeys int64
}

// getKeyStats gets key count statistics for a time period
func (s *Server) getKeyStats(startTime, endTime time.Time) (keyStatsResult, error) {
	var result keyStatsResult
	err := s.DB.Model(&models.APIKey{}).
		Select("COUNT(*) as total_keys").
		Scan(&result).Error
	return result, err
}
