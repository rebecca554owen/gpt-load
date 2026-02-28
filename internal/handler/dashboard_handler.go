package handler

import (
	"crypto/subtle"
	"fmt"
	"sort"
	"strings"
	"time"

	"gpt-load/internal/encryption"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/i18n"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"gpt-load/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	rpmCalculationWindowMinutes = 10
	rpmComparisonWindowMinutes  = 20

	// Time interval constants for chart granularity
	interval10Min  = 10  // 1 hour range
	interval15Min  = 15  // 1-5 hours range
	interval30Min  = 30  // 5-24 hours range
	interval2Hour  = 120 // 1 week range
	interval10Hour = 600 // 1 month range

	// Token speed chart configuration
	topCombosLimit = 7 // Number of top group-model combinations to track
)

// extractAuthKey extracts auth key from various sources
func extractAuthKey(c *gin.Context) string {
	if key := c.Query("key"); key != "" {
		query := c.Request.URL.Query()
		query.Del("key")
		c.Request.URL.RawQuery = query.Encode()
		return key
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		const bearerPrefix = "Bearer "
		if strings.HasPrefix(authHeader, bearerPrefix) {
			return authHeader[len(bearerPrefix):]
		}
	}

	if key := c.GetHeader("X-Api-Key"); key != "" {
		return key
	}

	if key := c.GetHeader("X-Goog-Api-Key"); key != "" {
		return key
	}

	return ""
}

// isAuthenticated checks if the request is authenticated
func (s *Server) isAuthenticated(c *gin.Context) bool {
	key := extractAuthKey(c)
	authConfig := s.config.GetAuthConfig()
	return key != "" && subtle.ConstantTimeCompare([]byte(key), []byte(authConfig.Key)) == 1
}

type timeGranularity int

const (
	granularityMinute timeGranularity = iota
	granularityHour
)

// boolToInt64 converts bool to int64 (1 for true, 0 for false)
func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// buildTimeSelectClause builds database-specific SELECT and GROUP clauses for time-based aggregation
func buildTimeSelectClause(dbType string, granularity timeGranularity, fields string) (selectClause, groupClause string) {
	switch dbType {
	case "mysql":
		if granularity == granularityMinute {
			selectClause = fmt.Sprintf("DATE_FORMAT(timestamp, '%%Y-%%m-%%d %%H:%%i:00') as time_slot, %s", fields)
		} else {
			selectClause = fmt.Sprintf("DATE_FORMAT(timestamp, '%%Y-%%m-%%d %%H:00:00') as time_slot, %s", fields)
		}
	case "postgres":
		if granularity == granularityMinute {
			selectClause = fmt.Sprintf("to_char(DATE_TRUNC('minute', timestamp), 'YYYY-MM-DD HH24:MI:00') as time_slot, %s", fields)
		} else {
			selectClause = fmt.Sprintf("to_char(DATE_TRUNC('hour', timestamp), 'YYYY-MM-DD HH24:00:00') as time_slot, %s", fields)
		}
	default: // sqlite and others
		if granularity == granularityMinute {
			selectClause = fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:%%M:00', timestamp) as time_slot, %s", fields)
		} else {
			selectClause = fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:00:00', timestamp) as time_slot, %s", fields)
		}
	}
	groupClause = "time_slot"
	return selectClause, groupClause
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

// parseHoursParameter converts hours string to hours integer with specified default
// Supports: 1, 5, 24(1 day), 168(1 week), 720(1 month)
func parseHoursParameter(hoursStr string, defaultHours int) int {
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
		return defaultHours
	}
}

type trendResult struct {
	value    float64
	isGrowth bool
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

// trendCard creates a StatCard with trend calculation
func trendCard(current, previous int64) models.StatCard {
	result := calculateTrend(current, previous)
	return models.StatCard{
		Value:         float64(current),
		Trend:         result.value,
		TrendIsGrowth: result.isGrowth,
	}
}

// errorRateCard creates a StatCard for error rate with proper trend direction
func errorRateCard(currentRate, previousRate float64, hasCurrent, hasPrevious bool) models.StatCard {
	result := calculateErrorRateTrend(currentRate, previousRate, hasCurrent, hasPrevious)
	return models.StatCard{
		Value:         currentRate,
		Trend:         result.value,
		TrendIsGrowth: result.isGrowth,
	}
}

// Stats Get dashboard statistics
func (s *Server) Stats(c *gin.Context) {
	// Support both 'hours' and 'days' parameters for backward compatibility
	// Priority: hours > days
	hoursStr := c.Query("hours")
	var hours int
	if hoursStr != "" {
		hours = parseHoursParameter(hoursStr, 1)
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

	// Calculate error rates
	currentErrorRate := 0.0
	if currentPeriod.TotalRequests > 0 {
		currentErrorRate = (float64(currentPeriod.TotalFailures) / float64(currentPeriod.TotalRequests)) * 100
	}
	previousErrorRate := 0.0
	if previousPeriod.TotalRequests > 0 {
		previousErrorRate = (float64(previousPeriod.TotalFailures) / float64(previousPeriod.TotalRequests)) * 100
	}

	// Calculate non-cached prompt tokens
	currentNonCachedPrompt := currentTokenStats.PromptTokens - currentTokenStats.CachedTokens
	if currentNonCachedPrompt < 0 {
		currentNonCachedPrompt = 0
	}
	previousNonCachedPrompt := previousTokenStats.PromptTokens - previousTokenStats.CachedTokens
	if previousNonCachedPrompt < 0 {
		previousNonCachedPrompt = 0
	}

	// Get security warning information (only for authenticated users)
	var securityWarnings []models.SecurityWarning
	if s.isAuthenticated(c) {
		securityWarnings = s.getSecurityWarnings(c)
	}

	stats := models.DashboardStatsResponse{
		KeyCount:              trendCard(currentKeyStats.TotalKeys, previousKeyStats.TotalKeys),
		TokenConsumption:      trendCard(currentTokenStats.TotalTokens, previousTokenStats.TotalTokens),
		PromptTokens:          trendCard(currentTokenStats.PromptTokens, previousTokenStats.PromptTokens),
		NonCachedPromptTokens: trendCard(currentNonCachedPrompt, previousNonCachedPrompt),
		CachedTokens:          trendCard(currentTokenStats.CachedTokens, previousTokenStats.CachedTokens),
		CompletionTokens:      trendCard(currentTokenStats.CompletionTokens, previousTokenStats.CompletionTokens),
		TotalTokens:           trendCard(currentTokenStats.TotalTokens, previousTokenStats.TotalTokens),
		RPM:                   rpmStats,
		RequestCount:          trendCard(currentPeriod.TotalRequests, previousPeriod.TotalRequests),
		ErrorRate:             errorRateCard(currentErrorRate, previousErrorRate, currentPeriod.TotalRequests > 0, previousPeriod.TotalRequests > 0),
		SecurityWarnings:      securityWarnings,
	}

	response.Success(c, stats)
}

// Chart Get dashboard chart data
func (s *Server) Chart(c *gin.Context) {
	viewType := c.DefaultQuery("view", "request")
	hours := parseHoursParameter(c.DefaultQuery("hours", "5"), 5)

	now := time.Now()
	endTime := now
	startTime := now.Add(-time.Duration(hours) * time.Hour)

	if viewType == "token" {
		// Token view - get token statistics from request_logs
		s.getTokenChart(c, startTime, endTime)
	} else if viewType == "token_speed" {
		// Token speed view - get token speed statistics from request_logs
		s.getTokenSpeedChart(c, startTime, endTime)
	} else {
		// Request view - get request statistics from group_hourly_stats
		s.getRequestChart(c, startTime, endTime)
	}
}

// getRequestChart returns request statistics chart data with dynamic granularity
func (s *Server) getRequestChart(c *gin.Context, startTime, endTime time.Time) {
	totalHours := int(endTime.Sub(startTime).Hours())
	if totalHours < 1 {
		totalHours = 1
	}

	var intervalMinutes int
	switch {
	case totalHours <= 1:
		intervalMinutes = interval10Min
	case totalHours <= 5:
		intervalMinutes = interval15Min
	case totalHours <= 24:
		intervalMinutes = interval30Min
	case totalHours <= 168:
		intervalMinutes = interval2Hour
	default:
		intervalMinutes = interval10Hour
	}

	var labels []string
	var successData, failureData []int64

	granularity := granularityMinute
	if intervalMinutes >= 60 {
		granularity = granularityHour
	}

	type requestResult struct {
		TimeSlot string
		Success  int64
		Failure  int64
	}
	var results []requestResult

	dbType := s.DB.Dialector.Name()
	fields := "SUM(CASE WHEN is_success = 1 THEN 1 ELSE 0 END) as success, SUM(CASE WHEN is_success = 0 THEN 1 ELSE 0 END) as failure"
	selectClause, groupClause := buildTimeSelectClause(dbType, granularity, fields)

	err := s.DB.Model(&models.RequestLog{}).
		Select(selectClause).
		Where("timestamp >= ? AND timestamp < ? AND request_type = ?", startTime, endTime, models.RequestTypeFinal).
		Where("group_id NOT IN (?)",
			s.DB.Table("groups").Select("id").Where("group_type = ?", "aggregate")).
		Group(groupClause).
		Order("time_slot asc").
		Scan(&results).Error

	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.chart_data_failed")
		return
	}

	statsByTime := make(map[time.Time]requestResult)
	for _, result := range results {
		slotTime, _ := time.ParseInLocation("2006-01-02 15:04:05", result.TimeSlot, time.Local)
		if granularity == granularityMinute {
			slotTime = slotTime.Truncate(time.Minute)
		} else {
			slotTime = slotTime.Truncate(time.Hour)
		}
		statsByTime[slotTime] = result
	}

	if s.RequestLogService != nil {
		pendingLogs, err := s.RequestLogService.GetPendingLogs()
		if err != nil {
			logrus.Warnf("Failed to get pending logs for real-time stats: %v", err)
		} else {
			aggregateGroupIDs, err := s.getAggregateGroupIDs()
			if err != nil {
				logrus.Warnf("Failed to get aggregate group IDs: %v", err)
			}

			for _, log := range pendingLogs {
				if log.Timestamp.Before(startTime) || log.Timestamp.After(endTime) {
					continue
				}

				if aggregateGroupIDs != nil {
					if _, isAggregate := aggregateGroupIDs[log.GroupID]; isAggregate {
						continue
					}
				}

				var slotTime time.Time
				if granularity == granularityMinute {
					slotTime = log.Timestamp.Truncate(time.Minute)
				} else {
					slotTime = log.Timestamp.Truncate(time.Hour)
				}

				if existing, ok := statsByTime[slotTime]; ok {
					if log.IsSuccess {
						existing.Success++
					} else {
						existing.Failure++
					}
					statsByTime[slotTime] = existing
				} else {
					statsByTime[slotTime] = requestResult{
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
	lastSlotEnd := startTime.Truncate(time.Minute).Add(time.Duration(intervals*intervalMinutes) * time.Minute)
	if lastSlotEnd.Before(endTime) {
		intervals++
	}

	startSlot := startTime.Truncate(time.Minute)
	for i := 0; i < intervals; i++ {
		slotStart := startSlot.Add(time.Duration(i*intervalMinutes) * time.Minute)
		slotEnd := slotStart.Add(time.Duration(intervalMinutes) * time.Minute)
		labels = append(labels, slotStart.Format(time.RFC3339))

		var successSum, failureSum int64
		var step time.Duration
		if granularity == granularityMinute {
			step = time.Minute
		} else {
			step = time.Hour
		}

		for t := slotStart; t.Before(slotEnd) && t.Before(endTime); t = t.Add(step) {
			var lookupTime time.Time
			if granularity == granularityMinute {
				lookupTime = t
			} else {
				lookupTime = t.Truncate(time.Hour)
			}
			if data, ok := statsByTime[lookupTime]; ok {
				successSum += data.Success
				failureSum += data.Failure
			}
		}
		successData = append(successData, successSum)
		failureData = append(failureData, failureSum)
	}

	chartData := models.ChartData{
		Labels: labels,
		Datasets: []models.ChartDataset{
			{
				Label:    i18n.Message(c, "dashboard.successRequests"),
				LabelKey: "dashboard.successRequests",
				Data:     successData,
			},
			{
				Label:    i18n.Message(c, "dashboard.failedRequests"),
				LabelKey: "dashboard.failedRequests",
				Data:     failureData,
			},
		},
	}

	response.Success(c, chartData)
}

// getTokenChart returns token statistics chart data with dynamic granularity
func (s *Server) getTokenChart(c *gin.Context, startTime, endTime time.Time) {
	totalHours := int(endTime.Sub(startTime).Hours())
	if totalHours < 1 {
		totalHours = 1
	}

	var intervalMinutes int
	switch {
	case totalHours <= 1:
		intervalMinutes = interval10Min
	case totalHours <= 5:
		intervalMinutes = interval15Min
	case totalHours <= 24:
		intervalMinutes = interval30Min
	case totalHours <= 168:
		intervalMinutes = interval2Hour
	default:
		intervalMinutes = interval10Hour
	}

	type tokenResult struct {
		TimeSlot         string
		PromptTokens     int64
		CompletionTokens int64
		TotalTokens      int64
		CachedTokens     int64
	}

	var results []tokenResult

	dbType := s.DB.Dialector.Name()
	fields := "COALESCE(SUM(prompt_tokens), 0) as prompt_tokens, COALESCE(SUM(completion_tokens), 0) as completion_tokens, COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cached_tokens), 0) as cached_tokens"
	selectClause, groupClause := buildTimeSelectClause(dbType, granularityMinute, fields)

	err := s.DB.Model(&models.RequestLog{}).
		Select(selectClause).
		Where("timestamp >= ? AND timestamp < ? AND request_type = ?", startTime, endTime, models.RequestTypeFinal).
		Group(groupClause).
		Order("time_slot asc").
		Scan(&results).Error

	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.chart_data_failed")
		return
	}

	statsBySlot := make(map[time.Time]tokenResult)
	for _, result := range results {
		slotTime, _ := time.ParseInLocation("2006-01-02 15:04:05", result.TimeSlot, time.Local)
		slotTime = slotTime.Truncate(time.Minute)
		statsBySlot[slotTime] = result
	}

	if s.RequestLogService != nil {
		pendingLogs, err := s.RequestLogService.GetPendingLogs()
		if err != nil {
			logrus.Warnf("Failed to get pending logs for real-time stats: %v", err)
		} else {
			aggregateGroupIDs, err := s.getAggregateGroupIDs()
			if err != nil {
				logrus.Warnf("Failed to get aggregate group IDs: %v", err)
			}

			pendingBySlot := make(map[time.Time]*tokenResult)
			for _, log := range pendingLogs {
				if log.Timestamp.Before(startTime) || log.Timestamp.After(endTime) || log.RequestType != models.RequestTypeFinal {
					continue
				}

				if aggregateGroupIDs != nil {
					if _, isAggregate := aggregateGroupIDs[log.GroupID]; isAggregate {
						continue
					}
				}

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

			for slotTime, pendingData := range pendingBySlot {
				if existing, ok := statsBySlot[slotTime]; ok {
					existing.PromptTokens += pendingData.PromptTokens
					existing.CompletionTokens += pendingData.CompletionTokens
					existing.TotalTokens += pendingData.TotalTokens
					existing.CachedTokens += pendingData.CachedTokens
					statsBySlot[slotTime] = existing
				} else {
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
	lastSlotEnd := startTime.Truncate(time.Minute).Add(time.Duration(intervals*intervalMinutes) * time.Minute)
	if lastSlotEnd.Before(endTime) {
		intervals++
	}

	startSlot := startTime.Truncate(time.Minute)
	for i := 0; i < intervals; i++ {
		slotStart := startSlot.Add(time.Duration(i*intervalMinutes) * time.Minute)
		slotEnd := slotStart.Add(time.Duration(intervalMinutes) * time.Minute)
		labels = append(labels, slotStart.Format(time.RFC3339))

		var promptSum, cachedSum, outputSum, totalSum int64

		for t := slotStart; t.Before(slotEnd) && t.Before(endTime); t = t.Add(time.Minute) {
			if data, ok := statsBySlot[t]; ok {
				promptSum += data.PromptTokens
				cachedSum += data.CachedTokens
				outputSum += data.CompletionTokens
				totalSum += data.TotalTokens
			}
		}

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
				Label:    i18n.Message(c, "dashboard.nonCachedPromptTokens"),
				LabelKey: "dashboard.nonCachedPromptTokens",
				Data:     nonCachedPromptData,
			},
			{
				Label:    i18n.Message(c, "dashboard.cachedTokens"),
				LabelKey: "dashboard.cachedTokens",
				Data:     cachedData,
			},
			{
				Label:    i18n.Message(c, "dashboard.completionTokens"),
				LabelKey: "dashboard.completionTokens",
				Data:     outputData,
			},
			{
				Label:    i18n.Message(c, "dashboard.totalTokens"),
				LabelKey: "dashboard.totalTokens",
				Data:     totalData,
			},
		},
	}

	response.Success(c, chartData)
}

// getTokenSpeedChart returns token speed statistics chart data
func (s *Server) getTokenSpeedChart(c *gin.Context, startTime, endTime time.Time) {
	totalHours := int(endTime.Sub(startTime).Hours())
	if totalHours < 1 {
		totalHours = 1
	}

	var intervalMinutes int
	switch {
	case totalHours <= 1:
		intervalMinutes = interval10Min
	case totalHours <= 5:
		intervalMinutes = interval15Min
	case totalHours <= 24:
		intervalMinutes = interval30Min
	case totalHours <= 168:
		intervalMinutes = interval2Hour
	default:
		intervalMinutes = interval10Hour
	}

	// Query with group_id and JOIN groups table to get current group name
	type speedRawData struct {
		GroupID           uint
		GroupName        string
		Model            string
		Timestamp        time.Time
		Duration         int64
		CompletionTokens int64
	}

	var rawData []speedRawData
	err := s.DB.Model(&models.RequestLog{}).
		Select("request_logs.group_id, COALESCE(groups.name, request_logs.group_name) as group_name, request_logs.model, request_logs.timestamp, request_logs.duration, request_logs.completion_tokens").
		Joins("LEFT JOIN `groups` ON groups.id = request_logs.group_id").
		Where("request_logs.timestamp >= ? AND request_logs.timestamp < ?", startTime, endTime).
		Where("request_logs.is_success = ? AND request_logs.request_type = ?", true, models.RequestTypeFinal).
		Where("request_logs.group_id NOT IN (?)",
			s.DB.Table("`groups`").Select("id").Where("group_type = ?", "aggregate")).
		Where("request_logs.duration > 0 AND request_logs.completion_tokens > 0").
		Scan(&rawData).Error
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.chart_data_failed")
		return
	}

	type timeComboKey struct {
		timeSlot time.Time
		comboID  string // internal identifier: groupID_model
	}
	type timeComboData struct {
		durations        []float64
		completionTokens []float64
	}
	dataByTimeCombo := make(map[timeComboKey]*timeComboData)
	comboSet := make(map[string]bool) // internal: groupID_model
	comboDisplayNames := make(map[string]string) // display: groupID_model -> groupName - model

	for _, data := range rawData {
		// Use groupID + model as internal identifier
		comboID := fmt.Sprintf("%d_%s", data.GroupID, data.Model)
		comboSet[comboID] = true

		// Store display name with current group name from JOIN
		comboDisplayName := fmt.Sprintf("%s - %s", data.GroupName, data.Model)
		comboDisplayNames[comboID] = comboDisplayName

		slotTime := data.Timestamp.Truncate(time.Duration(intervalMinutes) * time.Minute)
		key := timeComboKey{slotTime, comboID}

		if _, exists := dataByTimeCombo[key]; !exists {
			dataByTimeCombo[key] = &timeComboData{}
		}

		// Use duration directly (now contains accurate generation time from first/last token)
		if data.Duration > 0 {
			dataByTimeCombo[key].durations = append(dataByTimeCombo[key].durations, float64(data.Duration)/1000)
			dataByTimeCombo[key].completionTokens = append(dataByTimeCombo[key].completionTokens, float64(data.CompletionTokens))
		}
	}

	var labels []string
	totalMinutes := totalHours * 60
	intervals := totalMinutes / intervalMinutes
	if intervals < 1 {
		intervals = 1
	}
	lastSlotEnd := startTime.Truncate(time.Minute).Add(time.Duration(intervals*intervalMinutes) * time.Minute)
	if lastSlotEnd.Before(endTime) {
		intervals++
	}

	startSlot := startTime.Truncate(time.Minute)
	for i := 0; i < intervals; i++ {
		slotStart := startSlot.Add(time.Duration(i*intervalMinutes) * time.Minute)
		labels = append(labels, slotStart.Format(time.RFC3339))
	}

	comboList := make([]string, 0, len(comboSet))
	for combo := range comboSet {
		comboList = append(comboList, combo)
	}

	datasets := make(map[string][]float64)
	for _, combo := range comboList {
		datasets[combo] = make([]float64, intervals)
	}

	for i := 0; i < intervals; i++ {
		slotStart := startSlot.Add(time.Duration(i*intervalMinutes) * time.Minute)
		slotEnd := slotStart.Add(time.Duration(intervalMinutes) * time.Minute)
		for _, combo := range comboList {
			var allSpeeds []float64

			for t := slotStart; t.Before(slotEnd) && t.Before(endTime); t = t.Add(time.Duration(intervalMinutes) * time.Minute) {
				lookupTime := t.Truncate(time.Duration(intervalMinutes) * time.Minute)
				if data, ok := dataByTimeCombo[timeComboKey{lookupTime, combo}]; ok {
					for j := range data.durations {
						seconds := data.durations[j]
						tokens := data.completionTokens[j]
						if seconds > 0 {
							speed := tokens / seconds
							allSpeeds = append(allSpeeds, speed)
						}
					}
				}
			}

			// Calculate P90 speed (90th percentile)
			// P90 = 90% of requests can achieve this speed, excluding slowest 10%
			p90Speed := 0.0
			if len(allSpeeds) > 0 {
				sort.Float64s(allSpeeds)
				idx := int(float64(len(allSpeeds)) * 0.9)
				if idx >= len(allSpeeds) {
					idx = len(allSpeeds) - 1
				}
				p90Speed = allSpeeds[idx]
			}
			datasets[combo][i] = p90Speed
		}
	}

	// Calculate P90 speed for each combo across all time intervals
	comboP90Speed := make(map[string]float64)
	for _, combo := range comboList {
		var intervalSpeeds []float64
		for _, val := range datasets[combo] {
			if val > 0 {
				intervalSpeeds = append(intervalSpeeds, val)
			}
		}
		if len(intervalSpeeds) > 0 {
			sort.Float64s(intervalSpeeds)
			idx := int(float64(len(intervalSpeeds)) * 0.9)
			if idx >= len(intervalSpeeds) {
				idx = len(intervalSpeeds) - 1
			}
			comboP90Speed[combo] = intervalSpeeds[idx]
		}
	}

	// Sort combos by P90 speed (descending)
	sort.Slice(comboList, func(i, j int) bool {
		return comboP90Speed[comboList[i]] > comboP90Speed[comboList[j]]
	})

	// Select top 7 combos by P90 speed
	topCount := topCombosLimit
	if len(comboList) < topCount {
		topCount = len(comboList)
	}
	topComboList := comboList[:topCount]

	var chartDatasets []models.ChartDataset
	for _, combo := range topComboList {
		data := make([]int64, intervals)
		for i, val := range datasets[combo] {
			data[i] = int64(val)
		}
		// Use display name for chart label
		displayName := comboDisplayNames[combo]
		chartDatasets = append(chartDatasets, models.ChartDataset{
			Label:    displayName,
			LabelKey: "token_speed." + combo,
			Data:     data,
		})
	}

	// If no datasets, create an empty dataset to ensure chart renders with axes
	if len(chartDatasets) == 0 {
		emptyData := make([]int64, intervals)
		chartDatasets = append(chartDatasets, models.ChartDataset{
			Label:    "",
			LabelKey: "",
			Data:     emptyData,
		})
	}

	chartData := models.ChartData{
		Labels:   labels,
		Datasets: chartDatasets,
	}

	response.Success(c, chartData)
}

type hourlyStatResult struct {
	TotalRequests int64
	TotalFailures int64
}

func (s *Server) getHourlyStats(startTime, endTime time.Time) (hourlyStatResult, error) {
	var result hourlyStatResult

	// Query directly from request_logs table for real-time data (same as token stats)
	// Only count final requests, not retry requests
	err := s.DB.Model(&models.RequestLog{}).
		Where("timestamp >= ? AND timestamp < ? AND request_type = ?", startTime, endTime, models.RequestTypeFinal).
		Where("group_id NOT IN (?)",
			s.DB.Table("groups").Select("id").Where("group_type = ?", "aggregate")).
		Select("COUNT(*) as total_requests, SUM(CASE WHEN is_success = 0 THEN 1 ELSE 0 END) as total_failures").
		Scan(&result).Error

	return result, err
}

// getAggregateGroupIDs returns a set of aggregate group IDs for fast lookup
func (s *Server) getAggregateGroupIDs() (map[uint]struct{}, error) {
	var groupIDs []uint
	err := s.DB.Table("groups").
		Where("group_type = ?", "aggregate").
		Pluck("id", &groupIDs).Error

	if err != nil {
		return nil, err
	}

	aggregateSet := make(map[uint]struct{})
	for _, id := range groupIDs {
		aggregateSet[id] = struct{}{}
	}

	return aggregateSet, nil
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
	// Only return encryption status to authenticated users
	if !s.isAuthenticated(c) {
		response.Success(c, gin.H{
			"has_mismatch":  false,
			"scenario_type": "",
			"message":       "",
			"suggestion":    "",
		})
		return
	}

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
