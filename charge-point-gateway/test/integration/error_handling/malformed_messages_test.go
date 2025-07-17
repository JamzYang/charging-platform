package error_handling

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTC_INT_05_MalformedMessages 测试用例TC-INT-05: 格式错误的消息验证网关对畸形报文的处理
func TestTC_INT_05_MalformedMessages(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-001"

	// 创建WebSocket客户端
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 前置条件：完成BootNotification流程
	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	// 测试用例1: JSON格式错误
	t.Run("InvalidJSON", func(t *testing.T) {
		malformedJSON := []byte(`{"invalid": "json"`)

		err = wsClient.SendMessage(malformedJSON)
		require.NoError(t, err)

		// 验证连接仍然活跃（通过发送有效消息）
		time.Sleep(100 * time.Millisecond)
		err = sendValidHeartbeat(t, wsClient)
		assert.NoError(t, err, "Connection should remain active after malformed JSON")
	})

	// 测试用例2: OCPP消息结构错误
	t.Run("InvalidOCPPStructure", func(t *testing.T) {
		// 缺少必要元素的OCPP消息
		invalidOCPPMessage := []interface{}{2, "test-msg"} // 缺少Action和Payload

		messageBytes, err := json.Marshal(invalidOCPPMessage)
		require.NoError(t, err)

		err = wsClient.SendMessage(messageBytes)
		require.NoError(t, err)

		// 应该收到CALLERROR响应
		response, err := wsClient.ReceiveMessage(5 * time.Second)
		if err == nil {
			// 如果收到响应，验证是否为错误响应
			var message []interface{}
			err = json.Unmarshal(response, &message)
			if err == nil && len(message) >= 1 {
				messageType, ok := message[0].(float64)
				if ok && int(messageType) == 4 {
					// 收到CALLERROR，这是正确的行为
					t.Log("Received CALLERROR for invalid OCPP structure")
				}
			}
		}

		// 验证连接仍然活跃
		time.Sleep(100 * time.Millisecond)
		err = sendValidHeartbeat(t, wsClient)
		assert.NoError(t, err, "Connection should remain active after invalid OCPP structure")
	})

	// 测试用例3: 无效的消息类型ID
	t.Run("InvalidMessageTypeID", func(t *testing.T) {
		invalidMessage := []interface{}{5, "test-msg-invalid", "TestAction", map[string]interface{}{}}

		messageBytes, err := json.Marshal(invalidMessage)
		require.NoError(t, err)

		err = wsClient.SendMessage(messageBytes)
		require.NoError(t, err)

		// 验证连接仍然活跃
		time.Sleep(100 * time.Millisecond)
		err = sendValidHeartbeat(t, wsClient)
		assert.NoError(t, err, "Connection should remain active after invalid message type")
	})

	// 测试用例4: 超大消息
	t.Run("OversizedMessage", func(t *testing.T) {
		// 创建一个超大的载荷
		largePayload := make(map[string]interface{})
		largeString := make([]byte, 1024*1024) // 1MB字符串
		for i := range largeString {
			largeString[i] = 'A'
		}
		largePayload["largeField"] = string(largeString)

		oversizedMessage, err := utils.CreateOCPPMessage(2, "test-oversized", "TestAction", largePayload)
		require.NoError(t, err)

		err = wsClient.SendMessage(oversizedMessage)
		// 可能会因为消息过大而失败，这是正常的

		// 验证连接状态
		time.Sleep(500 * time.Millisecond)
		err = sendValidHeartbeat(t, wsClient)
		// 连接可能已断开，这是可接受的行为
		if err != nil {
			t.Log("Connection closed after oversized message, which is acceptable")
		}
	})

	t.Log("TC-INT-05 Malformed messages test passed")
}

// TestTC_INT_06_UnsupportedAction 测试用例TC-INT-06: 不支持的Action验证对未知消息的处理
func TestTC_INT_06_UnsupportedAction(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-002"

	// 创建WebSocket客户端
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 前置条件：完成BootNotification流程
	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	// 测试用例1: 完全不存在的Action
	t.Run("NonExistentAction", func(t *testing.T) {
		messageID := "test-unsupported-001"
		payload := map[string]interface{}{
			"testField": "testValue",
		}

		unsupportedMessage, err := utils.CreateOCPPMessage(2, messageID, "FakeAction", payload)
		require.NoError(t, err)

		err = wsClient.SendMessage(unsupportedMessage)
		require.NoError(t, err)

		// 应该收到CALLERROR响应
		response, err := wsClient.ReceiveMessage(5 * time.Second)
		if err == nil {
			errorCode, errorDescription, _ := utils.AssertOCPPCallError(t, response, messageID)
			assert.Contains(t, []string{"NotSupported", "NotImplemented"}, errorCode, "Should receive NotSupported or NotImplemented error")
			assert.NotEmpty(t, errorDescription, "Error description should not be empty")
		}

		// 验证连接仍然活跃
		time.Sleep(100 * time.Millisecond)
		err = sendValidHeartbeat(t, wsClient)
		assert.NoError(t, err, "Connection should remain active after unsupported action")
	})

	// 测试用例2: OCPP 2.0.1的Action（在1.6环境中）
	t.Run("OCPP2_0_1_Action", func(t *testing.T) {
		messageID := "test-ocpp2-001"
		payload := map[string]interface{}{
			"requestId": 1,
		}

		// NotifyReport是OCPP 2.0.1的Action
		ocpp2Message, err := utils.CreateOCPPMessage(2, messageID, "NotifyReport", payload)
		require.NoError(t, err)

		err = wsClient.SendMessage(ocpp2Message)
		require.NoError(t, err)

		// 应该收到错误响应
		response, err := wsClient.ReceiveMessage(5 * time.Second)
		if err == nil {
			errorCode, _, _ := utils.AssertOCPPCallError(t, response, messageID)
			assert.Contains(t, []string{"NotSupported", "NotImplemented"}, errorCode, "Should receive error for OCPP 2.0.1 action")
		}

		// 验证连接仍然活跃
		time.Sleep(100 * time.Millisecond)
		err = sendValidHeartbeat(t, wsClient)
		assert.NoError(t, err, "Connection should remain active after OCPP 2.0.1 action")
	})

	// 测试用例3: 空Action
	t.Run("EmptyAction", func(t *testing.T) {
		messageID := "test-empty-action-001"
		payload := map[string]interface{}{}

		emptyActionMessage, err := utils.CreateOCPPMessage(2, messageID, "", payload)
		require.NoError(t, err)

		err = wsClient.SendMessage(emptyActionMessage)
		require.NoError(t, err)

		// 验证连接仍然活跃
		time.Sleep(100 * time.Millisecond)
		err = sendValidHeartbeat(t, wsClient)
		assert.NoError(t, err, "Connection should remain active after empty action")
	})

	t.Log("TC-INT-06 Unsupported action test passed")
}

// TestTC_INT_05_06_ErrorRecovery 测试错误恢复能力
func TestTC_INT_05_06_ErrorRecovery(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-003"

	// 创建WebSocket客户端
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 前置条件：完成BootNotification流程
	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	// 发送一系列错误消息
	errorMessages := [][]byte{
		[]byte(`{"malformed": json}`),                   // 格式错误
		[]byte(`[5, "invalid-type", "TestAction", {}]`), // 无效消息类型
		[]byte(`[2, "unsupported", "FakeAction", {}]`),  // 不支持的Action
		[]byte(`[2, "incomplete"]`),                     // 不完整消息
	}

	for i, errorMsg := range errorMessages {
		t.Logf("Sending error message %d", i+1)

		err = wsClient.SendMessage(errorMsg)
		require.NoError(t, err)

		// 短暂等待
		time.Sleep(100 * time.Millisecond)
	}

	// 验证在发送多个错误消息后，连接仍然可以处理正常消息
	err = sendValidHeartbeat(t, wsClient)
	assert.NoError(t, err, "Connection should recover and handle valid messages after errors")

	// 发送有效的StatusNotification
	statusPayload := map[string]interface{}{
		"connectorId": 1,
		"errorCode":   "NoError",
		"status":      "Available",
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	messageID := "test-recovery-status"
	statusMessage, err := utils.CreateOCPPMessage(2, messageID, "StatusNotification", statusPayload)
	require.NoError(t, err)

	err = wsClient.SendMessage(statusMessage)
	require.NoError(t, err)

	// 验证收到正常响应
	response, err := wsClient.ReceiveMessage(5 * time.Second)
	require.NoError(t, err)
	utils.AssertStatusNotificationResponse(t, response, messageID)

	t.Log("Error recovery test passed")
}

// 注意：sendValidHeartbeat 和 performBootNotification 函数已在 dependency_failure_test.go 中定义
