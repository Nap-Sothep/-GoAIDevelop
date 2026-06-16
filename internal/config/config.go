package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	GRPC    GRPCConfig    `mapstructure:"grpc"`
	Log     LogConfig     `mapstructure:"log"`
	MongoDB MongoDBConfig `mapstructure:"mongodb"`
	Redis   RedisConfig   `mapstructure:"redis"`
	Kafka   KafkaConfig   `mapstructure:"kafka"`
}

// ServerConfig HTTP服务配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug, release, test
}

// GRPCConfig gRPC后端配置
type GRPCConfig struct {
	Target  string `mapstructure:"target"`  // gRPC服务地址
	Timeout int    `mapstructure:"timeout"` // 超时时间(秒)
}

// LogConfig 日志配置
type LogConfig struct {
	Level string `mapstructure:"level"` // debug, info, warn, error
}

// MongoDBConfig MongoDB配置
type MongoDBConfig struct {
	URI            string `mapstructure:"uri"`
	Database       string `mapstructure:"database"`
	MaxPoolSize    uint64 `mapstructure:"max_pool_size"`
	MinPoolSize    uint64 `mapstructure:"min_pool_size"`
	ConnectTimeout int    `mapstructure:"connect_timeout"`
	SocketTimeout  int    `mapstructure:"socket_timeout"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string `mapstructure:"addr"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
	DefaultTTL   int    `mapstructure:"default_ttl"`
	DialTimeout  int    `mapstructure:"dial_timeout"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers  []string            `mapstructure:"brokers"`
	Producer KafkaProducerConfig `mapstructure:"producer"`
	Consumer KafkaConsumerConfig `mapstructure:"consumer"`
	Topics   map[string]string   `mapstructure:"topics"`
}

// KafkaProducerConfig Kafka生产者配置
type KafkaProducerConfig struct {
	RequiredAcks int `mapstructure:"required_acks"`
	RetryMax     int `mapstructure:"retry_max"`
	Timeout      int `mapstructure:"timeout"`
}

// KafkaConsumerConfig Kafka消费者配置
type KafkaConsumerConfig struct {
	GroupID        string `mapstructure:"group_id"`
	AutoCommit     bool   `mapstructure:"auto_commit"`
	SessionTimeout int    `mapstructure:"session_timeout"`
}

// Load 加载配置
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 支持环境变量覆盖，如 GATEWAY_MONGODB_URI=mongodb://...
	// 环境变量格式: GATEWAY_<配置路径>，嵌套字段用_分隔
	// 例如: GATEWAY_MONGODB_URI, GATEWAY_REDIS_ADDR, GATEWAY_KAFKA_BROKERS_0
	viper.SetEnvPrefix("GATEWAY")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	return &cfg, nil
}
