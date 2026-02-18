package handler

import (
	"gpt-load/internal/channel"
	"gpt-load/internal/response"

	"github.com/gin-gonic/gin"
)

// CommonHandler handles common, non-grouped requests.
type CommonHandler struct{}

// NewCommonHandler creates a new CommonHandler.
func NewCommonHandler() *CommonHandler {
	return new(CommonHandler)
}

// GetChannelTypes returns a list of available channel types.
func (h *CommonHandler) GetChannelTypes(c *gin.Context) {
	channelTypes := channel.GetChannels()
	response.Success(c, channelTypes)
}
