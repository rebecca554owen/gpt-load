package channel

import (
	"encoding/json"
	"fmt"
	"gpt-load/internal/config"
	"gpt-load/internal/httpclient"
	"gpt-load/internal/models"
	"gpt-load/internal/utils"
	"net/url"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// channelConstructor 定义创建新通道代理的函数签名
type channelConstructor func(f *Factory, group *models.Group) (ChannelProxy, error)

var (
	// channelRegistry 保存从通道类型字符串到其构造函数的映射
	channelRegistry = make(map[string]channelConstructor)
)

// Register 添加一个新的通道构造函数到注册表
func Register(channelType string, constructor channelConstructor) {
	if _, exists := channelRegistry[channelType]; exists {
		panic(fmt.Sprintf("channel type '%s' is already registered", channelType))
	}
	channelRegistry[channelType] = constructor
}

// GetChannels 返回所有已注册的通道类型名称的切片
func GetChannels() []string {
	supportedTypes := make([]string, 0, len(channelRegistry))
	for t := range channelRegistry {
		supportedTypes = append(supportedTypes, t)
	}
	return supportedTypes
}

// Factory 负责创建通道代理
type Factory struct {
	settingsManager *config.SystemSettingsManager
	clientManager   *httpclient.HTTPClientManager
	channelCache    map[uint]ChannelProxy
	cacheLock       sync.Mutex
}

// NewFactory 创建一个新的通道工厂
func NewFactory(settingsManager *config.SystemSettingsManager, clientManager *httpclient.HTTPClientManager) *Factory {
	return &Factory{
		settingsManager: settingsManager,
		clientManager:   clientManager,
		channelCache:    make(map[uint]ChannelProxy),
	}
}

// GetChannel 根据组的通道类型返回通道代理
func (f *Factory) GetChannel(group *models.Group) (ChannelProxy, error) {
	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()

	if channel, ok := f.channelCache[group.ID]; ok {
		if !channel.IsConfigStale(group) {
			return channel, nil
		}
	}

	logrus.Debugf("Creating new channel for group %d with type '%s'", group.ID, group.ChannelType)

	constructor, ok := channelRegistry[group.ChannelType]
	if !ok {
		return nil, fmt.Errorf("unsupported channel type: %s", group.ChannelType)
	}
	channel, err := constructor(f, group)
	if err != nil {
		return nil, err
	}
	f.channelCache[group.ID] = channel
	return channel, nil
}

// newBaseChannel 是创建和配置 BaseChannel 的辅助函数
func (f *Factory) newBaseChannel(name string, group *models.Group) (*BaseChannel, error) {
	type upstreamDef struct {
		URL    string `json:"url"`
		Weight int    `json:"weight"`
	}

	var defs []upstreamDef
	if err := json.Unmarshal(group.Upstreams, &defs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal upstreams for %s channel: %w", name, err)
	}

	if len(defs) == 0 {
		return nil, fmt.Errorf("at least one upstream is required for %s channel", name)
	}

	var upstreamInfos []UpstreamInfo
	for _, def := range defs {
		u, err := url.Parse(def.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse upstream url '%s' for %s channel: %w", def.URL, name, err)
		}
		if def.Weight <= 0 {
			continue
		}
		upstreamInfos = append(upstreamInfos, UpstreamInfo{URL: u, Weight: def.Weight})
	}

	// 基于组有效设置的常规请求基础配置
	clientConfig := &httpclient.Config{
		ConnectTimeout:        time.Duration(group.EffectiveConfig.ConnectTimeout) * time.Second,
		RequestTimeout:        time.Duration(group.EffectiveConfig.RequestTimeout) * time.Second,
		IdleConnTimeout:       time.Duration(group.EffectiveConfig.IdleConnTimeout) * time.Second,
		MaxIdleConns:          group.EffectiveConfig.MaxIdleConns,
		MaxIdleConnsPerHost:   group.EffectiveConfig.MaxIdleConnsPerHost,
		ResponseHeaderTimeout: time.Duration(group.EffectiveConfig.ResponseHeaderTimeout) * time.Second,
		ProxyURL:              group.EffectiveConfig.ProxyURL,
		DisableCompression:    false,
		WriteBufferSize:       32 * 1024,
		ReadBufferSize:        32 * 1024,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 为流式请求创建专用配置
	streamConfig := *clientConfig
	streamConfig.RequestTimeout = 0
	streamConfig.DisableCompression = true
	streamConfig.WriteBufferSize = 0
	streamConfig.ReadBufferSize = 0
	// 为流式客户端使用更大、独立的连接池以避免耗尽
	streamConfig.MaxIdleConns = max(group.EffectiveConfig.MaxIdleConns*2, 50)
	streamConfig.MaxIdleConnsPerHost = max(group.EffectiveConfig.MaxIdleConnsPerHost*2, 20)

	// 使用各自的配置从管理器获取两个客户端
	httpClient := f.clientManager.GetClient(clientConfig)
	streamClient := f.clientManager.GetClient(&streamConfig)

	return &BaseChannel{
		Name:                name,
		Upstreams:           upstreamInfos,
		HTTPClient:          httpClient,
		StreamClient:        streamClient,
		TestModel:           group.TestModel,
		ValidationEndpoint:  utils.GetValidationEndpoint(group),
		channelType:         group.ChannelType,
		groupUpstreams:      group.Upstreams,
		effectiveConfig:     &group.EffectiveConfig,
		modelRedirectRules:  group.ModelRedirectRules,
		modelRedirectStrict: group.ModelRedirectStrict,
	}, nil
}
