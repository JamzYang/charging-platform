package performance

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTC_E2E_04_ConcurrentConnections 测试用例TC-E2E-04: 并发连接模拟大量桩同时在线
func TestTC_E2E_04_ConcurrentConnections(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	// 测试参数（在实际环境中可以设置为1000）
	connectionCount := 100 // 降低数量以适应测试环境
	testDuration := 10 * time.Second

	t.Logf("Starting concurrent connections test with %d connections for %v", connectionCount, testDuration)

	// 统计信息
	var (
		successfulConnections int64
		failedConnections     int64
		totalMessages         int64
		successfulMessages    int64
		failedMessages        int64
	)

	// 用于同步的WaitGroup
	var wg sync.WaitGroup

	// 启动时间
	startTime := time.Now()

	// 创建并发连接
	for i := 0; i < connectionCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			chargePointID := fmt.Sprintf("CP-%04d", index)

			// 创建WebSocket连接
			wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
			if err != nil {
				atomic.AddInt64(&failedConnections, 1)
				t.Logf("Failed to connect %s: %v", chargePointID, err)
				return
			}
			defer wsClient.Close()

			atomic.AddInt64(&successfulConnections, 1)

			// 执行BootNotification
			err = performBootNotification(t, wsClient, chargePointID)
			if err != nil {
				t.Logf("BootNotification failed for %s: %v", chargePointID, err)
				return
			}

			// 保持连接并定期发送心跳
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			endTime := startTime.Add(testDuration)

			for time.Now().Before(endTime) {
				select {
				case <-ticker.C:
					atomic.AddInt64(&totalMessages, 1)

					// 发送心跳
					err := sendHeartbeat(wsClient, chargePointID)
					if err != nil {
						atomic.AddInt64(&failedMessages, 1)
						t.Logf("Heartbeat failed for %s: %v", chargePointID, err)
						return
					}

					atomic.AddInt64(&successfulMessages, 1)

				case <-time.After(100 * time.Millisecond):
					// 继续循环
				}
			}
		}(i)
	}

	// 等待所有连接完成
	wg.Wait()

	totalTime := time.Since(startTime)

	// 计算统计信息
	successConnRate := float64(successfulConnections) / float64(connectionCount) * 100
	messageSuccessRate := float64(successfulMessages) / float64(totalMessages) * 100

	// 验证结果
	t.Logf("Test Results:")
	t.Logf("  Total time: %v", totalTime)
	t.Logf("  Successful connections: %d/%d (%.2f%%)", successfulConnections, connectionCount, successConnRate)
	t.Logf("  Failed connections: %d", failedConnections)
	t.Logf("  Total messages: %d", totalMessages)
	t.Logf("  Successful messages: %d (%.2f%%)", successfulMessages, messageSuccessRate)
	t.Logf("  Failed messages: %d", failedMessages)

	// 断言：至少80%的连接应该成功
	assert.Greater(t, successConnRate, 80.0, "At least 80% of connections should succeed")

	// 断言：至少90%的消息应该成功
	if totalMessages > 0 {
		assert.Greater(t, messageSuccessRate, 90.0, "At least 90% of messages should succeed")
	}

	t.Log("TC-E2E-04 Concurrent connections test passed")
}

// TestTC_E2E_05_HighThroughput 测试用例TC-E2E-05: 高吞吐量模拟消息洪峰
func TestTC_E2E_05_HighThroughput(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	// 测试参数
	connectionCount := 50 // 降低连接数以适应测试环境
	messagesPerConnection := 20
	messageInterval := 100 * time.Millisecond

	t.Logf("Starting high throughput test with %d connections, %d messages each",
		connectionCount, messagesPerConnection)

	// 统计信息
	var (
		totalMessages      int64
		successfulMessages int64
		failedMessages     int64
		totalLatency       int64
		messageCount       int64
	)

	// 建立连接
	var wsClients []*utils.WebSocketClient
	for i := 0; i < connectionCount; i++ {
		chargePointID := fmt.Sprintf("CP-%04d", i)

		wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
		require.NoError(t, err)
		defer wsClient.Close()

		wsClients = append(wsClients, wsClient)

		// 完成BootNotification
		err = performBootNotification(t, wsClient, chargePointID)
		require.NoError(t, err)
	}

	t.Log("All connections established, starting message flood...")

	// 开始时间
	startTime := time.Now()

	// 并发发送消息
	var wg sync.WaitGroup

	for i, wsClient := range wsClients {
		wg.Add(1)
		go func(clientIndex int, client *utils.WebSocketClient) {
			defer wg.Done()

			for j := 0; j < messagesPerConnection; j++ {
				atomic.AddInt64(&totalMessages, 1)

				// 记录消息发送时间
				msgStartTime := time.Now()

				// 创建MeterValues消息
				meterPayload := map[string]interface{}{
					"connectorId":   1,
					"transactionId": 12345 + clientIndex,
					"meterValue": []map[string]interface{}{
						{
							"timestamp": time.Now().Format(time.RFC3339),
							"sampledValue": []map[string]interface{}{
								{
									"value":     fmt.Sprintf("%.2f", 1234.56+float64(j)*0.1),
									"measurand": "Energy.Active.Import.Register",
									"unit":      "kWh",
								},
							},
						},
					},
				}

				messageID := fmt.Sprintf("meter-%d-%d", clientIndex, j)
				meterMessage, err := utils.CreateOCPPMessage(2, messageID, "MeterValues", meterPayload)
				if err != nil {
					atomic.AddInt64(&failedMessages, 1)
					continue
				}

				// 发送消息
				err = client.SendMessage(meterMessage)
				if err != nil {
					atomic.AddInt64(&failedMessages, 1)
					continue
				}

				// 等待响应
				response, err := client.ReceiveMessage(5 * time.Second)
				if err != nil {
					atomic.AddInt64(&failedMessages, 1)
					continue
				}

				// 计算延迟
				latency := time.Since(msgStartTime)
				atomic.AddInt64(&totalLatency, latency.Nanoseconds())
				atomic.AddInt64(&messageCount, 1)
				atomic.AddInt64(&successfulMessages, 1)

				// 验证响应
				utils.AssertMeterValuesResponse(t, response, messageID)

				// 控制发送频率
				time.Sleep(messageInterval)
			}
		}(i, wsClient)
	}

	// 等待所有消息发送完成
	wg.Wait()

	totalTime := time.Since(startTime)

	// 计算统计信息
	successRate := float64(successfulMessages) / float64(totalMessages) * 100
	avgLatency := time.Duration(totalLatency / messageCount)
	throughput := float64(successfulMessages) / totalTime.Seconds()

	t.Logf("High Throughput Test Results:")
	t.Logf("  Total time: %v", totalTime)
	t.Logf("  Total messages: %d", totalMessages)
	t.Logf("  Successful messages: %d (%.2f%%)", successfulMessages, successRate)
	t.Logf("  Failed messages: %d", failedMessages)
	t.Logf("  Average latency: %v", avgLatency)
	t.Logf("  Throughput: %.2f messages/second", throughput)

	// 验证性能指标
	assert.Greater(t, successRate, 95.0, "Success rate should be > 95%")
	assert.Less(t, avgLatency, 1*time.Second, "Average latency should be < 1 second")
	assert.Greater(t, throughput, 10.0, "Throughput should be > 10 messages/second")

	t.Log("TC-E2E-05 High throughput test passed")
}

// TestTC_E2E_04_05_LoadStability 测试负载稳定性
func TestTC_E2E_04_05_LoadStability(t *testing.T) {
	// 设置测试环境
	env := utils.SetupTestEnvironment(t)
	defer env.Cleanup()

	// 测试参数
	connectionCount := 30
	testDuration := 1200 * time.Second

	t.Logf("Starting load stability test with %d connections for %v", connectionCount, testDuration)

	// 建立连接
	var wsClients []*utils.WebSocketClient
	for i := 0; i < connectionCount; i++ {
		chargePointID := fmt.Sprintf("CP-%04d", i)

		wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
		require.NoError(t, err)
		defer wsClient.Close()

		wsClients = append(wsClients, wsClient)

		err = performBootNotification(t, wsClient, chargePointID)
		require.NoError(t, err)
	}

	// 统计信息
	var (
		totalOperations int64
		successfulOps   int64
		connectionDrops int64
	)

	// 运行负载测试
	var wg sync.WaitGroup
	endTime := time.Now().Add(testDuration)

	for i, wsClient := range wsClients {
		wg.Add(1)
		go func(clientIndex int, client *utils.WebSocketClient) {
			defer wg.Done()

			operationCount := 0

			for time.Now().Before(endTime) {
				atomic.AddInt64(&totalOperations, 1)
				operationCount++

				// 随机选择操作类型
				switch operationCount % 3 {
				case 0:
					// 发送心跳
					err := sendHeartbeat(client, fmt.Sprintf("CP-%04d", clientIndex))
					if err != nil {
						atomic.AddInt64(&connectionDrops, 1)
						return
					}

				case 1:
					// 发送状态通知
					err := sendStatusNotification(client, clientIndex)
					if err != nil {
						atomic.AddInt64(&connectionDrops, 1)
						return
					}

				case 2:
					// 发送计量数据
					err := sendMeterValues(client, clientIndex, operationCount)
					if err != nil {
						atomic.AddInt64(&connectionDrops, 1)
						return
					}
				}

				atomic.AddInt64(&successfulOps, 1)

				// 随机延迟
				time.Sleep(time.Duration(100+operationCount%400) * time.Millisecond)
			}
		}(i, wsClient)
	}

	wg.Wait()

	// 计算结果
	successRate := float64(successfulOps) / float64(totalOperations) * 100
	connectionStability := float64(int64(connectionCount)-connectionDrops) / float64(connectionCount) * 100

	t.Logf("Load Stability Test Results:")
	t.Logf("  Total operations: %d", totalOperations)
	t.Logf("  Successful operations: %d (%.2f%%)", successfulOps, successRate)
	t.Logf("  Connection drops: %d", connectionDrops)
	t.Logf("  Connection stability: %.2f%%", connectionStability)

	// 验证稳定性
	assert.Greater(t, successRate, 90.0, "Success rate should be > 90%")
	assert.Greater(t, connectionStability, 95.0, "Connection stability should be > 95%")

	t.Log("Load stability test passed")
}

// sendHeartbeat 发送心跳消息
func sendHeartbeat(wsClient *utils.WebSocketClient, chargePointID string) error {
	messageID := fmt.Sprintf("heartbeat-%s-%d", chargePointID, time.Now().Unix())
	payload := map[string]interface{}{}

	message, err := utils.CreateOCPPMessage(2, messageID, "Heartbeat", payload)
	if err != nil {
		return err
	}

	err = wsClient.SendMessage(message)
	if err != nil {
		return err
	}

	_, err = wsClient.ReceiveMessage(3 * time.Second)
	return err
}

// sendStatusNotification 发送状态通知
func sendStatusNotification(wsClient *utils.WebSocketClient, clientIndex int) error {
	messageID := fmt.Sprintf("status-%d-%d", clientIndex, time.Now().Unix())
	payload := map[string]interface{}{
		"connectorId": 1,
		"errorCode":   "NoError",
		"status":      "Available",
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	message, err := utils.CreateOCPPMessage(2, messageID, "StatusNotification", payload)
	if err != nil {
		return err
	}

	err = wsClient.SendMessage(message)
	if err != nil {
		return err
	}

	_, err = wsClient.ReceiveMessage(3 * time.Second)
	return err
}

// sendMeterValues 发送计量数据
func sendMeterValues(wsClient *utils.WebSocketClient, clientIndex, sequence int) error {
	messageID := fmt.Sprintf("meter-%d-%d", clientIndex, sequence)
	payload := map[string]interface{}{
		"connectorId": 1,
		"meterValue": []map[string]interface{}{
			{
				"timestamp": time.Now().Format(time.RFC3339),
				"sampledValue": []map[string]interface{}{
					{
						"value":     fmt.Sprintf("%.2f", 1000.0+float64(sequence)*0.1),
						"measurand": "Energy.Active.Import.Register",
						"unit":      "kWh",
					},
				},
			},
		},
	}

	message, err := utils.CreateOCPPMessage(2, messageID, "MeterValues", payload)
	if err != nil {
		return err
	}

	err = wsClient.SendMessage(message)
	if err != nil {
		return err
	}

	_, err = wsClient.ReceiveMessage(3 * time.Second)
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
	time.Sleep(50 * time.Millisecond) // 减少等待时间

	return nil
}
