package store

import (
	"context"
	"gpt-load/internal/types"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// NewStore creates a new store based on the application configuration.
func NewStore(cfg types.ConfigManager) (Store, error) {
	redisDSN := cfg.GetRedisDSN()
	if redisDSN != "" {
		opts, err := redis.ParseURL(redisDSN)
		if err != nil {
			logrus.Warnf("Failed to parse redis DSN: %v, falling back to in-memory store.", err)
			return NewMemoryStore(), nil
		}

		client := redis.NewClient(opts)
		if err := client.Ping(context.Background()).Err(); err != nil {
			logrus.Warnf("Failed to connect to redis: %v, falling back to in-memory store.", err)
			return NewMemoryStore(), nil
		}

		logrus.Debug("Successfully connected to Redis.")
		return NewRedisStore(client), nil
	}

	logrus.Info("Redis DSN not configured, falling back to in-memory store.")
	return NewMemoryStore(), nil
}
