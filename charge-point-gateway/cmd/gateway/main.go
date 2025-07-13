package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/charging-platform/charge-point-gateway/internal/config"
	"github.com/charging-platform/charge-point-gateway/internal/gateway"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
	"github.com/charging-platform/charge-point-gateway/internal/message"
	"github.com/charging-platform/charge-point-gateway/internal/metrics"
	"github.com/charging-platform/charge-point-gateway/internal/protocol/ocpp16"
	"github.com/charging-platform/charge-point-gateway/internal/storage"
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
	consumer, err := message.NewKafkaConsumer(cfg.Kafka.Brokers, "gateway-group", cfg.Kafka.DownstreamTopic, cfg.PodID, cfg.Kafka.PartitionNum, log)
	if err != nil {
		log.Fatalf("Failed to initialize Kafka consumer: %v", err)
	}
	log.Info("Kafka consumer initialized")

	// 6. 初始化业务模型转换器
	converter := gateway.NewUnifiedModelConverter(gateway.DefaultConverterConfig())
	log.Info("Model converter initialized")

	// 7. 初始化 OCPP 1.6 处理器
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig(), cfg.PodID, storage)
	log.Info("OCPP 1.6 processor initialized")

	// 8. 初始化中央消息分发器
	dispatcher := gateway.NewDefaultMessageDispatcher(gateway.DefaultDispatcherConfig())
	// 注册处理器
	handler := ocpp16.NewProtocolHandler(processor, converter, ocpp16.DefaultProtocolHandlerConfig())
	if err := dispatcher.RegisterHandler("ocpp1.6", handler); err != nil {
		log.Fatalf("Failed to register ocpp1.6 handler: %v", err)
	}
	log.Info("Dispatcher initialized and handlers registered")

	// 9. 初始化 WebSocket 管理器 (暂时注释)
	// wsManager := websocket.NewManager(websocket.DefaultConfig())
	// log.Info("WebSocket manager initialized")

	// 10. 定义下行指令处理器
	commandHandler := func(cmd *message.Command) {
		log.Infof("Received command for charge point %s: %s", cmd.ChargePointID, cmd.CommandName)
		// wsManager.SendCommand(cmd.ChargePointID, cmd)
	}
	log.Info("Command handler defined")

	// 11. 启动服务
	// 启动监控服务器
	metrics.RegisterMetrics()
	go startMetricsServer(cfg.GetMetricsAddr(), log)
	log.Info("Metrics server starting...")

	// 启动 Kafka 消费者
	go func() {
		if err := consumer.Start(commandHandler); err != nil {
			log.Errorf("Kafka consumer failed: %v", err)
		}
	}()
	log.Info("Kafka consumer starting...")

	// 启动 WebSocket 管理器 (暂时注释)
	// go func() {
	// 	if err := wsManager.Start(); err != nil {
	// 		log.Errorf("WebSocket manager failed: %v", err)
	// 	}
	// }()
	// log.Info("WebSocket manager starting...")

	log.Info("Charge Point Gateway started successfully")

	// 12. 监听并处理优雅停机
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	// 按顺序执行清理操作
	// ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// defer cancel()

	// 1. 关闭 WebSocket 服务器 (暂时注释)
	// if err := wsManager.Shutdown(ctx); err != nil {
	// 	log.Errorf("Error shutting down WebSocket manager: %v", err)
	// }
	// log.Info("WebSocket manager shut down")

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
