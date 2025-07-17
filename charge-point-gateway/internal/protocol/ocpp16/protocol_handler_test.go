package ocpp16

import (
	"context"
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/domain/ocpp16"
	"github.com/charging-platform/charge-point-gateway/internal/domain/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockModelConverter 模拟模型转换器
type MockModelConverter struct {
	convertFunc func(interface{}) (events.Event, error)
}

func NewMockModelConverter() *MockModelConverter {
	return &MockModelConverter{
		convertFunc: func(payload interface{}) (events.Event, error) {
			// 简单的模拟转换
			return &events.ChargePointConnectedEvent{
				BaseEvent: &events.BaseEvent{
					Type:          events.EventTypeChargePointConnected,
					ChargePointID: "CP001",
					Timestamp:     time.Now(),
				},
				ChargePointInfo: events.ChargePointInfo{
					Vendor: "TestVendor",
					Model:  "TestModel",
				},
			}, nil
		},
	}
}

func (m *MockModelConverter) GetSupportedActions() []string {
	return []string{"BootNotification", "Heartbeat", "StatusNotification"}
}

func (m *MockModelConverter) ConvertToUnifiedEvent(ctx context.Context, chargePointID string, action string, payload interface{}) (events.Event, error) {
	if m.convertFunc != nil {
		return m.convertFunc(payload)
	}
	return nil, nil
}

func (m *MockModelConverter) ConvertBootNotification(chargePointID string, req *ocpp16.BootNotificationRequest) (*events.ChargePointConnectedEvent, error) {
	return &events.ChargePointConnectedEvent{
		BaseEvent: &events.BaseEvent{
			Type:          events.EventTypeChargePointConnected,
			ChargePointID: chargePointID,
			Timestamp:     time.Now(),
		},
		ChargePointInfo: events.ChargePointInfo{
			Vendor: req.ChargePointVendor,
			Model:  req.ChargePointModel,
		},
	}, nil
}

func (m *MockModelConverter) ConvertHeartbeat(chargePointID string, req *ocpp16.HeartbeatRequest) (events.Event, error) {
	// 心跳事件可以使用基础事件
	return &events.ChargePointHeartbeatEvent{
		BaseEvent: events.NewBaseEvent(events.EventTypeChargePointHeartbeat, chargePointID, events.EventSeverityInfo, events.Metadata{}),
	}, nil
}

func (m *MockModelConverter) ConvertStatusNotification(chargePointID string, req *ocpp16.StatusNotificationRequest) (*events.ConnectorStatusChangedEvent, error) {
	return &events.ConnectorStatusChangedEvent{
		BaseEvent: &events.BaseEvent{
			Type:          events.EventTypeConnectorStatusChanged,
			ChargePointID: chargePointID,
			Timestamp:     time.Now(),
		},
		ConnectorInfo: events.ConnectorInfo{
			ID:            req.ConnectorId,
			ChargePointID: chargePointID,
			Status:        events.ConnectorStatus(req.Status),
		},
		PreviousStatus: events.ConnectorStatusAvailable, // 模拟前一个状态
	}, nil
}

func (m *MockModelConverter) ConvertMeterValues(chargePointID string, req *ocpp16.MeterValuesRequest) (*events.MeterValuesReceivedEvent, error) {
	return &events.MeterValuesReceivedEvent{
		BaseEvent: &events.BaseEvent{
			Type:          events.EventTypeMeterValuesReceived,
			ChargePointID: chargePointID,
			Timestamp:     time.Now(),
		},
		ConnectorID: req.ConnectorId,
		MeterValues: []events.MeterValue{},
	}, nil
}

func (m *MockModelConverter) ConvertStartTransaction(chargePointID string, req *ocpp16.StartTransactionRequest) (*events.TransactionStartedEvent, error) {
	return &events.TransactionStartedEvent{
		BaseEvent: &events.BaseEvent{
			Type:          events.EventTypeTransactionStarted,
			ChargePointID: chargePointID,
			Timestamp:     time.Now(),
		},
		TransactionInfo: events.TransactionInfo{
			ID:            12345,
			ConnectorID:   req.ConnectorId,
			ChargePointID: chargePointID,
		},
		AuthorizationInfo: events.AuthorizationInfo{
			IdTag: req.IdTag,
		},
	}, nil
}

func (m *MockModelConverter) ConvertStopTransaction(chargePointID string, req *ocpp16.StopTransactionRequest) (*events.TransactionStoppedEvent, error) {
	return &events.TransactionStoppedEvent{
		BaseEvent: &events.BaseEvent{
			Type:          events.EventTypeTransactionStopped,
			ChargePointID: chargePointID,
			Timestamp:     time.Now(),
		},
		TransactionInfo: events.TransactionInfo{
			ID:            req.TransactionId,
			ChargePointID: chargePointID,
			StopReason:    stringPtr("Local"),
		},
	}, nil
}

func TestNewProtocolHandler(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	converter := NewMockModelConverter()

	handler := NewProtocolHandler(processor, converter, nil)

	assert.NotNil(t, handler)
	assert.Equal(t, processor, handler.processor)
	assert.Equal(t, converter, handler.converter)
	assert.NotNil(t, handler.config)
	assert.NotNil(t, handler.eventChan)
	assert.NotNil(t, handler.logger)
	assert.False(t, handler.started)
}

func TestNewProtocolHandlerWithNilProcessor(t *testing.T) {
	converter := NewMockModelConverter()

	assert.Panics(t, func() {
		NewProtocolHandler(nil, converter, nil)
	})
}

func TestProtocolHandler_GetVersion(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	converter := NewMockModelConverter()
	handler := NewProtocolHandler(processor, converter, nil)

	version := handler.GetVersion()
	assert.Equal(t, protocol.OCPP_VERSION_1_6, version)
}

func TestProtocolHandler_GetSupportedActions(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	converter := NewMockModelConverter()
	handler := NewProtocolHandler(processor, converter, nil)

	actions := handler.GetSupportedActions()

	assert.NotEmpty(t, actions)
	assert.Contains(t, actions, "BootNotification")
	assert.Contains(t, actions, "Heartbeat")
	assert.Contains(t, actions, "StatusNotification")
	assert.Contains(t, actions, "MeterValues")
	assert.Contains(t, actions, "StartTransaction")
	assert.Contains(t, actions, "StopTransaction")

	// 验证包含Central System发起的动作
	assert.Contains(t, actions, "ChangeAvailability")
	assert.Contains(t, actions, "RemoteStartTransaction")
	assert.Contains(t, actions, "Reset")

	// 验证包含扩展Profile的动作
	assert.Contains(t, actions, "UpdateFirmware")
	assert.Contains(t, actions, "ReserveNow")
	assert.Contains(t, actions, "SetChargingProfile")
}

func TestProtocolHandler_StartStop(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	converter := NewMockModelConverter()
	handler := NewProtocolHandler(processor, converter, nil)

	// 测试启动
	err := handler.Start()
	assert.NoError(t, err)
	assert.True(t, handler.started)

	// 测试重复启动
	err = handler.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	// 测试停止
	err = handler.Stop()
	assert.NoError(t, err)
	assert.False(t, handler.started)

	// 测试重复停止
	err = handler.Stop()
	assert.NoError(t, err)
}

func TestProtocolHandler_ProcessMessage(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	converter := NewMockModelConverter()
	handler := NewProtocolHandler(processor, converter, nil)

	// 启动处理器
	err := handler.Start()
	require.NoError(t, err)
	defer handler.Stop()

	// 测试处理有效的OCPP消息
	ctx := context.Background()
	message := []byte(`[2,"12345","BootNotification",{"chargePointVendor":"TestVendor","chargePointModel":"TestModel"}]`)

	response, err := handler.ProcessMessage(ctx, "CP001", message)
	assert.NoError(t, err)
	assert.NotNil(t, response)

	// 验证响应类型
	processorResponse, ok := response.(*ProcessorResponse)
	assert.True(t, ok)
	assert.Equal(t, "12345", processorResponse.MessageID)
	assert.True(t, processorResponse.Success)
}

func TestProtocolHandler_ProcessMessage_InvalidMessage(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	converter := NewMockModelConverter()
	handler := NewProtocolHandler(processor, converter, nil)

	// 启动处理器
	err := handler.Start()
	require.NoError(t, err)
	defer handler.Stop()

	// 测试处理无效消息
	ctx := context.Background()
	invalidMessage := []byte(`invalid json`)

	response, err := handler.ProcessMessage(ctx, "CP001", invalidMessage)
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "OCPP message processing failed")
}

func TestProtocolHandler_GetEventChannel(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	converter := NewMockModelConverter()
	handler := NewProtocolHandler(processor, converter, nil)

	eventChan := handler.GetEventChannel()
	assert.NotNil(t, eventChan)

	// 测试通道是否可读
	select {
	case <-eventChan:
		// 通道可读
	default:
		// 通道为空，这是正常的
	}
}

func TestProtocolHandler_GetStats(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	converter := NewMockModelConverter()
	handler := NewProtocolHandler(processor, converter, nil)

	stats := handler.GetStats()

	assert.Equal(t, "ocpp1.6", stats["version"])
	assert.False(t, stats["started"].(bool))
	assert.Greater(t, stats["supported_actions"].(int), 0)
	assert.Equal(t, 1000, stats["event_channel_size"])
	assert.Equal(t, 0, stats["event_channel_len"])
	assert.Equal(t, 0, stats["pending_requests"])
}

func TestProtocolHandler_IsHealthy(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	converter := NewMockModelConverter()
	handler := NewProtocolHandler(processor, converter, nil)

	// 未启动时应该不健康
	assert.False(t, handler.IsHealthy())

	// 启动后应该健康
	err := handler.Start()
	require.NoError(t, err)
	defer handler.Stop()

	assert.True(t, handler.IsHealthy())
}

func TestProtocolHandler_EventForwarding(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	converter := NewMockModelConverter()

	// 使用较小的事件通道进行测试
	config := DefaultProtocolHandlerConfig()
	config.EventChannelSize = 10

	handler := NewProtocolHandler(processor, converter, config)

	// 启动处理器
	err := handler.Start()
	require.NoError(t, err)
	defer handler.Stop()

	// 获取事件通道
	eventChan := handler.GetEventChannel()

	// 等待一小段时间让事件转发协程启动
	time.Sleep(100 * time.Millisecond)

	// 验证事件通道可用
	assert.NotNil(t, eventChan)

	// 注意：由于我们没有实际的事件生成机制，这里只验证通道的基本功能
	select {
	case <-eventChan:
		// 如果有事件，这是正常的
	default:
		// 没有事件也是正常的
	}
}

func TestDefaultProtocolHandlerConfig(t *testing.T) {
	config := DefaultProtocolHandlerConfig()

	assert.Equal(t, 1000, config.EventChannelSize)
	assert.True(t, config.EnableEvents)
	assert.True(t, config.EnableConversion)
	assert.Equal(t, 100, config.EventBufferSize)
	assert.Equal(t, "info", config.LogLevel)
}

func TestProtocolHandler_ConvertEvent(t *testing.T) {
	processor := NewProcessor(DefaultProcessorConfig(), "pod-1", &mockConnectionStorage{})
	converter := NewMockModelConverter()
	handler := NewProtocolHandler(processor, converter, nil)

	// 创建测试事件
	originalEvent := &events.ChargePointConnectedEvent{
		BaseEvent: &events.BaseEvent{
			Type:          events.EventTypeChargePointConnected,
			ChargePointID: "CP001",
			Timestamp:     time.Now(),
		},
		ChargePointInfo: events.ChargePointInfo{
			Vendor: "TestVendor",
			Model:  "TestModel",
		},
	}

	// 测试事件转换
	convertedEvent, err := handler.convertEvent(originalEvent)
	assert.NoError(t, err)
	assert.NotNil(t, convertedEvent)

	// 验证转换后的事件
	assert.Equal(t, "CP001", convertedEvent.GetChargePointID())
	assert.Equal(t, events.EventTypeChargePointConnected, convertedEvent.GetType())
}
