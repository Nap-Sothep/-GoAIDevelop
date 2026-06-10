package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	GRPC   GRPCConfig   `mapstructure:"grpc"`
	Log    LogConfig    `mapstructure:"log"`
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

// Load 加载配置
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 支持环境变量覆盖，如 GATEWAY_SERVER_PORT=8080
	viper.SetEnvPrefix("GATEWAY")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	return &cfg, nil
}
