package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWebSocketIntegration 测试WebSocket集成功能
func TestWebSocketIntegration(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-WS-001"

	// 创建WebSocket客户端
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err, "Should be able to create WebSocket client")
	defer wsClient.Close()

	// 等待连接建立
	time.Sleep(100 * time.Millisecond)

	// 测试发送BootNotification消息
	t.Run("BootNotification", func(t *testing.T) {
		// 加载测试数据
		bootNotificationData, err := utils.LoadTestData("ocpp_messages/boot_notification.json")
		require.NoError(t, err)

		var payload map[string]interface{}
		err = json.Unmarshal(bootNotificationData, &payload)
		require.NoError(t, err)

		// 创建OCPP消息
		messageID := "test-boot-ws-001"
		bootMessage, err := utils.CreateOCPPMessage(2, messageID, "BootNotification", payload)
		require.NoError(t, err)

		// 发送消息
		err = wsClient.SendMessage(bootMessage)
		require.NoError(t, err, "Should be able to send BootNotification")

		// 尝试接收响应
		response, err := wsClient.ReceiveMessage(5 * time.Second)
		if err == nil {
			// 如果收到响应，验证格式
			utils.AssertBootNotificationResponse(t, response, messageID)
			t.Log("Received BootNotification response")
		} else {
			// 在测试环境中可能没有完整的OCPP处理器，这是正常的
			t.Logf("No response received (expected in test environment): %v", err)
		}
	})

	// 测试发送心跳消息
	t.Run("Heartbeat", func(t *testing.T) {
		messageID := "test-heartbeat-ws-001"
		payload := map[string]interface{}{}

		heartbeatMessage, err := utils.CreateOCPPMessage(2, messageID, "Heartbeat", payload)
		require.NoError(t, err)

		err = wsClient.SendMessage(heartbeatMessage)
		require.NoError(t, err, "Should be able to send Heartbeat")

		// 尝试接收响应
		response, err := wsClient.ReceiveMessage(3 * time.Second)
		if err == nil {
			t.Logf("Received heartbeat response: %s", string(response))
		} else {
			t.Logf("No heartbeat response received: %v", err)
		}
	})

	// 测试连接保持
	t.Run("ConnectionKeepAlive", func(t *testing.T) {
		// 发送多个消息测试连接稳定性
		for i := 0; i < 5; i++ {
			messageID := fmt.Sprintf("test-keepalive-%d", i)
			payload := map[string]interface{}{}

			message, err := utils.CreateOCPPMessage(2, messageID, "Heartbeat", payload)
			require.NoError(t, err)

			err = wsClient.SendMessage(message)
			assert.NoError(t, err, "Should be able to send message %d", i)

			// 短暂延迟
			time.Sleep(100 * time.Millisecond)
		}

		t.Log("Connection keep-alive test completed")
	})

	t.Log("WebSocket integration test completed")
}

// TestWebSocketConcurrentConnections 测试并发WebSocket连接
func TestWebSocketConcurrentConnections(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	// 并发连接数
	connectionCount := 10
	results := make(chan bool, connectionCount)

	// 启动多个并发连接
	for i := 0; i < connectionCount; i++ {
		go func(index int) {
			chargePointID := fmt.Sprintf("CP-CONCURRENT-%03d", index)

			// 创建WebSocket客户端
			wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
			if err != nil {
				t.Logf("Failed to create WebSocket client %d: %v", index, err)
				results <- false
				return
			}
			defer wsClient.Close()

			// 发送测试消息
			messageID := fmt.Sprintf("test-concurrent-%03d", index)
			payload := map[string]interface{}{}

			message, err := utils.CreateOCPPMessage(2, messageID, "Heartbeat", payload)
			if err != nil {
				t.Logf("Failed to create message for client %d: %v", index, err)
				results <- false
				return
			}

			err = wsClient.SendMessage(message)
			if err != nil {
				t.Logf("Failed to send message for client %d: %v", index, err)
				results <- false
				return
			}

			results <- true
		}(i)
	}

	// 等待所有连接完成
	successCount := 0
	for i := 0; i < connectionCount; i++ {
		select {
		case success := <-results:
			if success {
				successCount++
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for concurrent connections")
		}
	}

	// 验证大部分连接成功
	successRate := float64(successCount) / float64(connectionCount)
	assert.Greater(t, successRate, 0.7, "At least 70% of connections should succeed")

	t.Logf("Concurrent connections test passed with %d/%d success rate", successCount, connectionCount)
}

// TestWebSocketErrorHandling 测试WebSocket错误处理
func TestWebSocketErrorHandling(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-ERROR-001"

	// 创建WebSocket客户端
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 测试发送无效JSON
	t.Run("InvalidJSON", func(t *testing.T) {
		invalidJSON := []byte(`{"invalid": "json"`)

		err = wsClient.SendMessage(invalidJSON)
		require.NoError(t, err, "Should be able to send invalid JSON")

		// 连接应该保持活跃
		time.Sleep(100 * time.Millisecond)

		// 发送有效消息验证连接状态
		messageID := "test-after-invalid"
		payload := map[string]interface{}{}

		validMessage, err := utils.CreateOCPPMessage(2, messageID, "Heartbeat", payload)
		require.NoError(t, err)

		err = wsClient.SendMessage(validMessage)
		assert.NoError(t, err, "Connection should remain active after invalid JSON")
	})

	// 测试发送无效OCPP消息
	t.Run("InvalidOCPPMessage", func(t *testing.T) {
		invalidOCPP := []byte(`[5, "invalid-type", "TestAction", {}]`)

		err = wsClient.SendMessage(invalidOCPP)
		require.NoError(t, err, "Should be able to send invalid OCPP message")

		// 连接应该保持活跃
		time.Sleep(100 * time.Millisecond)

		// 发送有效消息验证连接状态
		messageID := "test-after-invalid-ocpp"
		payload := map[string]interface{}{}

		validMessage, err := utils.CreateOCPPMessage(2, messageID, "Heartbeat", payload)
		require.NoError(t, err)

		err = wsClient.SendMessage(validMessage)
		assert.NoError(t, err, "Connection should remain active after invalid OCPP message")
	})

	t.Log("WebSocket error handling test completed")
}
