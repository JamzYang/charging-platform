# 充电桩网关详细设计文档 (Phase 2)

## 1. 概述

本文档旨在为 `charge-point-gateway` 项目提供后续阶段的详细设计，以指导 `Code` 模式的开发工作。设计严格遵循 [`docs/high_availability_gateway_arch_design.md`](docs/high_availability_gateway_arch_design.md:1) 中定义的核心架构，并基于 [任务日志](.cursor/tasks/2025-01-14_1_gateway-architecture-implementation.md:1) 中已完成的实现（截至步骤4，并已纠正偏差）。

**核心目标**: 完成缓存、消息队列、可靠性及监控等关键模块的实现，最终组装成一个功能完整的、高可用的网关应用。

**技术栈**:
*   **Redis**: `go-redis/redis`
*   **Kafka**: `Shopify/sarama`
*   **监控**: `prometheus/client_golang`
*   **配置**: `spf13/viper`
*   **日志**: `rs/zerolog`

---

## 2. 缓存系统与状态管理 (`internal/storage`)

**目标**: 实现充电桩连接与 Gateway Pod 动态映射关系的管理，这是下行指令路由的核心。

### 2.1. 接口定义

在 `internal/storage/` 目录下定义存储层接口，实现与具体存储后端的解耦。

```go
// internal/storage/interface.go

package storage

import (
	"context"
	"time"
)

// ConnectionStorage 定义了管理充电桩连接映射的接口
type ConnectionStorage interface {
	// SetConnection 注册或更新一个充电桩的连接信息
	// chargePointID: 充电桩的唯一标识
	// gatewayID: 当前处理该连接的 Gateway Pod 的唯一标识
	// ttl: 键的过期时间，用于自动清理僵尸连接
	SetConnection(ctx context.Context, chargePointID string, gatewayID string, ttl time.Duration) error

	// GetConnection 获取指定充电桩当前连接的 Gateway Pod ID
	GetConnection(ctx context.Context, chargePointID string) (string, error)

	// DeleteConnection 删除一个充电桩的连接信息（例如，充电桩正常断连时）
	DeleteConnection(ctx context.Context, chargePointID string) error

	// Close 关闭与存储后端的连接
	Close() error
}
```

### 2.2. Redis 实现

创建 `RedisStorage` 结构体来实现 `ConnectionStorage` 接口。

```go
// internal/storage/redis_storage.go

package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/your-org/charge-point-gateway/internal/config"
)

// RedisStorage 使用 Redis 来存储连接映射
type RedisStorage struct {
	client *redis.Client
	prefix string // e.g., "conn:"
}

// NewRedisStorage 创建一个新的 RedisStorage 实例
func NewRedisStorage(cfg config.RedisConfig) (*RedisStorage, error) {
	// ... Redis 客户端初始化逻辑 ...
	// 使用 go-redis/redis 连接到 Redis Cluster
	// ...
	return &RedisStorage{client: client, prefix: "conn:"}, nil
}

// SetConnection 实现接口方法
func (r *RedisStorage) SetConnection(ctx context.Context, chargePointID string, gatewayID string, ttl time.Duration) error {
	key := fmt.Sprintf("%s%s", r.prefix, chargePointID)
	return r.client.Set(ctx, key, gatewayID, ttl).Err()
}

// GetConnection 实现接口方法
func (r *RedisStorage) GetConnection(ctx context.Context, chargePointID string) (string, error) {
    // ... 实现 GET conn:<chargePointID> ...
}

// DeleteConnection 实现接口方法
func (r *RedisStorage) DeleteConnection(ctx context.Context, chargePointID string) error {
    // ... 实现 DEL conn:<chargePointID> ...
}

// Close 实现接口方法
func (r *RedisStorage) Close() error {
	return r.client.Close()
}
```

**关键实现点**:
1.  **配置**: Redis 的地址、密码、数据库等信息从 `viper` 加载。
2.  **错误处理**: `GetConnection` 在未找到键时，应返回 `redis.Nil` 错误，上层需要妥善处理此情况。
3.  **TTL**: `SetConnection` 必须使用 TTL，这是防止因 Pod 异常宕机导致连接映射残留（僵尸数据）的关键机制。TTL 的值应略大于充电桩重连的间隔时间。

---

## 3. Kafka 集成与消息队列 (`internal/message`)

**目标**: 实现上行事件的发布和下行指令的消费。

### 3.1. 上行事件生产者

**接口定义**:
```go
// internal/message/interface.go

package message

import "github.com/your-org/charge-point-gateway/internal/domain/events"

// EventProducer 定义了向消息队列发布统一业务事件的接口
type EventProducer interface {
	// PublishEvent 异步发布一个事件
	PublishEvent(event events.Event) error
	// Close 关闭生产者
	Close() error
}
```

**Kafka 实现**:
```go
// internal/message/kafka_producer.go

package message

import (
	"encoding/json"
	"github.com/Shopify/sarama"
	"github.com/your-org/charge-point-gateway/internal/domain/events"
)

type KafkaProducer struct {
	producer sarama.AsyncProducer
	topic    string
}

// NewKafkaProducer 创建一个新的 KafkaProducer
func NewKafkaProducer(brokers []string, topic string) (*KafkaProducer, error) {
    // ... 初始化 sarama.AsyncProducer ...
    // 需要处理生产者的 Successes 和 Errors channel
}

func (p *KafkaProducer) PublishEvent(event events.Event) error {
	// 1. 将 event 序列化为 JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// 2. 创建 Kafka 消息
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(event.GetChargePointID()), // 使用充电桩ID作为Key，保证同一桩的消息落入同一分区
		Value: sarama.ByteEncoder(eventData),
	}

	// 3. 发送消息
	p.producer.Input() <- msg
	return nil
}

// ... Close 方法 ...
```

### 3.2. 下行指令消费者

**目标**: 实现“共享主题 + 分区路由”方案。每个 Gateway Pod 只消费属于自己的分区。

**接口定义**:
```go
// internal/message/interface.go

// Command 是下行指令的统一数据结构
type Command struct {
	ChargePointID string      `json:"chargePointId"`
	CommandName   string      `json:"commandName"`
	Payload       interface{} `json:"payload"`
}

// CommandHandler 是处理下行指令的函数类型
type CommandHandler func(cmd *Command)

// CommandConsumer 定义了消费下行指令的接口
type CommandConsumer interface {
	// Start 开始消费，并将接收到的指令传递给 handler
	Start(handler CommandHandler) error
	// Close 关闭消费者
	Close() error
}
```

**Kafka 实现**:
```go
// internal/message/kafka_consumer.go

package message

import (
	"context"
	"github.com/Shopify/sarama"
	"hash/fnv"
)

type KafkaConsumer struct {
	consumerGroup sarama.ConsumerGroup
	topic         string
	podID         string // 当前 Pod 的唯一标识
	partitionNum  int    // 主题的总分区数
}

// ... NewKafkaConsumer 初始化 ...

// Start 启动消费者组
func (c *KafkaConsumer) Start(handler CommandHandler) error {
	ctx := context.Background()
	// 启动一个循环来消费消息
	for {
		// `Consume` 会在一个循环中处理，直到 session 结束
		if err := c.consumerGroup.Consume(ctx, []string{c.topic}, c); err != nil {
			// log error
		}
	}
}

// -- sarama.ConsumerGroupHandler 接口实现 --

func (c *KafkaConsumer) Setup(sarama.ConsumerGroupSession) error { return nil }
func (c *KafkaConsumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim 是核心消费逻辑
func (c *KafkaConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// 计算当前 Pod 应该消费哪个分区
	hasher := fnv.New32a()
	hasher.Write([]byte(c.podID))
	myPartition := int32(hasher.Sum32() % uint32(c.partitionNum))

	// 如果当前 claim 的分区不是我该消费的，则直接返回
	if claim.Partition() != myPartition {
		return nil
	}

	for message := range claim.Messages() {
		// 1. 反序列化消息为 Command
		// 2. 调用 handler(cmd)
		// 3. 标记消息已处理
		session.MarkMessage(message, "")
	}
	return nil
}
```

**关键实现点**:
1.  **Pod ID**: `podID` 必须是稳定的、唯一的，通常可以通过环境变量（如 Kubernetes Downward API）注入。
2.  **分区计算**: 消费者和生产者必须使用完全相同的哈希算法和分区总数来计算目标分区，以确保路由正确。
3.  **消费者组**: 使用消费者组可以简化偏移量管理和 rebalance 处理。

---

## 4. 可靠性与错误处理

### 4.1. 故障转移 (Failover)

**核心逻辑**: 在 `OCPP 1.6J 协议处理器` (`internal/protocol/ocpp16/processor.go`) 的 `BootNotification` 处理逻辑中，必须强制更新 Redis 映射。

```go
// internal/protocol/ocpp16/processor.go (伪代码)

func (p *Processor) handleBootNotification(conn *connection.Connection, call *ocpp16.Call) (*ocpp16.CallResult, error) {
    // ... 解析 BootNotification ...

    chargePointID := req.ChargePointModel // 假设这是充电桩ID

    // 【关键步骤】: 更新 Redis 连接映射
    // podID 应从配置或环境变量中获取
    // TTL 应大于充电桩重连间隔
    err := p.storage.SetConnection(context.Background(), chargePointID, p.podID, 5*time.Minute)
    if err != nil {
        // 记录严重错误，但这不应中断 BootNotification 的正常响应
        p.logger.Error("Failed to set connection mapping in Redis", "error", err)
    }

    // ... 生成 BootNotification.conf 响应 ...
    return &ocpp16.CallResult{...}, nil
}
```

### 4.2. 优雅停机 (Graceful Shutdown)

在 `cmd/gateway/main.go` 中实现。

```go
// cmd/gateway/main.go (伪代码)

func main() {
    // ... 初始化 ...
    // kafkaProducer, kafkaConsumer, redisStorage, webSocketServer ...

    // 创建一个 channel 来监听中断信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    // 启动所有服务 (goroutines)
    // go webSocketServer.Start()
    // go kafkaConsumer.Start(...)

    // 阻塞，直到接收到信号
    <-quit
    logger.Info("Shutting down server...")

    // 执行清理操作
    // 1. 关闭 WebSocket 服务器，停止接受新连接
    webSocketServer.Shutdown(context.Background())
    // 2. 关闭 Kafka 消费者
    kafkaConsumer.Close()
    // 3. 关闭 Kafka 生产者
    kafkaProducer.Close()
    // 4. 关闭 Redis 连接
    redisStorage.Close()

    logger.Info("Server gracefully stopped.")
}
```

---

## 5. 监控与可观测性 (`internal/metrics`)

**目标**: 使用 Prometheus 暴露关键指标。

### 5.1. 指标定义

```go
// internal/metrics/metrics.go

package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	ActiveConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gateway_active_connections",
			Help: "Number of active WebSocket connections.",
		},
	)
	MessagesReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_messages_received_total",
			Help: "Total number of messages received from charge points.",
		},
		[]string{"ocpp_version", "message_type"},
	)
	EventsPublished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_events_published_total",
			Help: "Total number of events published to Kafka.",
		},
		[]string{"event_type"},
	)
    // ... 其他指标，如 commands_consumed_total, message_processing_duration_seconds ...
)

func RegisterMetrics() {
	prometheus.MustRegister(ActiveConnections)
	prometheus.MustRegister(MessagesReceived)
	prometheus.MustRegister(EventsPublished)
}
```

### 5.2. 指标采集点

*   `ActiveConnections`: 在 WebSocket 管理器的 `AddConnection` 和 `RemoveConnection` 方法中 `Inc()` 和 `Dec()`。
*   `MessagesReceived`: 在消息路由器中，解析出消息类型后 `Inc()`。
*   `EventsPublished`: 在 `KafkaProducer` 的 `PublishEvent` 成功后 `Inc()`。

### 5.3. 暴露指标

在 `cmd/gateway/main.go` 中启动一个独立的 HTTP 服务器。

```go
// cmd/gateway/main.go (伪代码)

import "github.com/prometheus/client_golang/prometheus/promhttp"

func startMetricsServer(addr string) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.Fatal("Metrics server failed", "error", err)
		}
	}()
	logger.Info("Metrics server started", "addr", addr)
}

func main() {
    // ...
    metrics.RegisterMetrics()
    startMetricsServer(":9090") // 从配置读取端口
    // ...
}
```

---

## 6. 主程序组装 (`cmd/gateway/main.go`)

**目标**: 将所有独立的模块组装起来，形成一个完整的应用。

**启动流程 (伪代码)**:

```go
// cmd/gateway/main.go

func main() {
    // 1. 加载配置 (Viper)
    cfg := config.Load()

    // 2. 初始化日志 (Zerolog)
    logger.Init(cfg.Log)

    // 3. 初始化存储 (Redis)
    storage, err := storage.NewRedisStorage(cfg.Redis)
    // ... handle error

    // 4. 初始化 Kafka 生产者
    producer, err := message.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.UpstreamTopic)
    // ... handle error

    // 5. 初始化 Kafka 消费者
    consumer, err := message.NewKafkaConsumer(cfg.Kafka.Brokers, cfg.Kafka.DownstreamTopic, cfg.PodID, cfg.Kafka.PartitionNum)
    // ... handle error

    // 6. 初始化业务模型转换器
    converter := gateway.NewUnifiedModelConverter()

    // 7. 初始化 OCPP 1.6 处理器
    // 处理器需要存储、转换器和生产者作为依赖
    ocpp16Processor := ocpp16.NewProcessor(storage, converter, producer, logger)

    // 8. 初始化中央消息分发器/路由器
    // 分发器需要知道哪个协议版本对应哪个处理器
    dispatcher := gateway.NewDispatcher()
    dispatcher.RegisterHandler("ocpp1.6", ocpp16Processor)

    // 9. 初始化 WebSocket 管理器
    wsManager := websocket.NewManager(cfg.Server, dispatcher) // dispatcher 作为消息处理器

    // 10. 定义下行指令处理器
    // 当消费者收到指令时，调用此函数
    commandHandler := func(cmd *message.Command) {
        // 使用 wsManager 找到对应的连接，并发送指令
        wsManager.SendCommand(cmd.ChargePointID, cmd)
    }

    // 11. 启动服务
    go consumer.Start(commandHandler)
    go wsManager.Start()
    startMetricsServer(cfg.Metrics.Addr)

    // 12. 监听并处理优雅停机
    // ... graceful shutdown logic ...
}
```

**组件依赖关系图**:

```mermaid
graph TD
    subgraph "main.go (Application Root)"
        direction LR
        Config --> Logger
        Config --> RedisStorage
        Config --> KafkaProducer
        Config --> KafkaConsumer
        Config --> WSManager

        RedisStorage --> OCPP16Processor
        ModelConverter --> OCPP16Processor
        KafkaProducer --> OCPP16Processor

        OCPP16Processor --> Dispatcher
        
        Dispatcher --> WSManager

        WSManager --> CommandHandler
        KafkaConsumer --> CommandHandler
    end