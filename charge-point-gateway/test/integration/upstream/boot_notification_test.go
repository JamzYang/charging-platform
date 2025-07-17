package upstream

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

// TestTC_INT_01_BootNotification 测试用例TC-INT-01: BootNotification验证桩上线流程
func TestTC_INT_01_BootNotification(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	// 测试参数
	chargePointID := "CP-001"
	podID := "test-pod-1"

	// 创建Kafka消费者来监听上行事件
	t.Logf("Creating partition consumer for topic: ocpp-events-up-test")
	partitionConsumer, err := env.KafkaConsumer.ConsumePartition("ocpp-events-up-test", 0, sarama.OffsetNewest)
	require.NoError(t, err)
	defer partitionConsumer.Close()

	// 创建WebSocket客户端连接到网关
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 等待连接建立
	time.Sleep(100 * time.Millisecond)

	// 加载BootNotification测试数据
	bootNotificationData, err := utils.LoadTestData("ocpp_messages/boot_notification.json")
	require.NoError(t, err)

	var payload map[string]interface{}
	err = json.Unmarshal(bootNotificationData, &payload)
	require.NoError(t, err)

	// 创建BootNotification OCPP消息
	messageID := "test-boot-001"
	bootMessage, err := utils.CreateOCPPMessage(2, messageID, "BootNotification", payload)
	require.NoError(t, err)

	// 发送BootNotification请求
	err = wsClient.SendMessage(bootMessage)
	require.NoError(t, err)

	// 步骤1: 验证桩收到BootNotification.conf响应，status为Accepted
	response, err := wsClient.ReceiveMessage(5 * time.Second)
	require.NoError(t, err)

	utils.AssertBootNotificationResponse(t, response, messageID)

	// 步骤2: 验证Redis中存在键conn:CP-001，其值为当前Gateway Pod的ID
	utils.AssertEventuallyTrue(t, func() bool {
		key := fmt.Sprintf("conn:%s", chargePointID)
		result, err := env.RedisClient.Get(context.Background(), key).Result()
		return err == nil && result == podID
	}, 5*time.Second, "Redis connection mapping should be created")

	// 步骤3: 验证后端模拟器在Kafka中消费到一条DeviceOnlineEvent事件
	utils.AssertEventuallyTrue(t, func() bool {
		select {
		case kafkaMessage := <-partitionConsumer.Messages():
			// 验证事件内容
			utils.AssertDeviceOnlineEvent(t, kafkaMessage.Value, chargePointID)
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}, 5*time.Second, "Should receive DeviceOnlineEvent in Kafka")

	t.Log("TC-INT-01 BootNotification test passed")
}

// TestTC_INT_01_BootNotification_InvalidPayload 测试无效载荷的BootNotification
func TestTC_INT_01_BootNotification_InvalidPayload(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	chargePointID := "CP-002"

	// 创建WebSocket客户端
	wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
	require.NoError(t, err)
	defer wsClient.Close()

	// 创建无效的BootNotification消息（缺少必要字段）
	invalidPayload := map[string]interface{}{
		"chargePointVendor": "TestVendor",
		// 缺少chargePointModel字段
	}

	messageID := "test-boot-invalid-001"
	bootMessage, err := utils.CreateOCPPMessage(2, messageID, "BootNotification", invalidPayload)
	require.NoError(t, err)

	// 发送无效的BootNotification请求
	err = wsClient.SendMessage(bootMessage)
	require.NoError(t, err)

	// 应该收到错误响应
	response, err := wsClient.ReceiveMessage(5 * time.Second)
	require.NoError(t, err)

	// 验证收到CALLERROR消息
	errorCode, errorDescription, _ := utils.AssertOCPPCallError(t, response, messageID)
	assert.Contains(t, []string{"FormationViolation", "PropertyConstraintViolation"}, errorCode)
	assert.NotEmpty(t, errorDescription)

	t.Log("BootNotification invalid payload test passed")
}

// TestTC_INT_01_BootNotification_Concurrent 测试并发BootNotification
func TestTC_INT_01_BootNotification_Concurrent(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	// 并发连接数
	concurrentCount := 10
	results := make(chan bool, concurrentCount)

	// 加载测试数据
	bootNotificationData, err := utils.LoadTestData("ocpp_messages/boot_notification.json")
	require.NoError(t, err)

	var payload map[string]interface{}
	err = json.Unmarshal(bootNotificationData, &payload)
	require.NoError(t, err)

	// 启动多个并发连接
	for i := 0; i < concurrentCount; i++ {
		go func(index int) {
			chargePointID := fmt.Sprintf("CP-%03d", index)

			// 创建WebSocket客户端
			wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
			if err != nil {
				results <- false
				return
			}
			defer wsClient.Close()

			// 创建BootNotification消息
			messageID := fmt.Sprintf("test-boot-concurrent-%03d", index)
			bootMessage, err := utils.CreateOCPPMessage(2, messageID, "BootNotification", payload)
			if err != nil {
				results <- false
				return
			}

			// 发送请求
			err = wsClient.SendMessage(bootMessage)
			if err != nil {
				results <- false
				return
			}

			// 接收响应
			response, err := wsClient.ReceiveMessage(10 * time.Second)
			if err != nil {
				results <- false
				return
			}

			// 验证响应
			utils.AssertBootNotificationResponse(t, response, messageID)
			results <- true
		}(i)
	}

	// 等待所有连接完成
	successCount := 0
	for i := 0; i < concurrentCount; i++ {
		select {
		case success := <-results:
			if success {
				successCount++
			}
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for concurrent connections")
		}
	}

	// 验证所有连接都成功
	assert.Equal(t, concurrentCount, successCount, "All concurrent connections should succeed")

	t.Logf("Concurrent BootNotification test passed with %d connections", successCount)
}
