// Package config 提供应用配置常量
package config

// 代理配置常量
const (
	// 请求和超时设置
	DefaultMaxRetries        = 3
	DefaultRequestTimeout    = 120
	MaxRequestBodyLogLength  = 65000
	MaxRequestPathLogLength  = 500
	MaxUpstreamAddrLogLength = 500

	// 密钥服务批处理大小
	MaxRequestKeys = 5000
	KeyChunkSize   = 500

	// 密钥池设置
	DBLoadBatchSize = 10000

	// 密码最小长度
	MinPasswordLength       = 16
	RecommendedPasswordLength = 32
)
