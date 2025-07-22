package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
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
		Async:  cfg.Log.Async, // 使用配置中的异步设置
	})
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	log.Info("Logger initialized")

	// 确保全局 logger 也被正确设置（这在 logger.New 中已经完成，但这里再次确认）
	// 这样其他组件使用 zerolog 的全局函数时也会使用正确的配置

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

	// 7. 初始化 OCPP 1.6 处理器（暂时不传入messageSender，稍后设置）
	processorConfig := ocpp16.DefaultProcessorConfig()
	processorConfig.WorkerCount = cfg.OCPP.WorkerCount // 使用配置文件中的Worker数量
	processor := ocpp16.NewProcessor(processorConfig, cfg.PodID, storage, nil, log)
	log.Infof("OCPP 1.6 processor initialized with %d workers", cfg.OCPP.WorkerCount)

	// 8. 初始化中央消息分发器
	dispatcher := gateway.NewDefaultMessageDispatcher(gateway.DefaultDispatcherConfig(), log)
	// 注册处理器
	handler := ocpp16.NewProtocolHandler(processor, converter, ocpp16.DefaultProtocolHandlerConfig(), log)
	if err := dispatcher.RegisterHandler(protocol.OCPP_VERSION_1_6, handler); err != nil {
		log.Fatalf("Failed to register %s handler: %v", protocol.OCPP_VERSION_1_6, err)
	}
	log.Info("Dispatcher initialized and handlers registered")

	// 9. 初始化 WebSocket 管理器 - 使用配置文件中的完整配置
	wsConfig := &websocket.Config{
		Host: cfg.Server.Host,
		Port: cfg.Server.Port,
		Path: cfg.Server.WebSocketPath,

		ReadBufferSize:    cfg.WebSocket.ReadBufferSize,
		WriteBufferSize:   cfg.WebSocket.WriteBufferSize,
		HandshakeTimeout:  cfg.WebSocket.HandshakeTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		PingInterval:      cfg.WebSocket.PingInterval,
		PongTimeout:       cfg.WebSocket.PongTimeout,
		MaxMessageSize:    cfg.WebSocket.MaxMessageSize,
		EnableCompression: cfg.WebSocket.EnableCompression,

		MaxConnections:  cfg.Server.MaxConnections,
		IdleTimeout:     cfg.WebSocket.IdleTimeout,
		CleanupInterval: cfg.WebSocket.CleanupInterval,

		CheckOrigin:       cfg.WebSocket.CheckOrigin,
		AllowedOrigins:    cfg.WebSocket.AllowedOrigins,
		EnableSubprotocol: cfg.WebSocket.EnableSubprotocol,
		Subprotocols:      protocol.GetSupportedVersions(),
	}
	wsManager := websocket.NewManager(wsConfig, dispatcher, log, cfg.EventChannels.BufferSize)
	log.Info("WebSocket manager initialized with dispatcher")
	log.Info("WebSocket manager initialized")

	// 设置processor的消息发送器（实现统一消息处理模式）
	processor.SetMessageSender(wsManager)
	log.Info("Processor message sender configured")

	// 启动全局Ping服务 - 减少Goroutine数量的优化
	wsManager.StartGlobalPingService()
	log.Info("Global ping service started")

	// 10. 定义下行指令处理器（统一消息处理模式）
	commandHandler := func(cmd *message.Command) {
		log.Infof("Received command for charge point %s: %s", cmd.ChargePointID, cmd.CommandName)
		if err := processor.SendDownlinkCommand(cmd.ChargePointID, cmd.CommandName, cmd.Payload); err != nil {
			log.Errorf("Failed to send command to %s: %v", cmd.ChargePointID, err)
		}
	}
	log.Info("Command handler defined (using unified message processing)")

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
	log.Info("About to start message dispatcher")
	if err := dispatcher.Start(); err != nil {
		log.Fatalf("Failed to start message dispatcher: %v", err)
	}
	log.Infof("Message dispatcher started successfully")

	// 验证处理器注册
	versions := dispatcher.GetRegisteredVersions()
	log.Infof("Registered protocol versions: %v", versions)

	// 启动 WebSocket 管理器
	// 创建主应用的路由器
	mainMux := http.NewServeMux()
	// 将 WebSocket 处理器注册到主路由器 - 添加 "/" 以匹配子路径
	wsPath := wsConfig.Path + "/"
	log.Infof("Registering WebSocket handler at path: %s", wsPath)
	mainMux.HandleFunc(wsPath, wsManager.ServeWS)
	// 注册健康检查处理器 - 使用 WebSocket 管理器的健康检查
	mainMux.HandleFunc("/health", wsManager.HandleHealthCheck)

	// 启动独立的metrics服务器 (已在前面启动，此处移除重复启动)
	// go startMetricsServer(cfg.GetMetricsAddr(), log)

	// 启动主应用服务器 - 使用优化的TCP监听器
	go func() {
		log.Infof("Main server starting on %s", cfg.GetServerAddr())

		// 创建优化的TCP监听器，增加监听队列大小
		// 使用标准的监听器，Docker容器内的优化主要通过sysctls实现
		listener, err := net.Listen("tcp", cfg.GetServerAddr())
		if err != nil {
			log.Fatalf("Failed to create listener: %v", err)
		}

		// 创建HTTP服务器
		server := &http.Server{
			Handler:        mainMux,
			ReadTimeout:    cfg.Server.ReadTimeout,
			WriteTimeout:   cfg.Server.WriteTimeout,
			IdleTimeout:    120 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1MB
		}

		log.Infof("Optimized server listening on %s with enhanced backlog", listener.Addr().String())
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Main server failed: %v", err)
		}
	}()

	// 启动业务事件处理器 - 将分发器的事件发送到Kafka
	go func() {
		log.Info("Business event handler started")
		for event := range dispatcher.GetEventChannel() {
			if err := producer.PublishEvent(event); err != nil {
				log.Errorf("Failed to publish event to Kafka: %v", err)
			} else {
				log.Debugf("Published event %s from charge point %s to Kafka", event.GetType(), event.GetChargePointID())
			}
		}
	}()

	// 启动业务事件处理器 - 将分发器的事件发送到Kafka
	go func() {
		log.Info("Business event handler started")
		for event := range dispatcher.GetEventChannel() {
			if err := producer.PublishEvent(event); err != nil {
				log.Errorf("Failed to publish event to Kafka: %v", err)
			} else {
				log.Debugf("Published event %s from charge point %s to Kafka", event.GetType(), event.GetChargePointID())
			}
		}
	}()

	// 启动WebSocket事件处理器 - 优化为高性能模式
	go func() {
		log.Debugf("WebSocket event handler started")

		// 统计计数器，减少日志输出频率
		var (
			connectedCount    int64
			disconnectedCount int64
			messageCount      int64
			errorCount        int64
		)

		// 定期输出统计信息
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		go func() {
			for range ticker.C {
				if atomic.LoadInt64(&connectedCount) > 0 || atomic.LoadInt64(&messageCount) > 0 {
					log.Infof("Event stats - Connected: %d, Disconnected: %d, Messages: %d, Errors: %d",
						atomic.SwapInt64(&connectedCount, 0),
						atomic.SwapInt64(&disconnectedCount, 0),
						atomic.SwapInt64(&messageCount, 0),
						atomic.SwapInt64(&errorCount, 0))
				}
			}
		}()

		for event := range wsManager.GetEventChannel() {
			// 快速处理，减少同步操作
			switch event.Type {
			case websocket.EventTypeConnected:
				atomic.AddInt64(&connectedCount, 1)
				// 只记录重要的连接事件，不是每个都记录
				if atomic.LoadInt64(&connectedCount)%1000 == 0 {
					log.Infof("Milestone: %d charge points connected", atomic.LoadInt64(&connectedCount))
				}
			case websocket.EventTypeDisconnected:
				atomic.AddInt64(&disconnectedCount, 1)
			case websocket.EventTypeMessage:
				atomic.AddInt64(&messageCount, 1)
				// 消息事件不记录日志，避免I/O瓶颈
			case websocket.EventTypeError:
				atomic.AddInt64(&errorCount, 1)
				// 错误事件仍然需要记录，但使用异步方式
				go log.Errorf("WebSocket error for %s: %v", event.ChargePointID, event.Error)
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
