package message

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationEventConverter_ConvertToIntegrationFormat(t *testing.T) {
	converter := NewIntegrationEventConverter("gateway-pod-123")

	// 创建测试用的内部事件
	metadata := events.Metadata{
		Source:          "test-gateway",
		ProtocolVersion: "1.6",
	}

	// 测试 MeterValuesReceivedEvent 转换
	meterValues := []events.MeterValue{
		{
			Type:      events.MeterValueTypeEnergyActiveImport,
			Value:     "1234.56",
			Unit:      stringPtr("kWh"),
			Timestamp: time.Date(2025, 8, 3, 8, 34, 2, 0, time.UTC),
		},
		{
			Type:      events.MeterValueTypePowerActiveImport,
			Value:     "7200",
			Unit:      stringPtr("W"),
			Phase:     stringPtr("L1"),
			Timestamp: time.Date(2025, 8, 3, 8, 34, 2, 0, time.UTC),
		},
	}

	internalEvent := &events.MeterValuesReceivedEvent{
		BaseEvent:     events.NewBaseEvent(events.EventTypeMeterValuesReceived, "CP-001", events.EventSeverityInfo, metadata),
		ConnectorID:   1,
		TransactionID: intPtr(12345),
		MeterValues:   meterValues,
	}

	// 转换为集成事件格式
	integrationEvent := converter.ConvertToIntegrationFormat(internalEvent)

	// 验证基本字段
	assert.NotEmpty(t, integrationEvent.EventID)
	assert.Equal(t, "transaction.meter_values", integrationEvent.EventType)
	assert.Equal(t, "CP-001", integrationEvent.ChargePointID)
	assert.Equal(t, "gateway-pod-123", integrationEvent.GatewayID)
	assert.NotEmpty(t, integrationEvent.Timestamp)

	// 验证载荷结构
	payload, ok := integrationEvent.Payload.(map[string]interface{})
	require.True(t, ok, "Payload should be a map")

	assert.Equal(t, 1, payload["connectorId"])
	assert.Equal(t, 12345, payload["transactionId"])

	meterValuesPayload, ok := payload["meterValues"].([]map[string]interface{})
	require.True(t, ok, "meterValues should be an array of maps")
	require.Len(t, meterValuesPayload, 2)

	// 验证第一个电表值
	firstMeterValue := meterValuesPayload[0]
	assert.Equal(t, "2025-08-03T08:34:02Z", firstMeterValue["timestamp"])
	
	sampledValue, ok := firstMeterValue["sampledValue"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "1234.56", sampledValue["value"])
	assert.Equal(t, "Energy.Active.Import.Register", sampledValue["measurand"])
	assert.Equal(t, "kWh", sampledValue["unit"])
}

func TestIntegrationEventConverter_ConvertChargePointConnectedEvent(t *testing.T) {
	converter := NewIntegrationEventConverter("gateway-pod-456")

	metadata := events.Metadata{
		Source:          "test-gateway",
		ProtocolVersion: "1.6",
	}

	chargePointInfo := events.ChargePointInfo{
		ID:              "CP-002",
		Vendor:          "TestVendor",
		Model:           "TestModel",
		FirmwareVersion: stringPtr("v1.2.3"),
	}

	internalEvent := &events.ChargePointConnectedEvent{
		BaseEvent:       events.NewBaseEvent(events.EventTypeChargePointConnected, "CP-002", events.EventSeverityInfo, metadata),
		ChargePointInfo: chargePointInfo,
	}

	// 转换为集成事件格式
	integrationEvent := converter.ConvertToIntegrationFormat(internalEvent)

	// 验证基本字段
	assert.Equal(t, "charge_point.connected", integrationEvent.EventType)
	assert.Equal(t, "CP-002", integrationEvent.ChargePointID)
	assert.Equal(t, "gateway-pod-456", integrationEvent.GatewayID)

	// 验证载荷结构
	payload, ok := integrationEvent.Payload.(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, "TestModel", payload["model"])
	assert.Equal(t, "TestVendor", payload["vendor"])
	assert.Equal(t, "v1.2.3", payload["firmwareVersion"])
}

func TestIntegrationEventConverter_SerializesToValidJSON(t *testing.T) {
	converter := NewIntegrationEventConverter("gateway-pod-789")

	metadata := events.Metadata{
		Source:          "test-gateway",
		ProtocolVersion: "1.6",
	}

	internalEvent := &events.ChargePointDisconnectedEvent{
		BaseEvent: events.NewBaseEvent(events.EventTypeChargePointDisconnected, "CP-003", events.EventSeverityInfo, metadata),
	}

	// 转换为集成事件格式
	integrationEvent := converter.ConvertToIntegrationFormat(internalEvent)

	// 序列化为JSON
	jsonData, err := json.Marshal(integrationEvent)
	require.NoError(t, err)

	// 验证JSON结构
	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	require.NoError(t, err)

	// 验证必需字段存在
	assert.Contains(t, result, "eventId")
	assert.Contains(t, result, "eventType")
	assert.Contains(t, result, "chargePointId")
	assert.Contains(t, result, "gatewayId")
	assert.Contains(t, result, "timestamp")
	assert.Contains(t, result, "payload")

	// 验证字段值
	assert.Equal(t, "charge_point.disconnected", result["eventType"])
	assert.Equal(t, "CP-003", result["chargePointId"])
	assert.Equal(t, "gateway-pod-789", result["gatewayId"])
}

func TestMapEventType(t *testing.T) {
	converter := NewIntegrationEventConverter("test-gateway")

	testCases := []struct {
		internal events.EventType
		expected string
	}{
		{events.EventTypeChargePointConnected, "charge_point.connected"},
		{events.EventTypeChargePointDisconnected, "charge_point.disconnected"},
		{events.EventTypeConnectorStatusChanged, "connector.status_changed"},
		{events.EventTypeTransactionStarted, "transaction.started"},
		{events.EventTypeMeterValuesReceived, "transaction.meter_values"},
		{events.EventTypeTransactionStopped, "transaction.stopped"},
		{events.EventTypeRemoteCommandExecuted, "command.response"},
		{events.EventType("unknown.event"), "unknown.event"}, // 未映射的事件类型保持原样
	}

	for _, tc := range testCases {
		t.Run(string(tc.internal), func(t *testing.T) {
			result := converter.mapEventType(tc.internal)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMapMeterValueType(t *testing.T) {
	converter := NewIntegrationEventConverter("test-gateway")

	testCases := []struct {
		internal events.MeterValueType
		expected string
	}{
		{events.MeterValueTypeEnergyActiveImport, "Energy.Active.Import.Register"},
		{events.MeterValueTypePowerActiveImport, "Power.Active.Import"},
		{events.MeterValueTypeVoltage, "Voltage"},
		{events.MeterValueTypeCurrentImport, "Current.Import"},
		{events.MeterValueType("unknown"), "unknown"}, // 未映射的类型保持原样
	}

	for _, tc := range testCases {
		t.Run(string(tc.internal), func(t *testing.T) {
			result := converter.mapMeterValueType(tc.internal)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// 辅助函数
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
