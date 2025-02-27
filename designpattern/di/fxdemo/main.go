package main

import (
	"errors"
	"fmt"

	"go.uber.org/fx"
)

type Logger interface {
	Log(message string)
}

// Soft Value Group 的依赖参数
type LoggerParams struct {
	fx.In
	Loggers []Logger `group:"loggers"` // 添加 "soft" 标记
}

// 结果类型 - 用于标记组成员
type LoggerResult struct {
	fx.Out
	Logger Logger `group:"loggers"` // 添加 soft 标记
}

// 初始化文件日志（模拟失败）
func NewFileLogger() LoggerResult {
	logger, err := createFileLogger()
	if err != nil {
		fmt.Println("文件日志初始化失败:", err)
		return LoggerResult{}
	}
	return LoggerResult{Logger: logger}
}

func createFileLogger() (Logger, error) {
	return nil, errors.New("file logger init failed")
}

// 初始化网络日志（模拟失败）
func NewNetworkLogger() LoggerResult {
	logger, err := createNetworkLogger()
	if err != nil {
		fmt.Println("网络日志初始化失败:", err)
		return LoggerResult{}
	}
	return LoggerResult{Logger: logger}
}

func createNetworkLogger() (Logger, error) {
	return nil, errors.New("network logger init failed")
}

// 初始化控制台日志（成功）
func NewConsoleLogger() LoggerResult {
	logger, err := createConsoleLogger()
	if err != nil {
		fmt.Println("控制台日志初始化失败:", err)
		return LoggerResult{}
	}
	return LoggerResult{Logger: logger}
}

func createConsoleLogger() (Logger, error) {
	return &consoleLogger{}, nil
}

type consoleLogger struct{}

func (c *consoleLogger) Log(message string) {
	fmt.Println("Console Log:", message)
}

func main() {
	app := fx.New(
		fx.Provide(
			NewFileLogger,    // 失败
			NewNetworkLogger, // 失败
			NewConsoleLogger, // 成功
		),
		fx.Invoke(func(params LoggerParams) {
			fmt.Println("成功初始化的日志服务数量:", len(params.Loggers))
			for _, logger := range params.Loggers {
				if logger == nil {
					continue
				}
				logger.Log("Hello from Value Group!")
			}
		}),
	)

	app.Run()
}
