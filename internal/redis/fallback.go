package redis

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FallbackCache 带降级功能的缓存包装器（规则R6: Redis不可用时降级）
type FallbackCache struct {
	cache        Cache
	client       *Client
	degraded     bool
	lastCheck    time.Time
	checkMu      sync.RWMutex
	checkInterval time.Duration
}

// NewFallbackCache 创建带降级的缓存
func NewFallbackCache(client *Client) *FallbackCache {
	return &FallbackCache{
		cache:         NewCache(client),
		client:        client,
		checkInterval: 10 * time.Second, // 每10秒检查一次Redis状态
	}
}

// shouldCheckDegraded 是否需要检查降级状态
func (f *FallbackCache) shouldCheckDegraded() bool {
	f.checkMu.RLock()
	defer f.checkMu.RUnlock()
	return time.Since(f.lastCheck) > f.checkInterval
}

// checkDegraded 检查并更新降级状态
func (f *FallbackCache) checkDegraded() {
	f.checkMu.Lock()
	defer f.checkMu.Unlock()

	// 双重检查
	if time.Since(f.lastCheck) <= f.checkInterval {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	available := f.client.IsAvailable(ctx)
	if available && f.degraded {
		zap.L().Info("Redis已恢复，取消降级模式")
		f.degraded = false
	} else if !available && !f.degraded {
		zap.L().Warn("Redis不可用，进入降级模式（跳过缓存，直接查数据库）")
		f.degraded = true
	}

	f.lastCheck = time.Now()
}

// IsDegraded 是否处于降级状态
func (f *FallbackCache) IsDegraded() bool {
	if f.shouldCheckDegraded() {
		f.checkDegraded()
	}
	f.checkMu.RLock()
	defer f.checkMu.RUnlock()
	return f.degraded
}

// GetWithFallback 带降级的Get操作
// 如果Redis不可用，返回空字符串但不报错（遵循规则R6）
func (f *FallbackCache) GetWithFallback(ctx context.Context, key string) (string, error) {
	// 检查降级状态
	if f.shouldCheckDegraded() {
		f.checkDegraded()
	}

	f.checkMu.RLock()
	degraded := f.degraded
	f.checkMu.RUnlock()

	// 降级模式：直接返回空，不访问Redis
	if degraded {
		zap.L().Debug("Redis降级模式，跳过缓存读取", zap.String("key", key))
		return "", nil
	}

	// 正常模式：尝试读取缓存
	val, err := f.cache.Get(ctx, key)
	if err != nil {
		// Redis出错，记录日志但不阻塞主流程
		zap.L().Warn("Redis读取失败，降级处理",
			zap.String("key", key),
			zap.Error(err))
		return "", nil
	}

	return val, nil
}

// SetWithFallback 带降级的Set操作
// 如果Redis不可用，记录日志但不报错
func (f *FallbackCache) SetWithFallback(ctx context.Context, key string, value interface{}, ttl ...time.Duration) {
	// 检查降级状态
	if f.shouldCheckDegraded() {
		f.checkDegraded()
	}

	f.checkMu.RLock()
	degraded := f.degraded
	f.checkMu.RUnlock()

	// 降级模式：直接返回，不访问Redis
	if degraded {
		zap.L().Debug("Redis降级模式，跳过缓存写入", zap.String("key", key))
		return
	}

	// 正常模式：尝试写入缓存
	var t time.Duration
	if len(ttl) > 0 {
		t = ttl[0]
	}
	if err := f.cache.Set(ctx, key, value, t); err != nil {
		// Redis出错，记录日志但不阻塞主流程
		zap.L().Warn("Redis写入失败，降级处理",
			zap.String("key", key),
			zap.Error(err))
	}
}

// DeleteWithFallback 带降级的Delete操作
func (f *FallbackCache) DeleteWithFallback(ctx context.Context, keys ...string) {
	// 检查降级状态
	if f.shouldCheckDegraded() {
		f.checkDegraded()
	}

	f.checkMu.RLock()
	degraded := f.degraded
	f.checkMu.RUnlock()

	// 降级模式：直接返回
	if degraded {
		zap.L().Debug("Redis降级模式，跳过缓存删除", zap.Strings("keys", keys))
		return
	}

	// 正常模式：尝试删除
	if err := f.cache.Delete(ctx, keys...); err != nil {
		zap.L().Warn("Redis删除失败，降级处理",
			zap.Strings("keys", keys),
			zap.Error(err))
	}
}

// GetCache 获取底层Cache接口（用于需要精确控制的场景）
func (f *FallbackCache) GetCache() Cache {
	return f.cache
}
