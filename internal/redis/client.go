package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"go-gateway/internal/config"
)

// Client Redis客户端封装
type Client struct {
	client *redis.Client
	cfg    *config.RedisConfig
}

// NewClient 创建Redis客户端
func NewClient(cfg *config.RedisConfig) (*Client, error) {
	// 创建Redis客户端
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
	})

	// 规则R1: 启动时必须PING测试连接
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.DialTimeout)*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis连接失败 [addr=%s]: %w", cfg.Addr, err)
	}

	zap.L().Info("Redis连接成功",
		zap.String("addr", cfg.Addr),
		zap.Int("db", cfg.DB),
		zap.Int("pool_size", cfg.PoolSize),
		zap.Int("min_idle_conns", cfg.MinIdleConns))

	return &Client{
		client: rdb,
		cfg:    cfg,
	}, nil
}

// GetClient 获取原生Redis客户端
func (c *Client) GetClient() *redis.Client {
	return c.client
}

// Close 关闭Redis连接
func (c *Client) Close() error {
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("Redis关闭连接失败: %w", err)
	}
	zap.L().Info("Redis连接已关闭")
	return nil
}

// HealthCheck 健康检查
func (c *Client) HealthCheck(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis健康检查失败: %w", err)
	}
	return nil
}

// IsAvailable 检查Redis是否可用（用于降级判断）
func (c *Client) IsAvailable(ctx context.Context) bool {
	return c.HealthCheck(ctx) == nil
}
