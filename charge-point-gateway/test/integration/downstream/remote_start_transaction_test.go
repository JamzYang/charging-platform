package downstream

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/test/utils"
	"github.com/stretchr/testify/require"
)

// TestTC_INT_04_RemoteStartTransaction 测试用例TC-INT-04: RemoteStartTransaction验证远程启动指令
func TestTC_INT_04_RemoteStartTransaction(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-001"
	podID := "test-pod-1"

	// 创建WebSocket客户端
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 前置条件：完成BootNotification流程
	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	// 步骤1: 后端模拟器查询Redis，获取CP-001所在的Pod ID
	ctx := context.Background()
	connectionKey := fmt.Sprintf("conn:%s", chargePointID)
	
	// 验证连接映射存在
	utils.AssertEventuallyTrue(t, func() bool {
		result, err := env.RedisClient.Get(ctx, connectionKey).Result()
		return err == nil && result == podID
	}, 5*time.Second, "Connection mapping should exist in Redis")

	// 步骤2: 后端模拟器计算分区，并向commands-down主题的对应分区发送RemoteStartTransaction指令
	
	// 加载RemoteStartTransaction测试数据
	remoteStartData, err := utils.LoadTestData("ocpp_messages/remote_start_transaction.json")
	require.NoError(t, err)

	var commandPayload map[string]interface{}
	err = json.Unmarshal(remoteStartData, &commandPayload)
	require.NoError(t, err)

	// 创建下行指令消息
	command := map[string]interface{}{
		"charge_point_id": chargePointID,
		"command_name":    "RemoteStartTransaction",
		"message_id":      "test-remote-start-001",
		"payload":         commandPayload,
		"timestamp":       time.Now().Format(time.RFC3339),
	}

	commandBytes, err := json.Marshal(command)
	require.NoError(t, err)

	// 计算分区（模拟后端的分区计算逻辑）
	// 这里简化为分区0，实际应该根据Pod ID计算
	partition := int32(0)

	// 发送指令到Kafka
	message := &sarama.ProducerMessage{
		Topic:     "commands-down-test",
		Partition: partition,
		Value:     sarama.StringEncoder(commandBytes),
	}

	_, _, err = env.KafkaProducer.SendMessage(message)
	require.NoError(t, err)

	// 步骤3: 验证模拟充电桩CP-001收到RemoteStartTransaction请求
	utils.AssertEventuallyTrue(t, func() bool {
		select {
		case response, err := wsClient.ReceiveMessage(100 * time.Millisecond):
			if err != nil {
				return false
			}
			
			// 验证收到的是RemoteStartTransaction请求
			messageID, payload := utils.AssertRemoteStartTransactionRequest(t, response)
			
			// 验证载荷内容
			require.Contains(t, payload, "idTag", "Payload should contain idTag")
			idTag := payload["idTag"].(string)
			require.Equal(t, "RFID123456", idTag, "IdTag should match")
			
			// 发送响应
			responsePayload := map[string]interface{}{
				"status": "Accepted",
			}
			
			responseMessage, err := utils.CreateOCPPMessage(3, messageID, "", responsePayload)
			require.NoError(t, err)
			
			err = wsClient.SendMessage(responseMessage)
			require.NoError(t, err)
			
			return true
		default:
			return false
		}
	}, 10*time.Second, "Should receive RemoteStartTransaction request")

	t.Log("TC-INT-04 RemoteStartTransaction test passed")
}

// TestTC_INT_04_RemoteStartTransaction_InvalidChargePoint 测试向不存在的充电桩发送指令
func TestTC_INT_04_RemoteStartTransaction_InvalidChargePoint(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	nonExistentChargePointID := "CP-999"

	// 创建下行指令消息（向不存在的充电桩发送）
	command := map[string]interface{}{
		"charge_point_id": nonExistentChargePointID,
		"command_name":    "RemoteStartTransaction",
		"message_id":      "test-remote-start-invalid-001",
		"payload": map[string]interface{}{
			"idTag": "RFID123456",
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	commandBytes, err := json.Marshal(command)
	require.NoError(t, err)

	// 发送指令到Kafka
	message := &sarama.ProducerMessage{
		Topic:     "commands-down-test",
		Partition: 0,
		Value:     sarama.StringEncoder(commandBytes),
	}

	_, _, err = env.KafkaProducer.SendMessage(message)
	require.NoError(t, err)

	// 等待一段时间，确保消息被处理
	time.Sleep(2 * time.Second)

	// 验证Redis中没有该充电桩的连接映射
	utils.AssertRedisConnectionNotExists(t, env.RedisClient, nonExistentChargePointID)

	t.Log("RemoteStartTransaction to invalid charge point test passed")
}

// TestTC_INT_04_RemoteStartTransaction_MultipleCommands 测试多个并发指令
func TestTC_INT_04_RemoteStartTransaction_MultipleCommands(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	// 创建多个充电桩连接
	chargePointCount := 5
	wsClients := make([]*utils.WebSocketClient, chargePointCount)
	
	// 建立连接并完成BootNotification
	for i := 0; i < chargePointCount; i++ {
		chargePointID := fmt.Sprintf("CP-%03d", i+1)
		
		wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
		require.NoError(t, err)
		defer wsClient.Close()
		
		wsClients[i] = wsClient
		
		// 完成BootNotification
		err = performBootNotification(t, wsClient, chargePointID)
		require.NoError(t, err)
	}

	// 并发发送RemoteStartTransaction指令
	for i := 0; i < chargePointCount; i++ {
		chargePointID := fmt.Sprintf("CP-%03d", i+1)
		
		command := map[string]interface{}{
			"charge_point_id": chargePointID,
			"command_name":    "RemoteStartTransaction",
			"message_id":      fmt.Sprintf("test-remote-start-multi-%03d", i),
			"payload": map[string]interface{}{
				"idTag":       fmt.Sprintf("RFID%06d", i+1),
				"connectorId": 1,
			},
			"timestamp": time.Now().Format(time.RFC3339),
		}

		commandBytes, err := json.Marshal(command)
		require.NoError(t, err)

		// 发送指令到Kafka
		message := &sarama.ProducerMessage{
			Topic:     "commands-down-test",
			Partition: 0,
			Value:     sarama.StringEncoder(commandBytes),
		}

		_, _, err = env.KafkaProducer.SendMessage(message)
		require.NoError(t, err)
	}

	// 验证所有充电桩都收到指令
	successCount := 0
	for i := 0; i < chargePointCount; i++ {
		wsClient := wsClients[i]
		
		// 等待接收指令
		utils.AssertEventuallyTrue(t, func() bool {
			select {
			case response, err := wsClient.ReceiveMessage(100 * time.Millisecond):
				if err != nil {
					return false
				}
				
				// 验证收到RemoteStartTransaction请求
				messageID, _ := utils.AssertRemoteStartTransactionRequest(t, response)
				
				// 发送响应
				responsePayload := map[string]interface{}{
					"status": "Accepted",
				}
				
				responseMessage, err := utils.CreateOCPPMessage(3, messageID, "", responsePayload)
				require.NoError(t, err)
				
				err = wsClient.SendMessage(responseMessage)
				require.NoError(t, err)
				
				successCount++
				return true
			default:
				return false
			}
		}, 10*time.Second, fmt.Sprintf("Charge point %d should receive command", i+1))
	}

	require.Equal(t, chargePointCount, successCount, "All charge points should receive commands")

	t.Logf("Multiple RemoteStartTransaction commands test passed with %d charge points", successCount)
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
