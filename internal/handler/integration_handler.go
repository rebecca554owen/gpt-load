package handler

import (
	"strings"

	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"gpt-load/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// IntegrationGroupInfo 表示集成响应的组信息
type IntegrationGroupInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	ChannelType string `json:"channel_type"`
	Path        string `json:"path"`
}

// IntegrationInfoResponse 表示集成信息响应
type IntegrationInfoResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    []IntegrationGroupInfo `json:"data"`
}

// GetIntegrationInfo 处理集成信息请求
func (s *Server) GetIntegrationInfo(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Proxy key is required"))
		return
	}

	path := c.Request.URL.Path
	isGroupSpecific := strings.HasPrefix(path, "/proxy/")

	var groupsToCheck []*models.Group

	if isGroupSpecific {
		parts := strings.Split(strings.TrimPrefix(path, "/proxy/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Invalid group path"))
			return
		}

		groupName := parts[0]

		// 从 GroupManager 缓存获取组（已解析 ProxyKeysMap）
		group, err := s.GroupManager.GetGroupByName(groupName)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrResourceNotFound, "Group not found"))
			return
		}

		groupsToCheck = []*models.Group{group}
	} else {
		// 获取所有组
		groups, err := s.GroupService.ListGroups(c.Request.Context())
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, "Internal server error"))
			return
		}

		// 转换为指针切片并从缓存加载以获取 ProxyKeysMap
		for i := range groups {
			cachedGroup, err := s.GroupManager.GetGroupByName(groups[i].Name)
			if err != nil {
				logrus.Warnf("Failed to get group %s from cache: %v", groups[i].Name, err)
				continue
			}
			groupsToCheck = append(groupsToCheck, cachedGroup)
		}
	}

	var result []IntegrationGroupInfo
	for _, group := range groupsToCheck {
		if hasProxyKeyPermission(group, key) {
			channelType := getEffectiveChannelType(group)
			path := buildPath(isGroupSpecific, group.Name, channelType, group.ValidationEndpoint)

			result = append(result, IntegrationGroupInfo{
				Name:        group.Name,
				DisplayName: group.DisplayName,
				ChannelType: channelType,
				Path:        path,
			})
		}
	}

	if len(result) == 0 {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "Invalid or unauthorized proxy key"))
		return
	}

	response.Success(c, result)
}

// getEffectiveChannelType 返回有效的通道类型
func getEffectiveChannelType(group *models.Group) string {
	if group.ChannelType != "openai" && group.ChannelType != "openai-response" {
		return group.ChannelType
	}

	effectiveEndpoint := utils.GetValidationEndpoint(group)
	if effectiveEndpoint == "" {
		return group.ChannelType
	}

	defaultEndpoint := ""
	switch group.ChannelType {
	case "openai":
		defaultEndpoint = "/v1/chat/completions"
	case "openai-response":
		defaultEndpoint = "/v1/responses"
	}

	if effectiveEndpoint == defaultEndpoint {
		return group.ChannelType
	}

	return "custom"
}

// hasProxyKeyPermission 检查密钥是否有权限访问该组
func hasProxyKeyPermission(group *models.Group, key string) bool {
	_, exists1 := group.ProxyKeysMap[key]
	_, exists2 := group.EffectiveConfig.ProxyKeysMap[key]
	return exists1 || exists2
}

// buildPath 根据请求类型和通道类型返回适当的路径
func buildPath(isGroupSpecific bool, groupName string, channelType string, validationEndpoint string) string {
	if channelType == "custom" {
		if isGroupSpecific {
			return validationEndpoint
		}
		return "/proxy/" + groupName + validationEndpoint
	}

	if isGroupSpecific {
		return ""
	}
	return "/proxy/" + groupName
}
