package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/types"
	"gpt-load/internal/utils"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
)

// UpstreamInfo 保存单个上游服务器的信息，包括其权重
type UpstreamInfo struct {
	URL           *url.URL
	Weight        int
	CurrentWeight int
}

func (u *UpstreamInfo) GetWeight() int {
	return u.Weight
}

func (u *UpstreamInfo) GetCurrentWeight() int {
	return u.CurrentWeight
}

func (u *UpstreamInfo) SetCurrentWeight(w int) {
	u.CurrentWeight = w
}

// BaseChannel 为通道代理提供通用功能
type BaseChannel struct {
	Name               string
	Upstreams          []UpstreamInfo
	HTTPClient         *http.Client
	StreamClient       *http.Client
	TestModel          string
	ValidationEndpoint string
	upstreamLock       sync.Mutex

	// 从组缓存用于配置过期的字段
	channelType         string
	groupUpstreams      datatypes.JSON
	effectiveConfig     *types.SystemSettings
	modelRedirectRules  datatypes.JSONMap
	modelRedirectStrict bool
}

// GetChannelType 返回通道类型标识符
func (b *BaseChannel) GetChannelType() string {
	return b.channelType
}

// getUpstreamURL 使用平滑加权轮询算法选择上游 URL
func (b *BaseChannel) getUpstreamURL() *url.URL {
	b.upstreamLock.Lock()
	defer b.upstreamLock.Unlock()

	if len(b.Upstreams) == 0 {
		return nil
	}
	if len(b.Upstreams) == 1 {
		return b.Upstreams[0].URL
	}

	items := make([]utils.WeightedItem, len(b.Upstreams))
	for i := range b.Upstreams {
		items[i] = &b.Upstreams[i]
	}

	selected := utils.SelectByWeightedRoundRobin(items)
	if selected == nil {
		logrus.WithField("channel", b.Name).Warn("Weighted selection failed, using first upstream as fallback")
		return b.Upstreams[0].URL
	}

	return selected.(*UpstreamInfo).URL
}

// BuildUpstreamURL 构建上游服务的目标 URL
func (b *BaseChannel) BuildUpstreamURL(originalURL *url.URL, groupName string) (string, error) {
	base := b.getUpstreamURL()
	if base == nil {
		return "", fmt.Errorf("no upstream URL configured for channel %s", b.Name)
	}

	finalURL := *base
	proxyPrefix := "/proxy/" + groupName
	requestPath := originalURL.Path
	requestPath = strings.TrimPrefix(requestPath, proxyPrefix)

	finalURL.Path = strings.TrimRight(finalURL.Path, "/") + requestPath

	finalURL.RawQuery = originalURL.RawQuery

	return finalURL.String(), nil
}

// BuildUpstreamURLForAggregate 为聚合组的子组构建目标 URL
// 它使用验证端点而非请求路径，以确保与不同上游端点的兼容性
func (b *BaseChannel) BuildUpstreamURLForAggregate(originalURL *url.URL, groupName string) (string, error) {
	base := b.getUpstreamURL()
	if base == nil {
		return "", fmt.Errorf("no upstream URL configured for channel %s", b.Name)
	}

	finalURL := *base

	// 对于聚合组，始终使用验证端点
	// 这确保每个子组使用其配置的端点路径
	targetPath := b.ValidationEndpoint
	if targetPath == "" {
		// 如果未配置验证端点，则回退到请求路径
		proxyPrefix := "/proxy/" + groupName
		requestPath := originalURL.Path
		requestPath = strings.TrimPrefix(requestPath, proxyPrefix)
		targetPath = requestPath
	}

	finalURL.Path = strings.TrimRight(finalURL.Path, "/") + targetPath
	finalURL.RawQuery = originalURL.RawQuery

	return finalURL.String(), nil
}

// IsConfigStale 检查通道的配置是否相对于提供的组已过期
func (b *BaseChannel) IsConfigStale(group *models.Group) bool {
	if b.channelType != group.ChannelType {
		return true
	}
	if b.TestModel != group.TestModel {
		return true
	}
	if b.ValidationEndpoint != utils.GetValidationEndpoint(group) {
		return true
	}
	if !bytes.Equal(b.groupUpstreams, group.Upstreams) {
		return true
	}
	if !reflect.DeepEqual(b.effectiveConfig, &group.EffectiveConfig) {
		return true
	}
	// 检查模型重定向规则的更改
	if !reflect.DeepEqual(b.modelRedirectRules, group.ModelRedirectRules) {
		return true
	}
	if b.modelRedirectStrict != group.ModelRedirectStrict {
		return true
	}
	return false
}

// GetHTTPClient 返回用于标准请求的客户端
func (b *BaseChannel) GetHTTPClient() *http.Client {
	return b.HTTPClient
}

// GetStreamClient 返回用于流式请求的客户端
func (b *BaseChannel) GetStreamClient() *http.Client {
	return b.StreamClient
}

// ApplyModelRedirect 根据组的重定向规则应用模型重定向
// Note: For most channels, model redirect is already applied in HandleProxy (server.go)
// This method is kept for channel-specific implementations (e.g., Gemini native format)
func (b *BaseChannel) ApplyModelRedirect(req *http.Request, bodyBytes []byte, group *models.Group) ([]byte, error) {
	// Base channel implementation: no additional processing needed
	// Model redirect is already applied in server.go HandleProxy
	return bodyBytes, nil
}

// TransformModelList 根据重定向规则转换模型列表响应
func (b *BaseChannel) TransformModelList(req *http.Request, bodyBytes []byte, group *models.Group) (map[string]any, error) {
	var response map[string]any
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		logrus.WithError(err).Debug("Failed to parse model list response, returning empty")
		return nil, err
	}

	dataInterface, exists := response["data"]
	if !exists {
		return response, nil
	}

	upstreamModels, ok := dataInterface.([]any)
	if !ok {
		return response, nil
	}

	// 构建配置的源模型列表（两种模式的通用逻辑）
	configuredModels := buildConfiguredModels(group.ModelRedirectMap)

	// 严格模式：仅返回配置的模型（白名单）
	if group.ModelRedirectStrict {
		response["data"] = configuredModels

		logrus.WithFields(logrus.Fields{
			"group":       group.Name,
			"model_count": len(configuredModels),
			"strict_mode": true,
		}).Debug("Model list returned (strict mode - configured models only)")

		return response, nil
	}

	// 非严格模式：合并上游 + 配置的模型（上游优先）
	merged := mergeModelLists(upstreamModels, configuredModels)
	response["data"] = merged

	logrus.WithFields(logrus.Fields{
		"group":            group.Name,
		"upstream_count":   len(upstreamModels),
		"configured_count": len(configuredModels),
		"merged_count":     len(merged),
		"strict_mode":      false,
	}).Debug("Model list merged (non-strict mode)")

	return response, nil
}

// buildConfiguredModels 从重定向规则构建模型列表
func buildConfiguredModels(redirectMap map[string]string) []any {
	if len(redirectMap) == 0 {
		return []any{}
	}

	models := make([]any, 0, len(redirectMap))
	for sourceModel := range redirectMap {
		models = append(models, map[string]any{
			"id":       sourceModel,
			"object":   "model",
			"created":  0,
			"owned_by": "system",
		})
	}
	return models
}

// mergeModelLists 合并上游和配置的模型列表
func mergeModelLists(upstream []any, configured []any) []any {
	// 创建上游模型 ID 集合
	upstreamIDs := make(map[string]bool)
	for _, item := range upstream {
		if modelObj, ok := item.(map[string]any); ok {
			if modelID, ok := modelObj["id"].(string); ok {
				upstreamIDs[modelID] = true
			}
		}
	}

	// 从所有上游模型开始
	result := make([]any, len(upstream))
	copy(result, upstream)

	// 添加上游中不存在的配置模型
	for _, item := range configured {
		if modelObj, ok := item.(map[string]any); ok {
			if modelID, ok := modelObj["id"].(string); ok {
				if !upstreamIDs[modelID] {
					result = append(result, item)
				}
			}
		}
	}

	return result
}

// ExtractModel 使用标准 OpenAI 格式从请求体中提取 model 字段
func (b *BaseChannel) ExtractModel(_ *gin.Context, bodyBytes []byte) string {
	type modelPayload struct {
		Model string `json:"model"`
	}
	var p modelPayload
	if err := json.Unmarshal(bodyBytes, &p); err == nil {
		return p.Model
	}
	return ""
}

// IsStreamRequest 使用通用指示符检查请求是否为流式响应
func (b *BaseChannel) IsStreamRequest(c *gin.Context, bodyBytes []byte) bool {
	// 检查 Accept 头
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
		return true
	}

	// 检查 stream 查询参数
	if c.Query("stream") == "true" {
		return true
	}

	// 检查请求体中的 stream 字段
	type streamPayload struct {
		Stream bool `json:"stream"`
	}
	var p streamPayload
	if err := json.Unmarshal(bodyBytes, &p); err == nil {
		return p.Stream
	}

	return false
}

// ValidateKeyConfig 保存通道特定的验证逻辑
type ValidateKeyConfig struct {
	// BuildPayload 构造验证请求的请求体
	BuildPayload func(model string) (map[string]any, error)
	// SetAuthHeaders 设置通道特定的认证头
	SetAuthHeaders func(req *http.Request, apiKey *models.APIKey)
	// BuildRequestURL 构建验证请求 URL（可选，默认为 ValidationEndpoint）
	BuildRequestURL func(upstreamURL *url.URL) (string, error)
}

// validateKeyCommon 为所有通道提供通用密钥验证逻辑
func (b *BaseChannel) validateKeyCommon(ctx context.Context, apiKey *models.APIKey, group *models.Group, model string, config ValidateKeyConfig) (bool, error) {
	upstreamURL := b.getUpstreamURL()
	if upstreamURL == nil {
		return false, fmt.Errorf("no upstream URL configured for channel %s", b.Name)
	}

	// 构建请求 URL
	var reqURL string
	var err error
	if config.BuildRequestURL != nil {
		reqURL, err = config.BuildRequestURL(upstreamURL)
	} else {
		endpointURL, parseErr := url.Parse(b.ValidationEndpoint)
		if parseErr != nil {
			return false, fmt.Errorf("failed to parse validation endpoint: %w", parseErr)
		}
		finalURL := *upstreamURL
		finalURL.Path = strings.TrimRight(finalURL.Path, "/") + endpointURL.Path
		finalURL.RawQuery = endpointURL.RawQuery
		reqURL = finalURL.String()
	}

	if err != nil {
		return false, err
	}

	// 使用提供的模型或回退到通道的 TestModel
	testModel := model
	if testModel == "" {
		testModel = b.TestModel
	}

	// 使用通道特定函数构建 payload
	payload, err := config.BuildPayload(testModel)
	if err != nil {
		return false, fmt.Errorf("failed to build validation payload: %w", err)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal validation payload: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return false, fmt.Errorf("failed to create validation request: %w", err)
	}

	// 设置通道特定的认证头
	config.SetAuthHeaders(req, apiKey)
	req.Header.Set("Content-Type", "application/json")

	// 如果有自定义头规则，则应用
	if len(group.HeaderRuleList) > 0 {
		headerCtx := utils.NewHeaderVariableContext(group, apiKey)
		utils.ApplyHeaderRules(req, group.HeaderRuleList, headerCtx)
	}

	// 发送请求
	resp, err := b.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send validation request: %w", err)
	}
	defer resp.Body.Close()

	// 任何 2xx 状态码表示密钥有效
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, nil
	}

	// 对于非 200 响应，解析请求体以提供更具体的错误原因
	errorBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("key is invalid (status %d), but failed to read error body: %w", resp.StatusCode, err)
	}

	// 使用解析器提取清晰的错误消息
	parsedError := app_errors.ParseUpstreamError(errorBody)

	return false, fmt.Errorf("[status %d] %s", resp.StatusCode, parsedError)
}
