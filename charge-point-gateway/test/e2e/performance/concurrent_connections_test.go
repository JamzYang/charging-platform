package performance

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
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

	// 测试参数 - 支持环境变量配置（用于分布式测试）
	connectionCount := getEnvInt("CONNECTION_COUNT", 10000) // 默认10000，可通过环境变量覆盖
	idOffset := getEnvInt("ID_OFFSET", 0)                   // ID偏移量，用于分布式测试
	clientID := getEnvString("CLIENT_ID", "single")         // 客户端ID，用于日志标识
	testDuration := 420 * time.Second                       // 缩短测试时间

	// 优化连接建立的批次控制，减少TCP连接风暴
	batchSize := 50                      // 进一步减小批次大小，避免TCP监听队列溢出
	batchDelay := 200 * time.Millisecond // 增加批次间延迟，给系统更多处理时间

	t.Logf("[Client-%s] Starting concurrent connections test with %d connections for %v", clientID, connectionCount, testDuration)
	t.Logf("[Client-%s] ID range: CP-%d to CP-%d", clientID, idOffset, idOffset+connectionCount-1)
	t.Logf("[Client-%s] Using batch connection strategy: %d connections per batch, %v delay between batches", clientID, batchSize, batchDelay)

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

	// 创建并发连接 - 使用批次控制
	for batch := 0; batch < (connectionCount+batchSize-1)/batchSize; batch++ {
		batchStart := batch * batchSize
		batchEnd := batchStart + batchSize
		if batchEnd > connectionCount {
			batchEnd = connectionCount
		}

		// 启动当前批次的连接
		for i := batchStart; i < batchEnd; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				chargePointID := fmt.Sprintf("CP-%05d", idOffset+index)

				// 创建WebSocket连接
				wsClient, err := utils.NewWebSocketClient(env.GatewayURL, chargePointID)
				if err != nil {
					atomic.AddInt64(&failedConnections, 1)
					// Only log the first few errors to avoid log spam
					failedCount := atomic.LoadInt64(&failedConnections)
					if failedCount <= 10 || failedCount%1000 == 0 {
						t.Logf("[Client-%s] Failed to connect %s (total failed: %d): %v", clientID, chargePointID, failedCount, err)
					}
					return
				}
				defer wsClient.Close()

				atomic.AddInt64(&successfulConnections, 1)

				// 每1000个成功连接记录一次进度
				successCount := atomic.LoadInt64(&successfulConnections)
				if successCount%1000 == 0 {
					t.Logf("[Client-%s] Progress: %d successful connections established", clientID, successCount)
				}

				// 执行BootNotification
				err = performBootNotification(t, wsClient, chargePointID)
				if err != nil {
					// t.Logf("BootNotification failed for %s: %v", chargePointID, err)
					return
				}

				// 保持连接活跃，依赖WebSocket自动ping/pong机制
				// 移除高频应用层心跳，避免性能问题
				ticker := time.NewTicker(60 * time.Second) // 降低检查频率
				defer ticker.Stop()

				endTime := startTime.Add(testDuration)

				for time.Now().Before(endTime) {
					select {
					case <-ticker.C:
						// 偶尔发送轻量级状态消息，模拟真实充电桩行为
						if rand.Intn(10) == 0 { // 10%的概率发送状态
							atomic.AddInt64(&totalMessages, 1)
							err := sendStatusNotification(wsClient, int(atomic.LoadInt64(&successfulConnections)))
							if err != nil {
								atomic.AddInt64(&failedMessages, 1)
							} else {
								atomic.AddInt64(&successfulMessages, 1)
							}
						}
					case <-time.After(1 * time.Second):
						// 保持循环运行，让WebSocket自动处理ping/pong
						continue
					}
				}
			}(i)
		}

		// 批次间延迟，避免连接风暴
		if batch < (connectionCount+batchSize-1)/batchSize-1 {
			time.Sleep(batchDelay)
			t.Logf("[Client-%s] Batch %d completed, waiting %v before next batch...", clientID, batch+1, batchDelay)
		}
	}

	// 等待所有连接完成
	wg.Wait()

	totalTime := time.Since(startTime)

	// 计算统计信息
	successConnRate := float64(successfulConnections) / float64(connectionCount) * 100
	messageSuccessRate := float64(successfulMessages) / float64(totalMessages) * 100

	// 验证结果
	t.Logf("[Client-%s] Test Results:", clientID)
	t.Logf("[Client-%s]   Total time: %v", clientID, totalTime)
	t.Logf("[Client-%s]   Successful connections: %d/%d (%.2f%%)", clientID, successfulConnections, connectionCount, successConnRate)
	t.Logf("[Client-%s]   Failed connections: %d", clientID, failedConnections)
	t.Logf("[Client-%s]   Total messages: %d", clientID, totalMessages)
	t.Logf("[Client-%s]   Successful messages: %d (%.2f%%)", clientID, successfulMessages, messageSuccessRate)
	t.Logf("[Client-%s]   Failed messages: %d", clientID, failedMessages)

	// 断言：至少80%的连接应该成功
	assert.Greater(t, successConnRate, 99.0, "At least 80% of connections should succeed")

	// 断言：至少90%的消息应该成功
	if totalMessages > 0 {
		assert.Greater(t, messageSuccessRate, 99.0, "At least 90% of messages should succeed")
	}

	if t.Failed() {
		t.Log("TC-E2E-04 Concurrent connections test FAILED")
	} else {
		t.Log("TC-E2E-04 Concurrent connections test PASSED")
	}
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
					// 发送心跳（异步，不等待响应）
					err := sendHeartbeatAsync(client, fmt.Sprintf("CP-%04d", clientIndex))
					if err != nil {
						atomic.AddInt64(&connectionDrops, 1)
						// 发送失败不立即退出，继续尝试
						continue
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

// sendHeartbeatAsync 异步发送心跳，不等待响应
func sendHeartbeatAsync(wsClient *utils.WebSocketClient, chargePointID string) error {
	messageID := fmt.Sprintf("heartbeat-%s-%d", chargePointID, time.Now().Unix())
	payload := map[string]interface{}{}

	message, err := utils.CreateOCPPMessage(2, messageID, "Heartbeat", payload)
	if err != nil {
		return err
	}

	// 只发送消息，不等待响应，避免阻塞
	err = wsClient.SendMessage(message)
	return err
}

// sendStatusNotification 异步发送状态通知，不等待响应
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

	// 只发送消息，不等待响应，避免阻塞
	err = wsClient.SendMessage(message)
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

// getEnvInt 获取环境变量整数值，如果不存在则返回默认值
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvString 获取环境变量字符串值，如果不存在则返回默认值
func getEnvString(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
