package services

import (
	"fmt"
	"gpt-load/internal/models"
	"gpt-load/internal/store"
	"gpt-load/internal/utils"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

const (
	maxSelectionAttemptsMultiplier = 2
)

// SubGroupManager manages weighted round-robin selection for all aggregate groups
type SubGroupManager struct {
	store     store.Store
	selectors map[uint]*groupSelectors
	mu        sync.RWMutex
}

type groupSelectors struct {
	// Each model alias has its own selector
	aliasSelectors map[string]*modelLevelSelector
	// Default selector for unmatched aliases
	defaultSelector *modelLevelSelector
}

// modelSelectionItem represents a model with its weight and sub-group info for round-robin
type modelSelectionItem struct {
	model         string
	subGroupID    uint
	subGroupName  string
	weight        int
	currentWeight int
}

func (m *modelSelectionItem) GetWeight() int {
	return m.weight
}

func (m *modelSelectionItem) GetCurrentWeight() int {
	return m.currentWeight
}

func (m *modelSelectionItem) SetCurrentWeight(w int) {
	m.currentWeight = w
}

// SelectionResult captures the selected model and sub-group info
type SelectionResult struct {
	GroupName     string
	SubGroupID    uint
	SelectedModel string
}

// NewSubGroupManager creates a new sub-group manager service
func NewSubGroupManager(store store.Store) *SubGroupManager {
	return &SubGroupManager{
		store:     store,
		selectors: make(map[uint]*groupSelectors),
	}
}

// SelectSubGroup selects an appropriate sub-group for the given aggregate group
func (m *SubGroupManager) SelectSubGroup(group *models.Group, modelAlias string) (*SelectionResult, error) {
	if group.GroupType != "aggregate" {
		return nil, nil
	}

	selectors := m.getSelectors(group)
	if selectors == nil {
		return nil, fmt.Errorf("no valid selectors available for aggregate group '%s'", group.Name)
	}

	alias := strings.TrimSpace(modelAlias)

	// Try to match model alias first
	if alias != "" && len(selectors.aliasSelectors) > 0 {
		if sel, ok := selectors.aliasSelectors[strings.ToLower(alias)]; ok {
			if selectedItem := sel.selectNextModel(); selectedItem != nil {
				logrus.WithFields(logrus.Fields{
					"aggregate_group": group.Name,
					"model_alias":     alias,
					"selected_model":  selectedItem.model,
					"sub_group":       selectedItem.subGroupName,
				}).Debug("Selected model from alias mapping")

				return &SelectionResult{
					GroupName:     selectedItem.subGroupName,
					SubGroupID:    selectedItem.subGroupID,
					SelectedModel: selectedItem.model,
				}, nil
			}
			logrus.WithFields(logrus.Fields{
				"aggregate_group": group.Name,
				"model_alias":     alias,
			}).Warn("Model alias selector has no available models")
		}
	}

	// Try wildcard matching (for patterns with * or ?)
	if alias != "" && len(selectors.aliasSelectors) > 0 {
		for pattern, sel := range selectors.aliasSelectors {
			if utils.HasWildcard(pattern) && utils.MatchWildcard(pattern, alias) {
				if selectedItem := sel.selectNextModel(); selectedItem != nil {
					logrus.WithFields(logrus.Fields{
						"aggregate_group":   group.Name,
						"model_alias":         alias,
						"matched_pattern":     pattern,
						"selected_model":      selectedItem.model,
						"sub_group":           selectedItem.subGroupName,
					}).Debug("Selected model from wildcard pattern mapping")

					return &SelectionResult{
						GroupName:     selectedItem.subGroupName,
						SubGroupID:    selectedItem.subGroupID,
						SelectedModel: selectedItem.model,
					}, nil
				}
			}
		}
	}

	// If no alias match, try default selector
	if selectors.defaultSelector != nil {
		if selectedItem := selectors.defaultSelector.selectNextModel(); selectedItem != nil {
			logrus.WithFields(logrus.Fields{
				"aggregate_group": group.Name,
				"model_alias":     alias,
				"selected_model":  selectedItem.model,
				"sub_group":       selectedItem.subGroupName,
			}).Debug("Selected model from default mapping")

			return &SelectionResult{
				GroupName:     selectedItem.subGroupName,
				SubGroupID:    selectedItem.subGroupID,
				SelectedModel: selectedItem.model,
			}, nil
		}
		logrus.WithFields(logrus.Fields{
			"aggregate_group": group.Name,
		}).Warn("Default selector has no available models")
	}

	// In strict mode, return error if no match found
	if group.ModelMappingStrict {
		availableAliases := make([]string, 0, len(selectors.aliasSelectors))
		for k := range selectors.aliasSelectors {
			availableAliases = append(availableAliases, k)
		}
		logrus.WithFields(logrus.Fields{
			"aggregate_group":   group.Name,
			"model_alias":       alias,
			"available_aliases": availableAliases,
			"strict_mode":       true,
		}).Warn("No matching model alias in strict mode")
		return nil, fmt.Errorf("no matching model alias '%s' in aggregate group '%s' (strict mode enabled)", alias, group.Name)
	}

	availableAliases := make([]string, 0, len(selectors.aliasSelectors))
	for k := range selectors.aliasSelectors {
		availableAliases = append(availableAliases, k)
	}
	logrus.WithFields(logrus.Fields{
		"aggregate_group":   group.Name,
		"model_alias":       alias,
		"available_aliases": availableAliases,
	}).Warn("No matching model alias found after trying exact match, wildcard match, and default selector")

	return nil, fmt.Errorf("no available models for model alias '%s' in aggregate group '%s'", alias, group.Name)
}

// RebuildSelectors rebuild all selectors based on the incoming group
func (m *SubGroupManager) RebuildSelectors(groups map[string]*models.Group) {
	newSelectors := make(map[uint]*groupSelectors)

	for _, group := range groups {
		if group.GroupType == "aggregate" && len(group.SubGroups) > 0 {
			if sel := m.createGroupSelectors(group); sel != nil {
				newSelectors[group.ID] = sel
			}
		}
	}

	m.mu.Lock()
	m.selectors = newSelectors
	m.mu.Unlock()

	logrus.WithField("new_count", len(newSelectors)).Debug("Rebuilt selectors for aggregate groups")
}

// getSelectors retrieves or creates selectors for the aggregate group
func (m *SubGroupManager) getSelectors(group *models.Group) *groupSelectors {
	m.mu.RLock()
	if sel, exists := m.selectors[group.ID]; exists {
		m.mu.RUnlock()
		return sel
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if sel, exists := m.selectors[group.ID]; exists {
		return sel
	}

	sel := m.createGroupSelectors(group)
	if sel != nil {
		m.selectors[group.ID] = sel
		logrus.WithFields(logrus.Fields{
			"group_id":   group.ID,
			"group_name": group.Name,
		}).Debug("Created sub-group selectors")
	}

	return sel
}

// createGroupSelectors creates model-level selectors for an aggregate group
func (m *SubGroupManager) createGroupSelectors(group *models.Group) *groupSelectors {
	if group.GroupType != "aggregate" || len(group.SubGroups) == 0 {
		return nil
	}

	if len(group.ModelMappingList) == 0 {
		return nil
	}

	result := &groupSelectors{
		aliasSelectors: make(map[string]*modelLevelSelector),
	}

	// Create sub-group mapping
	subGroupMap := make(map[uint]models.GroupSubGroup, len(group.SubGroups))
	for _, sg := range group.SubGroups {
		subGroupMap[sg.SubGroupID] = sg
	}

	// Process all model mappings
	for _, mapping := range group.ModelMappingList {
		alias := strings.TrimSpace(mapping.Model)
		if alias == "" {
			continue
		}

		// Create model-level selector
		var modelItems []modelSelectionItem
		for _, target := range mapping.Targets {
			sg, ok := subGroupMap[target.SubGroupID]
			if !ok {
				logrus.WithFields(logrus.Fields{
					"aggregate_group": group.Name,
					"model_alias":     alias,
					"sub_group_id":    target.SubGroupID,
				}).Warn("Model mapping target references unknown sub-group")
				continue
			}

			// Calculate final weight: sub-group weight × model mapping weight
			modelMappingWeight := target.Weight
			// Skip targets with weight 0 (disabled)
			if modelMappingWeight == 0 {
				logrus.WithFields(logrus.Fields{
					"aggregate_group": group.Name,
					"model_alias":     alias,
					"sub_group_id":    target.SubGroupID,
				}).Debug("Skipping model mapping target with weight 0 (disabled)")
				continue
			}

			subGroupWeight := sg.Weight
			// Sub-groups with weight=0 are filtered out during loading, so weight should always be > 0 here
			if subGroupWeight <= 0 {
				subGroupWeight = 1
			}

			finalWeight := subGroupWeight * modelMappingWeight

			logrus.WithFields(logrus.Fields{
				"aggregate_group":      group.Name,
				"model_alias":          alias,
				"sub_group_id":         target.SubGroupID,
				"sub_group_weight":     subGroupWeight,
				"model_mapping_weight": modelMappingWeight,
				"final_weight":         finalWeight,
			}).Debug("Calculated final model weight")

			name := sg.SubGroupName
			if name == "" {
				name = fmt.Sprintf("group-%d", sg.SubGroupID)
			}

			// Flatten model list - each model as an independent selection item
			var models []string
			if len(target.Models) > 0 {
				models = target.Models
			} else if target.Model != "" {
				models = []string{target.Model}
			}

			for _, model := range models {
				if model == "" {
					continue
				}
				modelItems = append(modelItems, modelSelectionItem{
					model:         model,
					subGroupID:    target.SubGroupID,
					subGroupName:  name,
					weight:        finalWeight,
					currentWeight: 0,
				})
			}
		}

		// Create model-level selector if there are model items
		if len(modelItems) > 0 {
			selector := newModelLevelSelector(group, alias, modelItems, m.store)
			// Use lowercase as key for case-insensitive matching
			result.aliasSelectors[strings.ToLower(alias)] = selector

			logrus.WithFields(logrus.Fields{
				"aggregate_group": group.Name,
				"model_alias":     alias,
				"model_count":     len(modelItems),
			}).Debug("Created selector for model alias")
		}
	}

	// Return nil if no alias selectors exist
	if len(result.aliasSelectors) == 0 {
		return nil
	}

	// Create default selector with all models (only in non-strict mode)
	if !group.ModelMappingStrict {
		var allModelItems []modelSelectionItem
		for _, selector := range result.aliasSelectors {
			allModelItems = append(allModelItems, selector.modelItems...)
		}

		if len(allModelItems) > 0 {
			result.defaultSelector = newModelLevelSelector(group, "", allModelItems, m.store)
			logrus.WithFields(logrus.Fields{
				"aggregate_group": group.Name,
				"total_models":    len(allModelItems),
			}).Debug("Created default selector with all models")
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"aggregate_group": group.Name,
		}).Debug("Model mapping strict mode enabled: default selector not created")
	}

	return result
}

func newModelLevelSelector(group *models.Group, alias string, items []modelSelectionItem, store store.Store) *modelLevelSelector {
	return &modelLevelSelector{
		groupID:    group.ID,
		groupName:  group.Name,
		modelAlias: alias,
		modelItems: items,
		store:      store,
	}
}

// modelLevelSelector encapsulates the weighted round-robin algorithm at model level
type modelLevelSelector struct {
	groupID    uint
	groupName  string
	modelAlias string
	modelItems []modelSelectionItem
	store      store.Store
	mu         sync.Mutex
}

// selectNextModel uses weighted round-robin algorithm to select a model with active keys
func (m *modelLevelSelector) selectNextModel() *modelSelectionItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.modelItems) == 0 {
		return nil
	}

	if len(m.modelItems) == 1 {
		if m.hasActiveKeys(m.modelItems[0].subGroupID) {
			return &m.modelItems[0]
		}
		return nil
	}

	type modelSubGroupKey struct {
		model      string
		subGroupID uint
	}
	attempted := make(map[modelSubGroupKey]bool)
	for len(attempted) < len(m.modelItems) {
		item := m.selectModelByWeight()
		if item == nil {
			break
		}

		itemKey := modelSubGroupKey{model: item.model, subGroupID: item.subGroupID}
		if attempted[itemKey] {
			continue
		}
		attempted[itemKey] = true

		if m.hasActiveKeys(item.subGroupID) {
			return item
		}
	}

	return nil
}

// selectNextModelExcluding selects a model, excluding specified sub-group IDs
func (m *modelLevelSelector) selectNextModelExcluding(excludedIDs utils.UintSet) *modelSelectionItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.modelItems) == 0 {
		return nil
	}

	if len(m.modelItems) == 1 {
		if excludedIDs.Contains(m.modelItems[0].subGroupID) {
			return nil
		}
		if m.hasActiveKeys(m.modelItems[0].subGroupID) {
			return &m.modelItems[0]
		}
		return nil
	}

	attempted := make(map[string]bool)
	maxAttempts := len(m.modelItems) * maxSelectionAttemptsMultiplier
	attempts := 0

	for attempts < maxAttempts {
		attempts++
		item := m.selectModelByWeight()
		if item == nil {
			break
		}

		// Skip excluded sub-groups
		if excludedIDs.Contains(item.subGroupID) {
			continue
		}

		itemKey := fmt.Sprintf("%s:%d", item.model, item.subGroupID)
		if attempted[itemKey] {
			continue
		}
		attempted[itemKey] = true

		if m.hasActiveKeys(item.subGroupID) {
			return item
		}
	}

	return nil
}

// selectModelByWeight implements smooth weighted round-robin algorithm for models
func (m *modelLevelSelector) selectModelByWeight() *modelSelectionItem {
	items := make([]utils.WeightedItem, len(m.modelItems))
	for i := range m.modelItems {
		items[i] = &m.modelItems[i]
	}

	selected := utils.SelectByWeightedRoundRobin(items)
	if selected == nil {
		return &m.modelItems[0]
	}

	return selected.(*modelSelectionItem)
}

// hasActiveKeys checks if a sub-group has available API keys
func (m *modelLevelSelector) hasActiveKeys(groupID uint) bool {
	key := fmt.Sprintf("group:%d:active_keys", groupID)
	length, err := m.store.LLen(key)
	if err != nil {
		return true
	}
	return length > 0
}

// SelectSubGroupByModelMapping selects a sub-group based on model mapping rules
// Returns: (subGroupName, actualModelName, error)
func (m *SubGroupManager) SelectSubGroupByModelMapping(
	group *models.Group,
	modelAlias string,
) (string, string, error) {
	// SelectSubGroup already implements smooth weighted round-robin for model mapping
	// Use it directly to get selection result
	selection, err := m.SelectSubGroup(group, modelAlias)
	if err != nil {
		return "", "", err
	}
	if selection == nil {
		return "", "", nil
	}

	return selection.GroupName, selection.SelectedModel, nil
}

// SelectSubGroupExcluding selects a sub-group, excluding specified sub-group IDs
func (m *SubGroupManager) SelectSubGroupExcluding(
	group *models.Group,
	modelAlias string,
	excludedIDs utils.UintSet,
) (*SelectionResult, error) {
	if group.GroupType != "aggregate" {
		return nil, nil
	}

	selectors := m.getSelectors(group)
	if selectors == nil {
		return nil, fmt.Errorf("no valid selectors available for aggregate group '%s'", group.Name)
	}

	alias := strings.TrimSpace(modelAlias)

	// Try to match model alias first
	if alias != "" && len(selectors.aliasSelectors) > 0 {
		if sel, ok := selectors.aliasSelectors[strings.ToLower(alias)]; ok {
			if selectedItem := sel.selectNextModelExcluding(excludedIDs); selectedItem != nil {
				logrus.WithFields(logrus.Fields{
					"aggregate_group": group.Name,
					"model_alias":     alias,
					"selected_model":  selectedItem.model,
					"sub_group":       selectedItem.subGroupName,
					"excluded_ids":    excludedIDs,
				}).Debug("Selected model from alias mapping (with exclusions)")

				return &SelectionResult{
					GroupName:     selectedItem.subGroupName,
					SubGroupID:    selectedItem.subGroupID,
					SelectedModel: selectedItem.model,
				}, nil
			}
			// Alias has mapping but all targets unavailable, return error without falling back to default selector
			return nil, fmt.Errorf("no available models for model alias '%s' in aggregate group '%s' (all mapped targets exhausted, excluded_ids: %v)", alias, group.Name, excludedIDs)
		}
	}

	// If no matching alias mapping, try default selector
	if selectors.defaultSelector != nil {
		if selectedItem := selectors.defaultSelector.selectNextModelExcluding(excludedIDs); selectedItem != nil {
			logrus.WithFields(logrus.Fields{
				"aggregate_group": group.Name,
				"model_alias":     alias,
				"selected_model":  selectedItem.model,
				"sub_group":       selectedItem.subGroupName,
				"excluded_ids":    excludedIDs,
			}).Debug("Selected model from default mapping (with exclusions)")

			return &SelectionResult{
				GroupName:     selectedItem.subGroupName,
				SubGroupID:    selectedItem.subGroupID,
				SelectedModel: selectedItem.model,
			}, nil
		}
		logrus.WithFields(logrus.Fields{
			"aggregate_group": group.Name,
			"excluded_ids":    excludedIDs,
		}).Warn("Default selector has no available models (after exclusions)")
	}

	return nil, fmt.Errorf("no available models for model alias '%s' in aggregate group '%s' (all targets exhausted)", alias, group.Name)
}
