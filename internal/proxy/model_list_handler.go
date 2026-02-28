package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"gpt-load/internal/channel"
	"gpt-load/internal/models"
	"gpt-load/internal/utils"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// shouldInterceptModelList checks if this is a model list request that should be intercepted
func shouldInterceptModelList(path string, method string) bool {
	if method != "GET" {
		return false
	}

	// Check various model list endpoints
	return strings.HasSuffix(path, "/v1/models") ||
		strings.HasSuffix(path, "/v1beta/models") ||
		strings.Contains(path, "/v1beta/openai/v1/models")
}

// isModelsRequest checks if the request is for /v1/models or /models endpoint
func (ps *ProxyServer) isModelsRequest(path string) bool {
	return strings.HasSuffix(path, "/v1/models") || strings.HasSuffix(path, "/models")
}

// handleModelListResponse processes the model list response and applies filtering based on redirect rules
func (ps *ProxyServer) handleModelListResponse(c *gin.Context, resp *http.Response, group *models.Group, channelHandler channel.ChannelProxy) {
	// Read the upstream response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).Error("Failed to read model list response body")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	// Decompress response data based on Content-Encoding
	contentEncoding := resp.Header.Get("Content-Encoding")
	decompressed, err := utils.DecompressResponse(contentEncoding, bodyBytes)
	if err != nil {
		logrus.WithError(err).Warn("Decompression failed, using original data")
		decompressed = bodyBytes
	}

	// Transform model list (returns map[string]any directly, no marshaling)
	response, err := channelHandler.TransformModelList(c.Request, decompressed, group)
	if err != nil {
		logrus.WithError(err).Error("Failed to transform model list")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process response"})
		return
	}

	c.JSON(http.StatusOK, response)
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
			"owned_by": ps.getModelProviderForAggregate(),
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
				if r := recover(); r != nil {
					logrus.WithField("panic", r).Error("Panic in fetchModelsFromAllSubGroups")
				}
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
func (ps *ProxyServer) getModelProviderForAggregate() string {
	return "gpt-load-aggregate"
}
