package handler

import (
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/i18n"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"gpt-load/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GetSettings 处理 GET /api/settings 请求
// 它检索所有系统设置，按类别分组，并返回它们
func (s *Server) GetSettings(c *gin.Context) {
	currentSettings := s.SettingsManager.GetSettings()
	settingsInfo := utils.GenerateSettingsMetadata(&currentSettings)

	// 翻译设置信息
	for i := range settingsInfo {
		// 如果是 i18n 键则翻译名称
		if strings.HasPrefix(settingsInfo[i].Name, "config.") {
			settingsInfo[i].Name = i18n.Message(c, settingsInfo[i].Name)
		}
		// 如果是 i18n 键则翻译描述
		if strings.HasPrefix(settingsInfo[i].Description, "config.") {
			settingsInfo[i].Description = i18n.Message(c, settingsInfo[i].Description)
		}
		// 如果是 i18n 键则翻译类别
		if strings.HasPrefix(settingsInfo[i].Category, "config.") {
			settingsInfo[i].Category = i18n.Message(c, settingsInfo[i].Category)
		}
	}

	// 按类别分组设置，同时保持顺序
	categorized := make(map[string][]models.SystemSettingInfo)
	var categoryOrder []string
	for _, s := range settingsInfo {
		if _, exists := categorized[s.Category]; !exists {
			categoryOrder = append(categoryOrder, s.Category)
		}
		categorized[s.Category] = append(categorized[s.Category], s)
	}

	// 按正确顺序创建响应结构
	var responseData []models.CategorizedSettings
	for _, categoryName := range categoryOrder {
		responseData = append(responseData, models.CategorizedSettings{
			CategoryName: categoryName,
			Settings:     categorized[categoryName],
		})
	}

	response.Success(c, responseData)
}

// UpdateSettings 处理 PUT /api/settings 请求
func (s *Server) UpdateSettings(c *gin.Context) {
	var settingsMap map[string]any
	if err := c.ShouldBindJSON(&settingsMap); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if len(settingsMap) == 0 {
		response.Success(c, nil)
		return
	}

	// 清理 proxy_keys 输入
	if proxyKeys, ok := settingsMap["proxy_keys"]; ok {
		if proxyKeysStr, ok := proxyKeys.(string); ok {
			cleanedKeys := utils.SplitAndTrim(proxyKeysStr, ",")
			settingsMap["proxy_keys"] = strings.Join(cleanedKeys, ",")
		}
	}

	// 更新配置
	if err := s.SettingsManager.UpdateSettings(settingsMap); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrDatabase, err.Error()))
		return
	}

	time.Sleep(100 * time.Millisecond) // 等待异步配置更新

	response.SuccessI18n(c, "settings.update_success", nil)
}
