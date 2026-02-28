package utils

import (
	"gpt-load/internal/models"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// HeaderVariableContext 保存用于变量解析的上下文数据
type HeaderVariableContext struct {
	ClientIP string
	Group    *models.Group
	APIKey   *models.APIKey
}

// ResolveHeaderVariables 解析标头值中的动态变量
func ResolveHeaderVariables(value string, ctx *HeaderVariableContext) string {
	if ctx == nil {
		return value
	}

	now := time.Now()
	result := value

	// 替换所有支持的变量
	variables := map[string]string{
		"${CLIENT_IP}":    ctx.ClientIP,
		"${TIMESTAMP_MS}": strconv.FormatInt(now.UnixMilli(), 10),
		"${TIMESTAMP_S}":  strconv.FormatInt(now.Unix(), 10),
	}

	if ctx.Group != nil {
		variables["${GROUP_NAME}"] = ctx.Group.Name
	}

	if ctx.APIKey != nil {
		variables["${API_KEY}"] = ctx.APIKey.KeyValue
	}

	// 替换值中的变量
	for variable, replacement := range variables {
		result = strings.ReplaceAll(result, variable, replacement)
	}

	return result
}

// ApplyHeaderRules 将标头规则应用到 HTTP 请求
func ApplyHeaderRules(req *http.Request, rules []models.HeaderRule, ctx *HeaderVariableContext) {
	if req == nil || len(rules) == 0 {
		return
	}

	for _, rule := range rules {
		canonicalKey := http.CanonicalHeaderKey(rule.Key)

		switch rule.Action {
		case "remove":
			req.Header.Del(canonicalKey)
		case "set":
			resolvedValue := ResolveHeaderVariables(rule.Value, ctx)
			req.Header.Set(canonicalKey, resolvedValue)
		}
	}
}

// NewHeaderVariableContextFromGin 从 Gin 上下文创建 HeaderVariableContext
func NewHeaderVariableContextFromGin(c *gin.Context, group *models.Group, apiKey *models.APIKey) *HeaderVariableContext {
	if c == nil {
		return nil
	}

	return &HeaderVariableContext{
		ClientIP: c.ClientIP(),
		Group:    group,
		APIKey:   apiKey,
	}
}

// NewHeaderVariableContext 创建不带 Gin 上下文的 HeaderVariableContext
func NewHeaderVariableContext(group *models.Group, apiKey *models.APIKey) *HeaderVariableContext {
	return &HeaderVariableContext{
		ClientIP: "127.0.0.1",
		Group:    group,
		APIKey:   apiKey,
	}
}
