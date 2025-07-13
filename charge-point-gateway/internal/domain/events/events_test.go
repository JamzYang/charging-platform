package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseEvent_Implementation(t *testing.T) {
	metadata := Metadata{
		Source:          "test-gateway",
		ProtocolVersion: "1.6",
		MessageID:       stringPtr("test-msg-123"),
	}

	event := NewBaseEvent(EventTypeChargePointConnected, "CP001", EventSeverityInfo, metadata)

	// 测试基础字段
	assert.NotEmpty(t, event.GetID())
	assert.Equal(t, EventTypeChargePointConnected, event.GetType())
	assert.Equal(t, "CP001", event.GetChargePointID())
	assert.Equal(t, EventSeverityInfo, event.GetSeverity())
	assert.Equal(t, metadata, event.GetMetadata())
	assert.WithinDuration(t, time.Now(), event.GetTimestamp(), time.Second)
}

func TestChargePointConnectedEvent(t *testing.T) {
	chargePointInfo := ChargePointInfo{
		ID:              "CP001",
		Vendor:          "TestVendor",
		Model:           "TestModel",
		SerialNumber:    stringPtr("SN123456"),
		FirmwareVersion: stringPtr("1.0.0"),
		ConnectorCount:  2,
		LastSeen:        time.Now().UTC(),
		ProtocolVersion: "1.6",
		SupportedFeatureProfiles: []string{"Core", "FirmwareManagement"},
	}

	metadata := Metadata{
		Source:          "test-gateway",
		ProtocolVersion: "1.6",
	}

	factory := NewEventFactory()
	event := factory.CreateChargePointConnectedEvent("CP001", chargePointInfo, metadata)

	// 测试事件属性
	assert.Equal(t, EventTypeChargePointConnected, event.GetType())
	assert.Equal(t, "CP001", event.GetChargePointID())
	assert.Equal(t, EventSeverityInfo, event.GetSeverity())

	// 测试载荷
	payload := event.GetPayload()
	assert.Equal(t, chargePointInfo, payload)

	// 测试JSON序列化
	jsonData, err := event.ToJSON()
	require.NoError(t, err)

	// 测试JSON反序列化
	var decoded ChargePointConnectedEvent
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.GetID(), decoded.GetID())
	assert.Equal(t, event.GetType(), decoded.GetType())
	assert.Equal(t, event.ChargePointInfo.ID, decoded.ChargePointInfo.ID)
	assert.Equal(t, event.ChargePointInfo.Vendor, decoded.ChargePointInfo.Vendor)
}

func TestConnectorStatusChangedEvent(t *testing.T) {
	connectorInfo := ConnectorInfo{
		ID:            1,
		ChargePointID: "CP001",
		Status:        ConnectorStatusCharging,
		MaxPower:      floatPtr(22000.0),
		ConnectorType: stringPtr("Type2"),
	}

	metadata := Metadata{
		Source:          "test-gateway",
		ProtocolVersion: "1.6",
		CorrelationID:   stringPtr("corr-123"),
	}

	factory := NewEventFactory()
	event := factory.CreateConnectorStatusChangedEvent("CP001", connectorInfo, ConnectorStatusAvailable, metadata)

	// 测试事件属性
	assert.Equal(t, EventTypeConnectorStatusChanged, event.GetType())
	assert.Equal(t, "CP001", event.GetChargePointID())
	assert.Equal(t, ConnectorStatusAvailable, event.PreviousStatus)

	// 测试载荷
	payload := event.GetPayload()
	payloadMap, ok := payload.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, payloadMap, "connector_info")
	assert.Contains(t, payloadMap, "previous_status")

	// 测试JSON序列化
	jsonData, err := event.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), "connector_info")
	assert.Contains(t, string(jsonData), "previous_status")
}

func TestTransactionStartedEvent(t *testing.T) {
	transactionInfo := TransactionInfo{
		ID:            12345,
		ChargePointID: "CP001",
		ConnectorID:   1,
		IdTag:         "RFID123456",
		Status:        TransactionStatusActive,
		StartTime:     time.Now().UTC(),
		MeterStart:    1000,
	}

	authInfo := AuthorizationInfo{
		IdTag:      "RFID123456",
		Result:     AuthorizationResultAccepted,
		ExpiryDate: timePtr(time.Now().Add(24 * time.Hour)),
	}

	metadata := Metadata{
		Source:          "test-gateway",
		ProtocolVersion: "1.6",
		UserID:          stringPtr("user123"),
	}

	factory := NewEventFactory()
	event := factory.CreateTransactionStartedEvent("CP001", transactionInfo, authInfo, metadata)

	// 测试事件属性
	assert.Equal(t, EventTypeTransactionStarted, event.GetType())
	assert.Equal(t, "CP001", event.GetChargePointID())

	// 测试载荷
	payload := event.GetPayload()
	payloadMap, ok := payload.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, payloadMap, "transaction_info")
	assert.Contains(t, payloadMap, "authorization_info")

	// 测试JSON序列化
	jsonData, err := event.ToJSON()
	require.NoError(t, err)

	// 验证JSON结构
	var decoded map[string]interface{}
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)
	assert.Contains(t, decoded, "transaction_info")
	assert.Contains(t, decoded, "authorization_info")
}

func TestMeterValuesReceivedEvent(t *testing.T) {
	meterValues := []MeterValue{
		{
			Type:      MeterValueTypeEnergyActiveImport,
			Value:     "1234.56",
			Unit:      stringPtr("kWh"),
			Timestamp: time.Now().UTC(),
		},
		{
			Type:      MeterValueTypePowerActiveImport,
			Value:     "7200",
			Unit:      stringPtr("W"),
			Phase:     stringPtr("L1"),
			Timestamp: time.Now().UTC(),
		},
	}

	metadata := Metadata{
		Source:          "test-gateway",
		ProtocolVersion: "1.6",
	}

	event := &MeterValuesReceivedEvent{
		BaseEvent:     NewBaseEvent(EventTypeMeterValuesReceived, "CP001", EventSeverityInfo, metadata),
		ConnectorID:   1,
		TransactionID: intPtr(12345),
		MeterValues:   meterValues,
	}

	// 测试事件属性
	assert.Equal(t, EventTypeMeterValuesReceived, event.GetType())
	assert.Equal(t, "CP001", event.GetChargePointID())
	assert.Equal(t, 1, event.ConnectorID)
	assert.Equal(t, 12345, *event.TransactionID)

	// 测试载荷
	payload := event.GetPayload()
	payloadMap, ok := payload.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 1, payloadMap["connector_id"])
	assert.Equal(t, intPtr(12345), payloadMap["transaction_id"])
	assert.Len(t, payloadMap["meter_values"], 2)

	// 测试JSON序列化
	jsonData, err := event.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), "meter_values")
}

func TestProtocolErrorEvent(t *testing.T) {
	errorInfo := ErrorInfo{
		Code:        ErrorCodeProtocolError,
		Description: "Invalid message format",
		Details: map[string]interface{}{
			"field":    "messageTypeId",
			"expected": "2, 3, or 4",
			"actual":   "5",
		},
		Timestamp: time.Now().UTC(),
	}

	originalMessage := map[string]interface{}{
		"messageTypeId": 5,
		"messageId":     "invalid-msg",
	}

	metadata := Metadata{
		Source:          "test-gateway",
		ProtocolVersion: "1.6",
		MessageID:       stringPtr("invalid-msg"),
	}

	factory := NewEventFactory()
	event := factory.CreateProtocolErrorEvent("CP001", errorInfo, originalMessage, metadata)

	// 测试事件属性
	assert.Equal(t, EventTypeProtocolError, event.GetType())
	assert.Equal(t, "CP001", event.GetChargePointID())
	assert.Equal(t, EventSeverityError, event.GetSeverity())

	// 测试载荷
	payload := event.GetPayload()
	payloadMap, ok := payload.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, payloadMap, "error_info")
	assert.Contains(t, payloadMap, "original_message")

	// 测试JSON序列化
	jsonData, err := event.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), "error_info")
	assert.Contains(t, string(jsonData), "original_message")
}

func TestEventInterface(t *testing.T) {
	// 测试所有事件类型都实现了Event接口
	var events []Event

	metadata := Metadata{
		Source:          "test-gateway",
		ProtocolVersion: "1.6",
	}

	factory := NewEventFactory()

	// 添加各种事件类型
	events = append(events, factory.CreateChargePointConnectedEvent("CP001", ChargePointInfo{}, metadata))
	events = append(events, factory.CreateConnectorStatusChangedEvent("CP001", ConnectorInfo{}, ConnectorStatusAvailable, metadata))
	events = append(events, factory.CreateTransactionStartedEvent("CP001", TransactionInfo{}, AuthorizationInfo{}, metadata))
	events = append(events, factory.CreateProtocolErrorEvent("CP001", ErrorInfo{}, nil, metadata))

	// 测试接口方法
	for i, event := range events {
		t.Run(string(event.GetType()), func(t *testing.T) {
			assert.NotEmpty(t, event.GetID(), "Event %d should have ID", i)
			assert.NotEmpty(t, event.GetType(), "Event %d should have type", i)
			assert.Equal(t, "CP001", event.GetChargePointID(), "Event %d should have charge point ID", i)
			assert.WithinDuration(t, time.Now(), event.GetTimestamp(), time.Second, "Event %d should have recent timestamp", i)
			assert.NotEmpty(t, event.GetSeverity(), "Event %d should have severity", i)
			assert.NotNil(t, event.GetPayload(), "Event %d should have payload", i)

			// 测试JSON序列化
			jsonData, err := event.ToJSON()
			assert.NoError(t, err, "Event %d should serialize to JSON", i)
			assert.NotEmpty(t, jsonData, "Event %d JSON should not be empty", i)

			// 验证JSON是有效的
			var decoded map[string]interface{}
			err = json.Unmarshal(jsonData, &decoded)
			assert.NoError(t, err, "Event %d JSON should be valid", i)
		})
	}
}

func TestEventTypes(t *testing.T) {
	// 测试所有事件类型常量
	eventTypes := []EventType{
		EventTypeChargePointConnected,
		EventTypeChargePointDisconnected,
		EventTypeChargePointRegistered,
		EventTypeConnectorStatusChanged,
		EventTypeTransactionStarted,
		EventTypeTransactionStopped,
		EventTypeMeterValuesReceived,
		EventTypeAuthorizationRequested,
		EventTypeRemoteCommandReceived,
		EventTypeProtocolError,
	}

	for _, eventType := range eventTypes {
		assert.NotEmpty(t, string(eventType), "Event type should not be empty")
		assert.Contains(t, string(eventType), ".", "Event type should contain namespace separator")
	}
}

// 辅助函数
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}

func timePtr(t time.Time) *time.Time {
	return &t
}
