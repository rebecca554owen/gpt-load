package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"gpt-load/internal/channel"
	"gpt-load/internal/config"
	"gpt-load/internal/encryption"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// I18nError 表示携带翻译元数据的错误。
type I18nError struct {
	APIError  *app_errors.APIError
	MessageID string
	Template  map[string]any
}

// Error 实现 error 接口。
func (e *I18nError) Error() string {
	if e == nil || e.APIError == nil {
		return ""
	}
	return e.APIError.Error()
}

// NewI18nError 是创建支持 i18n 的错误的辅助函数。
func NewI18nError(apiErr *app_errors.APIError, msgID string, template map[string]any) *I18nError {
	return &I18nError{
		APIError:  apiErr,
		MessageID: msgID,
		Template:  template,
	}
}

// GroupService 处理组操作的业务逻辑。
type GroupService struct {
	db                    *gorm.DB
	settingsManager       *config.SystemSettingsManager
	groupManager          *GroupManager
	keyService            *KeyService
	keyImportSvc          *KeyImportService
	encryptionSvc         encryption.Service
	aggregateGroupService *AggregateGroupService
	channelRegistry       []string
}

// NewGroupService 构造一个 GroupService。
func NewGroupService(
	db *gorm.DB,
	settingsManager *config.SystemSettingsManager,
	groupManager *GroupManager,
	keyService *KeyService,
	keyImportSvc *KeyImportService,
	encryptionSvc encryption.Service,
	aggregateGroupService *AggregateGroupService,
) *GroupService {
	return &GroupService{
		db:                    db,
		settingsManager:       settingsManager,
		groupManager:          groupManager,
		keyService:            keyService,
		keyImportSvc:          keyImportSvc,
		encryptionSvc:         encryptionSvc,
		aggregateGroupService: aggregateGroupService,
		channelRegistry:       channel.GetChannels(),
	}
}

// GroupCreateParams 捕获创建组所需的所有字段。
type GroupCreateParams struct {
	Name                string
	DisplayName         string
	Description         string
	GroupType           string
	Upstreams           json.RawMessage
	ChannelType         string
	Sort                int
	TestModel           string
	ValidationEndpoint  string
	ParamOverrides      map[string]any
	ModelRedirectRules  map[string]string
	ModelRedirectStrict bool
	Config              map[string]any
	HeaderRules         []models.HeaderRule
	ProxyKeys           string
	SubGroups           []SubGroupInput
	ModelMappings       json.RawMessage
}

// GroupUpdateParams 捕获组的可更新字段。
type GroupUpdateParams struct {
	Name                *string
	DisplayName         *string
	Description         *string
	GroupType           *string
	Upstreams           json.RawMessage
	HasUpstreams        bool
	ChannelType         *string
	Sort                *int
	TestModel           string
	HasTestModel        bool
	ValidationEndpoint  *string
	ParamOverrides      map[string]any
	ModelRedirectRules  map[string]string
	ModelRedirectStrict *bool
	ModelMappingStrict  *bool
	Config              map[string]any
	HeaderRules         *[]models.HeaderRule
	ProxyKeys           *string
	SubGroups           *[]SubGroupInput
	ModelMappings       json.RawMessage
	HasModelMappings    bool
}

// ModelMappingTargetInput 捕获模型别名映射中的单个目标组。
type ModelMappingTargetInput struct {
	SubGroupID uint   `json:"sub_group_id"`
	Weight     int    `json:"weight"`
	Model      string `json:"model"`
}

// ModelMappingInput 捕获模型别名与其候选子组之间的映射。
type ModelMappingInput struct {
	Model   string                    `json:"model"`
	Targets []ModelMappingTargetInput `json:"targets"`
}

// KeyStats 捕获组的聚合 API 密钥统计信息。
type KeyStats struct {
	TotalKeys   int64 `json:"total_keys"`
	ActiveKeys  int64 `json:"active_keys"`
	InvalidKeys int64 `json:"invalid_keys"`
}

// RequestStats 捕获时间窗口内的请求成功和失败比率。
type RequestStats struct {
	TotalRequests  int64   `json:"total_requests"`
	FailedRequests int64   `json:"failed_requests"`
	FailureRate    float64 `json:"failure_rate"`
}

// GroupStats 聚合仪表盘使用的所有按组指标。
type GroupStats struct {
	KeyStats    KeyStats     `json:"key_stats"`
	Stats24Hour RequestStats `json:"stats_24_hour"`
	Stats7Day   RequestStats `json:"stats_7_day"`
	Stats30Day  RequestStats `json:"stats_30_day"`
}

// ConfigOption 描述向客户端公开的可配置覆盖项。
type ConfigOption struct {
	Key          string
	Name         string
	Description  string
	DefaultValue any
}

// CreateGroup 验证并持久化新组。
func (s *GroupService) CreateGroup(ctx context.Context, params GroupCreateParams) (*models.Group, error) {
	name := strings.TrimSpace(params.Name)
	if !isValidGroupName(name) {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_group_name", nil)
	}

	channelType := strings.TrimSpace(params.ChannelType)
	if !s.isValidChannelType(channelType) {
		supported := strings.Join(s.channelRegistry, ", ")
		return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_channel_type", map[string]any{"types": supported})
	}

	groupType := strings.TrimSpace(params.GroupType)
	if groupType == "" {
		groupType = "standard"
	}
	if groupType != "standard" && groupType != "aggregate" {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_group_type", nil)
	}

	var cleanedUpstreams datatypes.JSON
	var testModel string
	var validationEndpoint string

	switch groupType {
	case "aggregate":
		validationEndpoint = ""
		cleanedUpstreams = datatypes.JSON("[]")
		testModel = "-"
	case "standard":
		testModel = strings.TrimSpace(params.TestModel)
		if testModel == "" {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.test_model_required", nil)
		}
		cleaned, err := s.validateAndCleanUpstreams(params.Upstreams)
		if err != nil {
			return nil, err
		}
		cleanedUpstreams = cleaned

		validationEndpoint = strings.TrimSpace(params.ValidationEndpoint)
		if !isValidValidationEndpoint(validationEndpoint) {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_test_path", nil)
		}
	}

	cleanedConfig, err := s.validateAndCleanConfig(params.Config)
	if err != nil {
		return nil, err
	}

	headerRulesJSON, err := s.normalizeHeaderRules(params.HeaderRules)
	if err != nil {
		return nil, err
	}
	if headerRulesJSON == nil {
		headerRulesJSON = datatypes.JSON("[]")
	}

	// Validate model redirect rules for aggregate groups
	if groupType == "aggregate" && len(params.ModelRedirectRules) > 0 {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.aggregate_no_model_redirect", nil)
	}

	// Validate model redirect rules format
	if err := validateModelRedirectRules(params.ModelRedirectRules); err != nil {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_model_redirect", map[string]any{"error": err.Error()})
	}

	group := models.Group{
		Name:                name,
		DisplayName:         strings.TrimSpace(params.DisplayName),
		Description:         strings.TrimSpace(params.Description),
		GroupType:           groupType,
		Upstreams:           cleanedUpstreams,
		ChannelType:         channelType,
		Sort:                params.Sort,
		TestModel:           testModel,
		ValidationEndpoint:  validationEndpoint,
		ParamOverrides:      params.ParamOverrides,
		ModelRedirectRules:  convertToJSONMap(params.ModelRedirectRules),
		ModelRedirectStrict: params.ModelRedirectStrict,
		Config:              cleanedConfig,
		HeaderRules:         headerRulesJSON,
		ProxyKeys:           strings.TrimSpace(params.ProxyKeys),
	}

	modelMappings, err := s.processModelMappings(ctx, 0, groupType, params.ModelMappings, nil)
	if err != nil {
		return nil, err
	}
	group.ModelMappings = modelMappings

	tx := s.db.WithContext(ctx).Begin()
	if err := tx.Error; err != nil {
		return nil, app_errors.ErrDatabase
	}

	if err := tx.Create(&group).Error; err != nil {
		tx.Rollback()
		return nil, app_errors.ParseDBError(err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	if err := s.groupManager.Invalidate(); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("failed to invalidate group cache")
	}

	return &group, nil
}

// ListGroups 返回所有组，不包括子组关系。
func (s *GroupService) ListGroups(ctx context.Context) ([]models.Group, error) {
	var groups []models.Group
	if err := s.db.WithContext(ctx).Order("sort asc, id desc").Find(&groups).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	// Parse model mappings JSON for each group
	for i := range groups {
		if len(groups[i].ModelMappings) > 0 {
			if err := json.Unmarshal(groups[i].ModelMappings, &groups[i].ModelMappingList); err != nil {
				logrus.WithError(err).WithField("group_name", groups[i].Name).Warn("Failed to parse model mappings for group")
				groups[i].ModelMappingList = nil
			}
		} else {
			groups[i].ModelMappingList = nil
		}
	}

	return groups, nil
}

// UpdateGroup 验证并更新现有组。
func (s *GroupService) UpdateGroup(ctx context.Context, id uint, params GroupUpdateParams) (*models.Group, error) {
	var group models.Group
	if err := s.db.WithContext(ctx).First(&group, id).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	tx := s.db.WithContext(ctx).Begin()
	if err := tx.Error; err != nil {
		return nil, app_errors.ErrDatabase
	}
	defer tx.Rollback()

	if err := s.validateAndUpdateGroupFields(ctx, &group, params, tx); err != nil {
		return nil, err
	}

	if err := tx.Save(&group).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, app_errors.ErrDatabase
	}

	if err := s.groupManager.Invalidate(); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("failed to invalidate group cache")
	}

	return &group, nil
}

// validateAndUpdateGroupFields 根据提供的参数验证并更新组字段。
func (s *GroupService) validateAndUpdateGroupFields(ctx context.Context, group *models.Group, params GroupUpdateParams, tx *gorm.DB) error {
	if params.Name != nil {
		cleanedName := strings.TrimSpace(*params.Name)
		if !isValidGroupName(cleanedName) {
			return NewI18nError(app_errors.ErrValidation, "validation.invalid_group_name", nil)
		}
		group.Name = cleanedName
	}

	if params.DisplayName != nil {
		group.DisplayName = strings.TrimSpace(*params.DisplayName)
	}

	if params.Description != nil {
		group.Description = strings.TrimSpace(*params.Description)
	}

	if params.HasUpstreams {
		cleanedUpstreams, err := s.validateAndCleanUpstreams(params.Upstreams)
		if err != nil {
			return err
		}
		group.Upstreams = cleanedUpstreams
	}

	if err := s.validateChannelTypeUpdate(ctx, group, params, tx); err != nil {
		return err
	}

	if params.Sort != nil {
		group.Sort = *params.Sort
	}

	if params.HasTestModel {
		cleanedTestModel := strings.TrimSpace(params.TestModel)
		if cleanedTestModel == "" {
			return NewI18nError(app_errors.ErrValidation, "validation.test_model_empty", nil)
		}
		group.TestModel = cleanedTestModel
	}

	if params.ParamOverrides != nil {
		group.ParamOverrides = params.ParamOverrides
	}

	if err := s.validateModelRedirectRulesForUpdate(group, params); err != nil {
		return err
	}

	if params.ValidationEndpoint != nil {
		validationEndpoint := strings.TrimSpace(*params.ValidationEndpoint)
		if !isValidValidationEndpoint(validationEndpoint) {
			return NewI18nError(app_errors.ErrValidation, "validation.invalid_test_path", nil)
		}
		group.ValidationEndpoint = validationEndpoint
	}

	if params.Config != nil {
		cleanedConfig, err := s.validateAndCleanConfig(params.Config)
		if err != nil {
			return err
		}
		group.Config = cleanedConfig
	}

	if params.ProxyKeys != nil {
		group.ProxyKeys = strings.TrimSpace(*params.ProxyKeys)
	}

	if params.HeaderRules != nil {
		headerRulesJSON, err := s.normalizeHeaderRules(*params.HeaderRules)
		if err != nil {
			return err
		}
		if headerRulesJSON == nil {
			headerRulesJSON = datatypes.JSON("[]")
		}
		group.HeaderRules = headerRulesJSON
	}

	if params.ModelMappingStrict != nil {
		group.ModelMappingStrict = *params.ModelMappingStrict
	}

	if params.HasModelMappings {
		modelMappings, err := s.processModelMappings(ctx, group.ID, group.GroupType, params.ModelMappings, tx)
		if err != nil {
			return err
		}
		group.ModelMappings = modelMappings
	}

	return nil
}

// validateChannelTypeUpdate 验证非聚合组的通道类型更新。
func (s *GroupService) validateChannelTypeUpdate(ctx context.Context, group *models.Group, params GroupUpdateParams, tx *gorm.DB) error {
	if group.GroupType == "aggregate" || params.ChannelType == nil {
		return nil
	}

	count, err := s.aggregateGroupService.CountAggregateGroupsUsingSubGroupTx(ctx, group.ID, tx)
	if err != nil {
		return err
	}

	cleanedChannelType := strings.TrimSpace(*params.ChannelType)
	if count > 0 && group.ChannelType != cleanedChannelType {
		return NewI18nError(app_errors.ErrValidation, "validation.sub_group_referenced_cannot_modify",
			map[string]any{"count": count})
	}

	if !s.isValidChannelType(cleanedChannelType) {
		supported := strings.Join(s.channelRegistry, ", ")
		return NewI18nError(app_errors.ErrValidation, "validation.invalid_channel_type", map[string]any{"types": supported})
	}

	group.ChannelType = cleanedChannelType
	return nil
}

// validateModelRedirectRulesForUpdate 在组更新期间验证模型重定向规则。
func (s *GroupService) validateModelRedirectRulesForUpdate(group *models.Group, params GroupUpdateParams) error {
	if group.GroupType == "aggregate" && params.ModelRedirectRules != nil && len(params.ModelRedirectRules) > 0 {
		return NewI18nError(app_errors.ErrValidation, "validation.aggregate_no_model_redirect", nil)
	}

	if params.ModelRedirectRules != nil {
		if err := validateModelRedirectRules(params.ModelRedirectRules); err != nil {
			return NewI18nError(app_errors.ErrValidation, "validation.invalid_model_redirect", map[string]any{"error": err.Error()})
		}
		group.ModelRedirectRules = convertToJSONMap(params.ModelRedirectRules)
	}

	if params.ModelRedirectStrict != nil {
		group.ModelRedirectStrict = *params.ModelRedirectStrict
	}

	return nil
}

// DeleteGroup 删除组及关联资源。
func (s *GroupService) DeleteGroup(ctx context.Context, id uint) (err error) {
	var apiKeys []models.APIKey
	if err := s.db.WithContext(ctx).Where("group_id = ?", id).Find(&apiKeys).Error; err != nil {
		return app_errors.ParseDBError(err)
	}

	keyIDs := make([]uint, 0, len(apiKeys))
	for _, key := range apiKeys {
		keyIDs = append(keyIDs, key.ID)
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return app_errors.ErrDatabase
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	var group models.Group
	if err := tx.First(&group, id).Error; err != nil {
		return app_errors.ParseDBError(err)
	}

	if err := tx.Where("group_id = ? OR sub_group_id = ?", id, id).Delete(&models.GroupSubGroup{}).Error; err != nil {
		return app_errors.ParseDBError(err)
	}

	if err := tx.Where("group_id = ?", id).Delete(&models.APIKey{}).Error; err != nil {
		return app_errors.ErrDatabase
	}

	if err := tx.Delete(&models.Group{}, id).Error; err != nil {
		return app_errors.ParseDBError(err)
	}

	if len(keyIDs) > 0 {
		if err := s.keyService.KeyProvider.RemoveKeysFromStore(id, keyIDs); err != nil {
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"groupID":  id,
				"keyCount": len(keyIDs),
			}).WithError(err).Error("failed to remove keys from memory store, rolling back transaction")
			return NewI18nError(app_errors.ErrDatabase, "error.delete_group_cache", nil)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return app_errors.ErrDatabase
	}

	if err := s.groupManager.Invalidate(); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("failed to invalidate group cache")
	}

	return nil
}

// CopyGroup 复制组，可选择复制活动密钥。
func (s *GroupService) CopyGroup(ctx context.Context, sourceGroupID uint, copyKeysOption string) (*models.Group, error) {
	option := strings.TrimSpace(copyKeysOption)
	if option == "" {
		option = "all"
	}
	if option != "none" && option != "valid_only" && option != "all" {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_copy_keys_value", nil)
	}

	var sourceGroup models.Group
	if err := s.db.WithContext(ctx).First(&sourceGroup, sourceGroupID).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	tx := s.db.WithContext(ctx).Begin()
	if err := tx.Error; err != nil {
		return nil, app_errors.ErrDatabase
	}
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()

	newGroup := sourceGroup
	newGroup.ID = 0
	newGroup.Name = s.generateUniqueGroupName(ctx, sourceGroup.Name)
	if sourceGroup.DisplayName != "" {
		newGroup.DisplayName = sourceGroup.DisplayName + " Copy"
	}
	newGroup.CreatedAt = time.Time{}
	newGroup.UpdatedAt = time.Time{}
	newGroup.LastValidatedAt = nil

	if err := tx.Create(&newGroup).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	var sourceKeyValues []string
	if option != "none" {
		var sourceKeys []models.APIKey
		query := tx.Where("group_id = ?", sourceGroupID)
		if option == "valid_only" {
			query = query.Where("status = ?", models.KeyStatusActive)
		}
		if err := query.Find(&sourceKeys).Error; err != nil {
			return nil, app_errors.ParseDBError(err)
		}

		for _, sourceKey := range sourceKeys {
			decryptedKey, err := s.encryptionSvc.Decrypt(sourceKey.KeyValue)
			if err != nil {
				logrus.WithContext(ctx).WithError(err).WithField("key_id", sourceKey.ID).Error("failed to decrypt key during group copy, skipping")
				continue
			}
			sourceKeyValues = append(sourceKeyValues, decryptedKey)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, app_errors.ErrDatabase
	}
	tx = nil

	if err := s.groupManager.Invalidate(); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("failed to invalidate group cache")
	}

	if len(sourceKeyValues) > 0 {
		keysText := strings.Join(sourceKeyValues, "\n")
		if _, err := s.keyImportSvc.StartImportTask(&newGroup, keysText); err != nil {
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"groupId":  newGroup.ID,
				"keyCount": len(sourceKeyValues),
			}).WithError(err).Error("failed to start async key import task for group copy")
		} else {
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"groupId":  newGroup.ID,
				"keyCount": len(sourceKeyValues),
			}).Info("started async key import task for group copy")
		}
	}

	return &newGroup, nil
}

// GetGroupStats 返回组的聚合使用统计信息。
func (s *GroupService) GetGroupStats(ctx context.Context, groupID uint) (*GroupStats, error) {
	var group models.Group
	if err := s.db.WithContext(ctx).First(&group, groupID).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	// Select different statistics logic based on group type
	if group.GroupType == "aggregate" {
		return s.getAggregateGroupStats(ctx, groupID)
	}

	return s.getStandardGroupStats(ctx, groupID)
}

// queryGroupHourlyStats 从 group_hourly_stats 表查询聚合的每小时统计信息
func (s *GroupService) queryGroupHourlyStats(ctx context.Context, groupID uint, hours int) (RequestStats, error) {
	var result struct {
		SuccessCount int64
		FailureCount int64
	}

	now := time.Now()
	currentHour := now.Truncate(time.Hour)
	endTime := currentHour.Add(time.Hour) // Include current hour
	startTime := endTime.Add(-time.Duration(hours) * time.Hour)

	if err := s.db.WithContext(ctx).Model(&models.GroupHourlyStat{}).
		Select("SUM(success_count) as success_count, SUM(failure_count) as failure_count").
		Where("group_id = ? AND time >= ? AND time < ?", groupID, startTime, endTime).
		Scan(&result).Error; err != nil {
		return RequestStats{}, err
	}

	return calculateRequestStats(result.SuccessCount+result.FailureCount, result.FailureCount), nil
}

// fetchKeyStats 获取组的 API 密钥统计信息
func (s *GroupService) fetchKeyStats(ctx context.Context, groupID uint) (KeyStats, error) {
	var totalKeys, activeKeys int64

	if err := s.db.WithContext(ctx).Model(&models.APIKey{}).
		Where("group_id = ?", groupID).
		Count(&totalKeys).Error; err != nil {
		return KeyStats{}, fmt.Errorf("failed to get total keys: %w", err)
	}

	if err := s.db.WithContext(ctx).Model(&models.APIKey{}).
		Where("group_id = ? AND status = ?", groupID, models.KeyStatusActive).
		Count(&activeKeys).Error; err != nil {
		return KeyStats{}, fmt.Errorf("failed to get active keys: %w", err)
	}

	return KeyStats{
		TotalKeys:   totalKeys,
		ActiveKeys:  activeKeys,
		InvalidKeys: totalKeys - activeKeys,
	}, nil
}

// fetchRequestStats 获取多个时间段的请求统计信息
func (s *GroupService) fetchRequestStats(ctx context.Context, groupID uint, stats *GroupStats) []error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	// Define time periods and their corresponding setters
	timePeriods := []struct {
		hours  int
		name   string
		setter func(RequestStats)
	}{
		{24, "24-hour", func(r RequestStats) { stats.Stats24Hour = r }},
		{7 * 24, "7-day", func(r RequestStats) { stats.Stats7Day = r }},
		{30 * 24, "30-day", func(r RequestStats) { stats.Stats30Day = r }},
	}

	// Fetch statistics for each time period concurrently
	for _, period := range timePeriods {
		wg.Add(1)
		go func(hours int, name string, setter func(RequestStats)) {
			defer wg.Done()

			res, err := s.queryGroupHourlyStats(ctx, groupID, hours)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to get %s stats: %w", name, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			setter(res)
			mu.Unlock()
		}(period.hours, period.name, period.setter)
	}

	wg.Wait()
	return errs
}

func (s *GroupService) getStandardGroupStats(ctx context.Context, groupID uint) (*GroupStats, error) {
	stats := new(GroupStats)
	var allErrors []error

	// Fetch key statistics (only for standard groups)
	keyStats, err := s.fetchKeyStats(ctx, groupID)
	if err != nil {
		allErrors = append(allErrors, err)
		// Log error but continue to fetch request stats
		logrus.WithContext(ctx).WithError(err).Warn("failed to fetch key stats, continuing with request stats")
	} else {
		stats.KeyStats = keyStats
	}

	// Fetch request statistics (common for all groups)
	if errs := s.fetchRequestStats(ctx, groupID, stats); len(errs) > 0 {
		allErrors = append(allErrors, errs...)
	}

	// Handle errors
	if len(allErrors) > 0 {
		logrus.WithContext(ctx).WithError(allErrors[0]).Error("errors occurred while fetching group stats")
		// Return partial stats if we have some data
		if stats.Stats24Hour.TotalRequests > 0 || stats.Stats7Day.TotalRequests > 0 || stats.Stats30Day.TotalRequests > 0 {
			return stats, nil
		}
		return nil, NewI18nError(app_errors.ErrDatabase, "database.group_stats_failed", nil)
	}

	return stats, nil
}

func (s *GroupService) getAggregateGroupStats(ctx context.Context, groupID uint) (*GroupStats, error) {
	stats := new(GroupStats)

	// Aggregate groups only need request statistics, not key statistics
	if errs := s.fetchRequestStats(ctx, groupID, stats); len(errs) > 0 {
		logrus.WithContext(ctx).WithError(errs[0]).Error("errors occurred while fetching aggregate group stats")
		// Return partial stats if we have some data
		if stats.Stats24Hour.TotalRequests > 0 || stats.Stats7Day.TotalRequests > 0 || stats.Stats30Day.TotalRequests > 0 {
			return stats, nil
		}
		return nil, NewI18nError(app_errors.ErrDatabase, "database.group_stats_failed", nil)
	}

	return stats, nil
}

// GetGroupConfigOptions 返回描述可用覆盖项的元数据。
func (s *GroupService) GetGroupConfigOptions() ([]ConfigOption, error) {
	defaultSettings := utils.DefaultSystemSettings()
	settingDefinitions := utils.GenerateSettingsMetadata(&defaultSettings)
	defMap := make(map[string]models.SystemSettingInfo)
	for _, def := range settingDefinitions {
		defMap[def.Key] = def
	}

	currentSettings := s.settingsManager.GetSettings()
	currentSettingsValue := reflect.ValueOf(currentSettings)
	currentSettingsType := currentSettingsValue.Type()
	jsonToFieldMap := make(map[string]string)
	for i := 0; i < currentSettingsType.NumField(); i++ {
		field := currentSettingsType.Field(i)
		jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonTag != "" {
			jsonToFieldMap[jsonTag] = field.Name
		}
	}

	groupConfigType := reflect.TypeOf(models.GroupConfig{})
	var options []ConfigOption
	for i := 0; i < groupConfigType.NumField(); i++ {
		field := groupConfigType.Field(i)
		jsonTag := field.Tag.Get("json")
		key := strings.Split(jsonTag, ",")[0]
		if key == "" || key == "-" {
			continue
		}

		definition, ok := defMap[key]
		if !ok {
			continue
		}

		var defaultValue any
		if fieldName, ok := jsonToFieldMap[key]; ok {
			defaultValue = currentSettingsValue.FieldByName(fieldName).Interface()
		}

		options = append(options, ConfigOption{
			Key:          key,
			Name:         definition.Name,
			Description:  definition.Description,
			DefaultValue: defaultValue,
		})
	}

	return options, nil
}

// validateAndCleanConfig 验证 GroupConfig 覆盖项。
func (s *GroupService) validateAndCleanConfig(configMap map[string]any) (map[string]any, error) {
	if configMap == nil {
		return nil, nil
	}

	var tempGroupConfig models.GroupConfig
	groupConfigType := reflect.TypeOf(tempGroupConfig)
	validFields := make(map[string]bool)
	for i := 0; i < groupConfigType.NumField(); i++ {
		jsonTag := groupConfigType.Field(i).Tag.Get("json")
		fieldName := strings.Split(jsonTag, ",")[0]
		if fieldName != "" && fieldName != "-" {
			validFields[fieldName] = true
		}
	}

	for key := range configMap {
		if !validFields[key] {
			message := fmt.Sprintf("unknown config field: '%s'", key)
			return nil, NewI18nError(app_errors.ErrValidation, "error.invalid_config_format", map[string]any{"error": message})
		}
	}

	if err := s.settingsManager.ValidateGroupConfigOverrides(configMap); err != nil {
		return nil, NewI18nError(app_errors.ErrValidation, "error.invalid_config_format", map[string]any{"error": err.Error()})
	}

	configBytes, err := json.Marshal(configMap)
	if err != nil {
		return nil, NewI18nError(app_errors.ErrValidation, "error.invalid_config_format", map[string]any{"error": err.Error()})
	}

	var validatedConfig models.GroupConfig
	if err := json.Unmarshal(configBytes, &validatedConfig); err != nil {
		return nil, NewI18nError(app_errors.ErrValidation, "error.invalid_config_format", map[string]any{"error": err.Error()})
	}

	validatedBytes, err := json.Marshal(validatedConfig)
	if err != nil {
		return nil, NewI18nError(app_errors.ErrValidation, "error.invalid_config_format", map[string]any{"error": err.Error()})
	}

	var finalMap map[string]any
	if err := json.Unmarshal(validatedBytes, &finalMap); err != nil {
		return nil, NewI18nError(app_errors.ErrValidation, "error.invalid_config_format", map[string]any{"error": err.Error()})
	}

	return finalMap, nil
}

// normalizeHeaderRules 去重并规范化头部规则。
func (s *GroupService) normalizeHeaderRules(rules []models.HeaderRule) (datatypes.JSON, error) {
	if len(rules) == 0 {
		return nil, nil
	}

	normalized := make([]models.HeaderRule, 0, len(rules))
	seenKeys := make(map[string]bool)

	for _, rule := range rules {
		key := strings.TrimSpace(rule.Key)
		if key == "" {
			continue
		}
		canonicalKey := http.CanonicalHeaderKey(key)
		if seenKeys[canonicalKey] {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.duplicate_header", map[string]any{"key": canonicalKey})
		}
		seenKeys[canonicalKey] = true
		normalized = append(normalized, models.HeaderRule{Key: canonicalKey, Value: rule.Value, Action: rule.Action})
	}

	if len(normalized) == 0 {
		return nil, nil
	}

	headerRulesBytes, err := json.Marshal(normalized)
	if err != nil {
		return nil, NewI18nError(app_errors.ErrInternalServer, "error.process_header_rules", map[string]any{"error": err.Error()})
	}

	return datatypes.JSON(headerRulesBytes), nil
}

// validateAndCleanUpstreams 验证上游定义。
func (s *GroupService) validateAndCleanUpstreams(upstreams json.RawMessage) (datatypes.JSON, error) {
	if len(upstreams) == 0 {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_upstreams", map[string]any{"error": "upstreams field is required"})
	}

	var defs []struct {
		URL    string `json:"url"`
		Weight int    `json:"weight"`
	}
	if err := json.Unmarshal(upstreams, &defs); err != nil {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_upstreams", map[string]any{"error": err.Error()})
	}

	if len(defs) == 0 {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_upstreams", map[string]any{"error": "at least one upstream is required"})
	}

	hasActiveUpstream := false
	for i := range defs {
		defs[i].URL = strings.TrimSpace(defs[i].URL)
		if defs[i].URL == "" {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_upstreams", map[string]any{"error": "upstream URL cannot be empty"})
		}
		if !strings.HasPrefix(defs[i].URL, "http://") && !strings.HasPrefix(defs[i].URL, "https://") {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_upstreams", map[string]any{"error": fmt.Sprintf("invalid URL format for upstream: %s", defs[i].URL)})
		}
		if defs[i].Weight < 0 {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_upstreams", map[string]any{"error": "upstream weight must be a non-negative integer"})
		}
		if defs[i].Weight > 0 {
			hasActiveUpstream = true
		}
	}

	if !hasActiveUpstream {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_upstreams", map[string]any{"error": "at least one upstream must have a weight greater than 0"})
	}

	cleanedUpstreams, err := json.Marshal(defs)
	if err != nil {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_upstreams", map[string]any{"error": err.Error()})
	}

	return datatypes.JSON(cleanedUpstreams), nil
}

func calculateRequestStats(total, failed int64) RequestStats {
	stats := RequestStats{
		TotalRequests:  total,
		FailedRequests: failed,
	}
	if total > 0 {
		rate := float64(failed) / float64(total)
		stats.FailureRate = math.Round(rate*10000) / 10000
	}
	return stats
}

func (s *GroupService) generateUniqueGroupName(ctx context.Context, baseName string) string {
	var groups []models.Group
	if err := s.db.WithContext(ctx).Select("name").Find(&groups).Error; err != nil {
		return baseName + "_copy"
	}

	existingNames := make(map[string]bool, len(groups))
	for _, group := range groups {
		existingNames[group.Name] = true
	}

	copyName := baseName + "_copy"
	if !existingNames[copyName] {
		return copyName
	}

	for i := 2; i <= 1000; i++ {
		candidate := fmt.Sprintf("%s_copy_%d", baseName, i)
		if !existingNames[candidate] {
			return candidate
		}
	}

	return copyName
}

// isValidGroupName 验证组名。
func isValidGroupName(name string) bool {
	if name == "" {
		return false
	}
	match, _ := regexp.MatchString("^[a-z0-9_-]{1,100}$", name)
	return match
}

// isValidValidationEndpoint 验证自定义验证端点路径。
func isValidValidationEndpoint(endpoint string) bool {
	if endpoint == "" {
		return true
	}
	if !strings.HasPrefix(endpoint, "/") {
		return false
	}
	if strings.Contains(endpoint, "://") {
		return false
	}
	return true
}

// isValidChannelType 根据已注册的通道检查通道类型。
func (s *GroupService) isValidChannelType(channelType string) bool {
	for _, t := range s.channelRegistry {
		if t == channelType {
			return true
		}
	}
	return false
}

// convertToJSONMap 将 map[string]string 转换为 datatypes.JSONMap
func convertToJSONMap(input map[string]string) datatypes.JSONMap {
	if len(input) == 0 {
		return datatypes.JSONMap{}
	}

	result := make(datatypes.JSONMap)
	for k, v := range input {
		result[k] = v
	}
	return result
}

// subGroupMapBuilder 为聚合组构建有效子组 ID 的映射
func (s *GroupService) subGroupMapBuilder(ctx context.Context, groupID uint, tx *gorm.DB) (map[uint]struct{}, error) {
	var subGroups []models.GroupSubGroup
	db := s.db
	if tx != nil {
		db = tx
	}
	if err := db.WithContext(ctx).Where("group_id = ?", groupID).Find(&subGroups).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	subGroupMap := make(map[uint]struct{}, len(subGroups))
	for _, sg := range subGroups {
		subGroupMap[sg.SubGroupID] = struct{}{}
	}
	return subGroupMap, nil
}

// validateModelAlias 验证模型别名并检查重复
func validateModelAlias(alias string, aliasSet map[string]struct{}) (string, string, error) {
	trimmed := strings.TrimSpace(alias)
	if trimmed == "" {
		return "", "", NewI18nError(app_errors.ErrValidation, "validation.model_mapping_alias_required", nil)
	}

	lowerAlias := strings.ToLower(trimmed)
	if _, exists := aliasSet[lowerAlias]; exists {
		return "", "", NewI18nError(app_errors.ErrValidation, "validation.model_mapping_duplicate_alias", map[string]any{"model": trimmed})
	}

	return trimmed, lowerAlias, nil
}

// validateAndNormalizeTargets 验证并规范化模型映射的所有目标
func (s *GroupService) validateAndNormalizeTargets(
	ctx context.Context,
	groupID uint,
	alias string,
	targets []ModelMappingTargetInput,
	subGroupMap map[uint]struct{},
) ([]models.ModelMappingTarget, error) {
	if len(targets) == 0 {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.model_mapping_targets_required", map[string]any{"model": alias})
	}

	normalizedTargets := make([]models.ModelMappingTarget, 0, len(targets))
	targetSet := make(map[string]struct{}, len(targets))

	for _, target := range targets {
		if _, ok := subGroupMap[target.SubGroupID]; !ok {
			logrus.WithContext(ctx).WithFields(logrus.Fields{
				"group_id":             groupID,
				"model_alias":          alias,
				"invalid_sub_group_id": target.SubGroupID,
			}).Warn("Model mapping target references invalid sub-group, allowing save for manual fix")
		}

		if target.Weight < 0 {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.model_mapping_invalid_weight",
				map[string]any{"model": alias, "sub_group_id": target.SubGroupID})
		}

		modelName := strings.TrimSpace(target.Model)
		if modelName == "" {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.model_mapping_target_model_required",
				map[string]any{"model": alias})
		}

		compositeKey := fmt.Sprintf("%d:%s", target.SubGroupID, strings.ToLower(modelName))
		if _, duplicated := targetSet[compositeKey]; duplicated {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.model_mapping_duplicate_target",
				map[string]any{"model": alias})
		}

		normalizedTargets = append(normalizedTargets, models.ModelMappingTarget{
			SubGroupID: target.SubGroupID,
			Weight:     target.Weight,
			Model:      modelName,
		})
		targetSet[compositeKey] = struct{}{}
	}

	return normalizedTargets, nil
}

// buildModelMappingList 创建规范化模型映射列表
func (s *GroupService) buildModelMappingList(
	ctx context.Context,
	groupID uint,
	inputs []ModelMappingInput,
	subGroupMap map[uint]struct{},
) ([]models.ModelMapping, error) {
	result := make([]models.ModelMapping, 0, len(inputs))
	aliasSet := make(map[string]struct{}, len(inputs))

	for _, mapping := range inputs {
		alias, lowerAlias, err := validateModelAlias(mapping.Model, aliasSet)
		if err != nil {
			return nil, err
		}

		normalizedTargets, err := s.validateAndNormalizeTargets(ctx, groupID, alias, mapping.Targets, subGroupMap)
		if err != nil {
			return nil, err
		}

		result = append(result, models.ModelMapping{
			Model:   alias,
			Targets: normalizedTargets,
		})
		aliasSet[lowerAlias] = struct{}{}
	}

	return result, nil
}

// processModelMappings 根据组类型验证并规范化模型映射
// 对于聚合组，它解析并规范化映射。
// 对于非聚合组，它存储原始 JSON。
// groupID 是组的 ID（对于尚未创建的组使用 0）。
func (s *GroupService) processModelMappings(ctx context.Context, groupID uint, groupType string, modelMappingsJSON json.RawMessage, tx *gorm.DB) (datatypes.JSON, error) {
	if groupType == "aggregate" && modelMappingsJSON != nil {
		var modelMappingInputs []ModelMappingInput
		if err := json.Unmarshal(modelMappingsJSON, &modelMappingInputs); err != nil {
			return nil, app_errors.NewAPIError(app_errors.ErrInvalidJSON, fmt.Sprintf("invalid model mappings format: %v", err))
		}

		normalizedMappings, err := s.normalizeModelMappings(ctx, groupID, modelMappingInputs, tx)
		if err != nil {
			return nil, err
		}
		return normalizedMappings, nil
	}

	if modelMappingsJSON != nil {
		return datatypes.JSON(modelMappingsJSON), nil
	}

	return datatypes.JSON("[]"), nil
}

// normalizeModelMappings 验证并规范化聚合组的模型映射
func (s *GroupService) normalizeModelMappings(ctx context.Context, groupID uint, inputs []ModelMappingInput, tx *gorm.DB) (datatypes.JSON, error) {
	if len(inputs) == 0 {
		return datatypes.JSON("[]"), nil
	}

	subGroupMap, err := s.subGroupMapBuilder(ctx, groupID, tx)
	if err != nil {
		return nil, err
	}

	result, err := s.buildModelMappingList(ctx, groupID, inputs, subGroupMap)
	if err != nil {
		return nil, err
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("failed to marshal model mappings: %v", err))
	}

	return datatypes.JSON(bytes), nil
}
// validateModelRedirectRules 验证模型重定向规则的格式和内容
func validateModelRedirectRules(rules map[string]string) error {
	if len(rules) == 0 {
		return nil
	}

	for key, value := range rules {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("model name cannot be empty")
		}
	}

	return nil
}
