package models

// SystemSettingInfo 表示详细的系统设置信息（用于 API 响应）
type SystemSettingInfo struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Value        any    `json:"value"`
	Type         string `json:"type"` // "int", "bool", "string"
	DefaultValue any    `json:"default_value"`
	Description  string `json:"description"`
	Category     string `json:"category"`
	MinValue     *int   `json:"min_value,omitempty"`
	Required     bool   `json:"required"`
}

// CategorizedSettings 按类别分组的设置列表
type CategorizedSettings struct {
	CategoryName string              `json:"category_name"`
	Settings     []SystemSettingInfo `json:"settings"`
}
