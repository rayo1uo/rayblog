package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"go.uber.org/fx"
)

func TestError(t *testing.T) {
	// 测试在未设置PORT环境变量时的情况
	t.Run("missing PORT env", func(t *testing.T) {
		// 确保没有设置PORT环境变量
		os.Unsetenv("PORT")

		// 创建返回错误选项的函数
		newHTTPServer := func() fx.Option {
			port := os.Getenv("PORT")
			if port == "" {
				return fx.Error(errors.New("$PORT is not set"))
			}
			return fx.Provide(func() *http.Server {
				return &http.Server{
					Addr: fmt.Sprintf("127.0.0.1:%s", port),
				}
			})
		}

		// 使用NopLogger避免测试输出中的日志
		app := fx.New(
			fx.NopLogger,
			newHTTPServer(),
		)

		// 检查应用是否返回了预期的错误
		err := app.Err()
		if err == nil {
			t.Fatal("应用程序应该返回错误，但返回nil")
		}
		if err.Error() != "$PORT is not set" {
			t.Fatalf("预期错误「$PORT is not set」, 但获得「%s」", err.Error())
		}
	})

	// 测试在设置PORT环境变量时的情况
	t.Run("with PORT env", func(t *testing.T) {
		// 设置测试用的PORT
		os.Setenv("PORT", "8080")
		defer os.Unsetenv("PORT") // 测试结束后清理环境变量

		// 创建提供HTTP服务器的函数
		newHTTPServer := func() fx.Option {
			port := os.Getenv("PORT")
			if port == "" {
				return fx.Error(errors.New("$PORT is not set"))
			}
			return fx.Provide(func() *http.Server {
				return &http.Server{
					Addr: fmt.Sprintf("127.0.0.1:%s", port),
				}
			})
		}

		// 构建应用但不启动，只检查是否能正确初始化
		app := fx.New(
			fx.NopLogger,
			newHTTPServer(),
			fx.Invoke(func(s *http.Server) {
				// 只验证服务器地址是否正确
				if s.Addr != "127.0.0.1:8080" {
					t.Fatalf("服务器地址不正确，预期「127.0.0.1:8080」，得到「%s」", s.Addr)
				}
			}),
		)

		// 验证没有初始化错误
		if err := app.Err(); err != nil {
			t.Fatalf("应用程序不应该返回错误，但返回：%v", err)
		}
	})
}
