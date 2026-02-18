package services

import (
	"context"
	"sync"

	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// SubGroupInput defines the input payload for aggregate group member configuration.
type SubGroupInput struct {
	GroupID uint `json:"group_id"`
	Weight  int  `json:"weight"`
}

// AggregateValidationResult captures the normalized aggregate group parameters.
type AggregateValidationResult struct {
	ValidationEndpoint string
	SubGroups          []models.GroupSubGroup
}

// AggregateGroupService encapsulates aggregate group specific behaviours.
type AggregateGroupService struct {
	db           *gorm.DB
	groupManager *GroupManager
}

// NewAggregateGroupService constructs an AggregateGroupService instance.
func NewAggregateGroupService(db *gorm.DB, groupManager *GroupManager) *AggregateGroupService {
	return &AggregateGroupService{
		db:           db,
		groupManager: groupManager,
	}
}

// ValidateSubGroups validates sub-groups with an optional existing validation endpoint for consistency check.
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

		// Each sub-group uses its own endpoint configuration, no longer enforcing consistency
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
		ValidationEndpoint: "", // No longer using unified validation endpoint
		SubGroups:          resultSubGroups,
	}, nil
}

// GetSubGroups returns sub groups for an aggregate group with complete information
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

// AddSubGroups adds new sub groups to an aggregate group
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

	// Check for existing sub-groups (for deduplication)
	var existingSubGroups []models.GroupSubGroup
	if err := s.db.WithContext(ctx).Where("group_id = ?", groupID).Find(&existingSubGroups).Error; err != nil {
		return err
	}

	// Validate sub-groups (no longer enforcing endpoint consistency)
	result, err := s.ValidateSubGroups(ctx, group.ChannelType, inputs, "")
	if err != nil {
		return err
	}

	// Check for duplicates with existing sub groups
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

	// Add new sub groups
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

	// Trigger cache update
	if err := s.groupManager.Invalidate(); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("failed to invalidate group cache after adding sub groups")
	}

	return nil
}

// UpdateSubGroupWeight updates the weight of a specific sub group
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

	// Check if sub-group association exists
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

	// Trigger cache update
	if err := s.groupManager.Invalidate(); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("failed to invalidate group cache after updating sub group weight")
	}

	return nil
}

// DeleteSubGroup removes a sub group from an aggregate group
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

	result := s.db.WithContext(ctx).
		Where("group_id = ? AND sub_group_id = ?", groupID, subGroupID).
		Delete(&models.GroupSubGroup{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return NewI18nError(app_errors.ErrResourceNotFound, "group.sub_group_not_found", nil)
	}

	// Trigger cache update
	if err := s.groupManager.Invalidate(); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("failed to invalidate group cache after deleting sub group")
	}

	return nil
}

// CountAggregateGroupsUsingSubGroup returns the number of aggregate groups that use the specified group as a sub-group
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

// CountAggregateGroupsUsingSubGroupTx returns number of aggregate groups that use specified group as a sub-group (transaction version)
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

// GetParentAggregateGroups returns the aggregate groups that use the specified group as a sub-group
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

// keyStatsResult stores key statistics for a single group
type keyStatsResult struct {
	GroupID     uint
	TotalKeys   int64
	ActiveKeys  int64
	InvalidKeys int64
	Err         error
}

// fetchSubGroupsKeyStats batch fetches key statistics for multiple sub-groups concurrently
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

			// Query total keys
			if err := s.db.WithContext(ctx).Model(&models.APIKey{}).
				Where("group_id = ?", gid).
				Count(&totalKeys).Error; err != nil {
				result.Err = err
				mu.Lock()
				results[gid] = result
				mu.Unlock()
				return
			}

			// Query active keys
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
