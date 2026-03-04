// Package main 提供 GPT-Load 代理服务器的入口点
package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gpt-load/internal/app"
	"gpt-load/internal/commands"
	"gpt-load/internal/container"
	"gpt-load/internal/types"
	"gpt-load/internal/utils"

	"github.com/sirupsen/logrus"
)

//go:embed web/dist
var buildFS embed.FS

//go:embed web/dist/index.html
var indexPage []byte

func main() {
	if len(os.Args) > 1 {
		runCommand()
	} else {
		runServer()
	}
}

// runCommand 分发到相应的命令处理器
func runCommand() {
	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "migrate-keys":
		commands.RunMigrateKeys(args)
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Run 'gpt-load help' for usage.")
		os.Exit(1)
	}
}

// printHelp 显示通用帮助信息
func printHelp() {
	fmt.Println("GPT-Load - Multi-channel AI proxy with intelligent key rotation.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gpt-load                    Start the proxy server")
	fmt.Println("  gpt-load <command> [args]   Execute a command")
	fmt.Println()
	fmt.Println("Available Commands:")
	fmt.Println("  migrate-keys    Migrate encryption keys")
	fmt.Println("  help            Display this help message")
	fmt.Println()
	fmt.Println("Use 'gpt-load <command> --help' for more information about a command.")
}

// runServer 运行应用服务器
func runServer() {
	// 构建依赖注入容器
	container, err := container.BuildContainer()
	if err != nil {
		logrus.Fatalf("Failed to build container: %v", err)
	}

	// 向容器提供 UI 资源
	if err := container.Provide(func() embed.FS { return buildFS }); err != nil {
		logrus.Fatalf("Failed to provide buildFS: %v", err)
	}
	if err := container.Provide(func() []byte { return indexPage }); err != nil {
		logrus.Fatalf("Failed to provide indexPage: %v", err)
	}

	// 初始化全局日志器
	if err := container.Invoke(func(configManager types.ConfigManager) {
		utils.SetupLogger(configManager)
	}); err != nil {
		logrus.Fatalf("Failed to setup logger: %v", err)
	}

	// 创建并运行应用
	if err := container.Invoke(func(application *app.App, configManager types.ConfigManager) {
		if err := application.Start(); err != nil {
			logrus.Fatalf("Failed to start application: %v", err)
		}

		// 等待中断信号以实现优雅关闭
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		// 创建带超时的上下文用于关闭
		serverConfig := configManager.GetEffectiveServerConfig()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(serverConfig.GracefulShutdownTimeout)*time.Second)
		defer cancel()

		// 执行优雅关闭
		application.Stop(shutdownCtx)

	}); err != nil {
		logrus.Fatalf("Failed to run application: %v", err)
	}
}
