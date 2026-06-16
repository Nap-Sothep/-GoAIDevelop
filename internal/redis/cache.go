package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Cache 缓存接口
type Cache interface {
	// Get 获取缓存值
	Get(ctx context.Context, key string) (string, error)
	// GetJSON 获取JSON反序列化后的数据
	GetJSON(ctx context.Context, key string, obj interface{}) error
	// Set 设置缓存（带TTL）
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	// Delete 删除缓存
	Delete(ctx context.Context, keys ...string) error
	// Exists 检查key是否存在
	Exists(ctx context.Context, key string) (bool, error)
	// Pipeline 批量操作
	Pipeline() Pipeline
}

// Pipeline 批量操作接口
type Pipeline interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Exec(ctx context.Context) ([]redis.Cmder, error)
}

// cacheImpl 缓存实现
type cacheImpl struct {
	client    *redis.Client
	defaultTTL time.Duration
}

// NewCache 创建缓存实例
func NewCache(client *Client) Cache {
	return &cacheImpl{
		client:     client.GetClient(),
		defaultTTL: time.Duration(client.cfg.DefaultTTL) * time.Second,
	}
}

// Get 获取缓存值
func (c *cacheImpl) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // key不存在，返回空字符串
	}
	if err != nil {
		return "", fmt.Errorf("Redis Get失败 [key=%s]: %w", key, err)
	}
	return val, nil
}

// GetJSON 获取JSON反序列化后的数据
func (c *cacheImpl) GetJSON(ctx context.Context, key string, obj interface{}) error {
	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	if val == "" {
		return nil // key不存在，正常情况
	}

	if err := json.Unmarshal([]byte(val), obj); err != nil {
		return fmt.Errorf("Redis GetJSON解析失败 [key=%s]: %w", key, err)
	}
	return nil
}

// Set 设置缓存（规则R2: 必须带TTL）
func (c *cacheImpl) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// 如果ttl为0，使用默认TTL
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	var strVal string
	switch v := value.(type) {
	case string:
		strVal = v
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("Redis Set序列化失败 [key=%s]: %w", key, err)
		}
		strVal = string(data)
	}

	if err := c.client.Set(ctx, key, strVal, ttl).Err(); err != nil {
		return fmt.Errorf("Redis Set失败 [key=%s, ttl=%v]: %w", key, ttl, err)
	}

	zap.L().Debug("Redis缓存设置成功",
		zap.String("key", key),
		zap.Duration("ttl", ttl))
	return nil
}

// Delete 删除缓存
func (c *cacheImpl) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("Redis Delete失败 [keys=%v]: %w", keys, err)
	}

	zap.L().Debug("Redis缓存删除成功", zap.Strings("keys", keys))
	return nil
}

// Exists 检查key是否存在
func (c *cacheImpl) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("Redis Exists失败 [key=%s]: %w", key, err)
	}
	return count > 0, nil
}

// Pipeline 创建批量操作管道（规则R3: 使用Pipeline批量操作）
func (c *cacheImpl) Pipeline() Pipeline {
	return &pipelineImpl{
		pipe: c.client.Pipeline(),
	}
}

// pipelineImpl Pipeline实现
type pipelineImpl struct {
	pipe redis.Pipeliner
}

// Get Pipeline Get
func (p *pipelineImpl) Get(ctx context.Context, key string) *redis.StringCmd {
	return p.pipe.Get(ctx, key)
}

// Set Pipeline Set
func (p *pipelineImpl) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	var strVal string
	switch v := value.(type) {
	case string:
		strVal = v
	default:
		data, _ := json.Marshal(value)
		strVal = string(data)
	}
	return p.pipe.Set(ctx, key, strVal, expiration)
}

// Exec 执行Pipeline
func (p *pipelineImpl) Exec(ctx context.Context) ([]redis.Cmder, error) {
	results, err := p.pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("Redis Pipeline执行失败: %w", err)
	}
	return results, nil
}
