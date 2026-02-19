package models

import (
	"gpt-load/internal/types"
	"time"

	"gorm.io/datatypes"
)

// Key status constants
const (
	KeyStatusActive  = "active"
	KeyStatusInvalid = "invalid"
)

// SystemSetting corresponds to system_settings table
type SystemSetting struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SettingKey   string    `gorm:"type:varchar(255);not null;unique" json:"setting_key"`
	SettingValue string    `gorm:"type:text;not null" json:"setting_value"`
	Description  string    `gorm:"type:varchar(512)" json:"description"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// GroupConfig stores group-specific configuration
type GroupConfig struct {
	RequestTimeout               *int    `json:"request_timeout,omitempty"`
	IdleConnTimeout              *int    `json:"idle_conn_timeout,omitempty"`
	ConnectTimeout               *int    `json:"connect_timeout,omitempty"`
	MaxIdleConns                 *int    `json:"max_idle_conns,omitempty"`
	MaxIdleConnsPerHost          *int    `json:"max_idle_conns_per_host,omitempty"`
	ResponseHeaderTimeout        *int    `json:"response_header_timeout,omitempty"`
	ProxyURL                     *string `json:"proxy_url,omitempty"`
	MaxRetries                   *int    `json:"max_retries,omitempty"`
	BlacklistThreshold           *int    `json:"blacklist_threshold,omitempty"`
	KeyValidationIntervalMinutes *int    `json:"key_validation_interval_minutes,omitempty"`
	KeyValidationConcurrency     *int    `json:"key_validation_concurrency,omitempty"`
	KeyValidationTimeoutSeconds  *int    `json:"key_validation_timeout_seconds,omitempty"`
	EnableRequestBodyLogging     *bool   `json:"enable_request_body_logging,omitempty"`
}

// HeaderRule defines a single rule for header manipulation.
type HeaderRule struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Action string `json:"action"` // "set" or "remove"
}

// GroupSubGroup association table for aggregate groups and sub-groups
type GroupSubGroup struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupID    uint      `gorm:"not null;uniqueIndex:idx_group_sub" json:"group_id"`
	SubGroupID uint      `gorm:"not null;uniqueIndex:idx_group_sub" json:"sub_group_id"`
	Weight     int       `gorm:"default:0" json:"weight"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Lightweight association - only store necessary info for performance
	SubGroupName string `gorm:"-" json:"sub_group_name,omitempty"`
}

// SubGroupInfo sub-group information for API response
type SubGroupInfo struct {
	Group       Group `json:"group"`
	Weight      int   `json:"weight"`
	TotalKeys   int64 `json:"total_keys"`
	ActiveKeys  int64 `json:"active_keys"`
	InvalidKeys int64 `json:"invalid_keys"`
}

// ParentAggregateGroupInfo parent aggregate group information for API response
type ParentAggregateGroupInfo struct {
	GroupID     uint   `json:"group_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Weight      int    `json:"weight"`
}

// ModelMappingTarget defines a single target for model mapping
type ModelMappingTarget struct {
	SubGroupID   uint     `json:"sub_group_id"`
	Weight       int      `json:"weight"`
	SubGroupName string   `json:"sub_group_name,omitempty"`
	Model        string   `json:"model"`  // For backward compatibility
	Models       []string `json:"models"` // Multi-model support
}

// ModelMapping defines mapping from model name to sub-group collection
type ModelMapping struct {
	Model   string               `json:"model"`
	Targets []ModelMappingTarget `json:"targets"`
}

// Group corresponds to groups table
type Group struct {
	ID                  uint                 `gorm:"primaryKey;autoIncrement" json:"id"`
	EffectiveConfig     types.SystemSettings `gorm:"-" json:"effective_config,omitempty"`
	Name                string               `gorm:"type:varchar(255);not null;unique" json:"name"`
	Endpoint            string               `gorm:"-" json:"endpoint"`
	DisplayName         string               `gorm:"type:varchar(255)" json:"display_name"`
	ProxyKeys           string               `gorm:"type:text" json:"proxy_keys"`
	Description         string               `gorm:"type:varchar(512)" json:"description"`
	GroupType           string               `gorm:"type:varchar(50);default:'standard'" json:"group_type"` // 'standard' or 'aggregate'
	Upstreams           datatypes.JSON       `gorm:"type:json;not null" json:"upstreams"`
	ValidationEndpoint  string               `gorm:"type:varchar(255)" json:"validation_endpoint"`
	ChannelType         string               `gorm:"type:varchar(50);not null" json:"channel_type"`
	Sort                int                  `gorm:"default:0" json:"sort"`
	TestModel           string               `gorm:"type:varchar(255);not null" json:"test_model"`
	ParamOverrides      datatypes.JSONMap    `gorm:"type:json" json:"param_overrides"`
	Config              datatypes.JSONMap    `gorm:"type:json" json:"config"`
	HeaderRules         datatypes.JSON       `gorm:"type:json" json:"header_rules"`
	ModelRedirectRules  datatypes.JSONMap    `gorm:"type:json" json:"model_redirect_rules"`
	ModelRedirectStrict bool                 `gorm:"default:false" json:"model_redirect_strict"`
	ModelMappings       datatypes.JSON       `gorm:"type:json" json:"model_mappings"`
	ModelMappingStrict  bool                 `gorm:"default:false" json:"model_mapping_strict"`
	ModelMappingList    []ModelMapping       `gorm:"-" json:"model_mappings_list,omitempty"`
	APIKeys             []APIKey             `gorm:"foreignKey:GroupID" json:"api_keys"`
	SubGroups           []GroupSubGroup      `gorm:"-" json:"sub_groups,omitempty"`
	LastValidatedAt     *time.Time           `json:"last_validated_at"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`

	// For cache
	ProxyKeysMap     map[string]struct{} `gorm:"-" json:"-"`
	HeaderRuleList   []HeaderRule        `gorm:"-" json:"-"`
	ModelRedirectMap map[string]string   `gorm:"-" json:"-"`
}

// APIKey corresponds to api_keys table
type APIKey struct {
	ID           uint       `gorm:"primaryKey;autoIncrement;index:idx_api_keys_group_last_used_id,priority:3" json:"id"`
	KeyValue     string     `gorm:"type:text;not null" json:"key_value"`
	KeyHash      string     `gorm:"type:varchar(128);index" json:"key_hash"`
	GroupID      uint       `gorm:"not null;index;index:idx_api_keys_group_last_used_id,priority:1" json:"group_id"`
	Status       string     `gorm:"type:varchar(50);not null;default:'active';index" json:"status"`
	Notes        string     `gorm:"type:varchar(255);default:''" json:"notes"`
	RequestCount int64      `gorm:"not null;default:0" json:"request_count"`
	FailureCount int64      `gorm:"not null;default:0" json:"failure_count"`
	LastUsedAt   *time.Time `gorm:"index:idx_api_keys_group_last_used_id,priority:2" json:"last_used_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// RequestType request type constants
const (
	RequestTypeRetry = "retry"
	RequestTypeFinal = "final"
)

// RequestLog corresponds to request_logs table
type RequestLog struct {
	ID              string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	Timestamp       time.Time `gorm:"not null;index:idx_request_logs_timestamp_request_type,priority:1" json:"timestamp"`
	GroupID         uint      `gorm:"not null;index" json:"group_id"`
	GroupName       string    `gorm:"type:varchar(255);index" json:"group_name"`
	ParentGroupID   uint      `gorm:"index" json:"parent_group_id"`
	ParentGroupName string    `gorm:"type:varchar(255);index" json:"parent_group_name"`
	KeyValue        string    `gorm:"type:text" json:"key_value"`
	KeyHash         string    `gorm:"type:varchar(128);index" json:"key_hash"`
	Model           string    `gorm:"type:varchar(255);index" json:"model"`
	OriginalModel   string    `gorm:"type:varchar(255);index" json:"original_model"` // Original requested model name, to distinguish from mapped model
	IsSuccess       bool      `gorm:"not null" json:"is_success"`
	SourceIP        string    `gorm:"type:varchar(64)" json:"source_ip"`
	StatusCode      int       `gorm:"not null" json:"status_code"`
	RequestPath     string    `gorm:"type:varchar(500)" json:"request_path"`
	Duration        int64     `gorm:"not null" json:"duration_ms"`
	ErrorMessage    string    `gorm:"type:text" json:"error_message"`
	UserAgent       string    `gorm:"type:varchar(512)" json:"user_agent"`
	RequestType     string    `gorm:"type:varchar(20);not null;default:'final';index:idx_request_logs_timestamp_request_type,priority:2" json:"request_type"`
	UpstreamAddr    string    `gorm:"type:varchar(500)" json:"upstream_addr"`
	IsStream        bool      `gorm:"not null" json:"is_stream"`
	RequestBody     string    `gorm:"type:text" json:"request_body"`
	// Token statistics fields
	PromptTokens     int64 `gorm:"default:0" json:"prompt_tokens"`     // Input prompt tokens
	CompletionTokens int64 `gorm:"default:0" json:"completion_tokens"` // Output completion tokens
	TotalTokens      int64 `gorm:"default:0" json:"total_tokens"`      // Total tokens
	CachedTokens     int64 `gorm:"default:0" json:"cached_tokens"`     // Cached tokens (Claude)
}

// StatCard stat card data for dashboard
type StatCard struct {
	Value         float64 `json:"value"`
	SubValue      int64   `json:"sub_value,omitempty"`
	SubValueTip   string  `json:"sub_value_tip,omitempty"`
	Trend         float64 `json:"trend"`
	TrendIsGrowth bool    `json:"trend_is_growth"`
}

// SecurityWarning security warning information
type SecurityWarning struct {
	Type       string `json:"type"`       // Warning type: auth_key, encryption_key, etc.
	Message    string `json:"message"`    // Warning message
	Severity   string `json:"severity"`   // Severity level: low, medium, high
	Suggestion string `json:"suggestion"` // Suggested solution
}

// DashboardStatsResponse API response for dashboard basic statistics
type DashboardStatsResponse struct {
	KeyCount         StatCard          `json:"key_count"`         // Key count
	TokenConsumption StatCard          `json:"token_consumption"` // Total tokens
	PromptTokens     StatCard          `json:"prompt_tokens"`     // Input tokens (total, includes cached)
	UncachedPromptTokens StatCard      `json:"uncached_prompt_tokens"` // Input tokens (non-cached, excludes cached)
	CachedTokens     StatCard          `json:"cached_tokens"`     // Cached tokens (input)
	CompletionTokens StatCard          `json:"completion_tokens"` // Completion tokens (output)
	TotalTokens      StatCard          `json:"total_tokens"`      // Total tokens
	RPM              StatCard          `json:"rpm"`               // RPM
	RequestCount     StatCard          `json:"request_count"`     // Request count
	ErrorRate        StatCard          `json:"error_rate"`        // Error rate
	SecurityWarnings []SecurityWarning `json:"security_warnings"`
}

// ChartDataset dataset for charts
type ChartDataset struct {
	Label     string  `json:"label"`     // Translated display name
	LabelKey  string  `json:"label_key"` // i18n key for color mapping
	Data      []int64 `json:"data"`
}

// ChartData API response for charts
type ChartData struct {
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
}

// GroupHourlyStat corresponds to group_hourly_stats table, stores hourly request statistics for each group
type GroupHourlyStat struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Time         time.Time `gorm:"not null;uniqueIndex:idx_group_time" json:"time"` // Hourly timestamp
	GroupID      uint      `gorm:"not null;uniqueIndex:idx_group_time" json:"group_id"`
	SuccessCount int64     `gorm:"not null;default:0" json:"success_count"`
	FailureCount int64     `gorm:"not null;default:0" json:"failure_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
