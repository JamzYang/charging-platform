package test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicWebSocketConnection 基础WebSocket连接测试
func TestBasicWebSocketConnection(t *testing.T) {
	// 检查是否在CI环境中，如果是则跳过需要Docker的测试
	if os.Getenv("CI") == "true" && os.Getenv("USE_TESTCONTAINERS") == "false" {
		t.Skip("Skipping test in CI environment without TestContainers")
	}

	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-BASIC-001"

	// 测试Redis连接
	t.Run("Redis Connection", func(t *testing.T) {
		err := env.RedisClient.Ping(env.RedisClient.Context()).Err()
		require.NoError(t, err, "Redis should be accessible")
	})

	// 测试Kafka连接
	t.Run("Kafka Connection", func(t *testing.T) {
		// 发送测试消息
		testMessage := "test-message"
		_, _, err := env.KafkaProducer.SendMessage(&sarama.ProducerMessage{
			Topic: "test-topic",
			Value: sarama.StringEncoder(testMessage),
		})
		require.NoError(t, err, "Should be able to send message to Kafka")
	})

	// 测试WebSocket客户端创建
	t.Run("WebSocket Client Creation", func(t *testing.T) {
		// 创建WebSocket客户端
		wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
		if err != nil {
			t.Skipf("WebSocket connection failed (gateway may not be running): %v", err)
		}
		defer wsClient.Close()

		// 发送简单的心跳消息
		heartbeatMessage, err := utils.CreateOCPPMessage(2, "test-heartbeat", "Heartbeat", map[string]interface{}{})
		require.NoError(t, err)

		err = wsClient.SendMessage(heartbeatMessage)
		require.NoError(t, err, "Should be able to send message")

		// 尝试接收响应（可能会超时，这是正常的）
		response, err := wsClient.ReceiveMessage(3 * time.Second)
		if err == nil {
			t.Logf("Received response: %s", string(response))
		} else {
			t.Logf("No response received (expected in test environment): %v", err)
		}
	})

	// 测试OCPP消息创建
	t.Run("OCPP Message Creation", func(t *testing.T) {
		// 测试BootNotification消息
		payload := map[string]interface{}{
			"chargePointVendor": "TestVendor",
			"chargePointModel":  "TestModel",
		}

		message, err := utils.CreateOCPPMessage(2, "test-001", "BootNotification", payload)
		require.NoError(t, err)
		assert.NotEmpty(t, message)

		// 验证JSON格式
		var parsed []interface{}
		err = json.Unmarshal(message, &parsed)
		require.NoError(t, err)
		assert.Len(t, parsed, 4)
		assert.Equal(t, float64(2), parsed[0])         // 消息类型
		assert.Equal(t, "test-001", parsed[1])         // 消息ID
		assert.Equal(t, "BootNotification", parsed[2]) // Action
	})

	// 测试断言工具
	t.Run("Assertion Tools", func(t *testing.T) {
		// 创建测试消息
		message, err := utils.CreateOCPPMessage(2, "test-001", "BootNotification", map[string]interface{}{
			"chargePointVendor": "TestVendor",
			"chargePointModel":  "TestModel",
		})
		require.NoError(t, err)

		// 测试OCPP消息断言
		utils.AssertOCPPMessage(t, message, 2, "BootNotification")
	})

	t.Log("Basic WebSocket test completed successfully")
}

// TestEnvironmentModes 测试不同的环境模式
func TestEnvironmentModes(t *testing.T) {
	// 测试TestContainers模式
	t.Run("TestContainers Mode", func(t *testing.T) {
		// 强制使用TestContainers
		os.Setenv("USE_TESTCONTAINERS", "true")
		defer os.Unsetenv("USE_TESTCONTAINERS")

		env := utils.SetupTestEnvironment(t)
		defer env.Cleanup()

		// 验证Redis连接
		err := env.RedisClient.Ping(env.RedisClient.Context()).Err()
		require.NoError(t, err, "Redis should be accessible in TestContainers mode")
	})

	// 测试外部服务模式
	t.Run("External Services Mode", func(t *testing.T) {
		// 强制使用外部服务
		os.Setenv("USE_TESTCONTAINERS", "false")
		defer os.Unsetenv("USE_TESTCONTAINERS")

		// 设置外部服务地址
		os.Setenv("REDIS_ADDR", "localhost:6380")
		os.Setenv("KAFKA_BROKERS", "localhost:9093")
		defer func() {
			os.Unsetenv("REDIS_ADDR")
			os.Unsetenv("KAFKA_BROKERS")
		}()

		env := utils.SetupTestEnvironment(t)
		defer env.Cleanup()

		// 这个测试可能会跳过，如果外部服务不可用
		t.Log("External services mode test completed")
	})
}
