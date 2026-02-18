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

// UpstreamInfo holds the information for a single upstream server, including its weight.
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

// BaseChannel provides common functionality for channel proxies.
type BaseChannel struct {
	Name               string
	Upstreams          []UpstreamInfo
	HTTPClient         *http.Client
	StreamClient       *http.Client
	TestModel          string
	ValidationEndpoint string
	upstreamLock       sync.Mutex

	// Cached fields from the group for stale check
	channelType         string
	groupUpstreams      datatypes.JSON
	effectiveConfig     *types.SystemSettings
	modelRedirectRules  datatypes.JSONMap
	modelRedirectStrict bool
}

// getUpstreamURL selects an upstream URL using a smooth weighted round-robin algorithm.
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
		return b.Upstreams[0].URL
	}

	return selected.(*UpstreamInfo).URL
}

// BuildUpstreamURL constructs the target URL for the upstream service.
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

// BuildUpstreamURLForAggregate constructs the target URL for aggregate group sub-groups.
// It uses the validation endpoint instead of the request path to ensure compatibility
// with different upstream endpoints.
func (b *BaseChannel) BuildUpstreamURLForAggregate(originalURL *url.URL, groupName string) (string, error) {
	base := b.getUpstreamURL()
	if base == nil {
		return "", fmt.Errorf("no upstream URL configured for channel %s", b.Name)
	}

	finalURL := *base

	// For aggregate groups, always use the validation endpoint
	// This ensures each sub-group uses its configured endpoint path
	targetPath := b.ValidationEndpoint
	if targetPath == "" {
		// Fallback to request path if no validation endpoint is configured
		proxyPrefix := "/proxy/" + groupName
		requestPath := originalURL.Path
		requestPath = strings.TrimPrefix(requestPath, proxyPrefix)
		targetPath = requestPath
	}

	finalURL.Path = strings.TrimRight(finalURL.Path, "/") + targetPath
	finalURL.RawQuery = originalURL.RawQuery

	return finalURL.String(), nil
}

// IsConfigStale checks if the channel's configuration is stale compared to the provided group.
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
	// Check for model redirect rules changes
	if !reflect.DeepEqual(b.modelRedirectRules, group.ModelRedirectRules) {
		return true
	}
	if b.modelRedirectStrict != group.ModelRedirectStrict {
		return true
	}
	return false
}

// GetHTTPClient returns the client for standard requests.
func (b *BaseChannel) GetHTTPClient() *http.Client {
	return b.HTTPClient
}

// GetStreamClient returns the client for streaming requests.
func (b *BaseChannel) GetStreamClient() *http.Client {
	return b.StreamClient
}

// ApplyModelRedirect applies model redirection based on the group's redirect rules.
func (b *BaseChannel) ApplyModelRedirect(req *http.Request, bodyBytes []byte, group *models.Group) ([]byte, error) {
	if len(group.ModelRedirectMap) == 0 || len(bodyBytes) == 0 {
		return bodyBytes, nil
	}

	var requestData map[string]any
	if err := json.Unmarshal(bodyBytes, &requestData); err != nil {
		return bodyBytes, nil
	}

	modelValue, exists := requestData["model"]
	if !exists {
		return bodyBytes, nil
	}

	model, ok := modelValue.(string)
	if !ok {
		return bodyBytes, nil
	}

	// Direct match without any prefix processing
	if targetModel, found := group.ModelRedirectMap[model]; found {
		requestData["model"] = targetModel

		// Log the redirection for audit
		logrus.WithFields(logrus.Fields{
			"group":          group.Name,
			"original_model": model,
			"target_model":   targetModel,
			"channel":        "json_body",
		}).Debug("Model redirected")

		return json.Marshal(requestData)
	}

	if group.ModelRedirectStrict {
		return nil, fmt.Errorf("model '%s' is not configured in redirect rules", model)
	}

	return bodyBytes, nil
}

// TransformModelList transforms the model list response based on redirect rules.
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

	// Build configured source models list (common logic for both modes)
	configuredModels := buildConfiguredModels(group.ModelRedirectMap)

	// Strict mode: return only configured models (whitelist)
	if group.ModelRedirectStrict {
		response["data"] = configuredModels

		logrus.WithFields(logrus.Fields{
			"group":       group.Name,
			"model_count": len(configuredModels),
			"strict_mode": true,
		}).Debug("Model list returned (strict mode - configured models only)")

		return response, nil
	}

	// Non-strict mode: merge upstream + configured models (upstream priority)
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

// buildConfiguredModels builds a list of models from redirect rules
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

// mergeModelLists merges upstream and configured model lists
func mergeModelLists(upstream []any, configured []any) []any {
	// Create set of upstream model IDs
	upstreamIDs := make(map[string]bool)
	for _, item := range upstream {
		if modelObj, ok := item.(map[string]any); ok {
			if modelID, ok := modelObj["id"].(string); ok {
				upstreamIDs[modelID] = true
			}
		}
	}

	// Start with all upstream models
	result := make([]any, len(upstream))
	copy(result, upstream)

	// Add configured models that don't exist in upstream
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

// ExtractModel extracts the model field from request body using standard OpenAI format
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

// IsStreamRequest checks if the request is for a streaming response using common indicators
func (b *BaseChannel) IsStreamRequest(c *gin.Context, bodyBytes []byte) bool {
	// Check Accept header
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
		return true
	}

	// Check stream query parameter
	if c.Query("stream") == "true" {
		return true
	}

	// Check stream field in request body
	type streamPayload struct {
		Stream bool `json:"stream"`
	}
	var p streamPayload
	if err := json.Unmarshal(bodyBytes, &p); err == nil {
		return p.Stream
	}

	return false
}

// ValidateKeyConfig holds channel-specific validation logic
type ValidateKeyConfig struct {
	// BuildPayload constructs the request body for validation
	BuildPayload func(model string) (map[string]any, error)
	// SetAuthHeaders sets channel-specific authentication headers
	SetAuthHeaders func(req *http.Request, apiKey *models.APIKey)
	// BuildRequestURL builds the validation request URL (optional, defaults to ValidationEndpoint)
	BuildRequestURL func(upstreamURL *url.URL) (string, error)
}

// validateKeyCommon provides common key validation logic for all channels
func (b *BaseChannel) validateKeyCommon(ctx context.Context, apiKey *models.APIKey, group *models.Group, model string, config ValidateKeyConfig) (bool, error) {
	upstreamURL := b.getUpstreamURL()
	if upstreamURL == nil {
		return false, fmt.Errorf("no upstream URL configured for channel %s", b.Name)
	}

	// Build request URL
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

	// Use provided model or fall back to channel's TestModel
	testModel := model
	if testModel == "" {
		testModel = b.TestModel
	}

	// Build payload using channel-specific function
	payload, err := config.BuildPayload(testModel)
	if err != nil {
		return false, fmt.Errorf("failed to build validation payload: %w", err)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal validation payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return false, fmt.Errorf("failed to create validation request: %w", err)
	}

	// Set channel-specific authentication headers
	config.SetAuthHeaders(req, apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Apply custom header rules if available
	if len(group.HeaderRuleList) > 0 {
		headerCtx := utils.NewHeaderVariableContext(group, apiKey)
		utils.ApplyHeaderRules(req, group.HeaderRuleList, headerCtx)
	}

	// Send request
	resp, err := b.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send validation request: %w", err)
	}
	defer resp.Body.Close()

	// Any 2xx status code indicates the key is valid
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, nil
	}

	// For non-200 responses, parse the body to provide a more specific error reason
	errorBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("key is invalid (status %d), but failed to read error body: %w", resp.StatusCode, err)
	}

	// Use the parser to extract a clean error message
	parsedError := app_errors.ParseUpstreamError(errorBody)

	return false, fmt.Errorf("[status %d] %s", resp.StatusCode, parsedError)
}
