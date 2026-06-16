package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"

	"go-gateway/internal/config"
)

// MessageHandler 消息处理函数类型
type MessageHandler func(ctx context.Context, event *Event) error

// Consumer Kafka消费者封装
type Consumer struct {
	group   sarama.ConsumerGroup
	config  *config.KafkaConfig
	handler MessageHandler
	topics  []string
}

// NewConsumer 创建Kafka消费者
func NewConsumer(cfg *config.KafkaConfig, topics []string, handler MessageHandler) (*Consumer, error) {
	// 创建Sarama配置
	saramaConfig := sarama.NewConfig()

	// 规则K6: 强制禁用自动Commit，使用手动MarkMessage
	saramaConfig.Consumer.Offsets.AutoCommit.Enable = false

	// 规则K5: 配置Session Timeout和Heartbeat（Heartbeat应为Timeout的1/3）
	sessionTimeout := time.Duration(cfg.Consumer.SessionTimeout) * time.Second
	if sessionTimeout == 0 {
		sessionTimeout = 30 * time.Second // 默认30秒
	}
	saramaConfig.Consumer.Group.Session.Timeout = sessionTimeout
	saramaConfig.Consumer.Group.Heartbeat.Interval = sessionTimeout / 3
	saramaConfig.Consumer.Group.Rebalance.Timeout = 60 * time.Second

	// 创建Consumer Group
	group, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.Consumer.GroupID, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("Kafka Consumer Group创建失败 [group=%s]: %w", cfg.Consumer.GroupID, err)
	}

	zap.L().Info("Kafka Consumer Group连接成功",
		zap.Strings("brokers", cfg.Brokers),
		zap.String("group_id", cfg.Consumer.GroupID),
		zap.Strings("topics", topics),
		zap.Duration("session_timeout", sessionTimeout),
		zap.Duration("heartbeat_interval", sessionTimeout/3))

	return &Consumer{
		group:   group,
		config:  cfg,
		handler: handler,
		topics:  topics,
	}, nil
}

// Start 启动消费者（阻塞调用，建议在goroutine中运行）
func (c *Consumer) Start(ctx context.Context) error {
	// 创建消费者处理器
	consumerHandler := &consumerHandler{
		handler: c.handler,
		ready:   make(chan bool),
	}

	// 在后台启动消费循环
	go func() {
		defer func() {
			if r := recover(); r != nil {
				zap.L().Error("Kafka消费者goroutine panic", zap.Any("panic", r))
			}
		}()
		for {
			// 规则K3: 处理Rebalance事件
			if err := c.group.Consume(ctx, c.topics, consumerHandler); err != nil {
				zap.L().Error("Kafka消费失败", zap.Error(err))
			}

			// 检查context是否已取消
			if ctx.Err() != nil {
				zap.L().Info("Context已取消，停止Kafka消费")
				return
			}

			// 重置就绪信号供下次循环使用
			select {
			case <-consumerHandler.ready:
				// 通道已关闭，重新创建
			default:
			}
			consumerHandler.ready = make(chan bool)
		}
	}()

	// 等待消费者就绪
	<-consumerHandler.ready
	zap.L().Info("Kafka消费者已就绪")

	// 等待context取消（不再单独监听系统信号，由调用方统一管理）
	<-ctx.Done()
	zap.L().Info("收到终止信号，正在关闭Kafka消费者...")

	return c.Close()
}

// Close 关闭消费者
func (c *Consumer) Close() error {
	if err := c.group.Close(); err != nil {
		return fmt.Errorf("Kafka Consumer关闭失败: %w", err)
	}
	zap.L().Info("Kafka Consumer已关闭")
	return nil
}

// HealthCheck 健康检查
func (c *Consumer) HealthCheck() error {
	if c.group == nil {
		return fmt.Errorf("Kafka Consumer未初始化")
	}
	// Consumer Group没有直接的Errors()方法，简单检查即可
	return nil
}

// consumerHandler Sarama消费者组处理器（规则K3: 处理Rebalance）
type consumerHandler struct {
	handler MessageHandler
	ready   chan bool
}

// Setup 规则K3: Rebalance开始时的回调
func (h *consumerHandler) Setup(session sarama.ConsumerGroupSession) error {
	close(h.ready)
	zap.L().Info("Kafka Rebalance: 开始分配分区",
		zap.String("member_id", session.MemberID()),
		zap.Int("claims_count", len(session.Claims())))
	return nil
}

// Cleanup 规则K3: Rebalance结束时的回调
func (h *consumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	zap.L().Info("Kafka Rebalance: 分区回收完成")
	return nil
}

// ConsumeClaim 消费单个分区的消息
func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	zap.L().Info("开始消费分区",
		zap.String("topic", claim.Topic()),
		zap.Int32("partition", claim.Partition()),
		zap.Int64("initial_offset", claim.InitialOffset()))

	// 遍历消息
	for msg := range claim.Messages() {
		// 解析事件
		event, err := FromJSON(msg.Value)
		if err != nil {
			zap.L().Error("解析Kafka消息失败",
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Error(err))
			// 解析失败也要Commit，避免阻塞
			session.MarkMessage(msg, "")
			continue
		}

		// 创建带超时的context，继承session的context以支持优雅关闭
		ctx, cancel := context.WithTimeout(session.Context(), 30*time.Second)

		// 规则K2: 先处理业务逻辑，成功后才Commit
		var handlerErr error
		func() {
			defer cancel() // 确保即使panic也能清理context
			defer func() {
				if r := recover(); r != nil {
					zap.L().Error("Kafka消息处理panic",
						zap.String("event_type", string(event.EventType)),
						zap.Any("panic", r))
					handlerErr = fmt.Errorf("panic: %v", r)
				}
			}()
			handlerErr = h.handler(ctx, event)
		}()

		if handlerErr != nil {
			zap.L().Error("处理Kafka消息失败，不Commit（将重试）",
				zap.String("event_type", string(event.EventType)),
				zap.Error(handlerErr))
			continue // 不MarkMessage，下次重试
		}

		// 规则K2: 处理成功后Commit
		session.MarkMessage(msg, "")

		zap.L().Debug("Kafka消息处理成功",
			zap.String("event_type", string(event.EventType)),
			zap.Int64("offset", msg.Offset))
	}

	return nil
}
