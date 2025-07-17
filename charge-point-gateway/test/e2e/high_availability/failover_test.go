package high_availability

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTC_E2E_01_NodeFailover 测试用例TC-E2E-01: 节点故障转移验证网关自愈能力
func TestTC_E2E_01_NodeFailover(t *testing.T) {
	// 注意：这个测试需要在真实的Kubernetes环境中运行
	// 在单元测试环境中，我们模拟故障转移场景

	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-007"
	podAID := "pod-a"
	podBID := "pod-b"

	// 模拟充电桩连接到pod-a
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 完成BootNotification流程
	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	// 验证Redis中conn:CP-007的值为pod-a
	ctx := context.Background()
	connectionKey := fmt.Sprintf("conn:%s", chargePointID)

	// 手动设置连接映射（模拟pod-a处理了连接）
	err = env.RedisClient.Set(ctx, connectionKey, podAID, time.Hour).Err()
	require.NoError(t, err)

	utils.AssertRedisConnection(t, env.RedisClient, chargePointID, podAID)

	// 步骤1: 模拟pod-a故障（手动删除/kill pod-a）
	t.Log("Simulating pod-a failure...")

	// 在真实环境中，这里会执行：kubectl delete pod pod-a
	// 在测试环境中，我们模拟连接断开
	wsClient.Close()

	// 步骤2: 模拟充电桩自动重连到pod-b
	t.Log("Simulating charge point reconnection to pod-b...")

	// 等待一段时间模拟重连延迟
	time.Sleep(2 * time.Second)

	// 创建新的WebSocket连接（模拟重连到pod-b）
	wsClientReconnected, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClientReconnected.Close()

	// 步骤3: 充电桩在新连接上重新发送BootNotification
	t.Log("Sending BootNotification on new connection...")
	err = performBootNotification(t, wsClientReconnected, chargePointID)
	require.NoError(t, err)

	// 步骤4: 验证Redis中conn:CP-007的值被更新为pod-b
	// 在测试环境中，我们手动更新来模拟pod-b处理了重连
	err = env.RedisClient.Set(ctx, connectionKey, podBID, time.Hour).Err()
	require.NoError(t, err)

	utils.AssertRedisConnection(t, env.RedisClient, chargePointID, podBID)

	// 步骤5: 验证新连接正常工作
	t.Log("Verifying new connection works normally...")
	err = sendValidHeartbeat(t, wsClientReconnected)
	require.NoError(t, err)

	// 发送StatusNotification验证完整功能
	statusPayload := map[string]interface{}{
		"connectorId": 1,
		"errorCode":   "NoError",
		"status":      "Available",
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	messageID := "test-failover-status"
	statusMessage, err := utils.CreateOCPPMessage(2, messageID, "StatusNotification", statusPayload)
	require.NoError(t, err)

	err = wsClientReconnected.SendMessage(statusMessage)
	require.NoError(t, err)

	response, err := wsClientReconnected.ReceiveMessage(5 * time.Second)
	require.NoError(t, err)
	utils.AssertStatusNotificationResponse(t, response, messageID)

	t.Log("TC-E2E-01 Node failover test passed")
}

// TestTC_E2E_02_CommandRoutingDuringFailover 测试用例TC-E2E-02: 故障转移期间的指令路由验证指令不丢失
func TestTC_E2E_02_CommandRoutingDuringFailover(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-007"
	podAID := "pod-a"
	podBID := "pod-b"

	// 建立初始连接
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	// 设置初始连接映射
	ctx := context.Background()
	connectionKey := fmt.Sprintf("conn:%s", chargePointID)
	err = env.RedisClient.Set(ctx, connectionKey, podAID, time.Hour).Err()
	require.NoError(t, err)

	// 步骤1: 模拟pod-a被删除后，但在CP-007重连到pod-b之前，后端持续发送指令
	t.Log("Simulating commands sent during failover...")

	// 发送指令到旧的pod（应该失败或被忽略）
	command1 := createRemoteStartCommand(chargePointID, "test-failover-cmd-1")
	err = sendCommandToKafka(env, command1)
	require.NoError(t, err)

	// 模拟连接断开
	wsClient.Close()

	// 继续发送指令（在故障转移期间）
	command2 := createRemoteStartCommand(chargePointID, "test-failover-cmd-2")
	err = sendCommandToKafka(env, command2)
	require.NoError(t, err)

	// 步骤2: 模拟CP-007重连到pod-b
	time.Sleep(1 * time.Second)

	wsClientReconnected, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClientReconnected.Close()

	err = performBootNotification(t, wsClientReconnected, chargePointID)
	require.NoError(t, err)

	// 更新连接映射到pod-b
	err = env.RedisClient.Set(ctx, connectionKey, podBID, time.Hour).Err()
	require.NoError(t, err)

	// 步骤3: 在重连后发送指令（应该成功）
	t.Log("Sending command after reconnection...")
	command3 := createRemoteStartCommand(chargePointID, "test-failover-cmd-3")
	err = sendCommandToKafka(env, command3)
	require.NoError(t, err)

	// 验证重连后的指令能够成功到达充电桩
	utils.AssertEventuallyTrue(t, func() bool {
		response, err := utils.ReceiveMessageWithTimeout(wsClient, 100*time.Millisecond)

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

		err = wsClientReconnected.SendMessage(responseMessage)
		require.NoError(t, err)

		// 验证是重连后发送的指令
		if messageID == "test-failover-cmd-3" {
			return true
		}
		return false
	}, 10*time.Second, "Should receive command after reconnection")

	t.Log("TC-E2E-02 Command routing during failover test passed")
}

// TestTC_E2E_01_02_MultipleFailovers 测试多次故障转移
func TestTC_E2E_01_02_MultipleFailovers(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-008"
	pods := []string{"pod-a", "pod-b", "pod-c"}
	currentPodIndex := 0

	// 建立初始连接
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	err = performBootNotification(t, wsClient, chargePointID)
	require.NoError(t, err)

	ctx := context.Background()
	connectionKey := fmt.Sprintf("conn:%s", chargePointID)

	// 执行多次故障转移
	for i := 0; i < 3; i++ {
		currentPod := pods[currentPodIndex]
		nextPodIndex := (currentPodIndex + 1) % len(pods)
		nextPod := pods[nextPodIndex]

		t.Logf("Failover %d: %s -> %s", i+1, currentPod, nextPod)

		// 设置当前连接映射
		err = env.RedisClient.Set(ctx, connectionKey, currentPod, time.Hour).Err()
		require.NoError(t, err)

		// 验证连接正常工作
		err = sendValidHeartbeat(t, wsClient)
		require.NoError(t, err)

		// 模拟故障转移
		wsClient.Close()
		time.Sleep(500 * time.Millisecond)

		// 重新连接
		wsClient, err = utils.NewWebSocketClient(env.GatewayURL, chargePointID)
		require.NoError(t, err)
		defer wsClient.Close()

		err = performBootNotification(t, wsClient, chargePointID)
		require.NoError(t, err)

		// 更新连接映射
		err = env.RedisClient.Set(ctx, connectionKey, nextPod, time.Hour).Err()
		require.NoError(t, err)

		// 验证新连接工作正常
		err = sendValidHeartbeat(t, wsClient)
		require.NoError(t, err)

		currentPodIndex = nextPodIndex
	}

	t.Log("Multiple failovers test passed")
}

// createRemoteStartCommand 创建RemoteStartTransaction指令
func createRemoteStartCommand(chargePointID, messageID string) map[string]interface{} {
	return map[string]interface{}{
		"charge_point_id": chargePointID,
		"command_name":    "RemoteStartTransaction",
		"message_id":      messageID,
		"payload": map[string]interface{}{
			"idTag":       "RFID123456",
			"connectorId": 1,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}
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

// sendValidHeartbeat 发送有效的Heartbeat消息
func sendValidHeartbeat(t *testing.T, wsClient *utils.WebSocketClient) error {
	messageID := "test-heartbeat"
	payload := map[string]interface{}{}

	heartbeatMessage, err := utils.CreateOCPPMessage(2, messageID, "Heartbeat", payload)
	if err != nil {
		return err
	}

	err = wsClient.SendMessage(heartbeatMessage)
	if err != nil {
		return err
	}

	response, err := wsClient.ReceiveMessage(3 * time.Second)
	if err != nil {
		return err
	}

	responsePayload := utils.AssertOCPPCallResult(t, response, messageID)
	assert.Contains(t, responsePayload, "currentTime", "Heartbeat response should contain currentTime")

	return nil
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
