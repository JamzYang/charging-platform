package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func main() {
	// 初始化日志
	initLogger()

	// 加载配置
	if err := initConfig(); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize configuration")
	}

	log.Info().Msg("Starting Charge Point Gateway...")

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// TODO: 初始化各个组件
	// - WebSocket服务器
	// - Redis客户端
	// - Kafka客户端
	// - 消息处理器
	_ = ctx // 暂时忽略未使用的变量，后续会使用

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Info().Msg("Received shutdown signal, gracefully shutting down...")

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// TODO: 关闭各个组件
	_ = shutdownCtx

	log.Info().Msg("Gateway shutdown completed")
}

func initLogger() {
	// 配置结构化日志
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	// 设置日志级别
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func initConfig() error {
	// 设置配置文件名和路径
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 设置环境变量前缀
	viper.SetEnvPrefix("GATEWAY")
	viper.AutomaticEnv()

	// 设置默认值
	setDefaultConfig()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Warn().Msg("Config file not found, using defaults and environment variables")
		} else {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	log.Info().Str("config_file", viper.ConfigFileUsed()).Msg("Configuration loaded")
	return nil
}

func setDefaultConfig() {
	// 服务器配置
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.websocket_path", "/ocpp")

	// Redis配置
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// Kafka配置
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.upstream_topic", "ocpp-events-up")
	viper.SetDefault("kafka.downstream_topic", "commands-down")

	// 缓存配置
	viper.SetDefault("cache.max_size", 10000)
	viper.SetDefault("cache.ttl", "1h")

	// 日志配置
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "console")
}
