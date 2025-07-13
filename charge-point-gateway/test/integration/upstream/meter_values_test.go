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

// TestTC_INT_02_MeterValues 测试用例TC-INT-02: MeterValues验证计量数据上报
func TestTC_INT_02_MeterValues(t *testing.T) {
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

	// 加载MeterValues测试数据
	meterValuesData, err := utils.LoadTestData("ocpp_messages/meter_values.json")
	require.NoError(t, err)

	var payload map[string]interface{}
	err = json.Unmarshal(meterValuesData, &payload)
	require.NoError(t, err)

	// 创建MeterValues OCPP消息
	messageID := "test-meter-001"
	meterMessage, err := utils.CreateOCPPMessage(2, messageID, "MeterValues", payload)
	require.NoError(t, err)

	// 发送MeterValues请求
	err = wsClient.SendMessage(meterMessage)
	require.NoError(t, err)

	// 步骤1: 验证桩收到MeterValues.conf响应
	response, err := wsClient.ReceiveMessage(5 * time.Second)
	require.NoError(t, err)

	utils.AssertMeterValuesResponse(t, response, messageID)

	// 步骤2: 验证后端模拟器在Kafka中消费到一条MeterValuesEvent事件
	utils.AssertEventuallyTrue(t, func() bool {
		select {
		case kafkaMessage := <-partitionConsumer.Messages():
			// 验证事件内容
			utils.AssertMeterValuesEvent(t, kafkaMessage.Value, chargePointID)
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}, 5*time.Second, "Should receive MeterValuesEvent in Kafka")

	t.Log("TC-INT-02 MeterValues test passed")
}

// TestTC_INT_02_MeterValues_MultipleValues 测试多个计量值的MeterValues
func TestTC_INT_02_MeterValues_MultipleValues(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-003"

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

	// 创建包含多个计量值的MeterValues消息
	payload := map[string]interface{}{
		"connectorId": 1,
		"transactionId": 12345,
		"meterValue": []map[string]interface{}{
			{
				"timestamp": "2024-01-14T10:00:00Z",
				"sampledValue": []map[string]interface{}{
					{
						"value": "1234.56",
						"measurand": "Energy.Active.Import.Register",
						"unit": "kWh",
					},
					{
						"value": "7200",
						"measurand": "Power.Active.Import",
						"unit": "W",
					},
					{
						"value": "230.5",
						"measurand": "Voltage",
						"phase": "L1",
						"unit": "V",
					},
				},
			},
			{
				"timestamp": "2024-01-14T10:01:00Z",
				"sampledValue": []map[string]interface{}{
					{
						"value": "1235.12",
						"measurand": "Energy.Active.Import.Register",
						"unit": "kWh",
					},
				},
			},
		},
	}

	messageID := "test-meter-multiple-001"
	meterMessage, err := utils.CreateOCPPMessage(2, messageID, "MeterValues", payload)
	require.NoError(t, err)

	// 发送MeterValues请求
	err = wsClient.SendMessage(meterMessage)
	require.NoError(t, err)

	// 验证响应
	response, err := wsClient.ReceiveMessage(5 * time.Second)
	require.NoError(t, err)
	utils.AssertMeterValuesResponse(t, response, messageID)

	// 验证Kafka事件
	utils.AssertEventuallyTrue(t, func() bool {
		select {
		case kafkaMessage := <-partitionConsumer.Messages():
			utils.AssertMeterValuesEvent(t, kafkaMessage.Value, chargePointID)
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}, 5*time.Second, "Should receive MeterValuesEvent in Kafka")

	t.Log("MeterValues with multiple values test passed")
}

// TestTC_INT_02_MeterValues_HighFrequency 测试高频MeterValues上报
func TestTC_INT_02_MeterValues_HighFrequency(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-004"

	// 创建WebSocket客户端
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 前置条件：完成BootNotification
	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	// 高频发送MeterValues（每100ms一次，持续5秒）
	messageCount := 50
	successCount := 0

	for i := 0; i < messageCount; i++ {
		payload := map[string]interface{}{
			"connectorId": 1,
			"transactionId": 12345,
			"meterValue": []map[string]interface{}{
				{
					"timestamp": time.Now().Format(time.RFC3339),
					"sampledValue": []map[string]interface{}{
						{
							"value": "1234.56",
							"measurand": "Energy.Active.Import.Register",
							"unit": "kWh",
						},
					},
				},
			},
		}

		messageID := fmt.Sprintf("test-meter-freq-%03d", i)
		meterMessage, err := utils.CreateOCPPMessage(2, messageID, "MeterValues", payload)
		require.NoError(t, err)

		// 发送消息
		err = wsClient.SendMessage(meterMessage)
		require.NoError(t, err)

		// 尝试接收响应（非阻塞）
		select {
		case response := <-wsClient.ReceiveMessage(1 * time.Second):
			utils.AssertMeterValuesResponse(t, response, messageID)
			successCount++
		case <-time.After(1 * time.Second):
			t.Logf("Timeout waiting for response to message %d", i)
		}

		time.Sleep(100 * time.Millisecond)
	}

	// 验证大部分消息都成功处理
	successRate := float64(successCount) / float64(messageCount)
	require.Greater(t, successRate, 0.8, "Success rate should be > 80%")

	t.Logf("High frequency MeterValues test passed with %d/%d success rate", successCount, messageCount)
}

// performBootNotification 执行BootNotification流程的辅助函数
func performBootNotification(t *testing.T, wsClient *utils.WebSocketClient, chargePointID string) error {
	// 加载BootNotification数据
	bootNotificationData, err := utils.LoadTestData("ocpp_messages/boot_notification.json")
	if err != nil {
		return err
	}

	var payload map[string]interface{}
	err = json.Unmarshal(bootNotificationData, &payload)
	if err != nil {
		return err
	}

	// 创建并发送BootNotification
	messageID := "boot-prerequisite"
	bootMessage, err := utils.CreateOCPPMessage(2, messageID, "BootNotification", payload)
	if err != nil {
		return err
	}

	err = wsClient.SendMessage(bootMessage)
	if err != nil {
		return err
	}

	// 等待响应
	response, err := wsClient.ReceiveMessage(5 * time.Second)
	if err != nil {
		return err
	}

	// 验证响应
	utils.AssertBootNotificationResponse(t, response, messageID)
	
	// 等待一小段时间确保连接状态稳定
	time.Sleep(100 * time.Millisecond)
	
	return nil
}
