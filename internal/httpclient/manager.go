package httpclient

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Config 定义创建 HTTP 客户端的参数。
// 此结构体用于生成唯一指纹以供客户端重用。
type Config struct {
	ConnectTimeout        time.Duration
	RequestTimeout        time.Duration
	IdleConnTimeout       time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	ResponseHeaderTimeout time.Duration
	DisableCompression    bool
	WriteBufferSize       int
	ReadBufferSize        int
	ForceAttemptHTTP2     bool
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	ProxyURL              string
}

// HTTPClientManager 管理 HTTP 客户端的生命周期。
// 它基于配置指纹创建和缓存客户端，确保相同配置的客户端被重用。
type HTTPClientManager struct {
	clients map[string]*http.Client
	lock    sync.RWMutex
}

// NewHTTPClientManager 创建一个新的客户端管理器。
func NewHTTPClientManager() *HTTPClientManager {
	return &HTTPClientManager{
		clients: make(map[string]*http.Client),
	}
}

// GetClient 返回与给定配置匹配的 HTTP 客户端。
// 如果缓存中已存在匹配的客户端，则返回该客户端。
// 否则，创建一个新客户端，进行缓存，然后返回。
func (m *HTTPClientManager) GetClient(config *Config) *http.Client {
	fingerprint := config.getFingerprint()

	// 使用读锁的快速路径
	m.lock.RLock()
	client, exists := m.clients[fingerprint]
	m.lock.RUnlock()
	if exists {
		return client
	}

	// 使用写锁的慢速路径
	m.lock.Lock()
	defer m.lock.Unlock()

	// 双重检查，以防另一个 goroutine 在我们等待锁时创建了客户端。
	if client, exists = m.clients[fingerprint]; exists {
		return client
	}

	// 使用指定配置创建新的传输和客户端。
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.ConnectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     config.ForceAttemptHTTP2,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		DisableCompression:    config.DisableCompression,
		WriteBufferSize:       config.WriteBufferSize,
		ReadBufferSize:        config.ReadBufferSize,
	}

	// 设置 HTTP 代理。
	if config.ProxyURL != "" {
		proxyURL, err := url.Parse(config.ProxyURL)
		if err != nil {
			logrus.Warnf("Invalid proxy URL '%s' provided, falling back to environment settings: %v", config.ProxyURL, err)
			transport.Proxy = http.ProxyFromEnvironment
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	} else {
		transport.Proxy = http.ProxyFromEnvironment
	}

	newClient := &http.Client{
		Transport: transport,
		Timeout:   config.RequestTimeout,
	}

	m.clients[fingerprint] = newClient
	return newClient
}

// getFingerprint 生成客户端配置的唯一字符串表示。
func (c *Config) getFingerprint() string {
	return fmt.Sprintf(
		"ct:%.0fs|rt:%.0fs|it:%.0fs|mic:%d|mich:%d|rht:%.0fs|dc:%t|wbs:%d|rbs:%d|fh2:%t|tlst:%.0fs|ect:%.0fs|proxy:%s",
		c.ConnectTimeout.Seconds(),
		c.RequestTimeout.Seconds(),
		c.IdleConnTimeout.Seconds(),
		c.MaxIdleConns,
		c.MaxIdleConnsPerHost,
		c.ResponseHeaderTimeout.Seconds(),
		c.DisableCompression,
		c.WriteBufferSize,
		c.ReadBufferSize,
		c.ForceAttemptHTTP2,
		c.TLSHandshakeTimeout.Seconds(),
		c.ExpectContinueTimeout.Seconds(),
		c.ProxyURL,
	)
}
