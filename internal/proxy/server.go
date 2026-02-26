// Package proxy provides high-performance OpenAI multi-key proxy server
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gpt-load/internal/channel"
	"gpt-load/internal/config"
	"gpt-load/internal/encryption"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/keypool"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"gpt-load/internal/services"
	"gpt-load/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	modelFetchTimeout = 5 * time.Second
)

// ProxyServer represents the proxy server
type ProxyServer struct {
	keyProvider       *keypool.KeyProvider
	groupManager      *services.GroupManager
	subGroupManager   *services.SubGroupManager
	settingsManager   *config.SystemSettingsManager
	channelFactory    *channel.Factory
	requestLogService *services.RequestLogService
	encryptionSvc     encryption.Service
}

// NewProxyServer creates a new proxy server
func NewProxyServer(
	keyProvider *keypool.KeyProvider,
	groupManager *services.GroupManager,
	subGroupManager *services.SubGroupManager,
	settingsManager *config.SystemSettingsManager,
	channelFactory *channel.Factory,
	requestLogService *services.RequestLogService,
	encryptionSvc encryption.Service,
) (*ProxyServer, error) {
	return &ProxyServer{
		keyProvider:       keyProvider,
		groupManager:      groupManager,
		subGroupManager:   subGroupManager,
		settingsManager:   settingsManager,
		channelFactory:    channelFactory,
		requestLogService: requestLogService,
		encryptionSvc:     encryptionSvc,
	}, nil
}

// HandleProxy is the main entry point for proxy requests, refactored based on the stable .bak logic.
func (ps *ProxyServer) HandleProxy(c *gin.Context) {
	startTime := time.Now()
	groupName := c.Param("group_name")

	originalGroup, err := ps.groupManager.GetGroupByName(groupName)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	if originalGroup.GroupType == "aggregate" && ps.isModelsRequest(c.Request.URL.Path) {
		ps.handleAggregateModelsRequest(c, originalGroup)
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logrus.Errorf("Failed to read request body: %v", err)
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Failed to read request body"))
		return
	}
	c.Request.Body.Close()

	var channelHandler channel.ChannelProxy
	if originalGroup.GroupType == "aggregate" {
		// For aggregate groups, use first subgroup to extract model name
		if len(originalGroup.SubGroups) > 0 {
			tempGroup, err := ps.groupManager.GetGroupByName(originalGroup.SubGroups[0].SubGroupName)
			if err == nil {
				channelHandler, err = ps.channelFactory.GetChannel(tempGroup)
				if err != nil {
					response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to get channel for subgroup '%s': %v", tempGroup.Name, err)))
					return
				}
			}
		}
		if channelHandler == nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("No subgroups configured for aggregate group '%s'", originalGroup.Name)))
			return
		}
	} else {
		channelHandler, err = ps.channelFactory.GetChannel(originalGroup)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to get channel for group '%s': %v", originalGroup.Name, err)))
			return
		}
	}

	model := channelHandler.ExtractModel(c, bodyBytes)
	if model == "" {
		logrus.WithField("group", originalGroup.Name).Warn("Empty model name extracted from request")
	}

	// Now select subgroup based on model name (for aggregate groups)
	group := originalGroup
	var selectedModel string
	if originalGroup.GroupType == "aggregate" && model != "" {
		selection, err := ps.subGroupManager.SelectSubGroup(originalGroup, model)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"aggregate_group": originalGroup.Name,
				"model":           model,
				"error":           err,
			}).Error("Failed to select sub-group from aggregate by model")
			response.Error(c, app_errors.NewAPIError(app_errors.ErrNoKeysAvailable, "No available sub-groups"))
			return
		}
		if selection != nil {
			group, err = ps.groupManager.GetGroupByName(selection.GroupName)
			if err != nil {
				response.Error(c, app_errors.ParseDBError(err))
				return
			}
			channelHandler, err = ps.channelFactory.GetChannel(group)
			if err != nil {
				response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to get channel for sub-group '%s': %v", group.Name, err)))
				return
			}
		}
		if selection.SelectedModel != "" && selection.SelectedModel != model {
			selectedModel = selection.SelectedModel
		}
	}

	// Modify model in request body if mapping changed it
	finalBodyBytes := bodyBytes
	if selectedModel != "" {
		finalBodyBytes = utils.ModifyJSONField(bodyBytes, "model", selectedModel)
	}

	finalBodyBytes, err = ps.applyParamOverrides(finalBodyBytes, group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to apply parameter overrides: %v", err)))
		return
	}

	isStream := channelHandler.IsStreamRequest(c, bodyBytes)

	excludedSubGroupIDs := make(utils.UintSet)
	ps.executeRequestWithRetry(c, channelHandler, originalGroup, group, finalBodyBytes, isStream, startTime, 0, model, excludedSubGroupIDs)
}

// executeRequestWithRetry is the core recursive function for handling requests and retries.
// For aggregate groups, it supports cross-sub-group retry when all retries within a sub-group are exhausted.
func (ps *ProxyServer) executeRequestWithRetry(
	c *gin.Context,
	channelHandler channel.ChannelProxy,
	originalGroup *models.Group,
	group *models.Group,
	bodyBytes []byte,
	isStream bool,
	startTime time.Time,
	retryCount int,
	modelAlias string,
	excludedSubGroupIDs utils.UintSet,
) {
	cfg := group.EffectiveConfig

	apiKey, err := ps.keyProvider.SelectKey(group.ID)
	if err != nil {
		logrus.Errorf("Failed to select a key for group %s on attempt %d: %v", group.Name, retryCount+1, err)
		response.Error(c, app_errors.NewAPIError(app_errors.ErrNoKeysAvailable, err.Error()))
		ps.logRequest(c, originalGroup, group, nil, startTime, http.StatusServiceUnavailable, err, isStream, "", channelHandler, bodyBytes, nil, models.RequestTypeFinal, modelAlias)
		return
	}

	var upstreamURL string
	if originalGroup.GroupType == "aggregate" && originalGroup.ID != group.ID {
		upstreamURL, err = channelHandler.BuildUpstreamURLForAggregate(c.Request.URL, originalGroup.Name)
	} else {
		upstreamURL, err = channelHandler.BuildUpstreamURL(c.Request.URL, originalGroup.Name)
	}
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to build upstream URL: %v", err)))
		ps.logRequest(c, originalGroup, group, apiKey, startTime, http.StatusInternalServerError, err, isStream, "", channelHandler, bodyBytes, nil, models.RequestTypeFinal, modelAlias)
		return
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if isStream {
		ctx, cancel = context.WithCancel(c.Request.Context())
	} else {
		timeout := time.Duration(cfg.RequestTimeout) * time.Second
		ctx, cancel = context.WithTimeout(c.Request.Context(), timeout)
	}
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, c.Request.Method, upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		logrus.Errorf("Failed to create upstream request: %v", err)
		response.Error(c, app_errors.ErrInternalServer)
		return
	}
	req.ContentLength = int64(len(bodyBytes))

	req.Header = c.Request.Header.Clone()

	// Clean up client auth key
	req.Header.Del("Authorization")
	req.Header.Del("X-Api-Key")
	req.Header.Del("X-Goog-Api-Key")
	req.Header.Del("Api-Key")

	// Apply model redirection
	finalBodyBytes, err := channelHandler.ApplyModelRedirect(req, bodyBytes, group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, err.Error()))
		ps.logRequest(c, originalGroup, group, apiKey, startTime, http.StatusBadRequest, err, isStream, upstreamURL, channelHandler, bodyBytes, nil, models.RequestTypeFinal, modelAlias)
		return
	}

	// Update request body if it was modified by redirection
	if !bytes.Equal(finalBodyBytes, bodyBytes) {
		req.Body = io.NopCloser(bytes.NewReader(finalBodyBytes))
		req.ContentLength = int64(len(finalBodyBytes))
	}

	channelHandler.ModifyRequest(req, apiKey, group)

	// Apply custom header rules
	if len(group.HeaderRuleList) > 0 {
		headerCtx := utils.NewHeaderVariableContextFromGin(c, group, apiKey)
		utils.ApplyHeaderRules(req, group.HeaderRuleList, headerCtx)
	}

	var client *http.Client
	if isStream {
		client = channelHandler.GetStreamClient()
		req.Header.Set("X-Accel-Buffering", "no")
	} else {
		client = channelHandler.GetHTTPClient()
	}

	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}

	// Unified error handling for retries. Exclude 404 from being a retryable error.
	if err != nil || (resp != nil && resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound) {
		if err != nil && app_errors.IsIgnorableError(err) {
			logrus.Debugf("Client-side ignorable error for key %s, aborting retries: %v", utils.MaskAPIKey(apiKey.KeyValue), err)
			ps.logRequest(c, originalGroup, group, apiKey, startTime, 499, err, isStream, upstreamURL, channelHandler, bodyBytes, nil, models.RequestTypeFinal, modelAlias)
			return
		}

		var statusCode int
		var errorMessage string
		var parsedError string

		if err != nil {
			statusCode = 500
			errorMessage = err.Error()
			parsedError = errorMessage
			logrus.Debugf("Request failed (attempt %d/%d) for key %s: %v", retryCount+1, cfg.MaxRetries, utils.MaskAPIKey(apiKey.KeyValue), err)
		} else {
			// HTTP-level error (status >= 400)
			statusCode = resp.StatusCode
			errorBody, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				logrus.Errorf("Failed to read error body: %v", readErr)
				errorBody = []byte("Failed to read error body")
			}

			errorBody = handleGzipCompression(resp, errorBody)
			errorMessage = string(errorBody)
			parsedError = app_errors.ParseUpstreamError(errorBody)
			logrus.Debugf("Request failed with status %d (attempt %d/%d) for key %s. Parsed Error: %s", statusCode, retryCount+1, cfg.MaxRetries, utils.MaskAPIKey(apiKey.KeyValue), parsedError)
		}

		// Update key status using parsed error information
		ps.keyProvider.UpdateStatus(apiKey, group, false, parsedError)

		// Check if this is the last attempt
		isLastAttempt := retryCount >= cfg.MaxRetries
		requestType := models.RequestTypeRetry
		if isLastAttempt {
			requestType = models.RequestTypeFinal
		}

		ps.logRequest(c, originalGroup, group, apiKey, startTime, statusCode, errors.New(parsedError), isStream, upstreamURL, channelHandler, bodyBytes, nil, requestType, modelAlias)

		// If this is the last attempt, check if it's an aggregate group and try switching to another sub-group
		if isLastAttempt && originalGroup.GroupType == "aggregate" {
			if ps.trySwitchToAnotherSubGroup(c, originalGroup, group, modelAlias, finalBodyBytes, isStream, startTime, excludedSubGroupIDs, statusCode, errorMessage) {
				return
			}
			// All sub-groups have been tried, return error
			ps.returnUpstreamError(c, statusCode, errorMessage)
			return
		}

		// If this is the last attempt, return error directly without recursion
		if isLastAttempt {
			ps.returnUpstreamError(c, statusCode, errorMessage)
			return
		}

		ps.executeRequestWithRetry(c, channelHandler, originalGroup, group, finalBodyBytes, isStream, startTime, retryCount+1, modelAlias, excludedSubGroupIDs)
		return
	}

	// ps.keyProvider.UpdateStatus(apiKey, group, true) // No longer reset success count on successful request to reduce I/O overhead
	logrus.Debugf("Request for group %s succeeded on attempt %d with key %s", group.Name, retryCount+1, utils.MaskAPIKey(apiKey.KeyValue))

	// Check if this is a model list request (needs special handling)
	if shouldInterceptModelList(c.Request.URL.Path, c.Request.Method) {
		ps.handleModelListResponse(c, resp, group, channelHandler)
		ps.logRequest(c, originalGroup, group, apiKey, startTime, resp.StatusCode, nil, isStream, upstreamURL, channelHandler, bodyBytes, nil, models.RequestTypeFinal, modelAlias)
		return
	}

	var tokenUsage *TokenUsage
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	c.Status(resp.StatusCode)

	if isStream {
		tokenUsage = ps.handleStreamingResponse(c, resp, channelHandler)
	} else {
		tokenUsage = ps.handleNormalResponse(c, resp, channelHandler)
	}

	ps.logRequest(c, originalGroup, group, apiKey, startTime, resp.StatusCode, nil, isStream, upstreamURL, channelHandler, bodyBytes, tokenUsage, models.RequestTypeFinal, modelAlias)
}

// returnUpstreamError is a helper function to return upstream error response
func (ps *ProxyServer) returnUpstreamError(c *gin.Context, statusCode int, errorMessage string) {
	var errorJSON map[string]any
	if err := json.Unmarshal([]byte(errorMessage), &errorJSON); err == nil {
		c.JSON(statusCode, errorJSON)
	} else {
		response.Error(c, app_errors.NewAPIErrorWithUpstream(statusCode, "UPSTREAM_ERROR", errorMessage))
	}
}

// trySwitchToAnotherSubGroup attempts to switch to another sub-group for aggregate groups.
// Returns true if successfully switched and retry initiated, false otherwise.
func (ps *ProxyServer) trySwitchToAnotherSubGroup(
	c *gin.Context,
	originalGroup *models.Group,
	exhaustedGroup *models.Group,
	modelAlias string,
	bodyBytes []byte,
	isStream bool,
	startTime time.Time,
	excludedSubGroupIDs utils.UintSet,
	statusCode int,
	errorMessage string,
) bool {
	excludedSubGroupIDs.Add(exhaustedGroup.ID)

	newSelection, err := ps.subGroupManager.SelectSubGroupExcluding(originalGroup, modelAlias, excludedSubGroupIDs)
	if err != nil || newSelection == nil {
		return false
	}

	logrus.WithFields(logrus.Fields{
		"aggregate_group":    originalGroup.Name,
		"model":              modelAlias,
		"exhausted_subgroup": exhaustedGroup.Name,
		"next_subgroup":      newSelection.GroupName,
	}).Info("Sub-group exhausted, switching to next sub-group for model")

	newGroup, err := ps.groupManager.GetGroupByName(newSelection.GroupName)
	if err != nil {
		return false
	}

	newChannelHandler, err := ps.channelFactory.GetChannel(newGroup)
	if err != nil {
		return false
	}

	// Get the current model from the request body (bodyBytes may have been modified)
	var currentModel struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(bodyBytes, &currentModel); err != nil {
		return false
	}

	// Apply model mapping for the new subgroup
	updatedBodyBytes := bodyBytes
	updatedSelectedModel := newSelection.SelectedModel
	if updatedSelectedModel != "" && updatedSelectedModel != currentModel.Model {
		updatedBodyBytes = utils.ModifyJSONField(bodyBytes, "model", updatedSelectedModel)
	}

	ps.executeRequestWithRetry(c, newChannelHandler, originalGroup, newGroup, updatedBodyBytes, isStream, startTime, 0, modelAlias, excludedSubGroupIDs)
	return true
}

// logRequest is a helper function to create and record a request log.
func (ps *ProxyServer) logRequest(
	c *gin.Context,
	originalGroup *models.Group,
	group *models.Group,
	apiKey *models.APIKey,
	startTime time.Time,
	statusCode int,
	finalError error,
	isStream bool,
	upstreamAddr string,
	channelHandler channel.ChannelProxy,
	bodyBytes []byte,
	tokenUsage *TokenUsage,
	requestType string,
	originalModel string,
) {
	if ps.requestLogService == nil {
		return
	}

	var requestBodyToLog string
	userAgent := c.Request.UserAgent()

	if group.EffectiveConfig.EnableRequestBodyLogging {
		requestBodyToLog = utils.TruncateString(string(bodyBytes), 65000)
	}

	duration := time.Since(startTime).Milliseconds()

	logEntry := &models.RequestLog{
		GroupID:       group.ID,
		GroupName:     group.Name,
		IsSuccess:     finalError == nil && statusCode < 400,
		SourceIP:      c.ClientIP(),
		StatusCode:    statusCode,
		RequestPath:   utils.TruncateString(c.Request.URL.String(), 500),
		Duration:      duration,
		UserAgent:     userAgent,
		RequestType:   requestType,
		IsStream:      isStream,
		UpstreamAddr:  utils.TruncateString(upstreamAddr, 500),
		RequestBody:   requestBodyToLog,
		OriginalModel: originalModel,
	}

	// Set parent group
	if originalGroup != nil && originalGroup.GroupType == "aggregate" && originalGroup.ID != group.ID {
		logEntry.ParentGroupID = originalGroup.ID
		logEntry.ParentGroupName = originalGroup.Name
	}

	if channelHandler != nil && bodyBytes != nil {
		logEntry.Model = channelHandler.ExtractModel(c, bodyBytes)
	}

	// Set token usage if available
	if tokenUsage != nil {
		logEntry.PromptTokens = tokenUsage.PromptTokens
		logEntry.CompletionTokens = tokenUsage.CompletionTokens
		logEntry.TotalTokens = tokenUsage.Total()
		logEntry.CachedTokens = tokenUsage.CachedTokens
	}

	if apiKey != nil {
		// Encrypt key value for log storage
		encryptedKeyValue, err := ps.encryptionSvc.Encrypt(apiKey.KeyValue)
		if err != nil {
			logrus.WithError(err).Error("Failed to encrypt key value for logging")
			logEntry.KeyValue = "failed-to-encryption"
		} else {
			logEntry.KeyValue = encryptedKeyValue
		}
		// Add KeyHash for reverse lookup
		logEntry.KeyHash = ps.encryptionSvc.Hash(apiKey.KeyValue)
	}

	if finalError != nil {
		logEntry.ErrorMessage = finalError.Error()
	}

	if err := ps.requestLogService.Record(logEntry); err != nil {
		logrus.Errorf("Failed to record request log: %v", err)
	}
}

// isModelsRequest checks if the request is for /v1/models or /models endpoint
func (ps *ProxyServer) isModelsRequest(path string) bool {
	return strings.HasSuffix(path, "/v1/models") || strings.HasSuffix(path, "/models")
}

// handleAggregateModelsRequest handles /v1/models requests for aggregate groups
func (ps *ProxyServer) handleAggregateModelsRequest(c *gin.Context, aggregateGroup *models.Group) {
	allModels := make(map[string]bool)
	modelList := []string{}

	clientUserAgent := c.GetHeader("User-Agent")
	if clientUserAgent == "" {
		clientUserAgent = "claude-cli/2.0.10 (external, cli)"
	}

	upstreamModels := ps.fetchModelsFromAllSubGroups(c.Request.Context(), aggregateGroup, clientUserAgent)
	for _, model := range upstreamModels {
		if model != "" && model != "-" && !allModels[model] {
			modelList = append(modelList, model)
			allModels[model] = true
		}
	}

	if len(aggregateGroup.ModelMappingList) > 0 {
		aliasModels := []string{}
		for _, mapping := range aggregateGroup.ModelMappingList {
			if mapping.Model != "" && mapping.Model != "-" && !allModels[mapping.Model] {
				aliasModels = append(aliasModels, mapping.Model)
				allModels[mapping.Model] = true
			}
		}
		modelList = append(aliasModels, modelList...)
	}

	if len(modelList) == 0 && aggregateGroup.TestModel != "" && aggregateGroup.TestModel != "-" {
		modelList = append(modelList, aggregateGroup.TestModel)
		allModels[aggregateGroup.TestModel] = true
		logrus.WithField("group_name", aggregateGroup.Name).Info("Using test model as fallback for aggregate group")
	}

	models := make([]map[string]interface{}, len(modelList))
	for i, model := range modelList {
		models[i] = map[string]interface{}{
			"id":       model,
			"object":   "model",
			"created":  time.Now().Unix(),
			"owned_by": ps.getModelProviderForAggregate(model, aggregateGroup),
		}
	}

	response := map[string]interface{}{
		"object": "list",
		"data":   models,
	}

	c.JSON(http.StatusOK, response)
}

// fetchModelsFromAllSubGroups fetches models from all sub-groups of an aggregate group
func (ps *ProxyServer) fetchModelsFromAllSubGroups(ctx context.Context, aggregateGroup *models.Group, userAgent string) []string {
	if len(aggregateGroup.SubGroups) == 0 {
		return []string{}
	}

	type result struct {
		models []string
		err    error
	}

	const maxConcurrent = 10
	semaphore := make(chan struct{}, maxConcurrent)

	results := make(chan result, len(aggregateGroup.SubGroups))

	var wg sync.WaitGroup
	wg.Add(len(aggregateGroup.SubGroups))

	for _, subGroup := range aggregateGroup.SubGroups {
		semaphore <- struct{}{}
		go func(sg models.GroupSubGroup) {
			defer func() {
				<-semaphore
				wg.Done()
			}()

			subGroupModel, err := ps.groupManager.GetGroupByName(sg.SubGroupName)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"aggregate_group": aggregateGroup.Name,
					"sub_group":       sg.SubGroupName,
					"error":           err,
				}).Warn("Failed to get sub-group for models fetch")
				results <- result{nil, err}
				return
			}

			models, err := ps.fetchUpstreamModelsWithKey(ctx, subGroupModel, userAgent)
			results <- result{models, err}
		}(subGroup)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	allModels := make(map[string]bool)
	modelList := []string{}

	for res := range results {
		if res.err != nil {
			logrus.WithFields(logrus.Fields{
				"aggregate_group": aggregateGroup.Name,
				"error":           res.err,
			}).Debug("Failed to fetch models from sub-group")
			continue
		}

		for _, model := range res.models {
			if model != "" && model != "-" && !allModels[model] {
				modelList = append(modelList, model)
				allModels[model] = true
			}
		}
	}

	return modelList
}

// fetchUpstreamModelsWithKey fetches models from upstream using sub-group's key
func (ps *ProxyServer) fetchUpstreamModelsWithKey(ctx context.Context, group *models.Group, userAgent string) ([]string, error) {
	channelHandler, err := ps.channelFactory.GetChannel(group)
	if err != nil {
		return nil, err
	}

	apiKey, err := ps.keyProvider.SelectKey(group.ID)
	if err != nil {
		return nil, fmt.Errorf("no available keys for sub-group '%s'", group.Name)
	}

	modelsURL, err := ps.buildModelsURLForGroup(group)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, modelFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", modelsURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	channelHandler.ModifyRequest(req, apiKey, group)

	client := &http.Client{
		Timeout: modelFetchTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	var upstreamResponse struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&upstreamResponse); err != nil {
		return nil, err
	}

	models := make([]string, len(upstreamResponse.Data))
	for i, model := range upstreamResponse.Data {
		models[i] = model.ID
	}

	return models, nil
}

// buildModelsURLForGroup builds the upstream URL for models endpoint for a specific group
func (ps *ProxyServer) buildModelsURLForGroup(group *models.Group) (string, error) {
	channelHandler, err := ps.channelFactory.GetChannel(group)
	if err != nil {
		return "", err
	}

	mockURL, _ := url.Parse("http://localhost/v1/models")
	mockURL.Path = "/v1/models"

	upstreamURL, err := channelHandler.BuildUpstreamURL(mockURL, group.Name)
	if err != nil {
		return "", err
	}

	return upstreamURL, nil
}

// getModelProviderForAggregate determines the provider for a model in aggregate groups
func (ps *ProxyServer) getModelProviderForAggregate(model string, group *models.Group) string {
	return "gpt-load-aggregate"
}
