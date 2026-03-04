// Package handler 提供应用的 HTTP 处理器
package handler

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/i18n"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"gpt-load/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
)

func (s *Server) handleGroupError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	if svcErr, ok := err.(*services.I18nError); ok {
		if svcErr.Template != nil {
			response.ErrorI18nFromAPIError(c, svcErr.APIError, svcErr.MessageID, svcErr.Template)
		} else {
			response.ErrorI18nFromAPIError(c, svcErr.APIError, svcErr.MessageID)
		}
		return true
	}

	if apiErr, ok := err.(*app_errors.APIError); ok {
		response.Error(c, apiErr)
		return true
	}

	logrus.WithContext(c.Request.Context()).WithError(err).Error("unexpected group service error")
	response.Error(c, app_errors.ErrInternalServer)
	return true
}

// GroupCreateRequest 定义创建组的载荷
type GroupCreateRequest struct {
	Name                string              `json:"name"`
	DisplayName         string              `json:"display_name"`
	Description         string              `json:"description"`
	GroupType           string              `json:"group_type"` // 'standard' or 'aggregate'
	Upstreams           json.RawMessage     `json:"upstreams"`
	ChannelType         string              `json:"channel_type"`
	Sort                int                 `json:"sort"`
	TestModel           string              `json:"test_model"`
	ValidationEndpoint  string              `json:"validation_endpoint"`
	ParamOverrides      map[string]any      `json:"param_overrides"`
	ModelRedirectRules  map[string]string   `json:"model_redirect_rules"`
	ModelRedirectStrict bool                `json:"model_redirect_strict"`
	Config              map[string]any      `json:"config"`
	HeaderRules         []models.HeaderRule `json:"header_rules"`
	ProxyKeys           string              `json:"proxy_keys"`
	ModelMappings       json.RawMessage     `json:"model_mappings"`
}

// CreateGroup 处理创建新组
func (s *Server) CreateGroup(c *gin.Context) {
	var req GroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	params := services.GroupCreateParams{
		Name:                req.Name,
		DisplayName:         req.DisplayName,
		Description:         req.Description,
		GroupType:           req.GroupType,
		Upstreams:           req.Upstreams,
		ChannelType:         req.ChannelType,
		Sort:                req.Sort,
		TestModel:           req.TestModel,
		ValidationEndpoint:  req.ValidationEndpoint,
		ParamOverrides:      req.ParamOverrides,
		ModelRedirectRules:  req.ModelRedirectRules,
		ModelRedirectStrict: req.ModelRedirectStrict,
		Config:              req.Config,
		HeaderRules:         req.HeaderRules,
		ProxyKeys:           req.ProxyKeys,
		ModelMappings:       req.ModelMappings,
	}

	group, err := s.GroupService.CreateGroup(c.Request.Context(), params)
	if s.handleGroupError(c, err) {
		return
	}

	response.Success(c, s.newGroupResponse(group))
}

// ListGroups 处理列出所有组
func (s *Server) ListGroups(c *gin.Context) {
	groups, err := s.GroupService.ListGroups(c.Request.Context())
	if s.handleGroupError(c, err) {
		return
	}

	groupResponses := make([]GroupResponse, 0, len(groups))
	for i := range groups {
		groupResponses = append(groupResponses, *s.newGroupResponse(&groups[i]))
	}

	response.Success(c, groupResponses)
}

// GroupUpdateRequest 定义更新组的载荷
// 使用专用结构体可避免 GORM 的 Update 忽略零值的问题
type GroupUpdateRequest struct {
	Name                *string             `json:"name,omitempty"`
	DisplayName         *string             `json:"display_name,omitempty"`
	Description         *string             `json:"description,omitempty"`
	GroupType           *string             `json:"group_type,omitempty"`
	Upstreams           json.RawMessage     `json:"upstreams"`
	ChannelType         *string             `json:"channel_type,omitempty"`
	Sort                *int                `json:"sort"`
	TestModel           string              `json:"test_model"`
	ValidationEndpoint  *string             `json:"validation_endpoint,omitempty"`
	ParamOverrides      map[string]any      `json:"param_overrides"`
	ModelRedirectRules  map[string]string   `json:"model_redirect_rules"`
	ModelRedirectStrict *bool               `json:"model_redirect_strict"`
	ModelMappingStrict  *bool               `json:"model_mapping_strict"`
	Config              map[string]any      `json:"config"`
	HeaderRules         []models.HeaderRule `json:"header_rules"`
	ProxyKeys           *string             `json:"proxy_keys,omitempty"`
	ModelMappings       json.RawMessage     `json:"model_mappings"`
}

// UpdateGroup 处理更新现有组
func (s *Server) UpdateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.invalid_group_id")
		return
	}

	var req GroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	params := services.GroupUpdateParams{
		Name:                req.Name,
		DisplayName:         req.DisplayName,
		Description:         req.Description,
		GroupType:           req.GroupType,
		ChannelType:         req.ChannelType,
		Sort:                req.Sort,
		ValidationEndpoint:  req.ValidationEndpoint,
		ParamOverrides:      req.ParamOverrides,
		ModelRedirectRules:  req.ModelRedirectRules,
		ModelRedirectStrict: req.ModelRedirectStrict,
		ModelMappingStrict:  req.ModelMappingStrict,
		Config:              req.Config,
		ProxyKeys:           req.ProxyKeys,
	}

	if req.Upstreams != nil {
		params.Upstreams = req.Upstreams
		params.HasUpstreams = true
	}

	if req.TestModel != "" {
		params.TestModel = req.TestModel
		params.HasTestModel = true
	}

	if req.HeaderRules != nil {
		rules := req.HeaderRules
		params.HeaderRules = &rules
	}

	if req.ModelMappings != nil {
		params.ModelMappings = req.ModelMappings
		params.HasModelMappings = true
	}

	group, err := s.GroupService.UpdateGroup(c.Request.Context(), uint(id), params)
	if s.handleGroupError(c, err) {
		return
	}

	response.Success(c, s.newGroupResponse(group))
}

// GroupResponse 定义组响应的结构，排除敏感或大型字段
type GroupResponse struct {
	ID                  uint                      `json:"id"`
	Name                string                    `json:"name"`
	Endpoint            string                    `json:"endpoint"`
	DisplayName         string                    `json:"display_name"`
	Description         string                    `json:"description"`
	GroupType           string                    `json:"group_type"`
	Upstreams           datatypes.JSON            `json:"upstreams"`
	ChannelType         string                    `json:"channel_type"`
	Sort                int                       `json:"sort"`
	TestModel           string                    `json:"test_model"`
	ValidationEndpoint  string                    `json:"validation_endpoint"`
	ParamOverrides      datatypes.JSONMap         `json:"param_overrides"`
	ModelRedirectRules  datatypes.JSONMap         `json:"model_redirect_rules"`
	ModelRedirectStrict bool                      `json:"model_redirect_strict"`
	ModelMappingStrict  bool                      `json:"model_mapping_strict"`
	Config              datatypes.JSONMap         `json:"config"`
	HeaderRules         []models.HeaderRule       `json:"header_rules"`
	ProxyKeys           string                    `json:"proxy_keys"`
	ModelMappingList    []models.ModelMapping     `json:"model_mappings_list"`
	LastValidatedAt     *time.Time                `json:"last_validated_at"`
	CreatedAt           time.Time                 `json:"created_at"`
	UpdatedAt           time.Time                 `json:"updated_at"`
}

// newGroupResponse 从 models.Group 创建新的 GroupResponse
func (s *Server) newGroupResponse(group *models.Group) *GroupResponse {
	appURL := s.SettingsManager.GetAppUrl()
	endpoint := ""
	if appURL != "" {
		u, err := url.Parse(appURL)
		if err == nil {
			u.Path = strings.TrimRight(u.Path, "/") + "/proxy/" + group.Name
			endpoint = u.String()
		}
	}

	// 从 JSON 解析头部规则
	var headerRules []models.HeaderRule
	if len(group.HeaderRules) > 0 {
		if err := json.Unmarshal(group.HeaderRules, &headerRules); err != nil {
			logrus.WithError(err).Error("Failed to unmarshal header rules")
			headerRules = make([]models.HeaderRule, 0)
		}
	}

	return &GroupResponse{
		ID:                  group.ID,
		Name:                group.Name,
		Endpoint:            endpoint,
		DisplayName:         group.DisplayName,
		Description:         group.Description,
		GroupType:           group.GroupType,
		Upstreams:           group.Upstreams,
		ChannelType:         group.ChannelType,
		Sort:                group.Sort,
		TestModel:           group.TestModel,
		ValidationEndpoint:  group.ValidationEndpoint,
		ParamOverrides:      group.ParamOverrides,
		ModelRedirectRules:  group.ModelRedirectRules,
		ModelRedirectStrict: group.ModelRedirectStrict,
		ModelMappingStrict:  group.ModelMappingStrict,
		Config:              group.Config,
		HeaderRules:         headerRules,
		ProxyKeys:           group.ProxyKeys,
		ModelMappingList:    group.ModelMappingList,
		LastValidatedAt:     group.LastValidatedAt,
		CreatedAt:           group.CreatedAt,
		UpdatedAt:           group.UpdatedAt,
	}
}

// DeleteGroup 处理删除组
func (s *Server) DeleteGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.invalid_group_id")
		return
	}

	if s.handleGroupError(c, s.GroupService.DeleteGroup(c.Request.Context(), uint(id))) {
		return
	}
	response.SuccessI18n(c, "success.group_deleted", nil)
}

// ConfigOption 表示组的单个可配置选项
type ConfigOption struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	DefaultValue any    `json:"default_value"`
}

// GetGroupConfigOptions 返回组的可用配置选项列表
func (s *Server) GetGroupConfigOptions(c *gin.Context) {
	options, err := s.GroupService.GetGroupConfigOptions()
	if s.handleGroupError(c, err) {
		return
	}

	translated := make([]ConfigOption, 0, len(options))
	for _, option := range options {
		name := option.Name
		if strings.HasPrefix(name, "config.") {
			name = i18n.Message(c, name)
		}
		description := option.Description
		if strings.HasPrefix(description, "config.") {
			description = i18n.Message(c, description)
		}

		translated = append(translated, ConfigOption{
			Key:          option.Key,
			Name:         name,
			Description:  description,
			DefaultValue: option.DefaultValue,
		})
	}

	response.Success(c, translated)
}

// calculateRequestStats 是计算请求统计数据的辅助函数
func (s *Server) GetGroupStats(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.invalid_group_id")
		return
	}

	stats, err := s.GroupService.GetGroupStats(c.Request.Context(), uint(id))
	if s.handleGroupError(c, err) {
		return
	}

	response.Success(c, stats)
}

// GroupCopyRequest 定义复制组的载荷
type GroupCopyRequest struct {
	CopyKeys string `json:"copy_keys"` // "none"|"valid_only"|"all"
}

// GroupCopyResponse 定义组复制操作的响应
type GroupCopyResponse struct {
	Group *GroupResponse `json:"group"`
}

// CopyGroup 处理复制组及其可选内容

func (s *Server) CopyGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.invalid_group_id")
		return
	}

	var req GroupCopyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	newGroup, err := s.GroupService.CopyGroup(c.Request.Context(), uint(id), req.CopyKeys)
	if s.handleGroupError(c, err) {
		return
	}

	groupResponse := s.newGroupResponse(newGroup)
	copyResponse := &GroupCopyResponse{
		Group: groupResponse,
	}

	response.Success(c, copyResponse)
}

// List 列出组
func (s *Server) List(c *gin.Context) {
	var groups []models.Group
	if err := s.DB.Select("id, name,display_name").Find(&groups).Error; err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrDatabase, "database.cannot_get_groups")
		return
	}
	response.Success(c, groups)
}

// AddSubGroupsRequest 定义向聚合组添加子组的载荷
type AddSubGroupsRequest struct {
	SubGroups []services.SubGroupInput `json:"sub_groups"`
}

// UpdateSubGroupWeightRequest 定义更新子组权重的载荷
type UpdateSubGroupWeightRequest struct {
	Weight int `json:"weight"`
}

// GetSubGroups 处理获取聚合组的子组
func (s *Server) GetSubGroups(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.invalid_group_id")
		return
	}

	subGroups, err := s.AggregateGroupService.GetSubGroups(c.Request.Context(), uint(id))
	if s.handleGroupError(c, err) {
		return
	}

	response.Success(c, subGroups)
}

// AddSubGroups 处理向聚合组添加子组
func (s *Server) AddSubGroups(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.invalid_group_id")
		return
	}

	var req AddSubGroupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if err := s.AggregateGroupService.AddSubGroups(c.Request.Context(), uint(id), req.SubGroups); s.handleGroupError(c, err) {
		return
	}

	response.SuccessI18n(c, "success.sub_groups_added", nil)
}

// UpdateSubGroupWeight 处理更新子组的权重
func (s *Server) UpdateSubGroupWeight(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.invalid_group_id")
		return
	}

	subGroupID, err := strconv.Atoi(c.Param("subGroupId"))
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.invalid_sub_group_id")
		return
	}

	var req UpdateSubGroupWeightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if err := s.AggregateGroupService.UpdateSubGroupWeight(c.Request.Context(), uint(id), uint(subGroupID), req.Weight); s.handleGroupError(c, err) {
		return
	}

	response.SuccessI18n(c, "success.sub_group_weight_updated", nil)
}

// DeleteSubGroup 处理从聚合组删除子组
func (s *Server) DeleteSubGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.invalid_group_id")
		return
	}

	subGroupID, err := strconv.Atoi(c.Param("subGroupId"))
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.invalid_sub_group_id")
		return
	}

	if err := s.AggregateGroupService.DeleteSubGroup(c.Request.Context(), uint(id), uint(subGroupID)); s.handleGroupError(c, err) {
		return
	}

	response.SuccessI18n(c, "success.sub_group_deleted", nil)
}

// GetParentAggregateGroups 处理获取引用某个组的父聚合组
func (s *Server) GetParentAggregateGroups(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.invalid_group_id")
		return
	}

	parentGroups, err := s.AggregateGroupService.GetParentAggregateGroups(c.Request.Context(), uint(id))
	if s.handleGroupError(c, err) {
		return
	}

	response.Success(c, parentGroups)
}
