package ocpp16

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/domain/ocpp16"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConnectionStorage is a mock implementation of the ConnectionStorage interface for testing.
type mockConnectionStorage struct {
	SetConnectionFunc    func(ctx context.Context, chargePointID string, gatewayID string, ttl time.Duration) error
	GetConnectionFunc    func(ctx context.Context, chargePointID string) (string, error)
	DeleteConnectionFunc func(ctx context.Context, chargePointID string) error
	CloseFunc            func() error

	// Fields to track calls
	setConnectionCalled        bool
	setConnectionChargePointID string
	setConnectionGatewayID     string
	setConnectionTTL           time.Duration
}

func (m *mockConnectionStorage) SetConnection(ctx context.Context, chargePointID string, gatewayID string, ttl time.Duration) error {
	m.setConnectionCalled = true
	m.setConnectionChargePointID = chargePointID
	m.setConnectionGatewayID = gatewayID
	m.setConnectionTTL = ttl
	if m.SetConnectionFunc != nil {
		return m.SetConnectionFunc(ctx, chargePointID, gatewayID, ttl)
	}
	return nil
}

func (m *mockConnectionStorage) GetConnection(ctx context.Context, chargePointID string) (string, error) {
	if m.GetConnectionFunc != nil {
		return m.GetConnectionFunc(ctx, chargePointID)
	}
	return "", nil
}

func (m *mockConnectionStorage) DeleteConnection(ctx context.Context, chargePointID string) error {
	if m.DeleteConnectionFunc != nil {
		return m.DeleteConnectionFunc(ctx, chargePointID)
	}
	return nil
}

func (m *mockConnectionStorage) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestDefaultProcessorConfig(t *testing.T) {
	config := DefaultProcessorConfig()

	assert.Equal(t, 1024*1024, config.MaxMessageSize)
	assert.Equal(t, 30*time.Second, config.RequestTimeout)
	assert.Equal(t, 1000, config.MaxPendingRequests)
	assert.True(t, config.EnableValidation)
	assert.False(t, config.StrictValidation)
	assert.True(t, config.ValidateMessageSize)
	assert.Equal(t, 1000, config.EventChannelSize)
	assert.True(t, config.EnableEvents)
	assert.Equal(t, 4, config.WorkerCount)
	assert.Equal(t, 1*time.Minute, config.CleanupInterval)
	assert.True(t, config.EnableMetrics)
}

func TestNewProcessor(t *testing.T) {
	config := DefaultProcessorConfig()
	mockStorage := &mockConnectionStorage{}
	processor := NewProcessor(config, "pod-1", mockStorage, nil)

	assert.NotNil(t, processor)
	assert.Equal(t, config, processor.config)
	assert.NotNil(t, processor.serializer)
	assert.NotNil(t, processor.validator)
	assert.NotNil(t, processor.eventFactory)
	assert.NotNil(t, processor.eventChan)
	assert.NotNil(t, processor.pendingRequests)
	assert.NotNil(t, processor.logger)
	assert.Equal(t, "pod-1", processor.podID)
	assert.Equal(t, mockStorage, processor.storage)
}

func TestNewProcessorWithNilConfig(t *testing.T) {
	processor := NewProcessor(nil, "pod-1", &mockConnectionStorage{}, nil)

	assert.NotNil(t, processor)
	assert.NotNil(t, processor.config)
	assert.Equal(t, DefaultProcessorConfig().MaxMessageSize, processor.config.MaxMessageSize)
}

func TestProcessor_StartStop(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{}, nil)

	// 测试启动
	err := processor.Start()
	assert.NoError(t, err)

	// 验证初始状态
	assert.Equal(t, 0, processor.GetPendingRequestCount())

	// 测试停止
	err = processor.Stop()
	assert.NoError(t, err)
}

func TestProcessor_ProcessMessage_BootNotification(t *testing.T) {
	mockStorage := &mockConnectionStorage{}
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", mockStorage)
	err := processor.Start()
	require.NoError(t, err)
	defer processor.Stop()

	// 创建BootNotification请求
	request := ocpp16.BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	}

	// 序列化为OCPP消息
	messageData := createOCPPCallMessage(t, "12345", "BootNotification", request)

	// 处理消息
	response, err := processor.ProcessMessage("CP001", messageData)
	if err != nil {
		t.Logf("Error details: %v", err)
		t.Logf("Message data: %s", string(messageData))
	}
	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, "12345", response.MessageID)
	assert.True(t, response.Success)
	assert.NotNil(t, response.Payload)

	// 验证响应类型
	bootResponse, ok := response.Payload.(*ocpp16.BootNotificationResponse)
	require.True(t, ok)
	assert.Equal(t, ocpp16.RegistrationStatusAccepted, bootResponse.Status)
	assert.Equal(t, 300, bootResponse.Interval)
}

func TestProcessor_handleBootNotification_SetConnection(t *testing.T) {
	// Setup
	mockStorage := &mockConnectionStorage{}
	processor := NewProcessor(DefaultProcessorConfig(), "pod-123", mockStorage)
	chargePointID := "CP001"
	request := &ocpp16.BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	}

	// Execute
	_, err := processor.handleBootNotification(chargePointID, request)
	require.NoError(t, err)

	// Assert
	assert.True(t, mockStorage.setConnectionCalled, "SetConnection should have been called")
	assert.Equal(t, chargePointID, mockStorage.setConnectionChargePointID)
	assert.Equal(t, "pod-123", mockStorage.setConnectionGatewayID)
	assert.Equal(t, 5*time.Minute, mockStorage.setConnectionTTL)
}

func TestProcessor_ProcessMessage_Heartbeat(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	err := processor.Start()
	require.NoError(t, err)
	defer processor.Stop()

	// 创建Heartbeat请求
	request := ocpp16.HeartbeatRequest{}

	// 序列化为OCPP消息
	messageData := createOCPPCallMessage(t, "12346", "Heartbeat", request)

	// 处理消息
	response, err := processor.ProcessMessage("CP001", messageData)
	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, "12346", response.MessageID)
	assert.True(t, response.Success)

	// 验证响应类型
	heartbeatResponse, ok := response.Payload.(*ocpp16.HeartbeatResponse)
	require.True(t, ok)
	assert.WithinDuration(t, time.Now(), heartbeatResponse.CurrentTime.Time, time.Second)
}

func TestProcessor_ProcessMessage_StatusNotification(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	err := processor.Start()
	require.NoError(t, err)
	defer processor.Stop()

	// 创建StatusNotification请求
	request := ocpp16.StatusNotificationRequest{
		ConnectorId: 1,
		ErrorCode:   ocpp16.ChargePointErrorCodeNoError,
		Status:      ocpp16.ChargePointStatusAvailable,
	}

	// 序列化为OCPP消息
	messageData := createOCPPCallMessage(t, "12347", "StatusNotification", request)

	// 处理消息
	response, err := processor.ProcessMessage("CP001", messageData)
	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, "12347", response.MessageID)
	assert.True(t, response.Success)

	// 验证响应类型
	_, ok := response.Payload.(*ocpp16.StatusNotificationResponse)
	require.True(t, ok)
}

func TestProcessor_ProcessMessage_Authorize(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	err := processor.Start()
	require.NoError(t, err)
	defer processor.Stop()

	// 创建Authorize请求
	request := ocpp16.AuthorizeRequest{
		IdTag: "RFID123456",
	}

	// 序列化为OCPP消息
	messageData := createOCPPCallMessage(t, "12348", "Authorize", request)

	// 处理消息
	response, err := processor.ProcessMessage("CP001", messageData)
	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, "12348", response.MessageID)
	assert.True(t, response.Success)

	// 验证响应类型
	authResponse, ok := response.Payload.(*ocpp16.AuthorizeResponse)
	require.True(t, ok)
	assert.Equal(t, ocpp16.AuthorizationStatusAccepted, authResponse.IdTagInfo.Status)
}

func TestProcessor_ProcessMessage_StartTransaction(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	err := processor.Start()
	require.NoError(t, err)
	defer processor.Stop()

	// 创建StartTransaction请求
	request := ocpp16.StartTransactionRequest{
		ConnectorId: 1,
		IdTag:       "RFID123456",
		MeterStart:  1000,
		Timestamp:   ocpp16.DateTime{Time: time.Now().UTC()},
	}

	// 序列化为OCPP消息
	messageData := createOCPPCallMessage(t, "12349", "StartTransaction", request)

	// 处理消息
	response, err := processor.ProcessMessage("CP001", messageData)
	require.NoError(t, err)
	require.NotNil(t, response)

	assert.Equal(t, "12349", response.MessageID)
	assert.True(t, response.Success)

	// 验证响应类型
	startResponse, ok := response.Payload.(*ocpp16.StartTransactionResponse)
	require.True(t, ok)
	assert.Equal(t, ocpp16.AuthorizationStatusAccepted, startResponse.IdTagInfo.Status)
	assert.Greater(t, startResponse.TransactionId, 0)
}

func TestProcessor_ProcessMessage_InvalidJSON(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	err := processor.Start()
	require.NoError(t, err)
	defer processor.Stop()

	// 无效的JSON
	invalidJSON := []byte(`{"invalid": json}`)

	// 处理消息应该失败
	_, err = processor.ProcessMessage("CP001", invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JSON validation failed")
}

func TestProcessor_ProcessMessage_InvalidMessageType(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	err := processor.Start()
	require.NoError(t, err)
	defer processor.Stop()

	// 无效的消息类型
	invalidMessage := []byte(`[5, "12345", "BootNotification", {}]`)

	// 处理消息应该失败
	_, err = processor.ProcessMessage("CP001", invalidMessage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid message type")
}

func TestProcessor_ProcessMessage_UnsupportedAction(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	err := processor.Start()
	require.NoError(t, err)
	defer processor.Stop()

	// 不支持的action
	unsupportedMessage := []byte(`[2, "12345", "UnsupportedAction", {}]`)

	// 处理消息应该失败
	_, err = processor.ProcessMessage("CP001", unsupportedMessage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid OCPP action")
}

func TestProcessor_ProcessMessage_MessageSizeValidation(t *testing.T) {
	config := DefaultProcessorConfig()
	config.MaxMessageSize = 10 // 设置很小的限制

	processor := NewProcessor(config, "pod-1", &mockConnectionStorage{})
	err := processor.Start()
	require.NoError(t, err)
	defer processor.Stop()

	// 创建一个超过大小限制的消息
	largeMessage := []byte(`[2, "12345", "BootNotification", {"chargePointVendor": "VeryLongVendorNameThatExceedsTheLimit"}]`)

	// 处理消息应该失败
	_, err = processor.ProcessMessage("CP001", largeMessage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message size validation failed")
}

func TestProcessor_GetEventChannel(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})

	eventChan := processor.GetEventChannel()
	assert.NotNil(t, eventChan)

	// 测试通道类型
	assert.IsType(t, (<-chan events.Event)(nil), eventChan)
}

func TestProcessor_GetPendingRequestCount(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})

	// 初始应该为0
	assert.Equal(t, 0, processor.GetPendingRequestCount())
}

// 辅助函数：创建OCPP Call消息
func createOCPPCallMessage(t *testing.T, messageID, action string, payload interface{}) []byte {
	message := []interface{}{2, messageID, action, payload}
	data, err := json.Marshal(message)
	require.NoError(t, err)
	return data
}

// 辅助函数：创建OCPP CallResult消息
func createOCPPCallResultMessage(t *testing.T, messageID string, payload interface{}) []byte {
	message := []interface{}{3, messageID, payload}
	data, err := json.Marshal(message)
	require.NoError(t, err)
	return data
}

// 辅助函数：创建OCPP CallError消息
func createOCPPCallErrorMessage(t *testing.T, messageID, errorCode, errorDescription string) []byte {
	message := []interface{}{4, messageID, errorCode, errorDescription, map[string]string{}}
	data, err := json.Marshal(message)
	require.NoError(t, err)
	return data
}
