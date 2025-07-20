package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/config"
	"github.com/charging-platform/charge-point-gateway/internal/domain/protocol"
	"github.com/charging-platform/charge-point-gateway/internal/gateway"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
	"github.com/charging-platform/charge-point-gateway/internal/message"
	"github.com/charging-platform/charge-point-gateway/internal/metrics"
	"github.com/charging-platform/charge-point-gateway/internal/protocol/ocpp16"
	"github.com/charging-platform/charge-point-gateway/internal/storage"
	"github.com/charging-platform/charge-point-gateway/internal/transport/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 1. 加载配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化日志
	log, err := logger.New(&logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		Output: cfg.Log.Output,
	})
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	log.Info("Logger initialized")

	// 3. 初始化存储
	storage, err := storage.NewRedisStorage(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	log.Info("Storage initialized")

	// 4. 初始化 Kafka 生产者
	producer, err := message.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.UpstreamTopic)
	if err != nil {
		log.Fatalf("Failed to initialize Kafka producer: %v", err)
	}
	log.Info("Kafka producer initialized")

	// 5. 初始化 Kafka 消费者
	consumer, err := message.NewKafkaConsumer(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup, cfg.Kafka.DownstreamTopic, cfg.PodID, cfg.Kafka.PartitionNum, log)
	if err != nil {
		log.Fatalf("Failed to initialize Kafka consumer: %v", err)
	}
	log.Infof("Kafka consumer initialized with brokers: %v, group: %s", cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup)

	// 6. 初始化业务模型转换器
	converter := gateway.NewUnifiedModelConverter(gateway.DefaultConverterConfig())
	log.Info("Model converter initialized")

	// 7. 初始化 OCPP 1.6 处理器
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig(), cfg.PodID, storage)
	log.Info("OCPP 1.6 processor initialized")

	// 8. 初始化中央消息分发器
	dispatcher := gateway.NewDefaultMessageDispatcher(gateway.DefaultDispatcherConfig(), log)
	// 注册处理器
	handler := ocpp16.NewProtocolHandler(processor, converter, ocpp16.DefaultProtocolHandlerConfig())
	if err := dispatcher.RegisterHandler(protocol.OCPP_VERSION_1_6, handler); err != nil {
		log.Fatalf("Failed to register %s handler: %v", protocol.OCPP_VERSION_1_6, err)
	}
	log.Info("Dispatcher initialized and handlers registered")

	// 9. 初始化 WebSocket 管理器 - 按照架构设计传递分发器
	wsConfig := websocket.DefaultConfig()
	wsConfig.Host = cfg.Server.Host
	wsConfig.Port = cfg.Server.Port
	wsConfig.Path = cfg.Server.WebSocketPath
	wsManager := websocket.NewManager(wsConfig, dispatcher, log)
	log.Errorf("MAIN: WebSocket manager initialized with dispatcher")
	log.Info("WebSocket manager initialized")

	// 10. 定义下行指令处理器
	commandHandler := func(cmd *message.Command) {
		log.Infof("Received command for charge point %s: %s", cmd.ChargePointID, cmd.CommandName)
		if err := wsManager.SendCommand(cmd.ChargePointID, cmd); err != nil {
			log.Errorf("Failed to send command to %s: %v", cmd.ChargePointID, err)
		}
	}
	log.Info("Command handler defined")

	// 11. 启动服务
	// 启动监控服务器
	metrics.RegisterMetrics()
	go startMetricsServer(cfg.GetMetricsAddr(), log)
	log.Infof("Metrics server starting on %s...", cfg.GetMetricsAddr())

	// 启动 Kafka 消费者
	go func() {
		if err := consumer.Start(commandHandler); err != nil {
			log.Errorf("Kafka consumer failed: %v", err)
		}
	}()
	log.Info("Kafka consumer starting...")

	// 启动消息分发器
	log.Errorf("MAIN: About to start message dispatcher")
	if err := dispatcher.Start(); err != nil {
		log.Fatalf("Failed to start message dispatcher: %v", err)
	}
	log.Errorf("MAIN: Message dispatcher started successfully")

	// 验证处理器注册
	versions := dispatcher.GetRegisteredVersions()
	log.Errorf("MAIN: Registered protocol versions: %v", versions)

	// 启动 WebSocket 管理器
	// 创建主应用的路由器
	mainMux := http.NewServeMux()
	// 将 WebSocket 处理器注册到主路由器
	mainMux.HandleFunc(wsConfig.Path, wsManager.ServeWS)
	// 注册健康检查处理器
	mainMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 启动主应用服务器
	go func() {
		log.Infof("Main server starting on %s", cfg.GetServerAddr())
		if err := http.ListenAndServe(cfg.GetServerAddr(), mainMux); err != nil {
			log.Fatalf("Main server failed: %v", err)
		}
	}()

	// 启动WebSocket事件处理器
	go func() {
		log.Errorf("MAIN: WebSocket event handler started")
		for event := range wsManager.GetEventChannel() {
			log.Errorf("MAIN: Received event type: %s from %s", event.Type, event.ChargePointID)
			switch event.Type {
			case websocket.EventTypeConnected:
				log.Infof("Charge point %s connected", event.ChargePointID)
				// 可以在这里添加连接事件处理逻辑
			case websocket.EventTypeDisconnected:
				log.Infof("Charge point %s disconnected", event.ChargePointID)
				// 可以在这里添加断开连接事件处理逻辑
			case websocket.EventTypeMessage:
				// 消息处理已移至 websocket.ConnectionWrapper.handleMessage
				// 此处仅记录事件，用于监控和调试
				log.Debugf("Message event received from %s (size: %d)", event.ChargePointID, len(event.Message))
			case websocket.EventTypeError:
				log.Errorf("WebSocket error for %s: %v", event.ChargePointID, event.Error)
			}
		}
	}()
	log.Info("WebSocket event handler started")

	log.Info("Charge Point Gateway started successfully")

	// 12. 监听并处理优雅停机
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	// 按顺序执行清理操作
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. 关闭 WebSocket 服务器
	if err := wsManager.Shutdown(ctx); err != nil {
		log.Errorf("Error shutting down WebSocket manager: %v", err)
	}
	log.Info("WebSocket manager shut down")

	// 2. 关闭 Kafka 消费者
	if err := consumer.Close(); err != nil {
		log.Errorf("Error closing Kafka consumer: %v", err)
	}
	log.Info("Kafka consumer closed")

	// 3. 关闭 Kafka 生产者
	if err := producer.Close(); err != nil {
		log.Errorf("Error closing Kafka producer: %v", err)
	}
	log.Info("Kafka producer closed")

	// 4. 关闭 Redis 连接
	if err := storage.Close(); err != nil {
		log.Errorf("Error closing storage: %v", err)
	}
	log.Info("Storage closed")

	log.Info("Server gracefully stopped.")
}

// startMetricsServer 启动监控服务器
func startMetricsServer(addr string, log *logger.Logger) {
	http.Handle("/metrics", promhttp.Handler())
	log.Infof("Metrics server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Metrics server failed: %v", err)
	}
}
