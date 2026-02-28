package handler

import (
	"fmt"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/i18n"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// LogResponse 定义 API 响应中日志条目的结构
type LogResponse struct {
	models.RequestLog
}

// GetLogs 处理获取请求日志，支持过滤和分页
func (s *Server) GetLogs(c *gin.Context) {
	query := s.LogService.GetLogsQuery(c)

	var logs []models.RequestLog
	query = query.Order("timestamp desc")
	pagination, err := response.Paginate(c, query, &logs)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// 为前端显示解密日志中的所有密钥
	for i := range logs {
		if logs[i].KeyValue != "" {
			decryptedValue, err := s.EncryptionSvc.Decrypt(logs[i].KeyValue)
			if err != nil {
				logrus.WithError(err).WithField("log_id", logs[i].ID).Error("Failed to decrypt log key value")
				logs[i].KeyValue = "failed-to-decrypt"
			} else {
				logs[i].KeyValue = decryptedValue
			}
		}
	}

	pagination.Items = logs
	response.Success(c, pagination)
}

// ExportLogs 处理导出过滤后的日志密钥到 CSV 文件
func (s *Server) ExportLogs(c *gin.Context) {
	filename := fmt.Sprintf("log_keys_export_%s.csv", time.Now().Format("20060102150405"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "text/csv; charset=utf-8")

	// 流式传输响应
	err := s.LogService.StreamLogKeysToCSV(c, c.Writer)
	if err != nil {
		logrus.WithError(err).Error("Failed to stream log keys to CSV")
		c.JSON(500, gin.H{"error": i18n.Message(c, "error.export_logs")})
		return
	}
}
