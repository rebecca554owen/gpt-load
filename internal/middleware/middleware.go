// Package middleware 提供应用程序的 HTTP 中间件
package middleware

import (
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/response"
	"gpt-load/internal/services"
	"gpt-load/internal/types"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Logger 创建一个高性能日志中间件
func Logger(config types.LogConfig) gin.HandlerFunc {
	return func(c *gin.Context) {

		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 计算响应时间
		latency := time.Since(start)

		// 获取基本信息
		method := c.Request.Method
		statusCode := c.Writer.Status()

		// 构建完整路径（避免字符串拼接）
		fullPath := path
		if raw != "" {
			fullPath = path + "?" + raw
		}

		// 获取密钥信息（如果存在）
		keyInfo := ""
		if keyIndex, exists := c.Get("keyIndex"); exists {
			if keyPreview, exists := c.Get("keyPreview"); exists {
				keyInfo = fmt.Sprintf(" - Key[%v] %v", keyIndex, keyPreview)
			}
		}

		// 获取重试信息（如果存在）
		retryInfo := ""
		if retryCount, exists := c.Get("retryCount"); exists {
			retryInfo = fmt.Sprintf(" - Retry[%d]", retryCount)
		}

		// 过滤健康检查和其他监控端点日志以减少噪音
		if isMonitoringEndpoint(path) {
			// 监控端点仅记录错误
			if statusCode >= 400 {
				logrus.Warnf("%s %s - %d - %v", method, fullPath, statusCode, latency)
			}
			return
		}

		// 根据状态码选择日志级别
		if statusCode >= 500 {
			logrus.Errorf("%s %s - %d - %v%s%s", method, fullPath, statusCode, latency, keyInfo, retryInfo)
		} else if statusCode >= 400 {
			logrus.Warnf("%s %s - %d - %v%s%s", method, fullPath, statusCode, latency, keyInfo, retryInfo)
		} else {
			logrus.Infof("%s %s - %d - %v%s%s", method, fullPath, statusCode, latency, keyInfo, retryInfo)
		}
	}
}

// CORS 创建 CORS 中间件
func CORS(config types.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		origin := c.Request.Header.Get("Origin")

		// 检查来源是否允许
		allowed := false
		for _, allowedOrigin := range config.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		// 设置其他 CORS 头
		c.Header("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))

		if config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Auth 创建认证中间件
func Auth(authConfig types.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if isMonitoringEndpoint(path) {
			c.Next()
			return
		}

		key := extractAuthKey(c)

		isValid := key != "" && subtle.ConstantTimeCompare([]byte(key), []byte(authConfig.Key)) == 1

		if !isValid {
			response.Error(c, app_errors.ErrUnauthorized)
			c.Abort()
			return
		}

		c.Next()
	}
}

// ProxyAuth 创建代理认证中间件
func ProxyAuth(gm *services.GroupManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查密钥
		key := extractAuthKey(c)
		if key == "" {
			response.Error(c, app_errors.ErrUnauthorized)
			c.Abort()
			return
		}

		group, err := gm.GetGroupByName(c.Param("group_name"))
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, "Failed to retrieve proxy group"))
			c.Abort()
			return
		}

		// 检查两个密钥集合以防止时序攻击
		_, existsInEffective := group.EffectiveConfig.ProxyKeysMap[key]
		_, existsInGroup := group.ProxyKeysMap[key]

		if existsInEffective || existsInGroup {
			c.Next()
			return
		}

		response.Error(c, app_errors.ErrUnauthorized)
		c.Abort()
	}
}

// ProxyRouteDispatcher 在代理认证之前分发特殊路由
func ProxyRouteDispatcher(serverHandler interface{ GetIntegrationInfo(*gin.Context) }) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Param("path") == "/api/integration/info" {
			serverHandler.GetIntegrationInfo(c)
			c.Abort()
			return
		}

		c.Next()
	}
}

// Recovery 创建带有自定义错误处理的恢复中间件
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logrus.Errorf("Panic recovered: %v", recovered)
		response.Error(c, app_errors.ErrInternalServer)
		c.Abort()
	})
}

// RateLimiter 创建简单的限流中间件
func RateLimiter(config types.PerformanceConfig) gin.HandlerFunc {
	// 基于信号量的简单限流
	semaphore := make(chan struct{}, config.MaxConcurrentRequests)

	return func(c *gin.Context) {
		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
			c.Next()
		default:
			response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, "Too many concurrent requests"))
			c.Abort()
		}
	}
}

// ErrorHandler 创建错误处理中间件
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 处理请求处理期间发生的任何错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			// 检查是否为自定义错误类型
			if apiErr, ok := err.(*app_errors.APIError); ok {
				response.Error(c, apiErr)
				return
			}

			// 处理其他错误
			logrus.Errorf("Unhandled error: %v", err)
			response.Error(c, app_errors.ErrInternalServer)
		}
	}
}

var monitoringPaths = []string{"/health"}

// isMonitoringEndpoint 检查路径是否为监控端点
func isMonitoringEndpoint(path string) bool {
	for _, monitoringPath := range monitoringPaths {
		if path == monitoringPath {
			return true
		}
	}
	return false
}

// extractAuthKey 提取认证密钥
func extractAuthKey(c *gin.Context) string {
	// 查询密钥
	if key := c.Query("key"); key != "" {
		query := c.Request.URL.Query()
		query.Del("key")
		c.Request.URL.RawQuery = query.Encode()
		return key
	}

	// Bearer 令牌
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		const bearerPrefix = "Bearer "
		if strings.HasPrefix(authHeader, bearerPrefix) {
			return authHeader[len(bearerPrefix):]
		}
	}

	// X-Api-Key 头
	if key := c.GetHeader("X-Api-Key"); key != "" {
		return key
	}

	// X-Goog-Api-Key 头
	if key := c.GetHeader("X-Goog-Api-Key"); key != "" {
		return key
	}

	return ""
}

// StaticCache 创建用于缓存静态资源的中间件
func StaticCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if isStaticResource(path) {
			c.Header("Cache-Control", "public, max-age=2592000, immutable")
			c.Header("Expires", time.Now().AddDate(1, 0, 0).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
		}

		c.Next()
	}
}

// isStaticResource 检查是否为静态资源
func isStaticResource(path string) bool {
	staticPrefixes := []string{"/assets/"}
	staticSuffixes := []string{
		".js", ".css", ".ico", ".png", ".jpg", ".jpeg",
		".gif", ".svg", ".woff", ".woff2", ".ttf", ".eot",
		".webp", ".avif", ".map",
	}

	// 检查路径前缀
	for _, prefix := range staticPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// 检查文件扩展名
	for _, suffix := range staticSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}

	return false
}

// SecurityHeaders 创建用于添加安全相关头的中间件
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Next()
	}
}
