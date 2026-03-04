package services

import (
	"context"
	"fmt"
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"gpt-load/internal/store"
	"gpt-load/internal/syncer"
	"gpt-load/internal/utils"

	"github.com/goccy/go-json"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const GroupUpdateChannel = "groups:updated"

// GroupManager 管理组数据的缓存。
type GroupManager struct {
	syncer          *syncer.CacheSyncer[map[string]*models.Group]
	db              *gorm.DB
	store           store.Store
	settingsManager *config.SystemSettingsManager
	subGroupManager *SubGroupManager
}

// NewGroupManager 创建一个新的未初始化的 GroupManager。
func NewGroupManager(
	db *gorm.DB,
	store store.Store,
	settingsManager *config.SystemSettingsManager,
	subGroupManager *SubGroupManager,
) *GroupManager {
	return &GroupManager{
		db:              db,
		store:           store,
		settingsManager: settingsManager,
		subGroupManager: subGroupManager,
	}
}

// Initialize 设置 CacheSyncer。此方法单独调用以处理潜在的
func (gm *GroupManager) Initialize() error {
	loader := func() (map[string]*models.Group, error) {
		var groups []*models.Group
		if err := gm.db.Find(&groups).Error; err != nil {
			return nil, fmt.Errorf("failed to load groups from db: %w", err)
		}

		// Load all sub-group relationships for aggregate groups (only active ones with weight > 0)
		var allSubGroups []models.GroupSubGroup
		if err := gm.db.Where("weight > 0").Find(&allSubGroups).Error; err != nil {
			return nil, fmt.Errorf("failed to load sub groups: %w", err)
		}

		// Group sub-groups by aggregate group ID
		subGroupsByAggregateID := make(map[uint][]models.GroupSubGroup)
		for _, sg := range allSubGroups {
			subGroupsByAggregateID[sg.GroupID] = append(subGroupsByAggregateID[sg.GroupID], sg)
		}

		// Create group ID to group object mapping for sub-group lookups
		groupByID := make(map[uint]*models.Group)
		for _, group := range groups {
			groupByID[group.ID] = group
		}

		groupMap := make(map[string]*models.Group, len(groups))
		for _, group := range groups {
			g := *group
			g.EffectiveConfig = gm.settingsManager.GetEffectiveConfig(g.Config)
			g.ProxyKeysMap = utils.StringToSet(g.ProxyKeys, ",")

			// Parse header rules with error handling
			if len(group.HeaderRules) > 0 {
				if err := json.Unmarshal(group.HeaderRules, &g.HeaderRuleList); err != nil {
					logrus.WithError(err).WithField("group_name", g.Name).Warn("Failed to parse header rules for group")
					g.HeaderRuleList = []models.HeaderRule{}
				}
			} else {
				g.HeaderRuleList = []models.HeaderRule{}
			}

			// Parse model redirect rules with error handling
			g.ModelRedirectMap = make(map[string]string)
			if len(group.ModelRedirectRules) > 0 {
				hasInvalidRules := false
				for key, value := range group.ModelRedirectRules {
					if valueStr, ok := value.(string); ok {
						g.ModelRedirectMap[key] = valueStr
					} else {
						logrus.WithFields(logrus.Fields{
							"group_name": g.Name,
							"rule_key":   key,
							"value_type": fmt.Sprintf("%T", value),
							"value":      value,
						}).Error("Invalid model redirect rule value type, skipping this rule")
						hasInvalidRules = true
					}
				}
				if hasInvalidRules {
					logrus.WithField("group_name", g.Name).Warn("Group has invalid model redirect rules, some rules were skipped. Please check the configuration.")
				}
			}

			// Load sub-groups for aggregate groups
			if g.GroupType == "aggregate" {
				if subGroups, ok := subGroupsByAggregateID[g.ID]; ok {
					g.SubGroups = make([]models.GroupSubGroup, len(subGroups))
					for i, sg := range subGroups {
						g.SubGroups[i] = sg
						if subGroup, exists := groupByID[sg.SubGroupID]; exists {
							g.SubGroups[i].SubGroupName = subGroup.Name
						}
					}
				}
			}

			// Parse model mappings for aggregate groups
			if len(group.ModelMappings) > 0 {
				if err := json.Unmarshal(group.ModelMappings, &g.ModelMappingList); err != nil {
					logrus.WithError(err).WithField("group_name", g.Name).Warn("Failed to parse model mappings for group")
					g.ModelMappingList = nil
				}
			} else {
				g.ModelMappingList = nil
			}

			// Fill in sub-group names for model mappings
			if len(g.ModelMappingList) > 0 && len(g.SubGroups) > 0 {
				subGroupNameMap := make(map[uint]string, len(g.SubGroups))
				for _, sg := range g.SubGroups {
					name := sg.SubGroupName
					if name == "" {
						if subGroup, exists := groupByID[sg.SubGroupID]; exists {
							name = subGroup.Name
						}
					}
					subGroupNameMap[sg.SubGroupID] = name
				}

				for mi := range g.ModelMappingList {
					for ti := range g.ModelMappingList[mi].Targets {
						target := &g.ModelMappingList[mi].Targets[ti]
						target.SubGroupName = subGroupNameMap[target.SubGroupID]
					}
				}
			}

			groupMap[g.Name] = &g
			logrus.WithFields(logrus.Fields{
				"group_name":                 g.Name,
				"effective_config":           g.EffectiveConfig,
				"header_rules_count":         len(g.HeaderRuleList),
				"model_redirect_rules_count": len(g.ModelRedirectMap),
				"model_redirect_strict":      g.ModelRedirectStrict,
				"sub_group_count":            len(g.SubGroups),
			}).Debug("Loaded group with effective config")
		}

		return groupMap, nil
	}

	afterReload := func(newCache map[string]*models.Group) {
		gm.subGroupManager.RebuildSelectors(newCache)
	}

	syncer, err := syncer.NewCacheSyncer(
		loader,
		gm.store,
		GroupUpdateChannel,
		logrus.WithField("syncer", "groups"),
		afterReload,
	)
	if err != nil {
		return fmt.Errorf("failed to create group syncer: %w", err)
	}
	gm.syncer = syncer
	return nil
}

// GetGroupByName 从缓存中通过名称获取单个组。
func (gm *GroupManager) GetGroupByName(name string) (*models.Group, error) {
	if gm.syncer == nil {
		return nil, fmt.Errorf("GroupManager is not initialized")
	}

	groups := gm.syncer.Get()
	group, ok := groups[name]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return group, nil
}

// Invalidate 触发所有实例的缓存重载。
func (gm *GroupManager) Invalidate() error {
	if gm.syncer == nil {
		return fmt.Errorf("GroupManager is not initialized")
	}
	return gm.syncer.Invalidate()
}

// Stop 优雅地停止 GroupManager 的后台同步器。
func (gm *GroupManager) Stop(ctx context.Context) {
	if gm.syncer != nil {
		gm.syncer.Stop()
	}
}
