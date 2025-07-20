package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/charging-platform/charge-point-gateway/internal/domain/protocol"
)

// TestEnvironment 测试环境配置
type TestEnvironment struct {
	RedisContainer testcontainers.Container
	KafkaContainer testcontainers.Container
	RedisClient    *redis.Client
	KafkaProducer  sarama.SyncProducer
	KafkaConsumer  sarama.Consumer
	GatewayURL     string
	CleanupFuncs   []func()
}

// SetupTestEnvironment 设置测试环境
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	// 检查是否使用TestContainers
	if useTestContainers() {
		return setupWithTestContainers(t)
	}
	return setupWithExternalServices(t)
}

// useTestContainers 检查是否使用TestContainers
func useTestContainers() bool {
	// 仅在明确设置为 "true" 时才使用 TestContainers
	// E2E测试默认不使用，会走 setupWithExternalServices 路径
	return os.Getenv("USE_TESTCONTAINERS") == "true"
}

// setupWithTestContainers 使用TestContainers设置测试环境
func setupWithTestContainers(t *testing.T) *TestEnvironment {
	ctx := context.Background()
	env := &TestEnvironment{
		CleanupFuncs: make([]func(), 0),
	}

	// 创建一个自定义网络，让容器之间可以通过名称相互访问
	networkName := "test-network-" + fmt.Sprintf("%d", time.Now().UnixNano())
	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name: networkName,
		},
	})
	require.NoError(t, err)
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		network.Remove(ctx)
	})

	// 启动Redis容器
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			Networks:     []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"redis-test"},
			},
			WaitingFor: wait.ForLog("Ready to accept connections"),
		},
		Started: true,
	})
	require.NoError(t, err)
	env.RedisContainer = redisContainer
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		redisContainer.Terminate(ctx)
	})

	// 获取Redis连接信息
	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	// 创建Redis客户端
	env.RedisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort.Port()),
	})
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		env.RedisClient.Close()
	})

	// 启动Kafka容器 - 使用简化的配置，避免复杂的网络问题
	// 参考最佳实践：让TestContainers动态分配端口，然后获取实际的连接信息
	kafkaReq := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "confluentinc/cp-kafka:latest",
			ExposedPorts: []string{"9092/tcp"}, // 只声明需要暴露的端口
			Networks:     []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"kafka-test"},
			},
			Env: map[string]string{
				// 使用简化的单节点配置
				"KAFKA_NODE_ID":                          "1",
				"KAFKA_PROCESS_ROLES":                    "broker,controller",
				"KAFKA_CONTROLLER_QUORUM_VOTERS":         "1@localhost:9093",
				"KAFKA_LISTENERS":                        "PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093",
				"KAFKA_ADVERTISED_LISTENERS":             "PLAINTEXT://localhost:9092", // 容器内部使用
				"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT",
				"KAFKA_CONTROLLER_LISTENER_NAMES":        "CONTROLLER",
				"KAFKA_INTER_BROKER_LISTENER_NAME":       "PLAINTEXT",
				"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
				"KAFKA_AUTO_CREATE_TOPICS_ENABLE":        "true",
				"KAFKA_DELETE_TOPIC_ENABLE":              "true",
				"KAFKA_LOG_DIRS":                         "/tmp/kraft-combined-logs",
				"CLUSTER_ID":                             "test-cluster-id-12345",
			},
			WaitingFor: wait.ForLog("Kafka Server started"),
		},
		Started: true,
	}

	kafkaContainer, err := testcontainers.GenericContainer(ctx, kafkaReq)
	require.NoError(t, err)
	env.KafkaContainer = kafkaContainer

	// 确保容器在测试结束后被终止
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		kafkaContainer.Terminate(ctx)
	})

	// ✨ 关键步骤：获取动态映射的端口和主机
	kafkaHost, err := kafkaContainer.Host(ctx)
	require.NoError(t, err)
	kafkaPort, err := kafkaContainer.MappedPort(ctx, "9092")
	require.NoError(t, err)

	// 使用获取到的动态地址和端口构建连接字符串
	kafkaAddr := fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port())
	t.Logf("TestContainers Kafka address: %s", kafkaAddr)

	// 创建Kafka生产者
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer([]string{kafkaAddr}, config)
	require.NoError(t, err)
	env.KafkaProducer = producer
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		producer.Close()
	})

	// 创建Kafka消费者
	consumerConfig := sarama.NewConfig()
	consumerConfig.Consumer.Return.Errors = true
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetNewest

	t.Logf("Creating Kafka consumer with address: %s", kafkaAddr)
	consumer, err := sarama.NewConsumer([]string{kafkaAddr}, consumerConfig)
	require.NoError(t, err)
	env.KafkaConsumer = consumer
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		consumer.Close()
	})

	// 启动网关容器
	// 获取项目根目录（从当前文件位置向上查找）
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	projectRoot, err = filepath.Abs(projectRoot)
	require.NoError(t, err)

	t.Logf("Using project root: %s", projectRoot)

	gatewayContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    projectRoot, // 使用项目根目录
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"8080/tcp", "8081/tcp"},
			Networks:     []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"gateway-test"},
			},
			Env: map[string]string{
				"SERVER_HOST":                  "0.0.0.0",
				"SERVER_PORT":                  "8080",
				"SERVER_WEBSOCKET_PATH":        "/ocpp",
				"REDIS_ADDR":                   "redis-test:6379", // 使用容器网络内部地址
				"KAFKA_BROKERS":                "kafka-test:9092", // 使用容器网络内部地址
				"KAFKA_UPSTREAM_TOPIC":         "ocpp-events-up-test",
				"KAFKA_DOWNSTREAM_TOPIC":       "commands-down-test",
				"KAFKA_CONSUMER_GROUP":         "gateway-consumer-test",
				"LOG_LEVEL":                    "debug",
				"MONITORING_HEALTH_CHECK_PORT": "8081",
				"APP_PROFILE":                  "test", // 确保使用测试配置
			},
			WaitingFor: wait.ForHTTP("/health").WithPort("8081/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		gatewayContainer.Terminate(ctx)
	})

	// 获取网关连接信息
	gatewayHost, err := gatewayContainer.Host(ctx)
	require.NoError(t, err)
	gatewayPort, err := gatewayContainer.MappedPort(ctx, "8080")
	require.NoError(t, err)

	// 设置网关URL
	env.GatewayURL = fmt.Sprintf("ws://%s:%s/ocpp", gatewayHost, gatewayPort.Port())

	return env
}

// setupWithExternalServices 使用外部服务设置测试环境
func setupWithExternalServices(t *testing.T) *TestEnvironment {
	env := &TestEnvironment{
		CleanupFuncs: make([]func(), 0),
	}

	// 从环境变量获取服务地址，或使用Docker Compose测试环境的默认值
	redisAddr := getEnvOrDefault("REDIS_ADDR", "localhost:6379")
	kafkaBrokers := []string{getEnvOrDefault("KAFKA_BROKERS", "localhost:9092")}
	// 优先从环境变量读取GATEWAY_URL，以支持容器化测试客户端
	gatewayURL := getEnvOrDefault("GATEWAY_URL", "ws://localhost:8081/ocpp")

	t.Logf("Using external services - Redis: %s, Kafka: %v, Gateway: %s", redisAddr, kafkaBrokers, gatewayURL)

	// 创建Redis客户端
	env.RedisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		env.RedisClient.Close()
	})

	// 测试Redis连接
	ctx := context.Background()
	if err := env.RedisClient.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available at %s, skipping test: %v", redisAddr, err)
	}

	// 创建Kafka生产者
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3

	producer, err := sarama.NewSyncProducer(kafkaBrokers, config)
	if err != nil {
		t.Skipf("Kafka not available at %v, skipping test: %v", kafkaBrokers, err)
	}
	env.KafkaProducer = producer
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		if producer != nil {
			producer.Close()
		}
	})

	// 创建Kafka消费者
	consumerConfig := sarama.NewConfig()
	consumerConfig.Consumer.Return.Errors = true
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetNewest

	t.Logf("Creating Kafka consumer with brokers: %v", kafkaBrokers)
	consumer, err := sarama.NewConsumer(kafkaBrokers, consumerConfig)
	if err != nil {
		t.Logf("Kafka consumer creation failed: %v", err)
		t.Skipf("Kafka consumer not available at %v, skipping test: %v", kafkaBrokers, err)
	}
	env.KafkaConsumer = consumer
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		if consumer != nil {
			consumer.Close()
		}
	})

	// 设置网关URL
	env.GatewayURL = gatewayURL

	return env
}

// getEnvOrDefault 获取环境变量或返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Cleanup 清理测试环境
func (env *TestEnvironment) Cleanup() {
	for i := len(env.CleanupFuncs) - 1; i >= 0; i-- {
		env.CleanupFuncs[i]()
	}
}

// WebSocketClient WebSocket客户端封装
type WebSocketClient struct {
	conn          *websocket.Conn
	chargePointID string
	messageQueue  chan []byte
	errorQueue    chan error
	done          chan struct{}
}

// NewWebSocketClient 创建WebSocket客户端
func NewWebSocketClient(gatewayURL, chargePointID string) (*WebSocketClient, error) {
	u, err := url.Parse(gatewayURL)
	if err != nil {
		return nil, err
	}

	// 添加充电桩ID到URL路径
	u.Path = fmt.Sprintf("%s/%s", u.Path, chargePointID)

	// 设置OCPP子协议
	headers := make(map[string][]string)
	headers["Sec-WebSocket-Protocol"] = []string{protocol.OCPP_VERSION_1_6}

	// 创建优化的拨号器，强制使用IPv4并优化TCP参数
	dialer := &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			// 强制使用IPv4
			if network == "tcp" {
				network = "tcp4"
			}

			// 创建自定义拨号器，优化TCP参数
			d := &net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}

			conn, err := d.Dial(network, addr)
			if err != nil {
				return nil, err
			}

			// 优化TCP连接参数
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.SetNoDelay(true)
				tcpConn.SetKeepAlive(true)
				tcpConn.SetKeepAlivePeriod(30 * time.Second)
			}

			return conn, nil
		},
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}

	// 添加调试日志
	// fmt.Printf("DEBUG: WebSocket client requesting subprotocol: %s\n", protocol.OCPP_VERSION_1_6)

	conn, _, err := dialer.Dial(u.String(), headers)
	if err != nil {
		return nil, err
	}

	client := &WebSocketClient{
		conn:          conn,
		chargePointID: chargePointID,
		messageQueue:  make(chan []byte, 100),
		errorQueue:    make(chan error, 10),
		done:          make(chan struct{}),
	}

	// 启动消息接收协程
	go client.readMessages()

	return client, nil
}

// readMessages 读取消息协程
func (c *WebSocketClient) readMessages() {
	defer close(c.messageQueue)
	defer close(c.errorQueue)

	for {
		select {
		case <-c.done:
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				c.errorQueue <- err
				return
			}
			c.messageQueue <- message
		}
	}
}

// SendMessage 发送消息
func (c *WebSocketClient) SendMessage(message []byte) error {
	return c.conn.WriteMessage(websocket.TextMessage, message)
}

// ReceiveMessage 接收消息（带超时）
func (c *WebSocketClient) ReceiveMessage(timeout time.Duration) ([]byte, error) {
	select {
	case message := <-c.messageQueue:
		return message, nil
	case err := <-c.errorQueue:
		return nil, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for message")
	}
}

// TryReceiveMessage 尝试接收消息（非阻塞）
func (c *WebSocketClient) TryReceiveMessage() ([]byte, error, bool) {
	select {
	case message := <-c.messageQueue:
		return message, nil, true
	case err := <-c.errorQueue:
		return nil, err, true
	default:
		return nil, nil, false
	}
}

// Close 关闭连接
func (c *WebSocketClient) Close() error {
	close(c.done)
	return c.conn.Close()
}

// LoadTestData 加载测试数据
func LoadTestData(filename string) ([]byte, error) {
	// 获取当前文件的目录
	_, currentFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(filepath.Dir(currentFile))

	filePath := filepath.Join(testDir, "fixtures", filename)
	return os.ReadFile(filePath)
}

// CreateOCPPMessage 创建OCPP消息
func CreateOCPPMessage(messageType int, messageID string, action string, payload interface{}) ([]byte, error) {
	var message []interface{}

	switch messageType {
	case 2: // CALL
		message = []interface{}{messageType, messageID, action, payload}
	case 3: // CALLRESULT
		message = []interface{}{messageType, messageID, payload}
	case 4: // CALLERROR
		message = []interface{}{messageType, messageID, payload}
	default:
		return nil, fmt.Errorf("unsupported message type: %d", messageType)
	}

	return json.Marshal(message)
}

// WaitForCondition 等待条件满足
func WaitForCondition(condition func() bool, timeout time.Duration, interval time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if condition() {
			return nil
		}
		time.Sleep(interval)
	}

	return fmt.Errorf("condition not met within timeout")
}
