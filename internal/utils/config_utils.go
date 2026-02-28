package utils

import (
	"fmt"
	"gpt-load/internal/models"
	"gpt-load/internal/types"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// GenerateSettingsMetadata 使用反射从 SystemSettings 结构生成元数据
func GenerateSettingsMetadata(s *types.SystemSettings) []models.SystemSettingInfo {
	var settingsInfo []models.SystemSettingInfo
	v := reflect.ValueOf(s).Elem()
	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		fieldValue := v.Field(i)

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		nameTag := field.Tag.Get("name")
		descTag := field.Tag.Get("desc")
		defaultTag := field.Tag.Get("default")
		validateTag := field.Tag.Get("validate")
		categoryTag := field.Tag.Get("category")

		var minValue *int
		var required bool

		rules := strings.Split(validateTag, ",")
		for _, rule := range rules {
			rule = strings.TrimSpace(rule)
			if rule == "required" {
				required = true
			} else if strings.HasPrefix(rule, "min=") {
				valStr := strings.TrimPrefix(rule, "min=")
				if val, err := strconv.Atoi(valStr); err == nil {
					minValue = &val
				}
			}
		}

		info := models.SystemSettingInfo{
			Key:          jsonTag,
			Name:         nameTag,
			Value:        fieldValue.Interface(),
			Type:         field.Type.String(),
			DefaultValue: defaultTag,
			Description:  descTag,
			Category:     categoryTag,
			MinValue:     minValue,
			Required:     required,
		}
		settingsInfo = append(settingsInfo, info)
	}
	return settingsInfo
}

// DefaultSystemSettings 返回默认系统配置
func DefaultSystemSettings() types.SystemSettings {
	s := types.SystemSettings{}
	v := reflect.ValueOf(&s).Elem()
	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		defaultTag := field.Tag.Get("default")
		if defaultTag == "" {
			continue
		}

		fieldValue := v.Field(i)
		if fieldValue.CanSet() {
			if err := SetFieldFromString(fieldValue, defaultTag); err != nil {
				logrus.Warnf("Failed to set default value for field %s: %v", field.Name, err)
			}
		}
	}
	return s
}

// SetFieldFromString 根据字段的类型从字符串设置结构字段的值。
func SetFieldFromString(fieldValue reflect.Value, value string) error {
	if !fieldValue.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	switch fieldValue.Kind() {
	case reflect.Int:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value '%s': %w", value, err)
		}
		fieldValue.SetInt(int64(intVal))
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value '%s': %w", value, err)
		}
		fieldValue.SetBool(boolVal)
	case reflect.String:
		fieldValue.SetString(value)
	default:
		return fmt.Errorf("unsupported field kind: %s", fieldValue.Kind())
	}
	return nil
}

// ParseInteger 解析整数环境变量
func ParseInteger(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return defaultValue
}

// ParseBoolean 解析布尔环境变量
func ParseBoolean(value string, defaultValue bool) bool {
	if value == "" {
		return defaultValue
	}

	lowerValue := strings.ToLower(value)
	switch lowerValue {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return defaultValue
	}
}

// ParseArray 解析数组环境变量（逗号分隔）
func ParseArray(value string, defaultValue []string) []string {
	if value == "" {
		return defaultValue
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return defaultValue
	}
	return result
}

// GetEnvOrDefault 获取环境变量或默认值
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetValidationEndpoint 返回组的有效验证端点。
func GetValidationEndpoint(group *models.Group) string {
	if group.ValidationEndpoint != "" {
		return group.ValidationEndpoint
	}

	// 根据通道类型返回默认验证端点
	switch group.ChannelType {
	case "openai":
		return "/v1/chat/completions"
	case "openai-response":
		return "/v1/responses"
	case "anthropic":
		return "/v1/messages"
	default:
		return ""
	}
}
