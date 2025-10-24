package main

import (
	"context"
	"fmt"
	"fxdemo/logger"
	"fxdemo/service"

	"go.uber.org/fx"
)

func register(calc *service.Calculator) {
	// 使用Calculator服务进行一些计算
	result1 := calc.Add(5, 3)
	result2 := calc.Multiply(4, 2)
	fmt.Printf("计算结果: 5 + 3 = %d, 4 * 2 = %d\n", result1, result2)
}

func main() {
	app := fx.New(
		// 提供所需的依赖
		fx.Provide(
			logger.NewLogger,      // 提供Logger实例
			service.NewCalculator, // 提供Calculator实例
		),
		// 注册初始化函数
		fx.Invoke(register),
		// 添加生命周期钩子
		fx.Invoke(func(lc fx.Lifecycle, log logger.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					log.Info("应用启动")
					return nil
				},
				OnStop: func(context.Context) error {
					log.Info("应用停止")
					return nil
				},
			})
		}),
	)

	// 启动应用
	app.Run()
}
