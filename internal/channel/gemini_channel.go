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

// ModifyRequest adds the API key as a query parameter for Gemini requests.
func (ch *GeminiChannel) ModifyRequest(req *http.Request, apiKey *models.APIKey, group *models.Group) {
	if strings.Contains(req.URL.Path, "v1beta/openai") {
		req.Header.Set("Authorization", "Bearer "+apiKey.KeyValue)
	} else {
		q := req.URL.Query()
		q.Set("key", apiKey.KeyValue)
		req.URL.RawQuery = q.Encode()
	}
}

// IsStreamRequest checks if the request is for a streaming response.
func (ch *GeminiChannel) IsStreamRequest(c *gin.Context, bodyBytes []byte) bool {
	// Check for Gemini-specific streaming path
	if strings.HasSuffix(c.Request.URL.Path, ":streamGenerateContent") {
		return true
	}

	// Use base channel method for standard streaming indicators
	return ch.BaseChannel.IsStreamRequest(c, bodyBytes)
}

func (ch *GeminiChannel) ExtractModel(c *gin.Context, bodyBytes []byte) string {
	// gemini format: extract from URL path
	path := c.Request.URL.Path
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "models" && i+1 < len(parts) {
			modelPart := parts[i+1]
			return strings.Split(modelPart, ":")[0]
		}
	}

	// openai format: use base channel method
	return ch.BaseChannel.ExtractModel(c, bodyBytes)
}

// ValidateKey checks if the given API key is valid by making a generateContent request.
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
			// Gemini adds API key as query parameter, not header
		},
		BuildRequestURL: func(upstreamURL *url.URL) (string, error) {
			// Use provided model or fall back to channel's TestModel
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

// ApplyModelRedirect overrides the default implementation for Gemini channel.
func (ch *GeminiChannel) ApplyModelRedirect(req *http.Request, bodyBytes []byte, group *models.Group) ([]byte, error) {
	if len(group.ModelRedirectMap) == 0 {
		return bodyBytes, nil
	}

	if strings.Contains(req.URL.Path, "v1beta/openai") {
		return ch.BaseChannel.ApplyModelRedirect(req, bodyBytes, group)
	}

	return ch.applyNativeFormatRedirect(req, bodyBytes, group)
}

// applyNativeFormatRedirect handles model redirection for Gemini native format.
func (ch *GeminiChannel) applyNativeFormatRedirect(req *http.Request, bodyBytes []byte, group *models.Group) ([]byte, error) {
	path := req.URL.Path
	parts := strings.Split(path, "/")

	for i, part := range parts {
		if part == "models" && i+1 < len(parts) {
			modelPart := parts[i+1]
			originalModel := strings.Split(modelPart, ":")[0]

			if targetModel, found := group.ModelRedirectMap[originalModel]; found {
				suffix := ""
				if colonIndex := strings.Index(modelPart, ":"); colonIndex != -1 {
					suffix = modelPart[colonIndex:]
				}
				parts[i+1] = targetModel + suffix
				req.URL.Path = strings.Join(parts, "/")

				logrus.WithFields(logrus.Fields{
					"group":          group.Name,
					"original_model": originalModel,
					"target_model":   targetModel,
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

// TransformModelList transforms the model list response based on redirect rules.
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

// transformGeminiNativeFormat transforms Gemini native format model list
func (ch *GeminiChannel) transformGeminiNativeFormat(req *http.Request, response map[string]any, modelsInterface any, group *models.Group) map[string]any {
	upstreamModels, ok := modelsInterface.([]any)
	if !ok {
		return response
	}

	configuredModels := buildConfiguredGeminiModels(group.ModelRedirectMap)

	// Strict mode: return only configured models (whitelist)
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

	// Non-strict mode: merge upstream + configured models (upstream priority)
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

// buildConfiguredGeminiModels builds a list of models from redirect rules for Gemini format
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

// mergeGeminiModelLists merges upstream and configured model lists for Gemini format
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

	// Start with all upstream models
	result := make([]any, len(upstream))
	copy(result, upstream)

	// Add configured models that don't exist in upstream
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

// isFirstPage checks if this is the first page of a Gemini paginated request
func isFirstPage(req *http.Request) bool {
	pageToken := req.URL.Query().Get("pageToken")
	return pageToken == ""
}
