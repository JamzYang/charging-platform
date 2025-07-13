package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/config"
	"github.com/charging-platform/charge-point-gateway/internal/gateway"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
	"github.com/charging-platform/charge-point-gateway/internal/message"
	"github.com/charging-platform/charge-point-gateway/internal/protocol/ocpp16"
	"github.com/charging-platform/charge-point-gateway/internal/storage"
	"github.com/charging-platform/charge-point-gateway/internal/transport/router"
	"github.com/charging-platform/charge-point-gateway/internal/transport/websocket"
	"github.com/spf13/viper"
)

func main() {
	// 初始化配置
	if err := initConfig(); err != nil {
		fmt.Printf("Failed to initialize configuration: %v\n", err)
		os.Exit(1)
	}

	// 加载应用配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志系统
	logConfig := &logger.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		Output:     cfg.Log.Output,
		TimeFormat: time.RFC3339,
		Caller:     true,
	}

	if err := logger.InitGlobalLogger(logConfig); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Starting Charge Point Gateway...")
	logger.Infof("Server will listen on %s", cfg.GetServerAddr())

	// 初始化存储
	redisStorage, err := storage.NewRedisStorage(cfg.Redis)
	if err != nil {
		logger.Fatalf("Failed to initialize Redis storage: %v", err)
	}

	// 初始化 Kafka 生产者
	kafkaProducer, err := message.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.UpstreamTopic)
	if err != nil {
		logger.Fatalf("Failed to initialize Kafka producer: %v", err)
	}

	// 初始化 Kafka 消费者
	kafkaConsumer, err := message.NewKafkaConsumer(cfg.Kafka.Brokers, cfg.Kafka.DownstreamTopic, cfg.PodID, cfg.Kafka.PartitionNum)
	if err != nil {
		logger.Fatalf("Failed to initialize Kafka consumer: %v", err)
	}

	// 初始化网关组件
	gateway, err := initializeGateway(cfg, redisStorage, kafkaProducer)
	if err != nil {
		logger.Errorf("Failed to initialize gateway: %v", err)
		os.Exit(1)
	}

	// 定义下行指令处理器
	commandHandler := func(cmd *message.Command) {
		logger.Infof("Received command for charge point %s: %s", cmd.ChargePointID, cmd.CommandName)
		// 使用 wsManager 找到对应的连接，并发送指令
		// gateway.wsManager.SendCommand(cmd.ChargePointID, cmd)
	}

	// 启动服务
	go kafkaConsumer.Start(commandHandler)
	if err := gateway.Start(); err != nil {
		logger.Errorf("Failed to start gateway: %v", err)
		os.Exit(1)
	}

	logger.Info("Gateway started successfully")

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// 执行清理操作
	// 1. 关闭 WebSocket 服务器
	if err := gateway.wsManager.Stop(); err != nil {
		logger.Errorf("Error stopping WebSocket manager: %v", err)
	}
	// 2. 关闭 Kafka 消费者
	if err := kafkaConsumer.Close(); err != nil {
		logger.Errorf("Error closing Kafka consumer: %v", err)
	}
	// 3. 关闭 Kafka 生产者
	if err := kafkaProducer.Close(); err != nil {
		logger.Errorf("Error closing Kafka producer: %v", err)
	}
	// 4. 关闭 Redis 连接
	if err := redisStorage.Close(); err != nil {
		logger.Errorf("Error closing Redis storage: %v", err)
	}

	logger.Info("Server gracefully stopped.")
}

// GatewayComponents 网关组件集合
type GatewayComponents struct {
	wsManager  *websocket.Manager
	router     router.MessageRouter
	dispatcher gateway.MessageDispatcher
	converter  gateway.ModelConverter
	processor  *ocpp16.Processor
	handler    gateway.ProtocolHandler
}

// Start 启动所有组件
func (g *GatewayComponents) Start() error {
	logger.Info("Starting gateway components...")

	// 启动消息分发器
	if err := g.dispatcher.Start(); err != nil {
		return fmt.Errorf("failed to start message dispatcher: %w", err)
	}

	// 启动路由器
	if err := g.router.Start(); err != nil {
		return fmt.Errorf("failed to start router: %w", err)
	}

	// 启动WebSocket管理器
	if err := g.wsManager.Start(); err != nil {
		return fmt.Errorf("failed to start WebSocket manager: %w", err)
	}

	logger.Info("All gateway components started successfully")
	return nil
}

// Stop 停止所有组件
func (g *GatewayComponents) Stop() error {
	logger.Info("Stopping gateway components...")

	// 按相反顺序停止组件
	if err := g.wsManager.Stop(); err != nil {
		logger.Errorf("Error stopping WebSocket manager: %v", err)
	}

	if err := g.router.Stop(); err != nil {
		logger.Errorf("Error stopping router: %v", err)
	}

	if err := g.dispatcher.Stop(); err != nil {
		logger.Errorf("Error stopping message dispatcher: %v", err)
	}

	logger.Info("All gateway components stopped")
	return nil
}

// initializeGateway 初始化网关组件
func initializeGateway(cfg *config.Config, storage storage.ConnectionStorage, producer message.EventProducer) (*GatewayComponents, error) {
	logger.Info("Initializing gateway components...")

	// 1. 创建统一模型转换器
	converterConfig := gateway.DefaultConverterConfig()
	converter := gateway.NewUnifiedModelConverter(converterConfig)

	// 2. 创建消息分发器
	dispatcherConfig := gateway.DefaultDispatcherConfig()
	dispatcher := gateway.NewDefaultMessageDispatcher(dispatcherConfig)

	// 3. 创建OCPP16处理器
	processorConfig := ocpp16.DefaultProcessorConfig()
	processor := ocpp16.NewProcessor(processorConfig, cfg.PodID, storage)

	// 4. 创建OCPP16协议处理器适配器
	handlerConfig := ocpp16.DefaultProtocolHandlerConfig()
	handler := ocpp16.NewProtocolHandler(processor, converter, handlerConfig)

	// 5. 注册协议处理器到分发器
	if err := dispatcher.RegisterHandler("1.6", handler); err != nil {
		return nil, fmt.Errorf("failed to register OCPP16 handler: %w", err)
	}

	// 6. 创建消息路由器
	routerConfig := router.DefaultRouterConfig()
	messageRouter := router.NewDefaultMessageRouter(routerConfig)

	// 7. 设置路由器的依赖
	if err := messageRouter.SetMessageDispatcher(dispatcher); err != nil {
		return nil, fmt.Errorf("failed to set message dispatcher: %w", err)
	}

	// 8. 创建WebSocket管理器
	wsConfig := websocket.DefaultConfig()
	wsConfig.Host = cfg.Server.Host
	wsConfig.Port = cfg.Server.Port
	wsConfig.Path = cfg.Server.WebSocketPath
	wsManager := websocket.NewManager(wsConfig)

	// 9. 设置路由器的WebSocket管理器
	if err := messageRouter.SetWebSocketManager(wsManager); err != nil {
		return nil, fmt.Errorf("failed to set WebSocket manager: %w", err)
	}

	// 10. 设置WebSocket管理器的消息路由器
	// 注意：这里需要WebSocket管理器支持设置路由器的方法
	// 目前简化处理，假设WebSocket管理器会自动使用路由器

	logger.Info("Gateway components initialized successfully")

	return &GatewayComponents{
		wsManager:  wsManager,
		router:     messageRouter,
		dispatcher: dispatcher,
		converter:  converter,
		processor:  processor,
		handler:    handler,
	}, nil
}

// initLogger 函数已被移除，现在使用 logger 包进行日志初始化

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
			fmt.Println("Config file not found, using defaults and environment variables")
		} else {
			return fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		fmt.Printf("Configuration loaded from: %s\n", viper.ConfigFileUsed())
	}
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
