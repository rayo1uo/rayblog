# FX 依赖注入示例

这是一个使用uber-go/fx框架的依赖注入示例项目。该示例展示了：

1. 如何定义和实现接口（Logger接口）
2. 如何注入依赖（Calculator依赖Logger）
3. 如何使用fx框架管理依赖
4. 如何处理应用生命周期

## 项目结构

```
fxdemo/
├── logger/
│   └── logger.go    # 定义Logger接口和SimpleLogger实现
├── service/
│   └── service.go   # 定义Calculator服务，展示依赖注入
└── main.go          # 使用fx框架组装所有组件
```

## 运行方法

```bash
go run main.go
```

## 示例说明

1. logger包提供了一个简单的日志接口和实现
2. service包中的Calculator服务依赖于Logger接口
3. main.go使用fx.Provide注册依赖，使用fx.Invoke执行初始化
4. 应用启动时会执行一些计算操作并通过Logger记录日志

## 关键概念

1. 依赖提供（Provide）
   - 使用`fx.Provide`注册构造函数
   - fx会自动解析和注入依赖

2. 依赖使用（Invoke）
   - 使用`fx.Invoke`注册需要执行的函数
   - 这些函数可以使用任何已提供的依赖

3. 生命周期管理
   - 使用`fx.Lifecycle`管理应用启动和关闭
   - 可以注册OnStart和OnStop钩子函数
