package upstream

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/test/utils"
	"github.com/stretchr/testify/require"
)

// TestTC_INT_03_StatusNotification 测试用例TC-INT-03: StatusNotification验证状态变更上报
func TestTC_INT_03_StatusNotification(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-001"

	// 创建Kafka消费者来监听上行事件
	partitionConsumer, err := env.KafkaConsumer.ConsumePartition("ocpp-events-up-test", 0, sarama.OffsetNewest)
	require.NoError(t, err)
	defer partitionConsumer.Close()

	// 创建WebSocket客户端
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 前置条件：先完成BootNotification流程
	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	// 加载StatusNotification测试数据
	statusNotificationData, err := utils.LoadTestData("ocpp_messages/status_notification.json")
	require.NoError(t, err)

	var payload map[string]interface{}
	err = json.Unmarshal(statusNotificationData, &payload)
	require.NoError(t, err)

	// 创建StatusNotification OCPP消息
	messageID := "test-status-001"
	statusMessage, err := utils.CreateOCPPMessage(2, messageID, "StatusNotification", payload)
	require.NoError(t, err)

	// 发送StatusNotification请求
	err = wsClient.SendMessage(statusMessage)
	require.NoError(t, err)

	// 步骤1: 验证桩收到StatusNotification.conf响应
	response, err := wsClient.ReceiveMessage(5 * time.Second)
	require.NoError(t, err)

	utils.AssertStatusNotificationResponse(t, response, messageID)

	// 步骤2: 验证后端模拟器在Kafka中消费到一条DeviceStatusEvent事件
	utils.AssertEventuallyTrue(t, func() bool {
		select {
		case kafkaMessage := <-partitionConsumer.Messages():
			// 验证事件内容
			event := utils.AssertKafkaMessage(t, kafkaMessage.Value, "device.status")
			
			// 检查充电桩ID
			chargePointIDFromEvent, ok := event["charge_point_id"].(string)
			require.True(t, ok, "Charge point ID should be a string")
			require.Equal(t, chargePointID, chargePointIDFromEvent, "Charge point ID mismatch")
			
			// 检查载荷
			require.Contains(t, event, "payload", "Event should have payload")
			eventPayload, ok := event["payload"].(map[string]interface{})
			require.True(t, ok, "Payload should be an object")
			
			// 检查状态信息
			require.Contains(t, eventPayload, "connector_id", "Payload should contain connector ID")
			require.Contains(t, eventPayload, "status", "Payload should contain status")
			require.Contains(t, eventPayload, "error_code", "Payload should contain error code")
			
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}, 5*time.Second, "Should receive DeviceStatusEvent in Kafka")

	t.Log("TC-INT-03 StatusNotification test passed")
}

// TestTC_INT_03_StatusNotification_StateTransitions 测试状态转换序列
func TestTC_INT_03_StatusNotification_StateTransitions(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-005"

	// 创建Kafka消费者
	partitionConsumer, err := env.KafkaConsumer.ConsumePartition("ocpp-events-up-test", 0, sarama.OffsetNewest)
	require.NoError(t, err)
	defer partitionConsumer.Close()

	// 创建WebSocket客户端
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 前置条件：完成BootNotification
	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	// 定义状态转换序列
	statusSequence := []string{
		"Available",
		"Preparing", 
		"Charging",
		"SuspendedEVSE",
		"Finishing",
		"Available",
	}

	// 逐个发送状态变更
	for i, status := range statusSequence {
		payload := map[string]interface{}{
			"connectorId": 1,
			"errorCode":   "NoError",
			"status":      status,
			"timestamp":   time.Now().Format(time.RFC3339),
		}

		messageID := fmt.Sprintf("test-status-seq-%03d", i)
		statusMessage, err := utils.CreateOCPPMessage(2, messageID, "StatusNotification", payload)
		require.NoError(t, err)

		// 发送状态通知
		err = wsClient.SendMessage(statusMessage)
		require.NoError(t, err)

		// 验证响应
		response, err := wsClient.ReceiveMessage(5 * time.Second)
		require.NoError(t, err)
		utils.AssertStatusNotificationResponse(t, response, messageID)

		// 验证Kafka事件
		utils.AssertEventuallyTrue(t, func() bool {
			select {
			case kafkaMessage := <-partitionConsumer.Messages():
				event := utils.AssertKafkaMessage(t, kafkaMessage.Value, "device.status")
				
				// 验证状态
				eventPayload := event["payload"].(map[string]interface{})
				eventStatus := eventPayload["status"].(string)
				return eventStatus == status
			case <-time.After(100 * time.Millisecond):
				return false
			}
		}, 3*time.Second, fmt.Sprintf("Should receive status event for %s", status))

		// 短暂延迟以避免消息过于密集
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("StatusNotification state transitions test passed")
}

// TestTC_INT_03_StatusNotification_ErrorStates 测试错误状态上报
func TestTC_INT_03_StatusNotification_ErrorStates(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-006"

	// 创建Kafka消费者
	partitionConsumer, err := env.KafkaConsumer.ConsumePartition("ocpp-events-up-test", 0, sarama.OffsetNewest)
	require.NoError(t, err)
	defer partitionConsumer.Close()

	// 创建WebSocket客户端
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 前置条件：完成BootNotification
	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	// 定义错误状态序列
	errorStates := []map[string]interface{}{
		{
			"connectorId": 1,
			"errorCode":   "ConnectorLockFailure",
			"status":      "Faulted",
			"timestamp":   time.Now().Format(time.RFC3339),
			"info":        "Connector lock mechanism failed",
		},
		{
			"connectorId": 1,
			"errorCode":   "GroundFailure",
			"status":      "Faulted",
			"timestamp":   time.Now().Format(time.RFC3339),
			"info":        "Ground fault detected",
		},
		{
			"connectorId": 1,
			"errorCode":   "NoError",
			"status":      "Available",
			"timestamp":   time.Now().Format(time.RFC3339),
			"info":        "Error cleared",
		},
	}

	// 逐个发送错误状态
	for i, errorState := range errorStates {
		messageID := fmt.Sprintf("test-error-status-%03d", i)
		statusMessage, err := utils.CreateOCPPMessage(2, messageID, "StatusNotification", errorState)
		require.NoError(t, err)

		// 发送状态通知
		err = wsClient.SendMessage(statusMessage)
		require.NoError(t, err)

		// 验证响应
		response, err := wsClient.ReceiveMessage(5 * time.Second)
		require.NoError(t, err)
		utils.AssertStatusNotificationResponse(t, response, messageID)

		// 验证Kafka事件
		utils.AssertEventuallyTrue(t, func() bool {
			select {
			case kafkaMessage := <-partitionConsumer.Messages():
				event := utils.AssertKafkaMessage(t, kafkaMessage.Value, "device.status")
				
				// 验证错误代码和状态
				eventPayload := event["payload"].(map[string]interface{})
				eventErrorCode := eventPayload["error_code"].(string)
				eventStatus := eventPayload["status"].(string)
				
				return eventErrorCode == errorState["errorCode"] && eventStatus == errorState["status"]
			case <-time.After(100 * time.Millisecond):
				return false
			}
		}, 3*time.Second, fmt.Sprintf("Should receive error status event for %s", errorState["errorCode"]))

		time.Sleep(100 * time.Millisecond)
	}

	t.Log("StatusNotification error states test passed")
}
