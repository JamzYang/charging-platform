package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertOCPPMessage 断言OCPP消息格式
func AssertOCPPMessage(t *testing.T, data []byte, expectedMessageType int, expectedAction string) {
	var message []interface{}
	err := json.Unmarshal(data, &message)
	require.NoError(t, err, "Failed to unmarshal OCPP message")
	require.Len(t, message, 4, "OCPP message should have 4 elements")

	// 检查消息类型
	messageType, ok := message[0].(float64)
	require.True(t, ok, "Message type should be a number")
	assert.Equal(t, expectedMessageType, int(messageType), "Message type mismatch")

	// 检查消息ID
	messageID, ok := message[1].(string)
	require.True(t, ok, "Message ID should be a string")
	assert.NotEmpty(t, messageID, "Message ID should not be empty")

	// 检查Action（仅对CALL消息）
	if expectedMessageType == 2 {
		action, ok := message[2].(string)
		require.True(t, ok, "Action should be a string")
		assert.Equal(t, expectedAction, action, "Action mismatch")
	}
}

// AssertOCPPCallResult 断言OCPP CALLRESULT消息
func AssertOCPPCallResult(t *testing.T, data []byte, expectedMessageID string) map[string]interface{} {
	var message []interface{}
	err := json.Unmarshal(data, &message)
	require.NoError(t, err, "Failed to unmarshal OCPP message")
	require.Len(t, message, 3, "CALLRESULT message should have 3 elements")

	// 检查消息类型
	messageType, ok := message[0].(float64)
	require.True(t, ok, "Message type should be a number")
	assert.Equal(t, 3, int(messageType), "Should be CALLRESULT message")

	// 检查消息ID
	messageID, ok := message[1].(string)
	require.True(t, ok, "Message ID should be a string")
	assert.Equal(t, expectedMessageID, messageID, "Message ID mismatch")

	// 返回载荷
	payload, ok := message[2].(map[string]interface{})
	require.True(t, ok, "Payload should be an object")
	return payload
}

// AssertOCPPCallError 断言OCPP CALLERROR消息
func AssertOCPPCallError(t *testing.T, data []byte, expectedMessageID string) (string, string, map[string]interface{}) {
	var message []interface{}
	err := json.Unmarshal(data, &message)
	require.NoError(t, err, "Failed to unmarshal OCPP message")
	require.Len(t, message, 4, "CALLERROR message should have 4 elements")

	// 检查消息类型
	messageType, ok := message[0].(float64)
	require.True(t, ok, "Message type should be a number")
	assert.Equal(t, 4, int(messageType), "Should be CALLERROR message")

	// 检查消息ID
	messageID, ok := message[1].(string)
	require.True(t, ok, "Message ID should be a string")
	assert.Equal(t, expectedMessageID, messageID, "Message ID mismatch")

	// 检查错误代码
	errorCode, ok := message[2].(string)
	require.True(t, ok, "Error code should be a string")

	// 检查错误描述
	errorDescription, ok := message[3].(string)
	require.True(t, ok, "Error description should be a string")

	// 返回错误信息
	return errorCode, errorDescription, nil
}

// AssertRedisConnection 断言Redis连接映射
func AssertRedisConnection(t *testing.T, redisClient *redis.Client, chargePointID, expectedPodID string) {
	ctx := context.Background()
	key := fmt.Sprintf("conn:%s", chargePointID)

	result, err := redisClient.Get(ctx, key).Result()
	require.NoError(t, err, "Failed to get connection mapping from Redis")
	assert.Equal(t, expectedPodID, result, "Pod ID mismatch in Redis")
}

// AssertRedisConnectionNotExists 断言Redis连接映射不存在
func AssertRedisConnectionNotExists(t *testing.T, redisClient *redis.Client, chargePointID string) {
	ctx := context.Background()
	key := fmt.Sprintf("conn:%s", chargePointID)

	_, err := redisClient.Get(ctx, key).Result()
	assert.Equal(t, redis.Nil, err, "Connection mapping should not exist in Redis")
}

// AssertKafkaMessage 断言Kafka消息
func AssertKafkaMessage(t *testing.T, message []byte, expectedEventType string) map[string]interface{} {
	var event map[string]interface{}
	err := json.Unmarshal(message, &event)
	require.NoError(t, err, "Failed to unmarshal Kafka message")

	// 检查事件类型
	eventType, ok := event["type"].(string)
	require.True(t, ok, "Event type should be a string")
	assert.Equal(t, expectedEventType, eventType, "Event type mismatch")

	// 检查基础字段
	assert.Contains(t, event, "id", "Event should have ID")
	assert.Contains(t, event, "timestamp", "Event should have timestamp")
	assert.Contains(t, event, "charge_point_id", "Event should have charge point ID")

	return event
}

// AssertEventuallyTrue 断言条件最终为真
func AssertEventuallyTrue(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	deadline := time.Now().Add(timeout)
	interval := timeout / 20 // 检查20次

	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}

	t.Fatalf("Condition not met within timeout: %s", message)
}

// AssertBootNotificationResponse 断言BootNotification响应
func AssertBootNotificationResponse(t *testing.T, data []byte, messageID string) {
	payload := AssertOCPPCallResult(t, data, messageID)

	// 检查状态
	status, ok := payload["status"].(string)
	require.True(t, ok, "Status should be a string")
	assert.Equal(t, "Accepted", status, "BootNotification should be accepted")

	// 检查心跳间隔
	assert.Contains(t, payload, "interval", "Response should contain heartbeat interval")
	interval, ok := payload["interval"].(float64)
	require.True(t, ok, "Interval should be a number")
	assert.Greater(t, interval, float64(0), "Heartbeat interval should be positive")

	// 检查当前时间
	assert.Contains(t, payload, "currentTime", "Response should contain current time")
}

// AssertMeterValuesResponse 断言MeterValues响应
func AssertMeterValuesResponse(t *testing.T, data []byte, messageID string) {
	payload := AssertOCPPCallResult(t, data, messageID)

	// MeterValues响应通常是空的
	assert.Empty(t, payload, "MeterValues response should be empty")
}

// AssertStatusNotificationResponse 断言StatusNotification响应
func AssertStatusNotificationResponse(t *testing.T, data []byte, messageID string) {
	payload := AssertOCPPCallResult(t, data, messageID)

	// StatusNotification响应通常是空的
	assert.Empty(t, payload, "StatusNotification response should be empty")
}

// AssertRemoteStartTransactionRequest 断言RemoteStartTransaction请求
func AssertRemoteStartTransactionRequest(t *testing.T, data []byte) (string, map[string]interface{}) {
	var message []interface{}
	err := json.Unmarshal(data, &message)
	require.NoError(t, err, "Failed to unmarshal OCPP message")
	require.Len(t, message, 4, "CALL message should have 4 elements")

	// 检查消息类型和Action
	messageType := int(message[0].(float64))
	assert.Equal(t, 2, messageType, "Should be CALL message")

	action := message[2].(string)
	assert.Equal(t, "RemoteStartTransaction", action, "Action should be RemoteStartTransaction")

	messageID := message[1].(string)
	payload := message[3].(map[string]interface{})

	// 检查必要字段
	assert.Contains(t, payload, "idTag", "Payload should contain idTag")

	return messageID, payload
}

// AssertDeviceOnlineEvent 断言设备上线事件
func AssertDeviceOnlineEvent(t *testing.T, message []byte, expectedChargePointID string) {
	event := AssertKafkaMessage(t, message, "device.online")

	// 检查充电桩ID
	chargePointID, ok := event["charge_point_id"].(string)
	require.True(t, ok, "Charge point ID should be a string")
	assert.Equal(t, expectedChargePointID, chargePointID, "Charge point ID mismatch")

	// 检查载荷
	assert.Contains(t, event, "payload", "Event should have payload")
	payload, ok := event["payload"].(map[string]interface{})
	require.True(t, ok, "Payload should be an object")

	// 检查设备信息
	assert.Contains(t, payload, "vendor", "Payload should contain vendor")
	assert.Contains(t, payload, "model", "Payload should contain model")
}

// AssertMeterValuesEvent 断言计量数据事件
func AssertMeterValuesEvent(t *testing.T, message []byte, expectedChargePointID string) {
	event := AssertKafkaMessage(t, message, "meter_values.received")

	// 检查充电桩ID
	chargePointID, ok := event["charge_point_id"].(string)
	require.True(t, ok, "Charge point ID should be a string")
	assert.Equal(t, expectedChargePointID, chargePointID, "Charge point ID mismatch")

	// 检查计量数据 - 直接从事件根级别读取
	assert.Contains(t, event, "connector_id", "Event should contain connector ID")
	assert.Contains(t, event, "meter_values", "Event should contain meter values")

	// 验证连接器ID
	connectorID, ok := event["connector_id"].(float64)
	require.True(t, ok, "Connector ID should be a number")
	assert.Greater(t, connectorID, float64(0), "Connector ID should be positive")

	// 验证计量数据
	meterValues, ok := event["meter_values"].([]interface{})
	require.True(t, ok, "Meter values should be an array")
	assert.NotEmpty(t, meterValues, "Meter values should not be empty")
}

// ReceiveMessageWithTimeout 尝试在给定的超时时间内接收一条消息
func ReceiveMessageWithTimeout(wsClient *WebSocketClient, timeout time.Duration) ([]byte, error) {
	resultChan := make(chan struct {
		response []byte
		err      error
	}, 1)

	go func() {
		// 使用一个稍长的内部超时以确保select先超时
		response, err := wsClient.ReceiveMessage(timeout + 50*time.Millisecond)
		resultChan <- struct {
			response []byte
			err      error
		}{response, err}
	}()

	select {
	case result := <-resultChan:
		return result.response, result.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timed out after %v waiting for message", timeout)
	}
}
