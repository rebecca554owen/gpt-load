package syncer

import (
	"fmt"
	"sync"
	"time"

	"gpt-load/internal/store"

	"github.com/sirupsen/logrus"
)

// LoaderFunc 定义用于从真实来源（例如数据库）加载数据的通用函数签名。
type LoaderFunc[T any] func() (T, error)

// CacheSyncer 是管理内存缓存和跨实例同步的通用服务。
type CacheSyncer[T any] struct {
	mu          sync.RWMutex
	cache       T
	loader      LoaderFunc[T]
	store       store.Store
	channelName string
	logger      *logrus.Entry
	stopChan    chan struct{}
	wg          sync.WaitGroup
	afterReload func(newValue T)
}

// NewCacheSyncer 创建并初始化新的 CacheSyncer。
func NewCacheSyncer[T any](
	loader LoaderFunc[T],
	store store.Store,
	channelName string,
	logger *logrus.Entry,
	afterReload func(newValue T),
) (*CacheSyncer[T], error) {
	s := &CacheSyncer[T]{
		loader:      loader,
		store:       store,
		channelName: channelName,
		logger:      logger,
		stopChan:    make(chan struct{}),
		afterReload: afterReload,
	}

	if err := s.reload(); err != nil {
		return nil, fmt.Errorf("initial load for %s failed: %w", channelName, err)
	}

	s.wg.Add(1)
	go s.listenForUpdates()

	return s, nil
}

// Get 安全地返回缓存数据。
func (s *CacheSyncer[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache
}

// Invalidate 向所有实例发布通知以重新加载其缓存。
func (s *CacheSyncer[T]) Invalidate() error {
	s.logger.Debug("publishing invalidation notification")
	return s.store.Publish(s.channelName, []byte("reload"))
}

// Stop 优雅地关闭同步器的后台 goroutine。
func (s *CacheSyncer[T]) Stop() {
	close(s.stopChan)
	s.wg.Wait()
	s.logger.Info("cache syncer stopped.")
}

// reload 使用加载器函数获取最新数据并更新缓存。
func (s *CacheSyncer[T]) reload() error {
	s.logger.Debug("reloading cache...")
	newData, err := s.loader()
	if err != nil {
		s.logger.Errorf("failed to reload cache: %v", err)
		return err
	}

	s.mu.Lock()
	s.cache = newData
	s.mu.Unlock()

	s.logger.Info("cache reloaded successfully")
	// 成功重新加载并更新缓存后，触发钩子。
	if s.afterReload != nil {
		s.logger.Debug("triggering afterReload hook")
		s.afterReload(newData)
	}
	return nil
}

// listenForUpdates 在后台运行，监听失效消息。
func (s *CacheSyncer[T]) listenForUpdates() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			s.logger.Info("received stop signal, exiting listener loop.")
			return
		default:
		}

		if s.store == nil {
			s.logger.Warn("store is not configured, stopping subscription listener.")
			return
		}

		subscription, err := s.store.Subscribe(s.channelName)
		if err != nil {
			s.logger.Errorf("failed to subscribe, retrying in 5s: %v", err)
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-s.stopChan:
				return
			}
		}

		s.logger.Debugf("subscribed to channel: %s", s.channelName)

	subscriberLoop:
		for {
			select {
			case msg, ok := <-subscription.Channel():
				if !ok {
					s.logger.Warn("subscription channel closed, attempting to re-subscribe...")
					break subscriberLoop
				}
				s.logger.Debugf("received invalidation notification, payload: %s", string(msg.Payload))
				if err := s.reload(); err != nil {
					s.logger.Errorf("failed to reload cache after notification: %v", err)
				}
			case <-s.stopChan:
				if err := subscription.Close(); err != nil {
					s.logger.Errorf("failed to close subscription: %v", err)
				}
				return
			}
		}

		// 在重试之前，确保旧订阅已关闭。
		if err := subscription.Close(); err != nil {
			s.logger.Errorf("failed to close subscription before retrying: %v", err)
		}

		// 等待片刻再重试，以避免在持久性错误上出现紧密循环。
		select {
		case <-time.After(2 * time.Second):
		case <-s.stopChan:
			return
		}
	}
}
