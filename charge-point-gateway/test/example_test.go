package test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExample 示例测试，展示如何使用测试工具
func TestExample(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	// 测试Redis连接
	t.Run("Redis Connection", func(t *testing.T) {
		err := env.RedisClient.Ping(env.RedisClient.Context()).Err()
		require.NoError(t, err, "Redis should be accessible")
	})

	// 测试Kafka连接
	t.Run("Kafka Connection", func(t *testing.T) {
		// 发送测试消息
		testMessage := "test-message"
		err := env.KafkaProducer.SendMessage(&sarama.ProducerMessage{
			Topic: "test-topic",
			Value: sarama.StringEncoder(testMessage),
		})
		require.NoError(t, err, "Should be able to send message to Kafka")
	})

	// 测试WebSocket连接
	t.Run("WebSocket Connection", func(t *testing.T) {
		chargePointID := "CP-TEST"
		
		// 创建WebSocket客户端
		wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
		require.NoError(t, err, "Should be able to create WebSocket client")
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

	// 测试断言工具
	t.Run("Assertion Tools", func(t *testing.T) {
		// 测试OCPP消息创建
		message, err := utils.CreateOCPPMessage(2, "test-001", "BootNotification", map[string]interface{}{
			"chargePointVendor": "TestVendor",
			"chargePointModel":  "TestModel",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, message)

		// 测试OCPP消息断言
		utils.AssertOCPPMessage(t, message, 2, "BootNotification")
	})

	t.Log("Example test completed successfully")
}

// TestLoadTestData 测试数据加载
func TestLoadTestData(t *testing.T) {
	// 测试加载BootNotification数据
	data, err := utils.LoadTestData("ocpp_messages/boot_notification.json")
	require.NoError(t, err, "Should be able to load test data")
	assert.NotEmpty(t, data, "Test data should not be empty")

	// 验证JSON格式
	var payload map[string]interface{}
	err = json.Unmarshal(data, &payload)
	require.NoError(t, err, "Test data should be valid JSON")
	
	// 验证必要字段
	assert.Contains(t, payload, "chargePointVendor")
	assert.Contains(t, payload, "chargePointModel")
}

// TestAssertionHelpers 测试断言辅助函数
func TestAssertionHelpers(t *testing.T) {
	// 测试EventuallyTrue断言
	counter := 0
	utils.AssertEventuallyTrue(t, func() bool {
		counter++
		return counter >= 3
	}, 1*time.Second, "Counter should reach 3")

	assert.Equal(t, 3, counter, "Counter should be 3")
}

// BenchmarkMessageCreation 性能基准测试
func BenchmarkMessageCreation(b *testing.B) {
	payload := map[string]interface{}{
		"chargePointVendor": "TestVendor",
		"chargePointModel":  "TestModel",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := utils.CreateOCPPMessage(2, "test-message", "BootNotification", payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSONMarshaling JSON序列化性能测试
func BenchmarkJSONMarshaling(b *testing.B) {
	payload := map[string]interface{}{
		"chargePointVendor": "TestVendor",
		"chargePointModel":  "TestModel",
		"meterValues": []map[string]interface{}{
			{
				"timestamp": "2024-01-14T10:00:00Z",
				"sampledValue": []map[string]interface{}{
					{
						"value":     "1234.56",
						"measurand": "Energy.Active.Import.Register",
						"unit":      "kWh",
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}
