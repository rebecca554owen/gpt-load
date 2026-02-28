package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"gpt-load/internal/channel"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/keypool"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"gpt-load/internal/services"
	"gpt-load/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RetryHandler handles request retry logic including cross-subgroup retry for aggregate groups
type RetryHandler struct {
	keyProvider     *keypool.KeyProvider
	groupManager    *services.GroupManager
	subGroupManager *services.SubGroupManager
	channelFactory  *channel.Factory
	requestLogger   *RequestLogger
}

// NewRetryHandler creates a new RetryHandler
func NewRetryHandler(
	keyProvider *keypool.KeyProvider,
	groupManager *services.GroupManager,
	subGroupManager *services.SubGroupManager,
	channelFactory *channel.Factory,
	requestLogger *RequestLogger,
) *RetryHandler {
	return &RetryHandler{
		keyProvider:     keyProvider,
		groupManager:    groupManager,
		subGroupManager: subGroupManager,
		channelFactory:  channelFactory,
		requestLogger:   requestLogger,
	}
}

// ExecuteResult contains the result of a request execution
type ExecuteResult struct {
	Response    *http.Response
	APIKey      *models.APIKey
	UpstreamURL string
}

// ExecuteRequestWithRetry is the core recursive function for handling requests and retries.
// For aggregate groups, it supports cross-sub-group retry when all retries within a sub-group are exhausted.
// Returns the response for successful requests, or nil if an error was already sent to the client.
func (rh *RetryHandler) ExecuteRequestWithRetry(
	c *gin.Context,
	channelHandler channel.ChannelProxy,
	originalGroup *models.Group,
	group *models.Group,
	bodyBytes []byte,
	isStream bool,
	startTime time.Time,
	retryCount int,
	originalModel string,
	excludedSubGroupIDs utils.UintSet,
) *ExecuteResult {
	cfg := group.EffectiveConfig

	apiKey, err := rh.keyProvider.SelectKey(group.ID)
	if err != nil {
		logrus.Errorf("Failed to select a key for group %s on attempt %d: %v", group.Name, retryCount+1, err)
		response.Error(c, app_errors.NewAPIError(app_errors.ErrNoKeysAvailable, err.Error()))
		rh.requestLogger.LogRequest(c, originalGroup, group, nil, startTime, http.StatusServiceUnavailable, err, isStream, "", channelHandler, bodyBytes, nil, models.RequestTypeFinal, originalModel)
		return nil
	}

	var upstreamURL string
	if originalGroup.GroupType == "aggregate" && originalGroup.ID != group.ID {
		upstreamURL, err = channelHandler.BuildUpstreamURLForAggregate(c.Request.URL, originalGroup.Name)
	} else {
		upstreamURL, err = channelHandler.BuildUpstreamURL(c.Request.URL, originalGroup.Name)
	}
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to build upstream URL: %v", err)))
		rh.requestLogger.LogRequest(c, originalGroup, group, apiKey, startTime, http.StatusInternalServerError, err, isStream, "", channelHandler, bodyBytes, nil, models.RequestTypeFinal, originalModel)
		return nil
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
		rh.requestLogger.LogRequest(c, originalGroup, group, apiKey, startTime, http.StatusInternalServerError, err, isStream, upstreamURL, channelHandler, bodyBytes, nil, models.RequestTypeFinal, originalModel)
		return nil
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
		rh.requestLogger.LogRequest(c, originalGroup, group, apiKey, startTime, http.StatusBadRequest, err, isStream, upstreamURL, channelHandler, bodyBytes, nil, models.RequestTypeFinal, originalModel)
		return nil
	}

	// Update request body if it was modified by redirection
	if !bytes.Equal(finalBodyBytes, bodyBytes) {
		req.Body = io.NopCloser(bytes.NewReader(finalBodyBytes))
		req.ContentLength = int64(len(finalBodyBytes))
	}

	channelHandler.ModifyRequest(req, apiKey, group)

	// Merge HeaderRules: aggregate first, then subgroup (subgroup takes precedence)
	if originalGroup.GroupType == "aggregate" && originalGroup.ID != group.ID {
		// Apply aggregate group's HeaderRules first (base configuration)
		if len(originalGroup.HeaderRuleList) > 0 {
			headerCtx := utils.NewHeaderVariableContextFromGin(c, originalGroup, apiKey)
			utils.ApplyHeaderRules(req, originalGroup.HeaderRuleList, headerCtx)
		}
		// Then apply subgroup's HeaderRules (takes precedence on conflicts)
		if len(group.HeaderRuleList) > 0 {
			headerCtx := utils.NewHeaderVariableContextFromGin(c, group, apiKey)
			utils.ApplyHeaderRules(req, group.HeaderRuleList, headerCtx)
		}
	} else {
		// Standard group or aggregate group acting as subgroup
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

	// Unified error handling for retries. Exclude 404 from being a retryable error.
	if err != nil || (resp != nil && resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound) {
		// Close response body on error paths
		if resp != nil {
			defer resp.Body.Close()
		}
		if err != nil && app_errors.IsIgnorableError(err) {
			logrus.Debugf("Client-side ignorable error for key %s, aborting retries: %v", utils.MaskAPIKey(apiKey.KeyValue), err)
			rh.requestLogger.LogRequest(c, originalGroup, group, apiKey, startTime, 499, err, isStream, upstreamURL, channelHandler, bodyBytes, nil, models.RequestTypeFinal, originalModel)
			return nil
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
		rh.keyProvider.UpdateStatus(apiKey, group, false, parsedError)

		// Check if this is the last attempt
		isLastAttempt := retryCount >= cfg.MaxRetries
		requestType := models.RequestTypeRetry
		if isLastAttempt {
			requestType = models.RequestTypeFinal
		}

		rh.requestLogger.LogRequest(c, originalGroup, group, apiKey, startTime, statusCode, errors.New(parsedError), isStream, upstreamURL, channelHandler, bodyBytes, nil, requestType, originalModel)

		// If this is the last attempt, check if it's an aggregate group and try switching to another sub-group
		if isLastAttempt && originalGroup.GroupType == "aggregate" {
			if rh.trySwitchToAnotherSubGroup(c, originalGroup, group, originalModel, finalBodyBytes, isStream, startTime, excludedSubGroupIDs, statusCode, errorMessage) {
				return nil
			}
			// All sub-groups have been tried, return error
			rh.returnUpstreamError(c, statusCode, errorMessage)
			return nil
		}

		// If this is the last attempt, return error directly without recursion
		if isLastAttempt {
			rh.returnUpstreamError(c, statusCode, errorMessage)
			return nil
		}

		return rh.ExecuteRequestWithRetry(c, channelHandler, originalGroup, group, finalBodyBytes, isStream, startTime, retryCount+1, originalModel, excludedSubGroupIDs)
	}

	logrus.Debugf("Request for group %s succeeded on attempt %d with key %s", group.Name, retryCount+1, utils.MaskAPIKey(apiKey.KeyValue))

	// Return the successful response for the caller to handle
	return &ExecuteResult{
		Response:    resp,
		APIKey:      apiKey,
		UpstreamURL: upstreamURL,
	}
}

// trySwitchToAnotherSubGroup attempts to switch to another sub-group for aggregate groups.
// Returns true if successfully switched and retry initiated, false otherwise.
func (rh *RetryHandler) trySwitchToAnotherSubGroup(
	c *gin.Context,
	originalGroup *models.Group,
	exhaustedGroup *models.Group,
	originalModel string,
	bodyBytes []byte,
	isStream bool,
	startTime time.Time,
	excludedSubGroupIDs utils.UintSet,
	statusCode int,
	errorMessage string,
) bool {
	excludedSubGroupIDs.Add(exhaustedGroup.ID)

	newSelection, err := rh.subGroupManager.SelectSubGroupExcluding(originalGroup, originalModel, excludedSubGroupIDs)
	if err != nil || newSelection == nil {
		return false
	}

	logrus.WithFields(logrus.Fields{
		"aggregate_group":    originalGroup.Name,
		"model":              originalModel,
		"exhausted_subgroup": exhaustedGroup.Name,
		"next_subgroup":      newSelection.GroupName,
	}).Info("Sub-group exhausted, switching to next sub-group for model")

	newGroup, err := rh.groupManager.GetGroupByName(newSelection.GroupName)
	if err != nil {
		return false
	}

	newChannelHandler, err := rh.channelFactory.GetChannel(newGroup)
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

	// Use the result from retry handler
	result := rh.ExecuteRequestWithRetry(c, newChannelHandler, originalGroup, newGroup, updatedBodyBytes, isStream, startTime, 0, originalModel, excludedSubGroupIDs)
	return result != nil
}

// returnUpstreamError is a helper function to return upstream error response
func (rh *RetryHandler) returnUpstreamError(c *gin.Context, statusCode int, errorMessage string) {
	var errorJSON map[string]any
	if err := json.Unmarshal([]byte(errorMessage), &errorJSON); err == nil {
		c.JSON(statusCode, errorJSON)
	} else {
		response.Error(c, app_errors.NewAPIErrorWithUpstream(statusCode, "UPSTREAM_ERROR", errorMessage))
	}
}
