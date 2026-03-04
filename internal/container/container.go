// Package container 提供应用的依赖注入容器
package container

import (
	"gpt-load/internal/app"
	"gpt-load/internal/channel"
	"gpt-load/internal/config"
	"gpt-load/internal/db"
	"gpt-load/internal/encryption"
	"gpt-load/internal/handler"
	"gpt-load/internal/httpclient"
	"gpt-load/internal/keypool"
	"gpt-load/internal/proxy"
	"gpt-load/internal/router"
	"gpt-load/internal/services"
	"gpt-load/internal/store"
	"gpt-load/internal/types"

	"go.uber.org/dig"
)

// BuildContainer 创建新的依赖注入容器并提供应用的所有服务
func BuildContainer() (*dig.Container, error) {
	container := dig.New()

	// 基础设施服务
	if err := container.Provide(config.NewManager); err != nil {
		return nil, err
	}
	if err := container.Provide(func(configManager types.ConfigManager) (encryption.Service, error) {
		return encryption.NewService(configManager.GetEncryptionKey())
	}); err != nil {
		return nil, err
	}
	if err := container.Provide(db.NewDB); err != nil {
		return nil, err
	}
	if err := container.Provide(config.NewSystemSettingsManager); err != nil {
		return nil, err
	}
	if err := container.Provide(store.NewStore); err != nil {
		return nil, err
	}
	if err := container.Provide(httpclient.NewHTTPClientManager); err != nil {
		return nil, err
	}
	if err := container.Provide(channel.NewFactory); err != nil {
		return nil, err
	}

	// 业务服务
	if err := container.Provide(services.NewTaskService); err != nil {
		return nil, err
	}
	if err := container.Provide(services.NewKeyManualValidationService); err != nil {
		return nil, err
	}
	if err := container.Provide(services.NewKeyService); err != nil {
		return nil, err
	}
	if err := container.Provide(services.NewKeyImportService); err != nil {
		return nil, err
	}
	if err := container.Provide(services.NewKeyDeleteService); err != nil {
		return nil, err
	}
	if err := container.Provide(services.NewLogService); err != nil {
		return nil, err
	}
	if err := container.Provide(services.NewLogCleanupService); err != nil {
		return nil, err
	}
	if err := container.Provide(services.NewRequestLogService); err != nil {
		return nil, err
	}
	if err := container.Provide(services.NewSubGroupManager); err != nil {
		return nil, err
	}
	if err := container.Provide(services.NewGroupManager); err != nil {
		return nil, err
	}
	if err := container.Provide(services.NewGroupService); err != nil {
		return nil, err
	}
	if err := container.Provide(services.NewAggregateGroupService); err != nil {
		return nil, err
	}
	if err := container.Provide(keypool.NewProvider); err != nil {
		return nil, err
	}
	if err := container.Provide(keypool.NewKeyValidator); err != nil {
		return nil, err
	}
	if err := container.Provide(keypool.NewCronChecker); err != nil {
		return nil, err
	}

	// 处理器
	if err := container.Provide(handler.NewServer); err != nil {
		return nil, err
	}
	if err := container.Provide(handler.NewCommonHandler); err != nil {
		return nil, err
	}

	// 代理和路由
	if err := container.Provide(proxy.NewProxyServer); err != nil {
		return nil, err
	}
	if err := container.Provide(router.NewRouter); err != nil {
		return nil, err
	}

	// 应用层
	if err := container.Provide(app.NewApp); err != nil {
		return nil, err
	}

	return container, nil
}
