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

// handleKeyServiceError handles common key service errors with consistent response.
func handleKeyServiceError(c *gin.Context, err error) {
	if errors.Is(err, app_errors.ErrBatchSizeExceedsLimit) || errors.Is(err, app_errors.ErrNoValidKeysFound) {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, err.Error()))
		return
	}
	response.Error(c, app_errors.ParseDBError(err))
}

// parseUintParam parses a uint from a string parameter.
// Returns the parsed uint and true, or 0 and false if parsing fails (error is already sent to client).
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

// validateGroupIDFromQuery validates and parses group ID from a query parameter.
// Returns 0 and false if validation fails (error is already sent to client)
func validateGroupIDFromQuery(c *gin.Context) (uint, bool) {
	return parseUintParam(c, c.Query("group_id"), "validation.group_id_required")
}

// validateKeysText validates the keys text input
// Returns false if validation fails (error is already sent to client)
func validateKeysText(c *gin.Context, keysText string) bool {
	if strings.TrimSpace(keysText) == "" {
		response.ErrorI18nFromAPIError(c, app_errors.ErrValidation, "validation.keys_text_empty")
		return false
	}

	return true
}

// findGroupByID is a helper function to find a group by its ID.
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

// KeyTextRequest defines a generic payload for operations requiring a group ID and a text block of keys.
type KeyTextRequest struct {
	GroupID  uint   `json:"group_id" binding:"required"`
	KeysText string `json:"keys_text" binding:"required"`
	Model    string `json:"model"` // Optional model override for testing
}

// GroupIDRequest defines a generic payload for operations requiring only a group ID.
type GroupIDRequest struct {
	GroupID uint `json:"group_id" binding:"required"`
}

// ValidateGroupKeysRequest defines the payload for validating keys in a group.
type ValidateGroupKeysRequest struct {
	GroupID uint   `json:"group_id" binding:"required"`
	Status  string `json:"status,omitempty"`
}

// AddMultipleKeys handles creating new keys from a text block within a specific group.
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

// AddMultipleKeysAsync handles creating new keys from a text block or file within a specific group.
func (s *Server) AddMultipleKeysAsync(c *gin.Context) {
	var groupID uint
	var keysText string

	// Check content type to determine if it's a file upload or JSON request
	contentType := c.ContentType()

	if strings.Contains(contentType, "multipart/form-data") {
		parsedGroupID, ok := parseUintParam(c, c.PostForm("group_id"), "validation.group_id_required")
		if !ok {
			return
		}
		groupID = parsedGroupID

		// Get uploaded file
		file, err := c.FormFile("file")
		if err != nil {
			response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.file_required")
			return
		}

		// Validate file extension
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".txt" {
			response.ErrorI18nFromAPIError(c, app_errors.ErrValidation, "validation.only_txt_supported")
			return
		}

		// Read file content
		fileContent, err := file.Open()
		if err != nil {
			response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.failed_to_open_file")
			return
		}
		defer fileContent.Close()

		// Read file content as string using io.ReadAll
		buf, err := io.ReadAll(fileContent)
		if err != nil {
			response.ErrorI18nFromAPIError(c, app_errors.ErrBadRequest, "validation.failed_to_read_file")
			return
		}
		keysText = string(buf)
	} else {
		// Handle JSON request (original behavior)
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

// ListKeysInGroup handles listing all keys within a specific group with pagination.
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

	// Decrypt all keys for display
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

// DeleteMultipleKeys handles deleting keys from a text block within a specific group.
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

// DeleteMultipleKeysAsync handles deleting keys from a text block within a specific group using async task.
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

// RestoreMultipleKeys handles restoring keys from a text block within a specific group.
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

// TestMultipleKeys handles a one-off validation test for multiple keys.
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

// TestNextKeyRequest defines the payload for testing the next key using round-robin.
type TestNextKeyRequest struct {
	GroupID uint   `json:"group_id" binding:"required"`
	Model   string `json:"model"` // Optional model override for testing
}

// TestNextKey handles testing the next key from the pool using round-robin selection.
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

	// Use round-robin mechanism to select next key
	apiKey, err := s.KeyService.KeyProvider.SelectKey(group.ID)
	if err != nil {
		response.Error(c, app_errors.ErrNoActiveKeys)
		return
	}

	// Execute validation
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

// ValidateGroupKeys initiates a manual validation task for all keys in a group.
func (s *Server) ValidateGroupKeys(c *gin.Context) {
	var req ValidateGroupKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	// Validate status if provided
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

// RestoreAllInvalidKeys sets the status of all 'inactive' keys in a group to 'active'.
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

// ClearAllInvalidKeys deletes all 'inactive' keys from a group.
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

// ClearAllKeys deletes all keys from a group.
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

// ExportKeys handles exporting keys to a text file.
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

// UpdateKeyNotesRequest defines the payload for updating a key's notes.
type UpdateKeyNotesRequest struct {
	Notes string `json:"notes"`
}

// UpdateKeyNotes handles updating the notes of a specific API key.
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

	// Normalize and enforce length explicitly
	req.Notes = strings.TrimSpace(req.Notes)
	if utf8.RuneCountInString(req.Notes) > 255 {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "notes length must be <= 255 characters"))
		return
	}

	// Check if the key exists and update its notes
	var key models.APIKey
	if err := s.DB.First(&key, keyID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, app_errors.ErrResourceNotFound)
		} else {
			response.Error(c, app_errors.ParseDBError(err))
		}
		return
	}

	// Update notes
	if err := s.DB.Model(&key).Update("notes", req.Notes).Error; err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, nil)
}
