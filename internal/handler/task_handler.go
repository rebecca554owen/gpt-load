package handler

import (
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/response"

	"github.com/gin-gonic/gin"
)

// GetTaskStatus 处理全局长运行任务的状态请求
func (s *Server) GetTaskStatus(c *gin.Context) {
	taskStatus, err := s.TaskService.GetTaskStatus()
	if err != nil {
		response.ErrorI18nFromAPIError(c, app_errors.ErrInternalServer, "task.get_status_failed")
		return
	}
	response.Success(c, taskStatus)
}
