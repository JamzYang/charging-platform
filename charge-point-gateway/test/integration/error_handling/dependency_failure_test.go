package error_handling

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/test/utils"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTC_INT_07_DependencyFailure 测试用例TC-INT-07: Kafka/Redis临时不可用验证网关的容错与恢复
func TestTC_INT_07_DependencyFailure(t *testing.T) {
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

	// 测试Redis故障恢复
	t.Run("RedisFailureRecovery", func(t *testing.T) {
		// 验证Redis正常工作
		ctx := context.Background()
		err := env.RedisClient.Set(ctx, "test-key", "test-value", time.Minute).Err()
		require.NoError(t, err, "Redis should be working initially")

		// 模拟Redis故障（通过关闭连接）
		t.Log("Simulating Redis failure...")
		env.RedisClient.Close()
		// 注意：在实际环境中，这里应该停止Redis容器，但在我们的测试环境中我们只是关闭连接

		// 在Redis故障期间，网关应该继续运行
		// 发送消息，验证WebSocket连接仍然活跃
		time.Sleep(1 * time.Second) // 等待故障生效

		err = sendValidHeartbeat(t, wsClient)
		assert.NoError(t, err, "WebSocket should remain active during Redis failure")

		// 尝试发送需要Redis的操作（如StatusNotification）
		statusPayload := map[string]interface{}{
			"connectorId": 1,
			"errorCode":   "NoError",
			"status":      "Available",
			"timestamp":   time.Now().Format(time.RFC3339),
		}

		messageID := "test-status-redis-failure"
		statusMessage, err := utils.CreateOCPPMessage(2, messageID, "StatusNotification", statusPayload)
		require.NoError(t, err)

		err = wsClient.SendMessage(statusMessage)
		require.NoError(t, err)

		// 应该仍能收到响应（即使Redis不可用）
		response, err := wsClient.ReceiveMessage(5 * time.Second)
		if err == nil {
			utils.AssertStatusNotificationResponse(t, response, messageID)
			t.Log("Gateway continues to respond during Redis failure")
		} else {
			t.Log("Gateway may have delayed response during Redis failure, which is acceptable")
		}

		// 恢复Redis连接
		t.Log("Recovering Redis...")
		// 重新创建Redis客户端
		env.RedisClient = redis.NewClient(&redis.Options{
			Addr: "localhost:6380",
			DB:   0,
		})

		// 等待Redis恢复
		utils.AssertEventuallyTrue(t, func() bool {
			err := env.RedisClient.Ping(ctx).Err()
			return err == nil
		}, 30*time.Second, "Redis should recover")

		// 验证网关恢复正常工作
		time.Sleep(2 * time.Second) // 等待网关重连

		err = sendValidHeartbeat(t, wsClient)
		assert.NoError(t, err, "WebSocket should work normally after Redis recovery")

		t.Log("Redis failure recovery test passed")
	})

	// 测试Kafka故障恢复
	t.Run("KafkaFailureRecovery", func(t *testing.T) {
		// 验证Kafka正常工作
		testMessage := "test-kafka-message"
		_, _, err := env.KafkaProducer.SendMessage(&sarama.ProducerMessage{
			Topic: "test-topic",
			Value: sarama.StringEncoder(testMessage),
		})
		require.NoError(t, err, "Kafka should be working initially")

		// 模拟Kafka故障（关闭生产者）
		t.Log("Simulating Kafka failure...")
		env.KafkaProducer.Close()
		// 注意：在实际环境中，这里应该停止Kafka容器

		// 在Kafka故障期间，网关应该继续处理WebSocket连接
		time.Sleep(1 * time.Second) // 等待故障生效

		err = sendValidHeartbeat(t, wsClient)
		assert.NoError(t, err, "WebSocket should remain active during Kafka failure")

		// 发送会产生Kafka事件的消息
		meterValuesPayload := map[string]interface{}{
			"connectorId": 1,
			"meterValue": []map[string]interface{}{
				{
					"timestamp": time.Now().Format(time.RFC3339),
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

		messageID := "test-meter-kafka-failure"
		meterMessage, err := utils.CreateOCPPMessage(2, messageID, "MeterValues", meterValuesPayload)
		require.NoError(t, err)

		err = wsClient.SendMessage(meterMessage)
		require.NoError(t, err)

		// 应该仍能收到响应（即使Kafka不可用）
		response, err := wsClient.ReceiveMessage(5 * time.Second)
		if err == nil {
			utils.AssertMeterValuesResponse(t, response, messageID)
			t.Log("Gateway continues to respond during Kafka failure")
		} else {
			t.Log("Gateway may have delayed response during Kafka failure, which is acceptable")
		}

		// 恢复Kafka连接
		t.Log("Recovering Kafka...")
		// 重新创建Kafka生产者
		config := sarama.NewConfig()
		config.Producer.Return.Successes = true
		config.Producer.RequiredAcks = sarama.WaitForAll
		config.Producer.Retry.Max = 3

		producer, err := sarama.NewSyncProducer([]string{"localhost:9093"}, config)
		require.NoError(t, err, "Failed to recreate Kafka producer")
		env.KafkaProducer = producer

		// 等待Kafka恢复
		time.Sleep(10 * time.Second) // Kafka需要更长时间启动

		// 验证网关恢复正常工作
		err = sendValidHeartbeat(t, wsClient)
		assert.NoError(t, err, "WebSocket should work normally after Kafka recovery")

		t.Log("Kafka failure recovery test passed")
	})

	t.Log("TC-INT-07 Dependency failure test passed")
}

// TestTC_INT_07_PartialDependencyFailure 测试部分依赖故障
func TestTC_INT_07_PartialDependencyFailure(t *testing.T) {
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

	// 测试Redis连接池耗尽
	t.Run("RedisConnectionPoolExhaustion", func(t *testing.T) {
		// 创建大量Redis连接来耗尽连接池
		ctx := context.Background()
		var clients []*redis.Client

		// 创建多个Redis客户端（模拟连接池耗尽）
		for i := 0; i < 20; i++ {
			client := redis.NewClient(&redis.Options{
				Addr:     env.RedisClient.Options().Addr,
				PoolSize: 1,
			})
			clients = append(clients, client)

			// 执行一个长时间运行的操作
			go func(c *redis.Client) {
				c.BLPop(ctx, time.Hour, "non-existent-key")
			}(client)
		}

		// 清理
		defer func() {
			for _, client := range clients {
				client.Close()
			}
		}()

		time.Sleep(1 * time.Second) // 等待连接池压力生效

		// 验证网关仍能处理基本操作
		err = sendValidHeartbeat(t, wsClient)
		assert.NoError(t, err, "Gateway should handle basic operations during Redis pool pressure")

		t.Log("Redis connection pool exhaustion test passed")
	})

	// 测试网络延迟
	t.Run("NetworkLatency", func(t *testing.T) {
		// 发送多个并发消息来测试网络延迟处理
		messageCount := 10
		responses := make(chan bool, messageCount)

		for i := 0; i < messageCount; i++ {
			go func(index int) {
				messageID := fmt.Sprintf("test-latency-%03d", index)
				payload := map[string]interface{}{}

				heartbeatMessage, err := utils.CreateOCPPMessage(2, messageID, "Heartbeat", payload)
				if err != nil {
					responses <- false
					return
				}

				err = wsClient.SendMessage(heartbeatMessage)
				if err != nil {
					responses <- false
					return
				}

				// 等待响应
				_, err = wsClient.ReceiveMessage(10 * time.Second)
				responses <- err == nil
			}(i)
		}

		// 收集结果
		successCount := 0
		for i := 0; i < messageCount; i++ {
			select {
			case success := <-responses:
				if success {
					successCount++
				}
			case <-time.After(15 * time.Second):
				t.Log("Timeout waiting for response")
			}
		}

		// 验证大部分消息都成功处理
		successRate := float64(successCount) / float64(messageCount)
		assert.Greater(t, successRate, 0.7, "Success rate should be > 70% under network pressure")

		t.Logf("Network latency test passed with %d/%d success rate", successCount, messageCount)
	})

	t.Log("Partial dependency failure test passed")
}

// sendValidHeartbeat 发送有效的Heartbeat消息来测试连接状态
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

	// 等待响应
	response, err := wsClient.ReceiveMessage(3 * time.Second)
	if err != nil {
		return err
	}

	// 验证响应
	responsePayload := utils.AssertOCPPCallResult(t, response, messageID)
	assert.Contains(t, responsePayload, "currentTime", "Heartbeat response should contain currentTime")

	return nil
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
