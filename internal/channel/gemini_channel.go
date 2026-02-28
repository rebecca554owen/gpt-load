package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"gpt-load/internal/models"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func init() {
	Register("gemini", newGeminiChannel)
}

type GeminiChannel struct {
	*BaseChannel
}

func newGeminiChannel(f *Factory, group *models.Group) (ChannelProxy, error) {
	base, err := f.newBaseChannel("gemini", group)
	if err != nil {
		return nil, err
	}

	return &GeminiChannel{
		BaseChannel: base,
	}, nil
}

// ModifyRequest 为 Gemini 请求添加 API 密钥作为查询参数
func (ch *GeminiChannel) ModifyRequest(req *http.Request, apiKey *models.APIKey, group *models.Group) {
	if strings.Contains(req.URL.Path, "v1beta/openai") {
		req.Header.Set("Authorization", "Bearer "+apiKey.KeyValue)
	} else {
		q := req.URL.Query()
		q.Set("key", apiKey.KeyValue)
		req.URL.RawQuery = q.Encode()
	}
}

// IsStreamRequest 检查请求是否为流式响应
func (ch *GeminiChannel) IsStreamRequest(c *gin.Context, bodyBytes []byte) bool {
	// 检查 Gemini 特定的流式路径
	if strings.HasSuffix(c.Request.URL.Path, ":streamGenerateContent") {
		return true
	}

	// 使用基础通道方法检查标准流式指示器
	return ch.BaseChannel.IsStreamRequest(c, bodyBytes)
}

func (ch *GeminiChannel) ExtractModel(c *gin.Context, bodyBytes []byte) string {
	// gemini 格式：从 URL 路径提取
	path := c.Request.URL.Path
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "models" && i+1 < len(parts) {
			modelPart := parts[i+1]
			return strings.Split(modelPart, ":")[0]
		}
	}

	// openai 格式：使用基础通道方法
	return ch.BaseChannel.ExtractModel(c, bodyBytes)
}

// ValidateKey 通过发送 generateContent 请求检查给定的 API 密钥是否有效
func (ch *GeminiChannel) ValidateKey(ctx context.Context, apiKey *models.APIKey, group *models.Group, model string) (bool, error) {
	return ch.validateKeyCommon(ctx, apiKey, group, model, ValidateKeyConfig{
		BuildPayload: func(testModel string) (map[string]any, error) {
			return gin.H{
				"contents": []gin.H{
					{
						"role": "user",
						"parts": []gin.H{
							{"text": "hi"},
						},
					},
				},
			}, nil
		},
		SetAuthHeaders: func(req *http.Request, apiKey *models.APIKey) {
			// Gemini 将 API 密钥作为查询参数添加，而非请求头
		},
		BuildRequestURL: func(upstreamURL *url.URL) (string, error) {
			// 使用提供的模型或回退到通道的 TestModel
			testModel := model
			if testModel == "" {
				testModel = ch.TestModel
			}
			reqURL, err := url.JoinPath(upstreamURL.String(), "v1beta", "models", testModel+":generateContent")
			if err != nil {
				return "", fmt.Errorf("failed to create gemini validation path: %w", err)
			}
			return reqURL + "?key=" + apiKey.KeyValue, nil
		},
	})
}

// ApplyModelRedirect 覆盖 Gemini 通道的默认实现
func (ch *GeminiChannel) ApplyModelRedirect(req *http.Request, bodyBytes []byte, group *models.Group) ([]byte, error) {
	if len(group.ModelRedirectMap) == 0 {
		return bodyBytes, nil
	}

	if strings.Contains(req.URL.Path, "v1beta/openai") {
		return ch.BaseChannel.ApplyModelRedirect(req, bodyBytes, group)
	}

	return ch.applyNativeFormatRedirect(req, bodyBytes, group)
}

// applyNativeFormatRedirect 处理 Gemini 原生格式的模型重定向
func (ch *GeminiChannel) applyNativeFormatRedirect(req *http.Request, bodyBytes []byte, group *models.Group) ([]byte, error) {
	path := req.URL.Path
	parts := strings.Split(path, "/")

	for i, part := range parts {
		if part == "models" && i+1 < len(parts) {
			modelPart := parts[i+1]
			originalModel := strings.Split(modelPart, ":")[0]

			if actualModel, found := group.ModelRedirectMap[originalModel]; found {
				suffix := ""
				if colonIndex := strings.Index(modelPart, ":"); colonIndex != -1 {
					suffix = modelPart[colonIndex:]
				}
				parts[i+1] = actualModel + suffix
				req.URL.Path = strings.Join(parts, "/")

				logrus.WithFields(logrus.Fields{
					"group":          group.Name,
					"original_model": originalModel,
					"model":          actualModel,
					"channel":        "gemini_native",
					"original_path":  path,
					"new_path":       req.URL.Path,
				}).Debug("Model redirected")

				return bodyBytes, nil
			}

			if group.ModelRedirectStrict {
				return nil, fmt.Errorf("model '%s' is not configured in redirect rules", originalModel)
			}
			return bodyBytes, nil
		}
	}

	return bodyBytes, nil
}

// TransformModelList 根据重定向规则转换模型列表响应
func (ch *GeminiChannel) TransformModelList(req *http.Request, bodyBytes []byte, group *models.Group) (map[string]any, error) {
	var response map[string]any
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		logrus.WithError(err).Debug("Failed to parse model list response, returning empty")
		return nil, err
	}

	if modelsInterface, hasModels := response["models"]; hasModels {
		return ch.transformGeminiNativeFormat(req, response, modelsInterface, group), nil
	}

	if _, hasData := response["data"]; hasData {
		return ch.BaseChannel.TransformModelList(req, bodyBytes, group)
	}

	return response, nil
}

// transformGeminiNativeFormat 转换 Gemini 原生格式的模型列表
func (ch *GeminiChannel) transformGeminiNativeFormat(req *http.Request, response map[string]any, modelsInterface any, group *models.Group) map[string]any {
	upstreamModels, ok := modelsInterface.([]any)
	if !ok {
		return response
	}

	configuredModels := buildConfiguredGeminiModels(group.ModelRedirectMap)

	// 严格模式：仅返回配置的模型（白名单）
	if group.ModelRedirectStrict {
		response["models"] = configuredModels
		delete(response, "nextPageToken")

		logrus.WithFields(logrus.Fields{
			"group":       group.Name,
			"model_count": len(configuredModels),
			"strict_mode": true,
			"format":      "gemini_native",
		}).Debug("Model list returned (strict mode - configured models only)")

		return response
	}

	// 非严格模式：合并上游 + 配置的模型（上游优先）
	var merged []any
	if isFirstPage(req) {
		merged = mergeGeminiModelLists(upstreamModels, configuredModels)
		logrus.WithFields(logrus.Fields{
			"group":            group.Name,
			"upstream_count":   len(upstreamModels),
			"configured_count": len(configuredModels),
			"merged_count":     len(merged),
			"strict_mode":      false,
			"format":           "gemini_native",
			"page":             "first",
		}).Debug("Model list merged (non-strict mode - first page)")
	} else {
		merged = upstreamModels
		logrus.WithFields(logrus.Fields{
			"group":          group.Name,
			"upstream_count": len(upstreamModels),
			"strict_mode":    false,
			"format":         "gemini_native",
			"page":           "subsequent",
		}).Debug("Model list returned (non-strict mode - subsequent page)")
	}

	response["models"] = merged
	return response
}

// buildConfiguredGeminiModels 为 Gemini 格式从重定向规则构建模型列表
func buildConfiguredGeminiModels(redirectMap map[string]string) []any {
	if len(redirectMap) == 0 {
		return []any{}
	}

	models := make([]any, 0, len(redirectMap))
	for sourceModel := range redirectMap {
		modelName := sourceModel
		if !strings.HasPrefix(sourceModel, "models/") {
			modelName = "models/" + sourceModel
		}

		models = append(models, map[string]any{
			"name":                       modelName,
			"displayName":                sourceModel,
			"supportedGenerationMethods": []string{"generateContent"},
		})
	}
	return models
}

// mergeGeminiModelLists 为 Gemini 格式合并上游和配置的模型列表
func mergeGeminiModelLists(upstream []any, configured []any) []any {
	upstreamNames := make(map[string]bool)
	for _, item := range upstream {
		if modelObj, ok := item.(map[string]any); ok {
			if modelName, ok := modelObj["name"].(string); ok {
				upstreamNames[modelName] = true
				cleanName := strings.TrimPrefix(modelName, "models/")
				upstreamNames[cleanName] = true
			}
		}
	}

	// 从所有上游模型开始
	result := make([]any, len(upstream))
	copy(result, upstream)

	// 添加上游中不存在的配置模型
	for _, item := range configured {
		if modelObj, ok := item.(map[string]any); ok {
			if modelName, ok := modelObj["name"].(string); ok {
				cleanName := strings.TrimPrefix(modelName, "models/")
				if !upstreamNames[modelName] && !upstreamNames[cleanName] {
					result = append(result, item)
				}
			}
		}
	}

	return result
}

// isFirstPage 检查这是否是 Gemini 分页请求的第一页
func isFirstPage(req *http.Request) bool {
	pageToken := req.URL.Query().Get("pageToken")
	return pageToken == ""
}
