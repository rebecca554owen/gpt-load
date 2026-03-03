package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisKeyPrefix 是 GPT-Load 使用的所有 Redis 键的前缀
const RedisKeyPrefix = "gpt-load:"

// RedisStore 是基于 Redis 的键值存储。
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore 创建新的 RedisStore 实例。
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// prefixKey 为键添加应用程序前缀
func (s *RedisStore) prefixKey(key string) string {
	return RedisKeyPrefix + key
}

// prefixKeys 为多个键添加应用程序前缀
func (s *RedisStore) prefixKeys(keys []string) []string {
	prefixed := make([]string, len(keys))
	for i, key := range keys {
		prefixed[i] = s.prefixKey(key)
	}
	return prefixed
}

// Set 在 Redis 中存储键值对。
func (s *RedisStore) Set(key string, value []byte, ttl time.Duration) error {
	return s.client.Set(context.Background(), s.prefixKey(key), value, ttl).Err()
}

// Get 从 Redis 检索值。
func (s *RedisStore) Get(key string) ([]byte, error) {
	val, err := s.client.Get(context.Background(), s.prefixKey(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return val, nil
}

// Delete 从 Redis 删除值。
func (s *RedisStore) Delete(key string) error {
	return s.client.Del(context.Background(), s.prefixKey(key)).Err()
}

// Del 从 Redis 删除多个值。
func (s *RedisStore) Del(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.client.Del(context.Background(), s.prefixKeys(keys)...).Err()
}

// Exists 检查键是否在 Redis 中存在。
func (s *RedisStore) Exists(key string) (bool, error) {
	val, err := s.client.Exists(context.Background(), s.prefixKey(key)).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

// SetNX 如果键不存在则在 Redis 中设置键值对。
func (s *RedisStore) SetNX(key string, value []byte, ttl time.Duration) (bool, error) {
	return s.client.SetNX(context.Background(), s.prefixKey(key), value, ttl).Result()
}

// Close 关闭 Redis 客户端连接。
func (s *RedisStore) Close() error {
	return s.client.Close()
}

// --- HASH 操作 ---

func (s *RedisStore) HSet(key string, values map[string]any) error {
	return s.client.HSet(context.Background(), s.prefixKey(key), values).Err()
}

func (s *RedisStore) HGetAll(key string) (map[string]string, error) {
	return s.client.HGetAll(context.Background(), s.prefixKey(key)).Result()
}

func (s *RedisStore) HIncrBy(key, field string, incr int64) (int64, error) {
	return s.client.HIncrBy(context.Background(), s.prefixKey(key), field, incr).Result()
}

// --- LIST 操作 ---

func (s *RedisStore) LPush(key string, values ...any) error {
	return s.client.LPush(context.Background(), s.prefixKey(key), values...).Err()
}

func (s *RedisStore) LRem(key string, count int64, value any) error {
	return s.client.LRem(context.Background(), s.prefixKey(key), count, value).Err()
}

func (s *RedisStore) Rotate(key string) (string, error) {
	prefixedKey := s.prefixKey(key)
	val, err := s.client.RPopLPush(context.Background(), prefixedKey, prefixedKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrNotFound
		}
		return "", err
	}
	return val, nil
}

// LLen 返回列表的长度。
func (s *RedisStore) LLen(key string) (int64, error) {
	return s.client.LLen(context.Background(), s.prefixKey(key)).Result()
}

// --- SET 操作 ---

func (s *RedisStore) SAdd(key string, members ...any) error {
	return s.client.SAdd(context.Background(), s.prefixKey(key), members...).Err()
}

func (s *RedisStore) SPopN(key string, count int64) ([]string, error) {
	return s.client.SPopN(context.Background(), s.prefixKey(key), count).Result()
}

func (s *RedisStore) SMembers(key string) ([]string, error) {
	return s.client.SMembers(context.Background(), s.prefixKey(key)).Result()
}

// --- Pipeliner 实现 ---

type redisPipeliner struct {
	pipe  redis.Pipeliner
	store *RedisStore
}

// HSet 将 HSET 命令添加到管道。
func (p *redisPipeliner) HSet(key string, values map[string]any) {
	p.pipe.HSet(context.Background(), p.store.prefixKey(key), values)
}

// Exec 执行管道中的所有命令。
func (p *redisPipeliner) Exec() error {
	_, err := p.pipe.Exec(context.Background())
	return err
}

// Pipeline 创建新管道。
func (s *RedisStore) Pipeline() Pipeliner {
	return &redisPipeliner{
		pipe:  s.client.Pipeline(),
		store: s,
	}
}

// --- Pub/Sub 操作 ---

// redisSubscription 包装 redis.PubSub 以实现 Subscription 接口。
type redisSubscription struct {
	pubsub  *redis.PubSub
	msgChan chan *Message
	once    sync.Once
}

// Channel 返回从订阅接收消息的通道。
func (rs *redisSubscription) Channel() <-chan *Message {
	rs.once.Do(func() {
		rs.msgChan = make(chan *Message, 10)
		go func() {
			defer close(rs.msgChan)
			for redisMsg := range rs.pubsub.Channel() {
				rs.msgChan <- &Message{
					Channel: redisMsg.Channel,
					Payload: []byte(redisMsg.Payload),
				}
			}
		}()
	})
	return rs.msgChan
}

// Close 关闭订阅。
func (rs *redisSubscription) Close() error {
	return rs.pubsub.Close()
}

// Publish 向给定频道发送消息。
func (s *RedisStore) Publish(channel string, message []byte) error {
	return s.client.Publish(context.Background(), s.prefixKey(channel), message).Err()
}

// Subscribe 监听给定频道的消息。
func (s *RedisStore) Subscribe(channel string) (Subscription, error) {
	prefixedChannel := s.prefixKey(channel)
	pubsub := s.client.Subscribe(context.Background(), prefixedChannel)

	_, err := pubsub.Receive(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to channel %s: %w", channel, err)
	}

	return &redisSubscription{pubsub: pubsub}, nil
}

// Clear 清除当前 Redis 数据库中所有具有 GPT-Load 前缀的键。
// 此方法仅删除属于 GPT-Load 的键，保留其他应用程序的数据。
func (s *RedisStore) Clear() error {
	ctx := context.Background()

	// 使用 SCAN 迭代所有带有我们前缀的键
	var cursor uint64
	var allKeys []string

	for {
		// 扫描带有我们前缀的键，每次 1000 个
		keys, nextCursor, err := s.client.Scan(ctx, cursor, RedisKeyPrefix+"*", 10000).Result()
		if err != nil {
			return fmt.Errorf("failed to scan keys: %w", err)
		}

		allKeys = append(allKeys, keys...)
		cursor = nextCursor

		// 当光标为 0 时，我们已完成完整迭代
		if cursor == 0 {
			break
		}
	}

	// 如果未找到键，提前返回
	if len(allKeys) == 0 {
		return nil
	}

	// 分批删除键以避免压垮 Redis
	const batchSize = 1000
	for i := 0; i < len(allKeys); i += batchSize {
		end := i + batchSize
		if end > len(allKeys) {
			end = len(allKeys)
		}

		batch := allKeys[i:end]
		if err := s.client.Del(ctx, batch...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
	}

	return nil
}
