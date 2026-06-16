package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"go-gateway/internal/kafka"
	"go-gateway/internal/model"
	"go-gateway/internal/redis"
	"go-gateway/internal/repository"
)

// UserService 用户服务层（编排 MongoDB + Redis + Kafka）
type UserService struct {
	userRepo   *repository.UserRepository
	cache      *redis.FallbackCache
	producer   *kafka.Producer
	group      singleflight.Group // 防缓存穿透
}

// NewUserService 创建用户服务
func NewUserService(
	userRepo *repository.UserRepository,
	cache *redis.FallbackCache,
	producer *kafka.Producer,
) *UserService {
	return &UserService{
		userRepo: userRepo,
		cache:    cache,
		producer: producer,
	}
}

// CreateUser 创建用户（写操作：MongoDB → Redis → Kafka）
func (s *UserService) CreateUser(ctx context.Context, user *model.User) error {
	// 1. 写入 MongoDB
	if err := s.userRepo.Create(ctx, user); err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}

	// 2. 更新 Redis 缓存（降级模式只记录日志）
	cacheKey := fmt.Sprintf("user:%s", user.ID.Hex())
	s.cache.SetWithFallback(ctx, cacheKey, user, 10*time.Minute)

	// 3. 发送 Kafka 事件（失败不阻塞主流程）
	userData := &kafka.UserData{
		ID:    user.ID.Hex(),
		Name:  user.Name,
		Email: user.Email,
		Age:   user.Age,
	}
	if err := s.producer.PublishUserEvent(ctx, kafka.UserCreated, userData); err != nil {
		zap.L().Warn("发送Kafka事件失败（不阻塞主流程）",
			zap.String("event_type", string(kafka.UserCreated)),
			zap.Error(err))
	}

	zap.L().Info("用户创建成功", zap.String("id", user.ID.Hex()), zap.String("name", user.Name))
	return nil
}

// GetUser 获取用户（读操作：Redis → MongoDB → 回写 Redis）
func (s *UserService) GetUser(ctx context.Context, id string) (*model.User, error) {
	cacheKey := fmt.Sprintf("user:%s", id)

	// 1. 先查 Redis 缓存（带降级）
	cached, err := s.cache.GetWithFallback(ctx, cacheKey)
	if err != nil {
		zap.L().Warn("Redis读取失败，降级查MongoDB", zap.Error(err))
	}

	// 缓存命中，直接返回
	if cached != "" {
		var user model.User
		if err := s.cache.GetCache().GetJSON(ctx, cacheKey, &user); err == nil && user.ID.Hex() != "" {
			zap.L().Debug("缓存命中", zap.String("key", cacheKey))
			return &user, nil
		}
	}

	// 2. 缓存未命中，用 Singleflight 防穿透
	resultChan := s.group.DoChan(cacheKey, func() (interface{}, error) {
		// 查 MongoDB
		user, err := s.userRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("查询MongoDB失败: %w", err)
		}
		if user == nil {
			return nil, nil // 用户不存在
		}

		// 3. 回写 Redis（带降级）
		s.cache.SetWithFallback(ctx, cacheKey, user, 10*time.Minute)

		return user, nil
	})

	// 等待结果
	select {
	case result := <-resultChan:
		if result.Err != nil {
			return nil, result.Err
		}
		if result.Val == nil {
			return nil, fmt.Errorf("用户不存在 [id=%s]", id) // 返回明确错误而非 (nil, nil)
		}
		return result.Val.(*model.User), nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// UpdateUser 更新用户（写操作：MongoDB → Redis → Kafka）
func (s *UserService) UpdateUser(ctx context.Context, user *model.User) error {
	// 1. 更新 MongoDB
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("更新用户失败: %w", err)
	}

	// 2. 更新 Redis 缓存（删除旧缓存，下次读取时重建）
	cacheKey := fmt.Sprintf("user:%s", user.ID.Hex())
	s.cache.DeleteWithFallback(ctx, cacheKey)

	// 3. 发送 Kafka 事件
	userData := &kafka.UserData{
		ID:    user.ID.Hex(),
		Name:  user.Name,
		Email: user.Email,
		Age:   user.Age,
	}
	if err := s.producer.PublishUserEvent(ctx, kafka.UserUpdated, userData); err != nil {
		zap.L().Warn("发送Kafka事件失败（不阻塞主流程）",
			zap.String("event_type", string(kafka.UserUpdated)),
			zap.Error(err))
	}

	zap.L().Info("用户更新成功", zap.String("id", user.ID.Hex()))
	return nil
}

// DeleteUser 删除用户（写操作：MongoDB → Redis → Kafka）
func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	// 1. 删除 MongoDB 记录
	if err := s.userRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除用户失败: %w", err)
	}

	// 2. 删除 Redis 缓存
	cacheKey := fmt.Sprintf("user:%s", id)
	s.cache.DeleteWithFallback(ctx, cacheKey)

	// 3. 发送 Kafka 事件
	userData := &kafka.UserData{ID: id}
	if err := s.producer.PublishUserEvent(ctx, kafka.UserDeleted, userData); err != nil {
		zap.L().Warn("发送Kafka事件失败（不阻塞主流程）",
			zap.String("event_type", string(kafka.UserDeleted)),
			zap.Error(err))
	}

	zap.L().Info("用户删除成功", zap.String("id", id))
	return nil
}

// ListUsers 分页查询用户列表（不使用缓存，直接查 MongoDB）
func (s *UserService) ListUsers(ctx context.Context, page, pageSize int64) ([]*model.User, int64, error) {
	users, total, err := s.userRepo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("查询用户列表失败: %w", err)
	}
	return users, total, nil
}

// BatchCreateUsers 批量创建用户
func (s *UserService) BatchCreateUsers(ctx context.Context, users []*model.User) error {
	if len(users) == 0 {
		return nil
	}

	// 1. 批量写入 MongoDB
	if err := s.userRepo.BatchCreate(ctx, users); err != nil {
		return fmt.Errorf("批量创建用户失败: %w", err)
	}

	// 2. 批量写入 Redis（使用 Pipeline）
	pipe := s.cache.GetCache().Pipeline()
	for _, user := range users {
		cacheKey := fmt.Sprintf("user:%s", user.ID.Hex())
		pipe.Set(ctx, cacheKey, user, 10*time.Minute)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		zap.L().Warn("批量写入Redis缓存部分失败", zap.Error(err))
		// 不返回错误，继续发送Kafka事件
	}

	// 3. 批量发送 Kafka 事件（使用信号量限制并发数，防止资源耗尽）
	const maxConcurrentKafkaPublishes = 10
	sem := make(chan struct{}, maxConcurrentKafkaPublishes)
	var wg sync.WaitGroup
	for _, user := range users {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量槽位
		go func(u *model.User) {
			defer func() {
				if r := recover(); r != nil {
					zap.L().Error("批量发送Kafka事件goroutine panic",
						zap.String("user_id", u.ID.Hex()),
						zap.Any("panic", r))
				}
				<-sem // 释放信号量槽位
				wg.Done()
			}()
			userData := &kafka.UserData{
				ID:    u.ID.Hex(),
				Name:  u.Name,
				Email: u.Email,
				Age:   u.Age,
			}
			if err := s.producer.PublishUserEvent(ctx, kafka.UserCreated, userData); err != nil {
				zap.L().Warn("批量发送Kafka事件失败",
					zap.String("user_id", u.ID.Hex()),
					zap.Error(err))
			}
		}(user)
	}
	wg.Wait()

	zap.L().Info("批量创建用户成功", zap.Int("count", len(users)))
	return nil
}
