// Package proxy 提供高性能 OpenAI 多密钥代理服务器
package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"

	"gpt-load/internal/channel"
	"gpt-load/internal/config"
	"gpt-load/internal/encryption"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/keypool"
	"gpt-load/internal/models"
	"gpt-load/internal/modifier"
	"gpt-load/internal/response"
	"gpt-load/internal/services"
	"gpt-load/internal/utils"
	"gpt-load/internal/utils/jsonutil"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	modelFetchTimeout = 5 * time.Second
)

// ProxyServer 表示代理服务器
type ProxyServer struct {
	keyProvider       *keypool.KeyProvider
	groupManager      *services.GroupManager
	subGroupManager   *services.SubGroupManager
	settingsManager   *config.SystemSettingsManager
	channelFactory    *channel.Factory
	requestLogService *services.RequestLogService
	encryptionSvc     encryption.Service
}

// NewProxyServer 创建新的代理服务器
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

// HandleProxy 是代理请求的主入口点，基于稳定的 .bak 逻辑重构
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
		// 对于聚合组，使用第一个子组提取模型名称
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

	// 现在根据模型名称选择子组（用于聚合组）
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

	// 创建修改上下文
	modCtx := &modifier.ModificationContext{
		Context:       c.Request.Context(),
		OriginalGroup: originalGroup,
		SelectedGroup: group,
		OriginalModel: model,
		SelectedModel: selectedModel,
		IsAggregate:   originalGroup.GroupType == "aggregate",
		RequestPath:   c.Request.URL.Path,
	}

	// 创建修改器链
	chain := modifier.NewModifierChain(
		modifier.NewModelMappingModifier(),
		modifier.NewModelRedirectModifier(),
		modifier.NewParamOverrideModifier(),
	)

	// 应用修改
	finalBodyBytes, err := chain.Apply(modCtx, bodyBytes)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, err.Error()))
		return
	}

	isStream := channelHandler.IsStreamRequest(c, bodyBytes)

	excludedSubGroupIDs := make(utils.UintSet)
	ps.executeRequestWithRetry(c, channelHandler, originalGroup, group, finalBodyBytes, isStream, startTime, 0, model, excludedSubGroupIDs)
}

// executeRequestWithRetry 是处理请求和重试的核心递归函数
// 对于聚合组，当子组内的所有重试耗尽时支持跨子组重试
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

		// 对于聚合组，当没有可用密钥时尝试切换到另一个子组
		if originalGroup.GroupType == "aggregate" {
			if ps.trySwitchToAnotherSubGroup(c, originalGroup, group, modelAlias, bodyBytes, isStream, startTime, excludedSubGroupIDs, http.StatusServiceUnavailable, err.Error()) {
				return
			}
		}

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
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, "Failed to create upstream request"))
		return
	}
	req.ContentLength = int64(len(bodyBytes))

	req.Header = c.Request.Header.Clone()

	// 清理客户端认证密钥
	req.Header.Del("Authorization")
	req.Header.Del("X-Api-Key")
	req.Header.Del("X-Goog-Api-Key")
	req.Header.Del("Api-Key")

	// 应用通道特定的模型重定向（例如，Gemini 原生格式需要修改 URL 路径）
	// 如果已经处理过，此操作不会再次修改请求体
	finalBodyBytes, err := channelHandler.ApplyModelRedirect(req, bodyBytes, group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, err.Error()))
		ps.logRequest(c, originalGroup, group, apiKey, startTime, http.StatusBadRequest, err, isStream, upstreamURL, channelHandler, bodyBytes, nil, models.RequestTypeFinal, modelAlias)
		return
	}

	// 如果请求体被通道特定的重定向修改，则更新请求体
	if !bytes.Equal(finalBodyBytes, bodyBytes) {
		req.Body = io.NopCloser(bytes.NewReader(finalBodyBytes))
		req.ContentLength = int64(len(finalBodyBytes))
	}

	channelHandler.ModifyRequest(req, apiKey, group)

	// 合并 HeaderRules：先应用聚合组，再应用子组（子组优先）
	if originalGroup.GroupType == "aggregate" && originalGroup.ID != group.ID {
		// 先应用聚合组的 HeaderRules（基础配置）
		if len(originalGroup.HeaderRuleList) > 0 {
			headerCtx := utils.NewHeaderVariableContextFromGin(c, originalGroup, apiKey)
			utils.ApplyHeaderRules(req, originalGroup.HeaderRuleList, headerCtx)
		}
		// 然后应用子组的 HeaderRules（冲突时优先）
		if len(group.HeaderRuleList) > 0 {
			headerCtx := utils.NewHeaderVariableContextFromGin(c, group, apiKey)
			utils.ApplyHeaderRules(req, group.HeaderRuleList, headerCtx)
		}
	} else {
		// 标准组或聚合组作为子组
		if len(group.HeaderRuleList) > 0 {
			headerCtx := utils.NewHeaderVariableContextFromGin(c, group, apiKey)
			utils.ApplyHeaderRules(req, group.HeaderRuleList, headerCtx)
		}
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

	// 统一的重试错误处理
	// - 5xx 服务器错误：在当前组内重试（最多 MaxRetries 次），然后为聚合组切换
	// - 429 限流：重试，使用不同的密钥可能成功
	// - 4xx 客户端错误（除了 404）：组内不重试，但聚合组可以尝试下一个子组
	// - 404：不重试，资源未找到
	isRetryableHTTPError := resp != nil && (resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests)

	if err != nil || isRetryableHTTPError {
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
			// HTTP 级别错误（状态 >= 400）
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

		// 使用解析的错误信息更新密钥状态
		ps.keyProvider.UpdateStatus(apiKey, group, false, parsedError)

		// 检查是否是当前组内的最后一次尝试
		isLastAttempt := retryCount >= cfg.MaxRetries
		requestType := models.RequestTypeRetry
		if isLastAttempt {
			requestType = models.RequestTypeFinal
		}

		ps.logRequest(c, originalGroup, group, apiKey, startTime, statusCode, errors.New(parsedError), isStream, upstreamURL, channelHandler, bodyBytes, nil, requestType, modelAlias)

		// 如果这是当前组内的最后一次尝试，检查是否为聚合组并尝试切换
		if isLastAttempt && originalGroup.GroupType == "aggregate" {
			if ps.trySwitchToAnotherSubGroup(c, originalGroup, group, modelAlias, finalBodyBytes, isStream, startTime, excludedSubGroupIDs, statusCode, errorMessage) {
				return
			}
			// 所有子组都已尝试，返回错误
			ps.returnUpstreamError(c, statusCode, errorMessage)
			return
		}

		// 如果这是最后一次尝试，直接返回错误而不递归
		if isLastAttempt {
			ps.returnUpstreamError(c, statusCode, errorMessage)
			return
		}

		ps.executeRequestWithRetry(c, channelHandler, originalGroup, group, finalBodyBytes, isStream, startTime, retryCount+1, modelAlias, excludedSubGroupIDs)
		return
	}

	// 处理 4xx 客户端错误（除了 404，它应该透传）
	// 404：透传以允许客户端处理
	// 其他 4xx：组内不重试，但聚合组可以尝试下一个子组
	if resp != nil && resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		statusCode := resp.StatusCode
		errorBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			logrus.Errorf("Failed to read error body: %v", readErr)
			errorBody = []byte("Failed to read error body")
		}

		errorBody = handleGzipCompression(resp, errorBody)
		errorMessage := string(errorBody)
		parsedError := app_errors.ParseUpstreamError(errorBody)

		logrus.Debugf("Request failed with status %d for key %s. Parsed Error: %s", statusCode, utils.MaskAPIKey(apiKey.KeyValue), parsedError)

		// 更新密钥状态
		ps.keyProvider.UpdateStatus(apiKey, group, false, parsedError)

		ps.logRequest(c, originalGroup, group, apiKey, startTime, statusCode, errors.New(parsedError), isStream, upstreamURL, channelHandler, finalBodyBytes, nil, models.RequestTypeFinal, modelAlias)

		// 对于聚合组，在 4xx 错误时尝试切换到另一个子组
		// 不同的提供商可能有不同的 API 格式，切换可能有所帮助
		if originalGroup.GroupType == "aggregate" {
			if ps.trySwitchToAnotherSubGroup(c, originalGroup, group, modelAlias, finalBodyBytes, isStream, startTime, excludedSubGroupIDs, statusCode, errorMessage) {
				return
			}
		}

		ps.returnUpstreamError(c, statusCode, errorMessage)
		return
	}

	// ps.keyProvider.UpdateStatus(apiKey, group, true) // 不再在成功请求时重置成功计数以减少 I/O 开销
	logrus.Debugf("Request for group %s succeeded on attempt %d with key %s", group.Name, retryCount+1, utils.MaskAPIKey(apiKey.KeyValue))

	// 检查是否为模型列表请求（需要特殊处理）
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
		tokenUsage = ps.handleStreamingResponse(c, resp, channelHandler, modelAlias, bodyBytes)
	} else {
		tokenUsage = ps.handleNormalResponse(c, resp, channelHandler, modelAlias, bodyBytes)
	}

	ps.logRequest(c, originalGroup, group, apiKey, startTime, resp.StatusCode, nil, isStream, upstreamURL, channelHandler, bodyBytes, tokenUsage, models.RequestTypeFinal, modelAlias)
}

// returnUpstreamError 返回上游错误响应的辅助函数
func (ps *ProxyServer) returnUpstreamError(c *gin.Context, statusCode int, errorMessage string) {
	var errorJSON map[string]any
	if err := json.Unmarshal([]byte(errorMessage), &errorJSON); err == nil {
		c.JSON(statusCode, errorJSON)
	} else {
		response.Error(c, app_errors.NewAPIErrorWithUpstream(statusCode, "UPSTREAM_ERROR", errorMessage))
	}
}

// trySwitchToAnotherSubGroup 尝试为聚合组切换到另一个子组
// 如果成功切换并启动重试则返回 true，否则返回 false
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

	// 从请求体获取当前模型（bodyBytes 可能已被修改）
	var currentModel struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(bodyBytes, &currentModel); err != nil {
		return false
	}

	// 为新子组应用模型映射
	updatedBodyBytes := bodyBytes
	updatedSelectedModel := newSelection.SelectedModel
	if updatedSelectedModel != "" && updatedSelectedModel != currentModel.Model {
		var err error
		updatedBodyBytes, err = jsonutil.SetField(bodyBytes, "model", updatedSelectedModel)
		if err != nil {
			logrus.WithError(err).Error("Failed to update model field for cross-sub-group retry")
			return false
		}
	}

	ps.executeRequestWithRetry(c, newChannelHandler, originalGroup, newGroup, updatedBodyBytes, isStream, startTime, 0, modelAlias, excludedSubGroupIDs)
	return true
}

// logRequest 创建并记录请求日志的辅助函数
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

	// 设置父组
	if originalGroup != nil && originalGroup.GroupType == "aggregate" && originalGroup.ID != group.ID {
		logEntry.ParentGroupID = originalGroup.ID
		logEntry.ParentGroupName = originalGroup.Name
	}

	if channelHandler != nil && bodyBytes != nil {
		logEntry.Model = channelHandler.ExtractModel(c, bodyBytes)
	}

	// 设置 token 使用情况（如果可用）
	if tokenUsage != nil {
		logEntry.PromptTokens = tokenUsage.PromptTokens
		logEntry.CompletionTokens = tokenUsage.CompletionTokens
		logEntry.TotalTokens = tokenUsage.Total()
		logEntry.CachedTokens = tokenUsage.CachedTokens
	}

	if apiKey != nil {
		// 加密密钥值用于日志存储
		encryptedKeyValue, err := ps.encryptionSvc.Encrypt(apiKey.KeyValue)
		if err != nil {
			logrus.WithError(err).Error("Failed to encrypt key value for logging")
			logEntry.KeyValue = "failed-to-encryption"
		} else {
			logEntry.KeyValue = encryptedKeyValue
		}
		// 添加 KeyHash 用于反向查找
		logEntry.KeyHash = ps.encryptionSvc.Hash(apiKey.KeyValue)
	}

	if finalError != nil {
		logEntry.ErrorMessage = finalError.Error()
	}

	if err := ps.requestLogService.Record(logEntry); err != nil {
		logrus.Errorf("Failed to record request log: %v", err)
	}
}

// isModelsRequest 检查请求是否为 /v1/models 或 /models 端点
func (ps *ProxyServer) isModelsRequest(path string) bool {
	return strings.HasSuffix(path, "/v1/models") || strings.HasSuffix(path, "/models")
}

// handleAggregateModelsRequest 处理聚合组的 /v1/models 请求
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

	models := make([]map[string]any, len(modelList))
	for i, model := range modelList {
		models[i] = map[string]any{
			"id":       model,
			"object":   "model",
			"created":  time.Now().Unix(),
			"owned_by": ps.getModelProviderForAggregate(model, aggregateGroup),
		}
	}

	response := map[string]any{
		"object": "list",
		"data":   models,
	}

	c.JSON(http.StatusOK, response)
}

// fetchModelsFromAllSubGroups 从聚合组的所有子组获取模型列表
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

// fetchUpstreamModelsWithKey 使用子组密钥从上游获取模型列表
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

// buildModelsURLForGroup 为指定组构建模型端点的上游 URL
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

// getModelProviderForAggregate 确定聚合组中模型的提供商
func (ps *ProxyServer) getModelProviderForAggregate(model string, group *models.Group) string {
	return "gpt-load-aggregate"
}
