package services

import (
	"encoding/csv"
	"fmt"
	"gpt-load/internal/encryption"
	"gpt-load/internal/models"
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ExportableLogKey 定义要导出到 CSV 的数据结构。
type ExportableLogKey struct {
	KeyValue   string `gorm:"column:key_value"`
	GroupName  string `gorm:"column:group_name"`
	StatusCode int    `gorm:"column:status_code"`
}

// LogService 提供与请求日志相关的服务。
type LogService struct {
	DB            *gorm.DB
	EncryptionSvc encryption.Service
}

// NewLogService 创建一个新的 LogService。
func NewLogService(db *gorm.DB, encryptionSvc encryption.Service) *LogService {
	return &LogService{
		DB:            db,
		EncryptionSvc: encryptionSvc,
	}
}

// logFiltersScope 返回应用 Gin 上下文过滤器的 GORM scope 函数。
func (s *LogService) logFiltersScope(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if parentGroupName := c.Query("parent_group_name"); parentGroupName != "" {
			db = db.Where("parent_group_name LIKE ?", "%"+parentGroupName+"%")
		}
		if groupName := c.Query("group_name"); groupName != "" {
			db = db.Where("group_name LIKE ?", "%"+groupName+"%")
		}
		if keyValue := c.Query("key_value"); keyValue != "" {
			keyHash := s.EncryptionSvc.Hash(keyValue)
			db = db.Where("key_hash = ?", keyHash)
		}
		if model := c.Query("model"); model != "" {
			db = db.Where("model LIKE ?", "%"+model+"%")
		}
		if isSuccessStr := c.Query("is_success"); isSuccessStr != "" {
			if isSuccess, err := strconv.ParseBool(isSuccessStr); err == nil {
				db = db.Where("is_success = ?", isSuccess)
			}
		}
		if requestType := c.Query("request_type"); requestType != "" {
			db = db.Where("request_type = ?", requestType)
		}
		if statusCodeStr := c.Query("status_code"); statusCodeStr != "" {
			if statusCode, err := strconv.Atoi(statusCodeStr); err == nil {
				db = db.Where("status_code = ?", statusCode)
			}
		}
		if sourceIP := c.Query("source_ip"); sourceIP != "" {
			db = db.Where("source_ip = ?", sourceIP)
		}
		if errorContains := c.Query("error_contains"); errorContains != "" {
			db = db.Where("error_message LIKE ?", "%"+errorContains+"%")
		}
		if startTimeStr := c.Query("start_time"); startTimeStr != "" {
			if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
				db = db.Where("timestamp >= ?", startTime)
			}
		}
		if endTimeStr := c.Query("end_time"); endTimeStr != "" {
			if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
				db = db.Where("timestamp <= ?", endTime)
			}
		}
		return db
	}
}

// GetLogsQuery 返回用于获取带过滤器的日志的 GORM 查询。
func (s *LogService) GetLogsQuery(c *gin.Context) *gorm.DB {
	return s.DB.Model(&models.RequestLog{}).Scopes(s.logFiltersScope(c))
}

// StreamLogKeysToCSV 根据过滤器从日志中获取唯一密钥，并将它们作为 CSV 流式传输。
func (s *LogService) StreamLogKeysToCSV(c *gin.Context, writer io.Writer) error {
	// Create a CSV writer
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Write CSV header
	header := []string{"key_value", "group_name", "status_code"}
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	var results []ExportableLogKey

	baseQuery := s.DB.Model(&models.RequestLog{}).Scopes(s.logFiltersScope(c)).Where("key_hash IS NOT NULL AND key_hash != ''")

	// Use window function to get latest record for each key_hash (avoid duplicates from multiple encryptions)
	err := s.DB.Raw(`
		SELECT
			key_value,
			group_name,
			status_code
		FROM (
			SELECT
				key_value,
				key_hash,
				group_name,
				status_code,
				ROW_NUMBER() OVER (PARTITION BY key_hash ORDER BY timestamp DESC) as rn
			FROM (?) as filtered_logs
		) ranked
		WHERE rn = 1
		ORDER BY key_hash
	`, baseQuery).Scan(&results).Error

	if err != nil {
		return fmt.Errorf("failed to fetch log keys: %w", err)
	}

	// Decrypt and write CSV data
	for _, record := range results {
		// Decrypt key for CSV export
		decryptedKey := record.KeyValue
		if record.KeyValue != "" {
			if decrypted, err := s.EncryptionSvc.Decrypt(record.KeyValue); err != nil {
				logrus.WithError(err).WithField("key_value", record.KeyValue).Error("Failed to decrypt key for CSV export")
				decryptedKey = "failed-to-decrypt"
			} else {
				decryptedKey = decrypted
			}
		}

		csvRecord := []string{
			decryptedKey,
			record.GroupName,
			strconv.Itoa(record.StatusCode),
		}
		if err := csvWriter.Write(csvRecord); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}
