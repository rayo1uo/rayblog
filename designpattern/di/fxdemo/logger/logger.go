package logger

import "fmt"

// Logger 定义日志记录接口
type Logger interface {
	Info(msg string)
}

// SimpleLogger 提供一个基础的日志实现
type SimpleLogger struct{}

// NewLogger 创建Logger实例
func NewLogger() Logger {
	return &SimpleLogger{}
}

// Info 实现Logger接口
func (l *SimpleLogger) Info(msg string) {
	fmt.Printf("[INFO] %s\n", msg)
}
