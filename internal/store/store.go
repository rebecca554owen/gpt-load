package store

import (
	"errors"
	"time"
)

// ErrNotFound 是在存储中未找到键时返回的错误。
var ErrNotFound = errors.New("store: key not found")

// Message 是接收的 pub/sub 消息的结构。
type Message struct {
	Channel string
	Payload []byte
}

// Subscription 表示对 pub/sub 频道的活动订阅。
type Subscription interface {
	Channel() <-chan *Message
	Close() error
}

// Store 是通用键值存储接口。
type Store interface {
	// Set 存储带有可选 TTL 的键值对。
	Set(key string, value []byte, ttl time.Duration) error

	// Get 通过键检索值。
	Get(key string) ([]byte, error)

	// Delete 通过键删除值。
	Delete(key string) error

	// Del 删除多个键。
	Del(keys ...string) error

	// Exists 检查键是否在存储中存在。
	Exists(key string) (bool, error)

	// SetNX 如果键不存在则设置键值对。
	SetNX(key string, value []byte, ttl time.Duration) (bool, error)

	// HASH 操作
	HSet(key string, values map[string]any) error
	HGetAll(key string) (map[string]string, error)
	HIncrBy(key, field string, incr int64) (int64, error)

	// LIST 操作
	LPush(key string, values ...any) error
	LRem(key string, count int64, value any) error
	Rotate(key string) (string, error)
	LLen(key string) (int64, error)

	// SET 操作
	SAdd(key string, members ...any) error
	SPopN(key string, count int64) ([]string, error)
	SMembers(key string) ([]string, error)

	// Close 关闭存储并释放所有底层资源。
	Close() error

	// Publish 向给定频道发送消息。
	Publish(channel string, message []byte) error

	// Subscribe 监听给定频道的消息。
	Subscribe(channel string) (Subscription, error)

	// Clear 清除所有数据。
	Clear() error
}

// Pipeliner 定义执行一批命令的接口。
type Pipeliner interface {
	HSet(key string, values map[string]any)
	Exec() error
}

// RedisPipeliner 是一个可选接口，Store 可以实现它以提供管道功能。
type RedisPipeliner interface {
	Pipeline() Pipeliner
}
