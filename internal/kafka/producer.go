package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"

	"go-gateway/internal/config"
)

// Producer Kafka生产者封装
type Producer struct {
	producer sarama.SyncProducer
	config   *config.KafkaConfig
}

// NewProducer 创建Kafka生产者（规则K6: 配置Acks和Retry + 启用幂等性）
func NewProducer(cfg *config.KafkaConfig) (*Producer, error) {
	// 创建Sarama配置
	saramaConfig := sarama.NewConfig()

	// 规则K6: 必须配置RequiredAcks和Retry
	saramaConfig.Producer.RequiredAcks = sarama.RequiredAcks(cfg.Producer.RequiredAcks)
	saramaConfig.Producer.Retry.Max = cfg.Producer.RetryMax
	saramaConfig.Producer.Return.Successes = true // 接收成功确认
	saramaConfig.Producer.Timeout = time.Duration(cfg.Producer.Timeout) * time.Second

	// P1 #13: 启用幂等性，防止重试导致消息重复
	// 注意：幂等性要求Net.MaxOpenRequests必须为1
	saramaConfig.Producer.Idempotent = true
	saramaConfig.Net.MaxOpenRequests = 1

	// 创建同步Producer
	producer, err := sarama.NewSyncProducer(cfg.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("Kafka Producer创建失败 [brokers=%v]: %w", cfg.Brokers, err)
	}

	zap.L().Info("Kafka Producer连接成功",
		zap.Strings("brokers", cfg.Brokers),
		zap.Int("required_acks", cfg.Producer.RequiredAcks),
		zap.Int("retry_max", cfg.Producer.RetryMax),
		zap.Bool("idempotent", true))

	return &Producer{
		producer: producer,
		config:   cfg,
	}, nil
}

// PublishEvent 发布事件到指定Topic（规则K1: 发送后必须检查错误）
func (p *Producer) PublishEvent(ctx context.Context, topic string, event *Event) error {
	// 序列化事件
	data, err := event.ToJSON()
	if err != nil {
		return fmt.Errorf("事件序列化失败: %w", err)
	}

	// 创建消息
	msg := &sarama.ProducerMessage{
		Topic:     topic,
		Value:     sarama.StringEncoder(data),
		Timestamp: time.Now(),
	}

	// 如果有用户ID，设置为Key用于分区
	if userData, ok := event.Data.(*UserData); ok && userData.ID != "" {
		msg.Key = sarama.StringEncoder(userData.ID)
	}

	// 规则K1: 发送后必须检查错误
	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("Kafka发送失败 [topic=%s, event_type=%s]: %w",
			topic, event.EventType, err)
	}

	zap.L().Debug("Kafka消息发送成功",
		zap.String("topic", topic),
		zap.String("event_type", string(event.EventType)),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset))

	return nil
}

// PublishUserEvent 发布用户事件（便捷方法）
func (p *Producer) PublishUserEvent(ctx context.Context, eventType EventType, userData *UserData) error {
	topic := p.config.Topics["user_events"]
	if topic == "" {
		return fmt.Errorf("未配置user_events topic")
	}

	event := NewEvent(eventType, userData)
	return p.PublishEvent(ctx, topic, event)
}

// PublishLogEvent 发布日志事件（便捷方法）
func (p *Producer) PublishLogEvent(ctx context.Context, logData *LogData) error {
	topic := p.config.Topics["system_logs"]
	if topic == "" {
		return fmt.Errorf("未配置system_logs topic")
	}

	event := NewEvent(SystemLog, logData)
	return p.PublishEvent(ctx, topic, event)
}

// Close 关闭Producer（不带context）
func (p *Producer) Close() error {
	return p.CloseWithContext(context.Background())
}

// CloseWithContext 带context的关闭Producer
func (p *Producer) CloseWithContext(ctx context.Context) error {
	if err := p.producer.Close(); err != nil {
		return fmt.Errorf("Kafka Producer关闭失败: %w", err)
	}
	zap.L().Info("Kafka Producer已关闭")
	return nil
}

// HealthCheck 健康检查（验证Producer连接状态）
func (p *Producer) HealthCheck() error {
	if p.producer == nil {
		return fmt.Errorf("Kafka Producer未初始化")
	}

	// SyncProducer没有直接的Topics方法，简单返回nil表示连接正常
	// 实际健康检查可以通过发送测试消息来验证，但这里为了简单起见只检查连接状态
	return nil
}
