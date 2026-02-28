package proxy

import (
	"time"

	"gpt-load/internal/channel"
	"gpt-load/internal/encryption"
	"gpt-load/internal/models"
	"gpt-load/internal/services"
	"gpt-load/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RequestLogger handles request logging functionality
type RequestLogger struct {
	requestLogService *services.RequestLogService
	encryptionSvc     encryption.Service
}

// NewRequestLogger creates a new RequestLogger
func NewRequestLogger(
	requestLogService *services.RequestLogService,
	encryptionSvc encryption.Service,
) *RequestLogger {
	return &RequestLogger{
		requestLogService: requestLogService,
		encryptionSvc:     encryptionSvc,
	}
}

// LogRequest creates and records a request log entry.
// LogRequest 会创建并记录请求日志条目。
func (rl *RequestLogger) LogRequest(
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
	if rl.requestLogService == nil {
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
		encryptedKeyValue, err := rl.encryptionSvc.Encrypt(apiKey.KeyValue)
		if err != nil {
			logrus.WithError(err).Error("Failed to encrypt key value for logging")
			logEntry.KeyValue = "failed-to-encryption"
		} else {
			logEntry.KeyValue = encryptedKeyValue
		}
		// Add KeyHash for reverse lookup
		logEntry.KeyHash = rl.encryptionSvc.Hash(apiKey.KeyValue)
	}

	if finalError != nil {
		logEntry.ErrorMessage = finalError.Error()
	}

	if err := rl.requestLogService.Record(logEntry); err != nil {
		logrus.Errorf("Failed to record request log: %v", err)
	}
}
