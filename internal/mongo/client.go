package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
	"go.uber.org/zap"

	"go-gateway/internal/config"
)

// Client MongoDB客户端封装
type Client struct {
	client *mongo.Client
	db     *mongo.Database
}

// NewClient 创建MongoDB客户端
func NewClient(cfg *config.MongoDBConfig) (*Client, error) {
	// 创建连接选项
	opts := options.Client().
		ApplyURI(cfg.URI).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetMinPoolSize(cfg.MinPoolSize).
		SetWriteConcern(writeconcern.Majority()).
		SetReadPreference(readpref.Primary())

	// 创建带超时的context用于连接
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ConnectTimeout)*time.Second)
	defer cancel()

	// 连接MongoDB
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("MongoDB连接失败 [uri=%s]: %w", cfg.URI, err)
	}

	// 测试连接是否可用
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("MongoDB Ping失败: %w", err)
	}

	zap.L().Info("MongoDB连接成功",
		zap.String("uri", cfg.URI),
		zap.String("database", cfg.Database),
		zap.Uint64("max_pool_size", cfg.MaxPoolSize),
		zap.Uint64("min_pool_size", cfg.MinPoolSize))

	return &Client{
		client: client,
		db:     client.Database(cfg.Database),
	}, nil
}

// GetDatabase 获取数据库实例
func (c *Client) GetDatabase() *mongo.Database {
	return c.db
}

// GetCollection 获取集合实例
func (c *Client) GetCollection(name string) *mongo.Collection {
	return c.db.Collection(name)
}

// Close 关闭MongoDB连接
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.client.Disconnect(ctx); err != nil {
		return fmt.Errorf("MongoDB断开连接失败: %w", err)
	}

	zap.L().Info("MongoDB连接已关闭")
	return nil
}

// HealthCheck 健康检查
func (c *Client) HealthCheck(ctx context.Context) error {
	if err := c.client.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("MongoDB健康检查失败: %w", err)
	}
	return nil
}
