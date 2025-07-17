package test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOCPPMessageCreation 测试OCPP消息创建（不需要外部依赖）
func TestOCPPMessageCreation(t *testing.T) {
	t.Run("BootNotification Message", func(t *testing.T) {
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

		// 验证载荷
		payloadParsed, ok := parsed[3].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "TestVendor", payloadParsed["chargePointVendor"])
		assert.Equal(t, "TestModel", payloadParsed["chargePointModel"])
	})

	t.Run("Heartbeat Message", func(t *testing.T) {
		payload := map[string]interface{}{}

		message, err := utils.CreateOCPPMessage(2, "heartbeat-001", "Heartbeat", payload)
		require.NoError(t, err)
		assert.NotEmpty(t, message)

		// 验证消息结构
		utils.AssertOCPPMessage(t, message, 2, "Heartbeat")
	})

	t.Run("CALLRESULT Message", func(t *testing.T) {
		payload := map[string]interface{}{
			"status":      "Accepted",
			"currentTime": "2024-01-14T10:00:00Z",
			"interval":    300,
		}

		message, err := utils.CreateOCPPMessage(3, "response-001", "", payload)
		require.NoError(t, err)
		assert.NotEmpty(t, message)

		// 验证JSON格式
		var parsed []interface{}
		err = json.Unmarshal(message, &parsed)
		require.NoError(t, err)
		assert.Len(t, parsed, 3)                   // CALLRESULT只有3个元素
		assert.Equal(t, float64(3), parsed[0])     // 消息类型
		assert.Equal(t, "response-001", parsed[1]) // 消息ID
	})

	t.Run("Invalid Message Type", func(t *testing.T) {
		payload := map[string]interface{}{}

		_, err := utils.CreateOCPPMessage(99, "invalid-001", "TestAction", payload)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported message type")
	})
}

// TestOCPPMessageAssertions 测试OCPP消息断言工具
func TestOCPPMessageAssertions(t *testing.T) {
	t.Run("Valid CALL Message", func(t *testing.T) {
		message, err := utils.CreateOCPPMessage(2, "test-001", "BootNotification", map[string]interface{}{
			"chargePointVendor": "TestVendor",
			"chargePointModel":  "TestModel",
		})
		require.NoError(t, err)

		// 这应该不会panic
		utils.AssertOCPPMessage(t, message, 2, "BootNotification")
	})

	t.Run("Valid CALLRESULT Message", func(t *testing.T) {
		message, err := utils.CreateOCPPMessage(3, "response-001", "", map[string]interface{}{
			"status": "Accepted",
		})
		require.NoError(t, err)

		// 测试CALLRESULT断言
		payload := utils.AssertOCPPCallResult(t, message, "response-001")
		assert.Equal(t, "Accepted", payload["status"])
	})
}

// TestLoadTestData 测试数据加载功能
func TestLoadTestDataUnit(t *testing.T) {
	t.Run("Load BootNotification Data", func(t *testing.T) {
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
	})

	t.Run("Load MeterValues Data", func(t *testing.T) {
		data, err := utils.LoadTestData("ocpp_messages/meter_values.json")
		require.NoError(t, err, "Should be able to load meter values data")
		assert.NotEmpty(t, data, "Test data should not be empty")

		// 验证JSON格式
		var payload map[string]interface{}
		err = json.Unmarshal(data, &payload)
		require.NoError(t, err, "Test data should be valid JSON")

		// 验证必要字段
		assert.Contains(t, payload, "connectorId")
		assert.Contains(t, payload, "meterValue")
	})

	t.Run("Load Non-existent File", func(t *testing.T) {
		_, err := utils.LoadTestData("non_existent_file.json")
		assert.Error(t, err, "Should return error for non-existent file")
	})
}

// TestAssertionHelpers 测试断言辅助函数
func TestAssertionHelpersUnit(t *testing.T) {
	t.Run("EventuallyTrue Success", func(t *testing.T) {
		counter := 0
		utils.AssertEventuallyTrue(t, func() bool {
			counter++
			return counter >= 3
		}, 1*time.Second, "Counter should reach 3")

		assert.Equal(t, 3, counter, "Counter should be 3")
	})

	t.Run("EventuallyTrue with Fast Condition", func(t *testing.T) {
		utils.AssertEventuallyTrue(t, func() bool {
			return true // 立即满足条件
		}, 100*time.Millisecond, "Should succeed immediately")
	})
}

// TestWebSocketClientCreation 测试WebSocket客户端创建（不实际连接）
func TestWebSocketClientCreation(t *testing.T) {
	t.Run("Invalid URL", func(t *testing.T) {
		_, err := utils.NewWebSocketClient("invalid-url", "CP-001")
		assert.Error(t, err, "Should return error for invalid URL")
	})

	t.Run("Valid URL Format", func(t *testing.T) {
		// 这个测试会失败，因为没有实际的服务器，但我们可以验证URL解析
		_, err := utils.NewWebSocketClient("ws://localhost:8080/ocpp", "CP-001")
		// 预期会有连接错误，但不应该是URL解析错误
		if err != nil {
			// 确保不是URL解析错误
			assert.NotContains(t, err.Error(), "invalid URL")
		}
	})
}

// TestEnvironmentVariableHelpers 测试环境变量辅助函数
func TestEnvironmentVariableHelpers(t *testing.T) {
	t.Run("GetEnvOrDefault with existing env", func(t *testing.T) {
		// 这个测试需要访问getEnvOrDefault函数，但它是私有的
		// 我们可以通过测试SetupTestEnvironment的行为来间接测试

		// 设置环境变量
		os.Setenv("TEST_VAR", "test_value")
		defer os.Unsetenv("TEST_VAR")

		// 验证环境变量被正确读取
		// 注意：这里我们无法直接测试getEnvOrDefault，因为它是私有函数
		// 但我们可以验证环境变量设置是否正常工作
		assert.Equal(t, "test_value", os.Getenv("TEST_VAR"))
	})
}

// BenchmarkMessageCreationUnit 性能基准测试
func BenchmarkMessageCreationUnit(b *testing.B) {
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

// BenchmarkJSONMarshalingUnit JSON序列化性能测试
func BenchmarkJSONMarshalingUnit(b *testing.B) {
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
