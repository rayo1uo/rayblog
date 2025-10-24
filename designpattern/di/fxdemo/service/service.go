package service

import "fxdemo/logger"

// Calculator 提供基本的计算服务
type Calculator struct {
	log logger.Logger
}

// NewCalculator 创建Calculator实例，注入Logger依赖
func NewCalculator(log logger.Logger) *Calculator {
	return &Calculator{
		log: log,
	}
}

// Add 执行加法运算并记录日志
func (c *Calculator) Add(a, b int) int {
	result := a + b
	c.log.Info("Performed addition operation")
	return result
}

// Multiply 执行乘法运算并记录日志
func (c *Calculator) Multiply(a, b int) int {
	result := a * b
	c.log.Info("Performed multiplication operation")
	return result
}
