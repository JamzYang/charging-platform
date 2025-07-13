package utils

import (
	"context"
	"encoding/json"
	"fmt"
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
)

// TestEnvironment 测试环境配置
type TestEnvironment struct {
	RedisContainer  testcontainers.Container
	KafkaContainer  testcontainers.Container
	RedisClient     *redis.Client
	KafkaProducer   sarama.SyncProducer
	KafkaConsumer   sarama.Consumer
	GatewayURL      string
	CleanupFuncs    []func()
}

// SetupTestEnvironment 设置测试环境
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	ctx := context.Background()
	env := &TestEnvironment{
		CleanupFuncs: make([]func(), 0),
	}

	// 启动Redis容器
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections"),
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

	// 启动Kafka容器
	kafkaContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "confluentinc/cp-kafka:latest",
			ExposedPorts: []string{"9092/tcp"},
			Env: map[string]string{
				"KAFKA_BROKER_ID":                 "1",
				"KAFKA_ZOOKEEPER_CONNECT":         "zookeeper:2181",
				"KAFKA_ADVERTISED_LISTENERS":      "PLAINTEXT://localhost:9092",
				"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
			},
			WaitingFor: wait.ForLog("started (kafka.server.KafkaServer)"),
		},
		Started: true,
	})
	require.NoError(t, err)
	env.KafkaContainer = kafkaContainer
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		kafkaContainer.Terminate(ctx)
	})

	// 获取Kafka连接信息
	kafkaHost, err := kafkaContainer.Host(ctx)
	require.NoError(t, err)
	kafkaPort, err := kafkaContainer.MappedPort(ctx, "9092")
	require.NoError(t, err)

	// 创建Kafka生产者
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer([]string{fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port())}, config)
	require.NoError(t, err)
	env.KafkaProducer = producer
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		producer.Close()
	})

	// 创建Kafka消费者
	consumer, err := sarama.NewConsumer([]string{fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port())}, nil)
	require.NoError(t, err)
	env.KafkaConsumer = consumer
	env.CleanupFuncs = append(env.CleanupFuncs, func() {
		consumer.Close()
	})

	// 设置网关URL（假设网关在本地运行）
	env.GatewayURL = "ws://localhost:8080/ocpp"

	return env
}

// Cleanup 清理测试环境
func (env *TestEnvironment) Cleanup() {
	for i := len(env.CleanupFuncs) - 1; i >= 0; i-- {
		env.CleanupFuncs[i]()
	}
}

// WebSocketClient WebSocket客户端封装
type WebSocketClient struct {
	conn         *websocket.Conn
	chargePointID string
	messageQueue chan []byte
	errorQueue   chan error
	done         chan struct{}
}

// NewWebSocketClient 创建WebSocket客户端
func NewWebSocketClient(gatewayURL, chargePointID string) (*WebSocketClient, error) {
	u, err := url.Parse(gatewayURL)
	if err != nil {
		return nil, err
	}

	// 添加充电桩ID到URL路径
	u.Path = fmt.Sprintf("%s/%s", u.Path, chargePointID)

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	client := &WebSocketClient{
		conn:         conn,
		chargePointID: chargePointID,
		messageQueue: make(chan []byte, 100),
		errorQueue:   make(chan error, 10),
		done:         make(chan struct{}),
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
