// Package response 提供标准化的 JSON 响应辅助函数。
package response

import (
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/i18n"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SuccessResponse 定义标准 JSON 成功响应结构。
type SuccessResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ErrorResponse 定义标准 JSON 错误响应结构。
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Success 发送标准化的成功响应。
func Success(c *gin.Context, data any) {
	message := i18n.Message(c, "common.success")
	c.JSON(http.StatusOK, SuccessResponse{
		Code:    0,
		Message: message,
		Data:    data,
	})
}

// Error 使用 APIError 发送标准化错误响应。
func Error(c *gin.Context, apiErr *app_errors.APIError) {
	c.JSON(apiErr.HTTPStatus, ErrorResponse{
		Code:    apiErr.Code,
		Message: apiErr.Message,
	})
}

// SuccessI18n 发送带有 i18n 消息的标准化成功响应。
func SuccessI18n(c *gin.Context, msgID string, data any, templateData ...map[string]any) {
	message := i18n.Message(c, msgID, templateData...)
	c.JSON(http.StatusOK, SuccessResponse{
		Code:    0,
		Message: message,
		Data:    data,
	})
}

// ErrorI18n 发送带有 i18n 消息的标准化错误响应。
func ErrorI18n(c *gin.Context, httpStatus int, code string, msgID string, templateData ...map[string]any) {
	message := i18n.Message(c, msgID, templateData...)
	c.JSON(httpStatus, ErrorResponse{
		Code:    code,
		Message: message,
	})
}

// ErrorI18nFromAPIError 使用 APIError 和 i18n 消息发送标准化错误响应。
func ErrorI18nFromAPIError(c *gin.Context, apiErr *app_errors.APIError, msgID string, templateData ...map[string]any) {
	message := i18n.Message(c, msgID, templateData...)
	c.JSON(apiErr.HTTPStatus, ErrorResponse{
		Code:    apiErr.Code,
		Message: message,
	})
}
