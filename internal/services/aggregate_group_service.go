package services

import (
	"context"
	"fmt"
	"sync"

	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/utils"

	"github.com/goccy/go-json"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SubGroupInput 定义聚合组成员的输入参数。
type SubGroupInput struct {
	GroupID uint `json:"group_id"`
	Weight  int  `json:"weight"`
}

// AggregateValidationResult 捕获规范化后的聚合组参数。
type AggregateValidationResult struct {
	ValidationEndpoint string
	SubGroups          []models.GroupSubGroup
}

// AggregateGroupService 封装聚合组的特定行为。
type AggregateGroupService struct {
	db           *gorm.DB
	groupManager *GroupManager
}

// NewAggregateGroupService 构造一个 AggregateGroupService 实例。
func NewAggregateGroupService(db *gorm.DB, groupManager *GroupManager) *AggregateGroupService {
	return &AggregateGroupService{
		db:           db,
		groupManager: groupManager,
	}
}

// ValidateSubGroups 验证子组，可选的现有验证端点用于一致性检查。
func (s *AggregateGroupService) ValidateSubGroups(ctx context.Context, channelType string, inputs []SubGroupInput, existingEndpoint string) (*AggregateValidationResult, error) {
	if len(inputs) == 0 {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.sub_groups_required", nil)
	}

	subGroupIDs := make([]uint, 0, len(inputs))
	for _, input := range inputs {
		if input.GroupID == 0 {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.invalid_sub_group_id", nil)
		}
		if input.Weight < 0 {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.sub_group_weight_negative", nil)
		}
		if input.Weight > 1000 {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.sub_group_weight_max_exceeded", nil)
		}
		subGroupIDs = append(subGroupIDs, input.GroupID)
	}

	var subGroupModels []models.Group
	if err := s.db.WithContext(ctx).Where("id IN ?", subGroupIDs).Find(&subGroupModels).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	if len(subGroupModels) != len(subGroupIDs) {
		return nil, NewI18nError(app_errors.ErrValidation, "validation.sub_group_not_found", nil)
	}

	subGroupMap := make(map[uint]models.Group, len(subGroupModels))

	for _, sg := range subGroupModels {
		if sg.GroupType == "aggregate" {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.sub_group_cannot_be_aggregate", nil)
		}
		if sg.ChannelType != channelType {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.sub_group_channel_mismatch", nil)
		}

		// 每个子组使用自己的端点配置，不再强制要求一致性
		subGroupMap[sg.ID] = sg
	}

	resultSubGroups := make([]models.GroupSubGroup, 0, len(inputs))
	for _, input := range inputs {
		if _, ok := subGroupMap[input.GroupID]; !ok {
			return nil, NewI18nError(app_errors.ErrValidation, "validation.sub_group_not_found", nil)
		}
		resultSubGroups = append(resultSubGroups, models.GroupSubGroup{
			SubGroupID: input.GroupID,
			Weight:     input.Weight,
		})
	}

	return &AggregateValidationResult{
		ValidationEndpoint: "", // 不再使用统一的验证端点
		SubGroups:          resultSubGroups,
	}, nil
}

// GetSubGroups 返回聚合组的子组，包含完整信息
func (s *AggregateGroupService) GetSubGroups(ctx context.Context, groupID uint) ([]models.SubGroupInfo, error) {
	var group models.Group
	if err := s.db.WithContext(ctx).First(&group, groupID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, NewI18nError(app_errors.ErrResourceNotFound, "group.not_found", nil)
		}
		return nil, err
	}

	if group.GroupType != "aggregate" {
		return nil, NewI18nError(app_errors.ErrBadRequest, "group.not_aggregate", nil)
	}

	var groupSubGroups []models.GroupSubGroup
	if err := s.db.WithContext(ctx).Where("group_id = ?", groupID).Find(&groupSubGroups).Error; err != nil {
		return nil, err
	}

	if len(groupSubGroups) == 0 {
		return []models.SubGroupInfo{}, nil
	}

	subGroupIDs := make([]uint, 0, len(groupSubGroups))
	weightMap := make(map[uint]int, len(groupSubGroups))

	for _, gsg := range groupSubGroups {
		subGroupIDs = append(subGroupIDs, gsg.SubGroupID)
		weightMap[gsg.SubGroupID] = gsg.Weight
	}

	var subGroupModels []models.Group
	if err := s.db.WithContext(ctx).Where("id IN ?", subGroupIDs).Find(&subGroupModels).Error; err != nil {
		return nil, err
	}

	keyStatsMap := s.fetchSubGroupsKeyStats(ctx, subGroupIDs)

	subGroups := make([]models.SubGroupInfo, 0, len(subGroupModels))
	for _, subGroup := range subGroupModels {
		stats := keyStatsMap[subGroup.ID]

		if stats.Err != nil {
			logrus.WithContext(ctx).WithError(stats.Err).
				WithField("group_id", subGroup.ID).
				Warn("failed to fetch key stats for sub-group, using zero values")
		}

		subGroups = append(subGroups, models.SubGroupInfo{
			Group:       subGroup,
			Weight:      weightMap[subGroup.ID],
			TotalKeys:   stats.TotalKeys,
			ActiveKeys:  stats.ActiveKeys,
			InvalidKeys: stats.InvalidKeys,
		})
	}

	return subGroups, nil
}

// AddSubGroups 向聚合组添加新的子组
func (s *AggregateGroupService) AddSubGroups(ctx context.Context, groupID uint, inputs []SubGroupInput) error {
	var group models.Group
	if err := s.db.WithContext(ctx).First(&group, groupID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return NewI18nError(app_errors.ErrResourceNotFound, "group.not_found", nil)
		}
		return err
	}

	if group.GroupType != "aggregate" {
		return NewI18nError(app_errors.ErrBadRequest, "group.not_aggregate", nil)
	}

	// 检查现有子组（用于去重）
	var existingSubGroups []models.GroupSubGroup
	if err := s.db.WithContext(ctx).Where("group_id = ?", groupID).Find(&existingSubGroups).Error; err != nil {
		return err
	}

	// 验证子组（不再强制端点一致性）
	result, err := s.ValidateSubGroups(ctx, group.ChannelType, inputs, "")
	if err != nil {
		return err
	}

	// 检查与现有子组的重复
	existingSubGroupIDs := utils.NewUintSet()
	for _, sg := range existingSubGroups {
		existingSubGroupIDs.Add(sg.SubGroupID)
	}

	for _, newSg := range result.SubGroups {
		if existingSubGroupIDs.Contains(newSg.SubGroupID) {
			return NewI18nError(app_errors.ErrBadRequest, "group.sub_group_already_exists",
				map[string]any{"sub_group_id": newSg.SubGroupID})
		}
	}

	// 添加新子组
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, newSg := range result.SubGroups {
			newSg.GroupID = groupID
			if err := tx.Create(&newSg).Error; err != nil {
				return app_errors.ParseDBError(err)
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 触发缓存更新
	if err := s.groupManager.Invalidate(); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("failed to invalidate group cache after adding sub groups")
	}

	return nil
}

// UpdateSubGroupWeight 更新特定子组的权重
func (s *AggregateGroupService) UpdateSubGroupWeight(ctx context.Context, groupID, subGroupID uint, weight int) error {
	var group models.Group
	if err := s.db.WithContext(ctx).First(&group, groupID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return NewI18nError(app_errors.ErrResourceNotFound, "group.not_found", nil)
		}
		return err
	}

	if group.GroupType != "aggregate" {
		return NewI18nError(app_errors.ErrBadRequest, "group.not_aggregate", nil)
	}

	if weight < 0 {
		return NewI18nError(app_errors.ErrValidation, "validation.sub_group_weight_negative", nil)
	}

	if weight > 1000 {
		return NewI18nError(app_errors.ErrValidation, "validation.sub_group_weight_max_exceeded", nil)
	}

	// 检查子组关联是否存在
	var existingRecord models.GroupSubGroup
	if err := s.db.WithContext(ctx).Where("group_id = ? AND sub_group_id = ?", groupID, subGroupID).First(&existingRecord).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return NewI18nError(app_errors.ErrResourceNotFound, "group.sub_group_not_found", nil)
		}
		return err
	}

	result := s.db.WithContext(ctx).
		Model(&models.GroupSubGroup{}).
		Where("group_id = ? AND sub_group_id = ?", groupID, subGroupID).
		Update("weight", weight)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return NewI18nError(app_errors.ErrResourceNotFound, "group.sub_group_not_found", nil)
	}

	// 触发缓存更新
	if err := s.groupManager.Invalidate(); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("failed to invalidate group cache after updating sub group weight")
	}

	return nil
}

// DeleteSubGroup 从聚合组中移除一个子组，并同步清理模型映射中的相关条目
func (s *AggregateGroupService) DeleteSubGroup(ctx context.Context, groupID, subGroupID uint) error {
	var group models.Group
	if err := s.db.WithContext(ctx).First(&group, groupID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return NewI18nError(app_errors.ErrResourceNotFound, "group.not_found", nil)
		}
		return err
	}

	if group.GroupType != "aggregate" {
		return NewI18nError(app_errors.ErrBadRequest, "group.not_aggregate", nil)
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("group_id = ? AND sub_group_id = ?", groupID, subGroupID).
			Delete(&models.GroupSubGroup{})

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return NewI18nError(app_errors.ErrResourceNotFound, "group.sub_group_not_found", nil)
		}

		return s.cleanupModelMappingsForSubGroup(ctx, tx, &group, subGroupID)
	})

	if err != nil {
		return err
	}

	if err := s.groupManager.Invalidate(); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("failed to invalidate group cache after deleting sub group")
	}

	return nil
}

// CleanupModelMappingsForDeletedSubGroup 清理所有引用已删除子组的聚合组的模型映射
func (s *AggregateGroupService) CleanupModelMappingsForDeletedSubGroup(ctx context.Context, tx *gorm.DB, subGroupID uint) error {
	var parentGroupIDs []uint
	if err := tx.WithContext(ctx).
		Model(&models.GroupSubGroup{}).
		Where("sub_group_id = ?", subGroupID).
		Pluck("group_id", &parentGroupIDs).Error; err != nil {
		return err
	}

	if len(parentGroupIDs) == 0 {
		return nil
	}

	var parentGroups []models.Group
	if err := tx.WithContext(ctx).Where("id IN ?", parentGroupIDs).Find(&parentGroups).Error; err != nil {
		return err
	}

	for _, group := range parentGroups {
		if err := s.cleanupModelMappingsForSubGroup(ctx, tx, &group, subGroupID); err != nil {
			return fmt.Errorf("cleanup model mappings for group %d failed: %w", group.ID, err)
		}
	}

	return nil
}

// cleanupModelMappingsForSubGroup 清理模型映射中引用已删除子组的条目
func (s *AggregateGroupService) cleanupModelMappingsForSubGroup(ctx context.Context, tx *gorm.DB, group *models.Group, subGroupID uint) error {
	if len(group.ModelMappings) == 0 {
		return nil
	}

	var mappings []models.ModelMapping
	if err := json.Unmarshal(group.ModelMappings, &mappings); err != nil {
		return fmt.Errorf("parse model mappings for group %d failed: %w", group.ID, err)
	}

	updatedMappings := make([]models.ModelMapping, 0, len(mappings))
	hasChanges := false
	for _, mapping := range mappings {
		var filteredTargets []models.ModelMappingTarget
		for _, target := range mapping.Targets {
			if target.SubGroupID != subGroupID {
				filteredTargets = append(filteredTargets, target)
			} else {
				hasChanges = true
			}
		}

		if len(filteredTargets) > 0 {
			mapping.Targets = filteredTargets
			updatedMappings = append(updatedMappings, mapping)
		}
	}

	if !hasChanges {
		return nil
	}

	removedCount := len(mappings) - len(updatedMappings)
	if removedCount > 0 {
		logrus.WithContext(ctx).WithFields(logrus.Fields{
			"group_id":         group.ID,
			"deleted_subgroup": subGroupID,
			"removed_mappings": removedCount,
		}).Info("Cleaned up model mappings after sub-group deletion")
	}

	updatedJSON, err := json.Marshal(updatedMappings)
	if err != nil {
		return fmt.Errorf("failed to marshal updated model mappings: %w", err)
	}

	if err := tx.WithContext(ctx).Model(&models.Group{}).
		Where("id = ?", group.ID).
		Update("model_mappings", datatypes.JSON(updatedJSON)).Error; err != nil {
		return fmt.Errorf("failed to update model mappings: %w", err)
	}

	return nil
}

// CountAggregateGroupsUsingSubGroup 返回使用指定组作为子组的聚合组数量
func (s *AggregateGroupService) CountAggregateGroupsUsingSubGroup(ctx context.Context, subGroupID uint) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&models.GroupSubGroup{}).
		Where("sub_group_id = ?", subGroupID).
		Count(&count).Error

	if err != nil {
		return 0, app_errors.ParseDBError(err)
	}

	return count, nil
}

// CountAggregateGroupsUsingSubGroupTx 返回使用指定组作为子组的聚合组数量（事务版本）
func (s *AggregateGroupService) CountAggregateGroupsUsingSubGroupTx(ctx context.Context, subGroupID uint, tx *gorm.DB) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).
		Model(&models.GroupSubGroup{}).
		Where("sub_group_id = ?", subGroupID).
		Count(&count).Error

	if err != nil {
		return 0, app_errors.ParseDBError(err)
	}

	return count, nil
}

// GetParentAggregateGroups 返回使用指定组作为子组的聚合组
func (s *AggregateGroupService) GetParentAggregateGroups(ctx context.Context, subGroupID uint) ([]models.ParentAggregateGroupInfo, error) {
	var groupSubGroups []models.GroupSubGroup
	if err := s.db.WithContext(ctx).Where("sub_group_id = ?", subGroupID).Find(&groupSubGroups).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	if len(groupSubGroups) == 0 {
		return []models.ParentAggregateGroupInfo{}, nil
	}

	aggregateGroupIDs := make([]uint, 0, len(groupSubGroups))
	weightMap := make(map[uint]int, len(groupSubGroups))

	for _, gsg := range groupSubGroups {
		aggregateGroupIDs = append(aggregateGroupIDs, gsg.GroupID)
		weightMap[gsg.GroupID] = gsg.Weight
	}

	var aggregateGroupModels []models.Group
	if err := s.db.WithContext(ctx).Where("id IN ?", aggregateGroupIDs).Find(&aggregateGroupModels).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	parentGroups := make([]models.ParentAggregateGroupInfo, 0, len(aggregateGroupModels))
	for _, group := range aggregateGroupModels {
		parentGroups = append(parentGroups, models.ParentAggregateGroupInfo{
			GroupID:     group.ID,
			Name:        group.Name,
			DisplayName: group.DisplayName,
			Weight:      weightMap[group.ID],
		})
	}

	return parentGroups, nil
}

// keyStatsResult 存储单个组的密钥统计数据
type keyStatsResult struct {
	GroupID     uint
	TotalKeys   int64
	ActiveKeys  int64
	InvalidKeys int64
	Err         error
}

// fetchSubGroupsKeyStats 并发批量获取多个子组的密钥统计信息
func (s *AggregateGroupService) fetchSubGroupsKeyStats(ctx context.Context, groupIDs []uint) map[uint]keyStatsResult {
	results := make(map[uint]keyStatsResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, groupID := range groupIDs {
		wg.Add(1)
		go func(gid uint) {
			defer wg.Done()

			var totalKeys, activeKeys int64
			result := keyStatsResult{GroupID: gid}

			// 查询总密钥数
			if err := s.db.WithContext(ctx).Model(&models.APIKey{}).
				Where("group_id = ?", gid).
				Count(&totalKeys).Error; err != nil {
				result.Err = err
				mu.Lock()
				results[gid] = result
				mu.Unlock()
				return
			}

			// 查询活跃密钥数
			if err := s.db.WithContext(ctx).Model(&models.APIKey{}).
				Where("group_id = ? AND status = ?", gid, models.KeyStatusActive).
				Count(&activeKeys).Error; err != nil {
				result.Err = err
				mu.Lock()
				results[gid] = result
				mu.Unlock()
				return
			}

			result.TotalKeys = totalKeys
			result.ActiveKeys = activeKeys
			result.InvalidKeys = totalKeys - activeKeys

			mu.Lock()
			results[gid] = result
			mu.Unlock()
		}(groupID)
	}

	wg.Wait()
	return results
}
