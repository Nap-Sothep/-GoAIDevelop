package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	hellov1 "go-gateway/api/hello/v1"
	"go-gateway/internal/config"
	"go-gateway/internal/kafka"
	"go-gateway/internal/mongo"
	"go-gateway/internal/redis"
	"go-gateway/internal/repository"
	"go-gateway/internal/service"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// helloServer 实现 HelloServiceServer 接口（带中间件依赖）
type helloServer struct {
	hellov1.UnimplementedHelloServiceServer
	userService *service.UserService
	mongoClient *mongo.Client
	redisClient *redis.Client
	kafkaProd   *kafka.Producer
	startTime   time.Time // 服务启动时间
}

// SayHello 实现 Unary RPC（演示完整中间件调用链）
func (s *helloServer) SayHello(ctx context.Context, req *hellov1.SayHelloRequest) (*hellov1.SayHelloResponse, error) {
	name := req.GetName()
	if name == "" {
		name = "陌生人"
	}

	// 演示：创建/查询用户，展示 MongoDB + Redis + Kafka 协作
	// 检查 userService 是否可用（降级模式下可能为 nil）
	if s.userService == nil {
		zap.L().Debug("UserService不可用（降级模式），返回基本问候")
		return &hellov1.SayHelloResponse{
			Message: fmt.Sprintf("你好, %s! (来自gRPC后端，服务降级模式)", name),
		}, nil
	}

	user, err := s.userService.GetUser(ctx, "demo_user_id")
	if err != nil {
		zap.L().Warn("查询用户失败", zap.Error(err))
		// 不阻断主流程，继续返回问候
	}

	if user != nil {
		// 用户存在，返回个性化问候
		return &hellov1.SayHelloResponse{
			Message: fmt.Sprintf("你好, %s! 欢迎回来 (来自gRPC后端，数据来自MongoDB+Redis)", user.Name),
		}, nil
	}

	// 用户不存在，创建一个示例用户
	newUser := &hellov1.ChatMessage{
		User:      name,
		Text:      "新用户首次访问",
		Timestamp: time.Now().Unix(),
	}

	return &hellov1.SayHelloResponse{
		Message: fmt.Sprintf("你好, %s! (来自gRPC后端，消息: %s)", name, newUser.Text),
	}, nil
}

// ChatStream 实现双向流 RPC
func (s *helloServer) ChatStream(stream hellov1.HelloService_ChatStreamServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// 收到消息后，回一条"已收到"的确认
		reply := &hellov1.ChatMessage{
			User:      "系统",
			Text:      fmt.Sprintf("已收到 %s 的消息: %s", msg.GetUser(), msg.GetText()),
			Timestamp: time.Now().Unix(),
		}
		if err := stream.Send(reply); err != nil {
			return err
		}
	}
}

// HealthCheck 实现健康检查 RPC（并行检查三个组件）
func (s *helloServer) HealthCheck(ctx context.Context, req *hellov1.HealthCheckRequest) (*hellov1.HealthCheckResponse, error) {
	resp := &hellov1.HealthCheckResponse{
		Version: "1.0.0",
		Uptime:  int64(time.Since(s.startTime).Seconds()),
	}

	// 使用 channel 收集各组件的健康状态
	type componentResult struct {
		name   string
		status *hellov1.HealthCheckResponse_ComponentStatus
	}
	results := make(chan componentResult, 3)

	// 并行检查 MongoDB
	if s.mongoClient != nil {
		go func() {
			start := time.Now()
			err := s.mongoClient.HealthCheck(ctx)
			latency := time.Since(start).Milliseconds()
			status := &hellov1.HealthCheckResponse_ComponentStatus{
				Available: err == nil,
				LatencyMs: latency,
			}
			if err == nil {
				status.Message = "正常"
			} else {
				status.Message = fmt.Sprintf("异常: %v", err)
			}
			results <- componentResult{name: "mongodb", status: status}
		}()
	} else {
		results <- componentResult{name: "mongodb", status: &hellov1.HealthCheckResponse_ComponentStatus{
			Available: false,
			Message:   "未连接",
		}}
	}

	// 并行检查 Redis
	if s.redisClient != nil {
		go func() {
			start := time.Now()
			err := s.redisClient.HealthCheck(ctx)
			latency := time.Since(start).Milliseconds()
			status := &hellov1.HealthCheckResponse_ComponentStatus{
				Available: err == nil,
				LatencyMs: latency,
			}
			if err == nil {
				status.Message = "正常"
			} else {
				status.Message = fmt.Sprintf("异常: %v", err)
			}
			results <- componentResult{name: "redis", status: status}
		}()
	} else {
		results <- componentResult{name: "redis", status: &hellov1.HealthCheckResponse_ComponentStatus{
			Available: false,
			Message:   "未连接",
		}}
	}

	// 并行检查 Kafka
	if s.kafkaProd != nil {
		go func() {
			start := time.Now()
			err := s.kafkaProd.HealthCheck()
			latency := time.Since(start).Milliseconds()
			status := &hellov1.HealthCheckResponse_ComponentStatus{
				Available: err == nil,
				LatencyMs: latency,
			}
			if err == nil {
				status.Message = "正常"
			} else {
				status.Message = fmt.Sprintf("异常: %v", err)
			}
			results <- componentResult{name: "kafka", status: status}
		}()
	} else {
		results <- componentResult{name: "kafka", status: &hellov1.HealthCheckResponse_ComponentStatus{
			Available: false,
			Message:   "未连接",
		}}
	}

	// 收集所有结果
	var mongoStatus, redisStatus, kafkaStatus *hellov1.HealthCheckResponse_ComponentStatus
	for i := 0; i < 3; i++ {
		result := <-results
		switch result.name {
		case "mongodb":
			mongoStatus = result.status
		case "redis":
			redisStatus = result.status
		case "kafka":
			kafkaStatus = result.status
		}
	}
	resp.Mongodb = mongoStatus
	resp.Redis = redisStatus
	resp.Kafka = kafkaStatus

	// 确定整体服务状态
	allAvailable := mongoStatus.Available && redisStatus.Available && kafkaStatus.Available
	anyAvailable := mongoStatus.Available || redisStatus.Available || kafkaStatus.Available

	if allAvailable {
		resp.Status = hellov1.HealthCheckResponse_SERVING
	} else if anyAvailable {
		resp.Status = hellov1.HealthCheckResponse_DEGRADED // 降级模式
	} else {
		resp.Status = hellov1.HealthCheckResponse_NOT_SERVING
	}

	return resp, nil
}

func main() {
	// 初始化日志
	logger := initLogger()
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	zap.L().Info("gRPC后端服务启动中...")

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		zap.L().Fatal("加载配置失败", zap.Error(err))
	}

	// 1. 初始化 MongoDB
	zap.L().Info("[1/4] 正在连接 MongoDB...")
	mongoClient, err := mongo.NewClient(&cfg.MongoDB)
	if err != nil {
		zap.L().Warn("MongoDB连接失败（将继续启动）", zap.Error(err))
		mongoClient = nil // 降级：不使用MongoDB
	} else {
		zap.L().Info("[1/4] MongoDB连接成功")
	}

	// 2. 初始化 Redis
	zap.L().Info("[2/4] 正在连接 Redis...")
	redisClient, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		zap.L().Warn("Redis连接失败（将启用降级模式）", zap.Error(err))
		redisClient = nil // 降级：不使用Redis
	} else {
		zap.L().Info("[2/4] Redis连接成功")
	}

	// 3. 初始化 Kafka Producer
	zap.L().Info("[3/4] 正在连接 Kafka...")
	kafkaProd, err := kafka.NewProducer(&cfg.Kafka)
	if err != nil {
		zap.L().Warn("Kafka连接失败（事件将不可用）", zap.Error(err))
		kafkaProd = nil // 降级：不使用Kafka
	} else {
		zap.L().Info("[3/4] Kafka连接成功")
	}

	// 4. 初始化 Service 层
	zap.L().Info("[4/4] 正在初始化服务层...")
	var userService *service.UserService

	if mongoClient != nil && redisClient != nil && kafkaProd != nil {
		// 全部中间件可用
		userRepo := repository.NewUserRepository(mongoClient.GetDatabase())
		fallbackCache := redis.NewFallbackCache(redisClient)
		userService = service.NewUserService(userRepo, fallbackCache, kafkaProd)
		zap.L().Info("[4/4] 服务层初始化成功（完整模式）")
	} else {
		zap.L().Warn("[4/4] 部分中间件不可用，服务层将在降级模式下运行")
		// 创建降级的userService，各方法会检查依赖可用性
		if mongoClient != nil {
			userRepo := repository.NewUserRepository(mongoClient.GetDatabase())
			var cache *redis.FallbackCache
			if redisClient != nil {
				cache = redis.NewFallbackCache(redisClient)
			}
			var prod *kafka.Producer
			if kafkaProd != nil {
				prod = kafkaProd
			}
			userService = service.NewUserService(userRepo, cache, prod)
		}
		// 如果mongoClient也为nil，userService保持nil，SayHello中会检查
	}

	// 启动 gRPC 服务
	port := "50051"
	if p := os.Getenv("GRPC_PORT"); p != "" {
		port = p
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		zap.L().Fatal("监听端口失败", zap.Error(err))
	}

	s := grpc.NewServer()
	hellov1.RegisterHelloServiceServer(s, &helloServer{
		userService: userService,
		mongoClient: mongoClient,
		redisClient: redisClient,
		kafkaProd:   kafkaProd,
		startTime:   time.Now(), // 记录启动时间
	})

	// 注册反射服务，方便用 grpcurl 调试
	reflection.Register(s)

	// 优雅关闭：等待信号后按顺序关闭资源
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		zap.L().Info("收到终止信号，正在关闭服务...", zap.String("signal", sig.String()))

		// 1. 先停止接收新请求（30秒超时）
		stopChan := make(chan struct{})
		go func() {
			s.GracefulStop()
			close(stopChan)
		}()

		// 等待 GracefulStop 完成或超时
		select {
		case <-stopChan:
			zap.L().Info("gRPC服务已停止")
		case <-time.After(30 * time.Second):
			zap.L().Warn("优雅关闭超时，强制停止gRPC服务")
			s.Stop()
		}

		// 2. 按顺序关闭中间件（Kafka → MongoDB → Redis）
		if kafkaProd != nil {
			ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel1()
			if err := kafkaProd.CloseWithContext(ctx1); err != nil {
				zap.L().Error("Kafka关闭失败", zap.Error(err))
			} else {
				zap.L().Info("Kafka已关闭")
			}
		}

		if mongoClient != nil {
			if err := mongoClient.Close(); err != nil {
				zap.L().Error("MongoDB关闭失败", zap.Error(err))
			} else {
				zap.L().Info("MongoDB已关闭")
			}
		}

		if redisClient != nil {
			if err := redisClient.Close(); err != nil {
				zap.L().Error("Redis关闭失败", zap.Error(err))
			} else {
				zap.L().Info("Redis已关闭")
			}
		}

		zap.L().Info("服务已完全关闭")
		os.Exit(0)
	}()

	zap.L().Info(fmt.Sprintf("gRPC后端服务已启动，监听端口 %s", port))
	if err := s.Serve(lis); err != nil {
		zap.L().Fatal("服务启动失败", zap.Error(err))
	}
}

// initLogger 初始化日志器
func initLogger() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := cfg.Build()
	if err != nil {
		panic(fmt.Sprintf("日志初始化失败: %v", err))
	}
	return logger
}
