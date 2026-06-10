package main

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"go-gateway/internal/client"
	"go-gateway/internal/config"
	"go-gateway/internal/handler"
	"go-gateway/internal/router"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logger := initLogger(cfg.Log.Level)
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	zap.L().Info("Gateway启动中...")

	// 初始化gRPC客户端
	helloClient, err := client.NewHelloClient(cfg.GRPC.Target)
	if err != nil {
		zap.L().Fatal("创建gRPC客户端失败", zap.Error(err))
	}
	defer helloClient.Close()

	// 初始化Handler
	httpHandler := handler.NewHelloHTTPHandler(helloClient)
	wsHandler := handler.NewHelloWSHandler(helloClient)

	// 配置路由
	r := router.SetupRouter(httpHandler, wsHandler)

	// 启动服务
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	zap.L().Info("Gateway已启动", zap.String("addr", addr), zap.String("grpc_target", cfg.GRPC.Target))
	if err := r.Run(addr); err != nil {
		zap.L().Fatal("服务启动失败", zap.Error(err))
	}
}

// initLogger 初始化日志器
func initLogger(level string) *zap.Logger {
	var lvl zapcore.Level
	// 忽略解析错误，默认使用info级别
	_ = lvl.UnmarshalText([]byte(level))
	if lvl == 0 {
		lvl = zapcore.InfoLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, _ := cfg.Build()
	return logger
}
