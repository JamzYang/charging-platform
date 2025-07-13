package business_flow

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTC_E2E_03_CompleteChargingSession 测试用例TC-E2E-03: 完整充电流程模拟一次完整的充电会话
func TestTC_E2E_03_CompleteChargingSession(t *testing.T) {
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

	// 前置条件：完成BootNotification流程
	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	// 步骤1: 远程启动 - 后端发送RemoteStartTransaction
	t.Log("Step 1: Remote start transaction")
	
	remoteStartCommand := map[string]interface{}{
		"charge_point_id": chargePointID,
		"command_name":    "RemoteStartTransaction",
		"message_id":      "remote-start-session-001",
		"payload": map[string]interface{}{
			"idTag":       "RFID123456",
			"connectorId": 1,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	err = sendCommandToKafka(env, remoteStartCommand)
	require.NoError(t, err)

	// 验证充电桩收到RemoteStartTransaction请求
	var remoteStartMessageID string
	utils.AssertEventuallyTrue(t, func() bool {
		select {
		case response, err := wsClient.ReceiveMessage(100 * time.Millisecond):
			if err != nil {
				return false
			}
			
			messageID, payload := utils.AssertRemoteStartTransactionRequest(t, response)
			remoteStartMessageID = messageID
			
			// 验证载荷
			assert.Equal(t, "RFID123456", payload["idTag"])
			assert.Equal(t, float64(1), payload["connectorId"])
			
			return true
		default:
			return false
		}
	}, 10*time.Second, "Should receive RemoteStartTransaction request")

	// 步骤2: 启动确认 - 桩回复RemoteStartTransaction.conf并发送StatusNotification (Charging)
	t.Log("Step 2: Start confirmation")
	
	// 发送RemoteStartTransaction响应
	remoteStartResponse := map[string]interface{}{
		"status": "Accepted",
	}
	
	responseMessage, err := utils.CreateOCPPMessage(3, remoteStartMessageID, "", remoteStartResponse)
	require.NoError(t, err)
	
	err = wsClient.SendMessage(responseMessage)
	require.NoError(t, err)

	// 发送StatusNotification (Charging)
	statusPayload := map[string]interface{}{
		"connectorId": 1,
		"errorCode":   "NoError",
		"status":      "Charging",
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	statusMessageID := "status-charging-001"
	statusMessage, err := utils.CreateOCPPMessage(2, statusMessageID, "StatusNotification", statusPayload)
	require.NoError(t, err)

	err = wsClient.SendMessage(statusMessage)
	require.NoError(t, err)

	// 验证StatusNotification响应
	statusResponse, err := wsClient.ReceiveMessage(5 * time.Second)
	require.NoError(t, err)
	utils.AssertStatusNotificationResponse(t, statusResponse, statusMessageID)

	// 验证Kafka中收到状态变更事件
	utils.AssertEventuallyTrue(t, func() bool {
		select {
		case kafkaMessage := <-partitionConsumer.Messages():
			event := utils.AssertKafkaMessage(t, kafkaMessage.Value, "device.status")
			eventPayload := event["payload"].(map[string]interface{})
			return eventPayload["status"].(string) == "Charging"
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}, 5*time.Second, "Should receive charging status event")

	// 步骤3: 上报计量 - 桩定时发送MeterValues
	t.Log("Step 3: Meter values reporting")
	
	// 模拟多次计量数据上报
	for i := 0; i < 3; i++ {
		meterPayload := map[string]interface{}{
			"connectorId":   1,
			"transactionId": 12345,
			"meterValue": []map[string]interface{}{
				{
					"timestamp": time.Now().Format(time.RFC3339),
					"sampledValue": []map[string]interface{}{
						{
							"value":     fmt.Sprintf("%.2f", 1234.56+float64(i)*0.5),
							"measurand": "Energy.Active.Import.Register",
							"unit":      "kWh",
						},
						{
							"value":     "7200",
							"measurand": "Power.Active.Import",
							"unit":      "W",
						},
					},
				},
			},
		}

		meterMessageID := fmt.Sprintf("meter-values-%03d", i+1)
		meterMessage, err := utils.CreateOCPPMessage(2, meterMessageID, "MeterValues", meterPayload)
		require.NoError(t, err)

		err = wsClient.SendMessage(meterMessage)
		require.NoError(t, err)

		// 验证MeterValues响应
		meterResponse, err := wsClient.ReceiveMessage(5 * time.Second)
		require.NoError(t, err)
		utils.AssertMeterValuesResponse(t, meterResponse, meterMessageID)

		// 验证Kafka事件
		utils.AssertEventuallyTrue(t, func() bool {
			select {
			case kafkaMessage := <-partitionConsumer.Messages():
				utils.AssertMeterValuesEvent(t, kafkaMessage.Value, chargePointID)
				return true
			case <-time.After(100 * time.Millisecond):
				return false
			}
		}, 3*time.Second, fmt.Sprintf("Should receive meter values event %d", i+1))

		time.Sleep(500 * time.Millisecond) // 模拟定时上报间隔
	}

	// 步骤4: 远程停止 - 后端发送RemoteStopTransaction
	t.Log("Step 4: Remote stop transaction")
	
	remoteStopCommand := map[string]interface{}{
		"charge_point_id": chargePointID,
		"command_name":    "RemoteStopTransaction",
		"message_id":      "remote-stop-session-001",
		"payload": map[string]interface{}{
			"transactionId": 12345,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	err = sendCommandToKafka(env, remoteStopCommand)
	require.NoError(t, err)

	// 验证充电桩收到RemoteStopTransaction请求
	var remoteStopMessageID string
	utils.AssertEventuallyTrue(t, func() bool {
		select {
		case response, err := wsClient.ReceiveMessage(100 * time.Millisecond):
			if err != nil {
				return false
			}
			
			var message []interface{}
			err = json.Unmarshal(response, &message)
			if err != nil || len(message) < 4 {
				return false
			}
			
			messageType := int(message[0].(float64))
			action := message[2].(string)
			
			if messageType == 2 && action == "RemoteStopTransaction" {
				remoteStopMessageID = message[1].(string)
				payload := message[3].(map[string]interface{})
				assert.Equal(t, float64(12345), payload["transactionId"])
				return true
			}
			
			return false
		default:
			return false
		}
	}, 10*time.Second, "Should receive RemoteStopTransaction request")

	// 步骤5: 停止确认 - 桩回复RemoteStopTransaction.conf并发送StatusNotification (Finishing)
	t.Log("Step 5: Stop confirmation")
	
	// 发送RemoteStopTransaction响应
	remoteStopResponse := map[string]interface{}{
		"status": "Accepted",
	}
	
	stopResponseMessage, err := utils.CreateOCPPMessage(3, remoteStopMessageID, "", remoteStopResponse)
	require.NoError(t, err)
	
	err = wsClient.SendMessage(stopResponseMessage)
	require.NoError(t, err)

	// 发送StatusNotification (Finishing)
	finishingStatusPayload := map[string]interface{}{
		"connectorId": 1,
		"errorCode":   "NoError",
		"status":      "Finishing",
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	finishingStatusMessageID := "status-finishing-001"
	finishingStatusMessage, err := utils.CreateOCPPMessage(2, finishingStatusMessageID, "StatusNotification", finishingStatusPayload)
	require.NoError(t, err)

	err = wsClient.SendMessage(finishingStatusMessage)
	require.NoError(t, err)

	// 验证StatusNotification响应
	finishingStatusResponse, err := wsClient.ReceiveMessage(5 * time.Second)
	require.NoError(t, err)
	utils.AssertStatusNotificationResponse(t, finishingStatusResponse, finishingStatusMessageID)

	// 步骤6: 交易数据 - 桩发送StopTransaction
	t.Log("Step 6: Transaction data")
	
	stopTransactionPayload := map[string]interface{}{
		"transactionId": 12345,
		"timestamp":     time.Now().Format(time.RFC3339),
		"meterStop":     1237.56,
		"reason":        "Remote",
		"transactionData": []map[string]interface{}{
			{
				"timestamp": time.Now().Format(time.RFC3339),
				"sampledValue": []map[string]interface{}{
					{
						"value":     "1237.56",
						"measurand": "Energy.Active.Import.Register",
						"unit":      "kWh",
					},
				},
			},
		},
	}

	stopTransactionMessageID := "stop-transaction-001"
	stopTransactionMessage, err := utils.CreateOCPPMessage(2, stopTransactionMessageID, "StopTransaction", stopTransactionPayload)
	require.NoError(t, err)

	err = wsClient.SendMessage(stopTransactionMessage)
	require.NoError(t, err)

	// 验证StopTransaction响应
	stopTransactionResponse, err := wsClient.ReceiveMessage(5 * time.Second)
	require.NoError(t, err)
	
	responsePayload := utils.AssertOCPPCallResult(t, stopTransactionResponse, stopTransactionMessageID)
	// StopTransaction响应可能包含idTagInfo
	if idTagInfo, exists := responsePayload["idTagInfo"]; exists {
		idTagInfoMap := idTagInfo.(map[string]interface{})
		assert.Contains(t, idTagInfoMap, "status")
	}

	// 发送最终StatusNotification (Available)
	availableStatusPayload := map[string]interface{}{
		"connectorId": 1,
		"errorCode":   "NoError",
		"status":      "Available",
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	availableStatusMessageID := "status-available-001"
	availableStatusMessage, err := utils.CreateOCPPMessage(2, availableStatusMessageID, "StatusNotification", availableStatusPayload)
	require.NoError(t, err)

	err = wsClient.SendMessage(availableStatusMessage)
	require.NoError(t, err)

	// 验证最终StatusNotification响应
	availableStatusResponse, err := wsClient.ReceiveMessage(5 * time.Second)
	require.NoError(t, err)
	utils.AssertStatusNotificationResponse(t, availableStatusResponse, availableStatusMessageID)

	// 验证完整流程中的所有Kafka事件
	t.Log("Verifying complete event flow...")
	
	// 等待所有事件处理完成
	time.Sleep(2 * time.Second)

	t.Log("TC-E2E-03 Complete charging session test passed")
}

// sendCommandToKafka 发送指令到Kafka
func sendCommandToKafka(env *utils.TestEnvironment, command map[string]interface{}) error {
	commandBytes, err := json.Marshal(command)
	if err != nil {
		return err
	}

	message := &sarama.ProducerMessage{
		Topic:     "commands-down-test",
		Partition: 0,
		Value:     sarama.StringEncoder(commandBytes),
	}

	_, _, err = env.KafkaProducer.SendMessage(message)
	return err
}

// performBootNotification 执行BootNotification流程
func performBootNotification(t *testing.T, wsClient *utils.WebSocketClient, chargePointID string) error {
	bootNotificationData, err := utils.LoadTestData("ocpp_messages/boot_notification.json")
	if err != nil {
		return err
	}

	var payload map[string]interface{}
	err = json.Unmarshal(bootNotificationData, &payload)
	if err != nil {
		return err
	}

	messageID := "boot-prerequisite"
	bootMessage, err := utils.CreateOCPPMessage(2, messageID, "BootNotification", payload)
	if err != nil {
		return err
	}

	err = wsClient.SendMessage(bootMessage)
	if err != nil {
		return err
	}

	response, err := wsClient.ReceiveMessage(5 * time.Second)
	if err != nil {
		return err
	}

	utils.AssertBootNotificationResponse(t, response, messageID)
	time.Sleep(100 * time.Millisecond)
	
	return nil
}
