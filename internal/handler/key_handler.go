package handler

import (
	"errors"
	"fmt"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/keypool"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// handleKeyServiceError 使用一致的响应处理常见的密钥服务错误
func handleKeyServiceError(c *gin.Context, err error) {
	if errors.Is(err, app_errors.ErrBatchSizeExceedsLimit) || errors.Is(err, app_errors.ErrNoValidKeysFound) {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, err.Error()))
		return
	}
	response.Error(c, app_errors.ParseDBError(err))
}

// parseUintParam 解析字符串参数为 uint
// 返回解析后的 uint 和 true，或解析失败时返回 0 和 false（错误已发送给客户端）
func parseUintParam(c *gin.Context, value string, i18nKey string) (uint, bool) {
	if value == "" {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, i18nKey)
		return 0, false
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, i18nKey)
		return 0, false
	}

	return uint(parsed), true
}

// validateGroupIDFromQuery 验证并解析来自查询参数的组 ID
// 如果验证失败返回 0 和 false（错误已发送给客户端）
func validateGroupIDFromQuery(c *gin.Context) (uint, bool) {
	return parseUintParam(c, c.Query("group_id"), "validation.group_id_required")
}

// validateKeysText 验证密钥文本输入
// 如果验证失败返回 false（错误已发送给客户端）
func validateKeysText(c *gin.Context, keysText string) bool {
	if strings.TrimSpace(keysText) == "" {
		response.ErrorI18nFromAPIError(c, app_errors.ErrValidation, "validation.keys_text_empty")
		return false
	}

	return true
}

// findGroupByID 是根据 ID 查找组的辅助函数
func (s *Server) findGroupByID(c *gin.Context, groupID uint) (*models.Group, bool) {
	var group models.Group
	if err := s.DB.First(&group, groupID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, app_errors.ErrResourceNotFound)
		} else {
			response.Error(c, app_errors.ParseDBError(err))
		}
		return nil, false
	}
	return &group, true
}

// KeyTextRequest 定义需要组 ID 和密钥文本块操作的通用载荷
type KeyTextRequest struct {
	GroupID  uint   `json:"group_id" binding:"required"`
	KeysText string `json:"keys_text" binding:"required"`
	Model    string `json:"model"` // 可选的模型覆盖用于测试
}

// GroupIDRequest 定义只需要组 ID 的操作的通用载荷
type GroupIDRequest struct {
	GroupID uint `json:"group_id" binding:"required"`
}

// ValidateGroupKeysRequest 定义验证组内密钥的载荷
type ValidateGroupKeysRequest struct {
	GroupID uint   `json:"group_id" binding:"required"`
	Status  string `json:"status,omitempty"`
}

// AddMultipleKeys 处理在特定组中从文本块创建新密钥
func (s *Server) AddMultipleKeys(c *gin.Context) {
	var req KeyTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if _, ok := s.findGroupByID(c, req.GroupID); !ok {
		return
	}

	if !validateKeysText(c, req.KeysText) {
		return
	}

	result, err := s.KeyService.AddMultipleKeys(req.GroupID, req.KeysText)
	if err != nil {
		handleKeyServiceError(c, err)
		return
	}

	response.Success(c, result)
}

// AddMultipleKeysAsync 处理在特定组中从文本块或文件创建新密钥
func (s *Server) AddMultipleKeysAsync(c *gin.Context) {
	var groupID uint
	var keysText string

	// 检查内容类型以确定是文件上传还是 JSON 请求
	contentType := c.ContentType()

	if strings.Contains(contentType, "multipart/form-data") {
		parsedGroupID, ok := parseUintParam(c, c.PostForm("group_id"), "validation.group_id_required")
		if !ok {
			return
		}
		groupID = parsedGroupID

		// 获取上传的文件
		file, err := c.FormFile("file")
		if err != nil {
			response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.file_required")
			return
		}

		// 验证文件扩展名
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".txt" {
			response.ErrorI18nFromAPIError(c, app_errors.ErrValidation, "validation.only_txt_supported")
			return
		}

		// 读取文件内容
		fileContent, err := file.Open()
		if err != nil {
			response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.failed_to_open_file")
			return
		}
		defer fileContent.Close()

		// 使用 io.ReadAll 将文件内容读取为字符串
		buf, err := io.ReadAll(fileContent)
		if err != nil {
			response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.failed_to_read_file")
			return
		}
		keysText = string(buf)
	} else {
		// 处理 JSON 请求（原始行为）
		var req KeyTextRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
			return
		}
		groupID = req.GroupID
		keysText = req.KeysText
	}

	group, ok := s.findGroupByID(c, groupID)
	if !ok {
		return
	}

	if !validateKeysText(c, keysText) {
		return
	}

	taskStatus, err := s.KeyImportService.StartImportTask(group, keysText)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrTaskInProgress, err.Error()))
		return
	}

	response.Success(c, taskStatus)
}

// ListKeysInGroup 处理列出特定组内的所有密钥，支持分页
func (s *Server) ListKeysInGroup(c *gin.Context) {
	groupID, ok := validateGroupIDFromQuery(c)
	if !ok {
		return
	}

	if _, ok := s.findGroupByID(c, groupID); !ok {
		return
	}

	statusFilter := c.Query("status")
	if statusFilter != "" && statusFilter != models.KeyStatusActive && statusFilter != models.KeyStatusInvalid {
		response.ErrorI18nFromAPIError(c, app_errors.ErrValidation, "validation.invalid_status_filter")
		return
	}

	searchKeyword := c.Query("key_value")
	searchHash := ""
	if searchKeyword != "" {
		searchHash = s.EncryptionSvc.Hash(searchKeyword)
	}

	query := s.KeyService.ListKeysInGroupQuery(groupID, statusFilter, searchHash)

	var keys []models.APIKey
	paginatedResult, err := response.Paginate(c, query, &keys)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// 解密所有密钥用于显示
	for i := range keys {
		decryptedValue, err := s.EncryptionSvc.Decrypt(keys[i].KeyValue)
		if err != nil {
			logrus.WithError(err).WithField("key_id", keys[i].ID).Error("Failed to decrypt key value for listing")
			keys[i].KeyValue = "failed-to-decrypt"
		} else {
			keys[i].KeyValue = decryptedValue
		}
	}
	paginatedResult.Items = keys

	response.Success(c, paginatedResult)
}

// DeleteMultipleKeys 处理从特定组的文本块中删除密钥
func (s *Server) DeleteMultipleKeys(c *gin.Context) {
	var req KeyTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if _, ok := s.findGroupByID(c, req.GroupID); !ok {
		return
	}

	if !validateKeysText(c, req.KeysText) {
		return
	}

	result, err := s.KeyService.DeleteMultipleKeys(req.GroupID, req.KeysText)
	if err != nil {
		handleKeyServiceError(c, err)
		return
	}

	response.Success(c, result)
}

// DeleteMultipleKeysAsync 处理使用异步任务从特定组的文本块中删除密钥
func (s *Server) DeleteMultipleKeysAsync(c *gin.Context) {
	var req KeyTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	group, ok := s.findGroupByID(c, req.GroupID)
	if !ok {
		return
	}

	if !validateKeysText(c, req.KeysText) {
		return
	}

	taskStatus, err := s.KeyDeleteService.StartDeleteTask(group, req.KeysText)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrTaskInProgress, err.Error()))
		return
	}

	response.Success(c, taskStatus)
}

// RestoreMultipleKeys 处理从特定组的文本块中恢复密钥
func (s *Server) RestoreMultipleKeys(c *gin.Context) {
	var req KeyTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if _, ok := s.findGroupByID(c, req.GroupID); !ok {
		return
	}

	if !validateKeysText(c, req.KeysText) {
		return
	}

	result, err := s.KeyService.RestoreMultipleKeys(req.GroupID, req.KeysText)
	if err != nil {
		handleKeyServiceError(c, err)
		return
	}

	response.Success(c, result)
}

// TestMultipleKeys 处理多个密钥的一次性验证测试
func (s *Server) TestMultipleKeys(c *gin.Context) {
	var req KeyTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	groupDB, ok := s.findGroupByID(c, req.GroupID)
	if !ok {
		return
	}

	group, err := s.GroupManager.GetGroupByName(groupDB.Name)
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrResourceNotFound, "validation.group_not_found")
		return
	}

	if !validateKeysText(c, req.KeysText) {
		return
	}

	start := time.Now()
	results, err := s.KeyService.TestMultipleKeys(group, req.KeysText, req.Model)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		handleKeyServiceError(c, err)
		return
	}

	response.Success(c, gin.H{
		"results":        results,
		"total_duration": duration,
	})
}

// TestNextKeyRequest 定义使用轮询测试下一个密钥的载荷
type TestNextKeyRequest struct {
	GroupID uint   `json:"group_id" binding:"required"`
	Model   string `json:"model"` // 可选的模型覆盖用于测试
}

// TestNextKey 处理使用轮询选择测试池中的下一个密钥
func (s *Server) TestNextKey(c *gin.Context) {
	var req TestNextKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	groupDB, ok := s.findGroupByID(c, req.GroupID)
	if !ok {
		return
	}

	group, err := s.GroupManager.GetGroupByName(groupDB.Name)
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrResourceNotFound, "validation.group_not_found")
		return
	}

	start := time.Now()

	// 使用轮询机制选择下一个密钥
	apiKey, err := s.KeyService.KeyProvider.SelectKey(group.ID)
	if err != nil {
		response.Error(c, app_errors.ErrNoActiveKeys)
		return
	}

	// 执行验证
	isValid, validationErr := s.KeyService.KeyValidator.ValidateSingleKey(apiKey, group, req.Model)

	duration := time.Since(start).Milliseconds()

	result := &keypool.KeyTestResult{
		KeyValue: apiKey.KeyValue,
		IsValid:  isValid,
		Error:    "",
	}
	if validationErr != nil {
		result.Error = validationErr.Error()
	}

	response.Success(c, gin.H{
		"result":         result,
		"total_duration": duration,
	})
}

// ValidateGroupKeys 启动组内所有密钥的手动验证任务
func (s *Server) ValidateGroupKeys(c *gin.Context) {
	var req ValidateGroupKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// 如果提供则验证状态
	if req.Status != "" && req.Status != models.KeyStatusActive && req.Status != models.KeyStatusInvalid {
		response.ErrorI18nFromAPIError(c, app_errors.ErrValidation, "validation.invalid_status_value")
		return
	}

	groupDB, ok := s.findGroupByID(c, req.GroupID)
	if !ok {
		return
	}

	group, err := s.GroupManager.GetGroupByName(groupDB.Name)
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrResourceNotFound, "validation.group_not_found")
		return
	}

	taskStatus, err := s.KeyManualValidationService.StartValidationTask(group, req.Status)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrTaskInProgress, err.Error()))
		return
	}

	response.Success(c, taskStatus)
}

// RestoreAllInvalidKeys 将组中所有 'inactive' 密钥的状态设置为 'active'
func (s *Server) RestoreAllInvalidKeys(c *gin.Context) {
	var req GroupIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if _, ok := s.findGroupByID(c, req.GroupID); !ok {
		return
	}

	rowsAffected, err := s.KeyService.RestoreAllInvalidKeys(req.GroupID)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.SuccessI18n(c, "success.keys_restored", nil, map[string]any{"count": rowsAffected})
}

// ClearAllInvalidKeys 从组中删除所有 'inactive' 密钥
func (s *Server) ClearAllInvalidKeys(c *gin.Context) {
	var req GroupIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if _, ok := s.findGroupByID(c, req.GroupID); !ok {
		return
	}

	rowsAffected, err := s.KeyService.ClearAllInvalidKeys(req.GroupID)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.SuccessI18n(c, "success.invalid_keys_cleared", nil, map[string]any{"count": rowsAffected})
}

// ClearAllKeys 从组中删除所有密钥
func (s *Server) ClearAllKeys(c *gin.Context) {
	var req GroupIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if _, ok := s.findGroupByID(c, req.GroupID); !ok {
		return
	}

	rowsAffected, err := s.KeyService.ClearAllKeys(req.GroupID)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.SuccessI18n(c, "success.all_keys_cleared", nil, map[string]any{"count": rowsAffected})
}

// ExportKeys 处理将密钥导出到文本文件
func (s *Server) ExportKeys(c *gin.Context) {
	groupID, ok := validateGroupIDFromQuery(c)
	if !ok {
		return
	}

	statusFilter := c.Query("status")
	if statusFilter == "" {
		statusFilter = "all"
	}

	switch statusFilter {
	case "all", models.KeyStatusActive, models.KeyStatusInvalid:
	default:
		response.ErrorI18nFromAPIError(c, app_errors.ErrValidation, "validation.invalid_status_filter")
		return
	}

	group, ok := s.findGroupByID(c, groupID)
	if !ok {
		return
	}

	filename := fmt.Sprintf("keys-%s-%s.txt", group.Name, statusFilter)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "text/plain; charset=utf-8")

	if err := s.KeyService.StreamKeysToWriter(groupID, statusFilter, c.Writer); err != nil {
		logrus.WithError(err).Error("Failed to stream keys")
	}
}

// UpdateKeyNotesRequest 定义更新密钥备注的载荷
type UpdateKeyNotesRequest struct {
	Notes string `json:"notes"`
}

// UpdateKeyNotes 处理更新特定 API 密钥的备注
func (s *Server) UpdateKeyNotes(c *gin.Context) {
	keyID, ok := parseUintParam(c, c.Param("id"), "validation.invalid_id_format")
	if !ok {
		return
	}

	var req UpdateKeyNotesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// 规范化并明确强制执行长度
	req.Notes = strings.TrimSpace(req.Notes)
	if utf8.RuneCountInString(req.Notes) > 255 {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "notes length must be <= 255 characters"))
		return
	}

	// 检查密钥是否存在并更新其备注
	var key models.APIKey
	if err := s.DB.First(&key, keyID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, app_errors.ErrResourceNotFound)
		} else {
			response.Error(c, app_errors.ParseDBError(err))
		}
		return
	}

	// 更新备注
	if err := s.DB.Model(&key).Update("notes", req.Notes).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, nil)
}
