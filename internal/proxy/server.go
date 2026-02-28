// Package proxy provides high-performance OpenAI multi-key proxy server
package proxy

import (
	"fmt"
	"io"
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
	requestLogger     *RequestLogger
	retryHandler      *RetryHandler
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
	requestLogger := NewRequestLogger(requestLogService, encryptionSvc)
	retryHandler := NewRetryHandler(keyProvider, groupManager, subGroupManager, channelFactory, requestLogger)

	return &ProxyServer{
		keyProvider:       keyProvider,
		groupManager:      groupManager,
		subGroupManager:   subGroupManager,
		settingsManager:   settingsManager,
		channelFactory:    channelFactory,
		requestLogService: requestLogService,
		encryptionSvc:     encryptionSvc,
		requestLogger:     requestLogger,
		retryHandler:      retryHandler,
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

	if normalizedBody, normalized := utils.NormalizeJSONRequestBody(bodyBytes, c.GetHeader("Content-Type")); normalized {
		logrus.WithFields(logrus.Fields{
			"group": originalGroup.Name,
		}).Debug("request body normalized from lenient JSON")
		bodyBytes = normalizedBody
	}

	var channelHandler channel.ChannelProxy
	if originalGroup.GroupType == "aggregate" {
		// For aggregate groups, use first subgroup to extract model name
		if len(originalGroup.SubGroups) > 0 {
			tempGroup, err := ps.groupManager.GetGroupByName(originalGroup.SubGroups[0].SubGroupName)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"aggregate_group": originalGroup.Name,
					"subgroup_name":   originalGroup.SubGroups[0].SubGroupName,
					"error":           err,
				}).Error("Failed to get first subgroup for model extraction")
			} else {
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

	originalModel := channelHandler.ExtractModel(c, bodyBytes)
	if originalModel == "" {
		logrus.WithField("group", originalGroup.Name).Warn("Empty model name extracted from request")
	}

	// Now select subgroup based on model name (for aggregate groups)
	group := originalGroup
	var selectedModel string
	if originalGroup.GroupType == "aggregate" && originalModel != "" {
		selection, err := ps.subGroupManager.SelectSubGroup(originalGroup, originalModel)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"aggregate_group": originalGroup.Name,
				"model":           originalModel,
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
		if selection.SelectedModel != "" && selection.SelectedModel != originalModel {
			selectedModel = selection.SelectedModel
		}
	}

	// Modify model in request body if mapping changed it
	finalBodyBytes := bodyBytes
	if selectedModel != "" {
		finalBodyBytes = utils.ModifyJSONField(bodyBytes, "model", selectedModel)
	}

	// Merge ParamOverrides: aggregate first, then subgroup (subgroup takes precedence)
	if originalGroup.GroupType == "aggregate" && originalGroup.ID != group.ID {
		// Apply aggregate group's ParamOverrides first (base configuration)
		finalBodyBytes, err = applyParamOverrides(finalBodyBytes, originalGroup)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to apply aggregate parameter overrides: %v", err)))
			return
		}
		// Then apply subgroup's ParamOverrides (takes precedence on conflicts)
		finalBodyBytes, err = applyParamOverrides(finalBodyBytes, group)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to apply subgroup parameter overrides: %v", err)))
			return
		}
	} else {
		finalBodyBytes, err = applyParamOverrides(finalBodyBytes, group)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to apply parameter overrides: %v", err)))
			return
		}
	}

	isStream := channelHandler.IsStreamRequest(c, bodyBytes)

	excludedSubGroupIDs := make(utils.UintSet)
	result := ps.retryHandler.ExecuteRequestWithRetry(c, channelHandler, originalGroup, group, finalBodyBytes, isStream, startTime, 0, originalModel, excludedSubGroupIDs)

	// Handle successful response
	if result != nil && result.Response != nil {
		resp := result.Response
		defer resp.Body.Close()
		apiKey := result.APIKey
		upstreamURL := result.UpstreamURL

		// Check if this is a model list request (needs special handling)
		if shouldInterceptModelList(c.Request.URL.Path, c.Request.Method) {
			ps.handleModelListResponse(c, resp, group, channelHandler)
			ps.requestLogger.LogRequest(c, originalGroup, group, apiKey, startTime, resp.StatusCode, nil, isStream, upstreamURL, channelHandler, finalBodyBytes, nil, models.RequestTypeFinal, originalModel)
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
			tokenUsage = ps.handleStreamingResponse(c, resp, channelHandler, originalModel, bodyBytes)
		} else {
			tokenUsage = ps.handleNormalResponse(c, resp, channelHandler, originalModel, bodyBytes)
		}

		ps.requestLogger.LogRequest(c, originalGroup, group, apiKey, startTime, resp.StatusCode, nil, isStream, upstreamURL, channelHandler, finalBodyBytes, tokenUsage, models.RequestTypeFinal, originalModel)
	}
}
