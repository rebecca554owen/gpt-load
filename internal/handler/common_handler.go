package handler

import (
	"gpt-load/internal/channel"
	"gpt-load/internal/response"

	"github.com/gin-gonic/gin"
)

// CommonHandler 处理通用的非分组请求
type CommonHandler struct{}

// NewCommonHandler 创建新的 CommonHandler
func NewCommonHandler() *CommonHandler {
	return new(CommonHandler)
}

// GetChannelTypes 返回可用的通道类型列表
func (h *CommonHandler) GetChannelTypes(c *gin.Context) {
	channelTypes := channel.GetChannels()
	response.Success(c, channelTypes)
}
