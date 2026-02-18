// Package config provides configuration constants for the application
package config

// Proxy configuration constants
const (
	// Request and timeout settings
	DefaultMaxRetries        = 3
	DefaultRequestTimeout    = 120
	MaxRequestBodyLogLength  = 65000
	MaxRequestPathLogLength  = 500
	MaxUpstreamAddrLogLength = 500

	// Key service batch sizes
	MaxRequestKeys = 5000
	KeyChunkSize   = 500

	// Key pool settings
	DBLoadBatchSize = 10000

	// Password minimum length
	MinPasswordLength       = 16
	RecommendedPasswordLength = 32
)
