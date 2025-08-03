package message

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationEventFormat_MeterValues 测试电表值事件的完整集成格式
func TestIntegrationEventFormat_MeterValues(t *testing.T) {
	converter := NewIntegrationEventConverter("gateway-pod-xyz")

	// 创建符合实际场景的电表值事件
	metadata := events.Metadata{
		Source:          "ocpp16-processor",
		ProtocolVersion: "1.6",
	}

	meterValues := []events.MeterValue{
		{
			Type:      events.MeterValueTypeEnergyActiveImport,
			Value:     "95.70",
			Unit:      stringPtr("kWh"),
			Location:  stringPtr("Outlet"),
			Context:   stringPtr("Sample.Periodic"),
			Timestamp: time.Date(2025, 8, 3, 8, 34, 2, 280000000, time.UTC),
		},
		{
			Type:      events.MeterValueTypePowerActiveImport,
			Value:     "7958",
			Unit:      stringPtr("W"),
			Location:  stringPtr("Outlet"),
			Context:   stringPtr("Sample.Periodic"),
			Timestamp: time.Date(2025, 8, 3, 8, 34, 2, 280000000, time.UTC),
		},
		{
			Type:      events.MeterValueTypeVoltage,
			Value:     "228.0",
			Unit:      stringPtr("V"),
			Phase:     stringPtr("L1"),
			Location:  stringPtr("Outlet"),
			Context:   stringPtr("Sample.Periodic"),
			Timestamp: time.Date(2025, 8, 3, 8, 34, 2, 280000000, time.UTC),
		},
	}

	internalEvent := &events.MeterValuesReceivedEvent{
		BaseEvent:     events.NewBaseEvent(events.EventTypeMeterValuesReceived, "CP673b4f7acfdb428a8e7a", events.EventSeverityInfo, metadata),
		ConnectorID:   1,
		TransactionID: intPtr(634),
		MeterValues:   meterValues,
	}

	// 转换为集成事件格式
	integrationEvent := converter.ConvertToIntegrationFormat(internalEvent)

	// 序列化为JSON
	jsonData, err := json.Marshal(integrationEvent)
	require.NoError(t, err)

	// 解析JSON以验证结构
	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	require.NoError(t, err)

	// 验证顶层结构符合对接文档
	assert.Contains(t, result, "eventId")
	assert.Contains(t, result, "eventType")
	assert.Contains(t, result, "chargePointId")
	assert.Contains(t, result, "gatewayId")
	assert.Contains(t, result, "timestamp")
	assert.Contains(t, result, "payload")

	// 验证具体值
	assert.Equal(t, "transaction.meter_values", result["eventType"])
	assert.Equal(t, "CP673b4f7acfdb428a8e7a", result["chargePointId"])
	assert.Equal(t, "gateway-pod-xyz", result["gatewayId"])

	// 验证载荷结构
	payload, ok := result["payload"].(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, float64(1), payload["connectorId"]) // JSON数字解析为float64
	assert.Equal(t, float64(634), payload["transactionId"])

	meterValuesPayload, ok := payload["meterValues"].([]interface{})
	require.True(t, ok)
	require.Len(t, meterValuesPayload, 3)

	// 验证第一个电表值（能量）
	firstMeterValue, ok := meterValuesPayload[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "2025-08-03T08:34:02Z", firstMeterValue["timestamp"])

	firstSampledValue, ok := firstMeterValue["sampledValue"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "95.70", firstSampledValue["value"])
	assert.Equal(t, "Energy.Active.Import.Register", firstSampledValue["measurand"])
	assert.Equal(t, "kWh", firstSampledValue["unit"])

	// 验证第二个电表值（功率）
	secondMeterValue, ok := meterValuesPayload[1].(map[string]interface{})
	require.True(t, ok)

	secondSampledValue, ok := secondMeterValue["sampledValue"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "7958", secondSampledValue["value"])
	assert.Equal(t, "Power.Active.Import", secondSampledValue["measurand"])
	assert.Equal(t, "W", secondSampledValue["unit"])

	// 验证第三个电表值（电压）
	thirdMeterValue, ok := meterValuesPayload[2].(map[string]interface{})
	require.True(t, ok)

	thirdSampledValue, ok := thirdMeterValue["sampledValue"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "228.0", thirdSampledValue["value"])
	assert.Equal(t, "Voltage", thirdSampledValue["measurand"])
	assert.Equal(t, "V", thirdSampledValue["unit"])

	// 打印完整的JSON以便调试
	t.Logf("Generated integration event JSON:\n%s", string(jsonData))
}

// TestIntegrationEventFormat_ChargePointConnected 测试充电桩连接事件
func TestIntegrationEventFormat_ChargePointConnected(t *testing.T) {
	converter := NewIntegrationEventConverter("gateway-pod-abc")

	metadata := events.Metadata{
		Source:          "ocpp16-processor",
		ProtocolVersion: "1.6",
	}

	chargePointInfo := events.ChargePointInfo{
		ID:              "CP-001",
		Vendor:          "Vendor-A",
		Model:           "Model-X",
		FirmwareVersion: stringPtr("v1.2.3"),
	}

	internalEvent := &events.ChargePointConnectedEvent{
		BaseEvent:       events.NewBaseEvent(events.EventTypeChargePointConnected, "CP-001", events.EventSeverityInfo, metadata),
		ChargePointInfo: chargePointInfo,
	}

	// 转换为集成事件格式
	integrationEvent := converter.ConvertToIntegrationFormat(internalEvent)

	// 序列化为JSON
	jsonData, err := json.Marshal(integrationEvent)
	require.NoError(t, err)

	// 解析JSON以验证结构
	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	require.NoError(t, err)

	// 验证符合对接文档格式
	assert.Equal(t, "charge_point.connected", result["eventType"])
	assert.Equal(t, "CP-001", result["chargePointId"])
	assert.Equal(t, "gateway-pod-abc", result["gatewayId"])

	// 验证载荷结构符合对接文档示例
	payload, ok := result["payload"].(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, "Model-X", payload["model"])
	assert.Equal(t, "Vendor-A", payload["vendor"])
	assert.Equal(t, "v1.2.3", payload["firmwareVersion"])

	t.Logf("Generated charge point connected event JSON:\n%s", string(jsonData))
}

// TestIntegrationEventFormat_ConnectorStatusChanged 测试连接器状态变更事件
func TestIntegrationEventFormat_ConnectorStatusChanged(t *testing.T) {
	converter := NewIntegrationEventConverter("gateway-pod-def")

	metadata := events.Metadata{
		Source:          "ocpp16-processor",
		ProtocolVersion: "1.6",
	}

	connectorInfo := events.ConnectorInfo{
		ID:            1,
		ChargePointID: "CP-002",
		Status:        events.ConnectorStatusCharging,
		ErrorCode:     stringPtr("NoError"),
	}

	internalEvent := &events.ConnectorStatusChangedEvent{
		BaseEvent:      events.NewBaseEvent(events.EventTypeConnectorStatusChanged, "CP-002", events.EventSeverityInfo, metadata),
		ConnectorInfo:  connectorInfo,
		PreviousStatus: events.ConnectorStatusPreparing,
	}

	// 转换为集成事件格式
	integrationEvent := converter.ConvertToIntegrationFormat(internalEvent)

	// 序列化为JSON
	jsonData, err := json.Marshal(integrationEvent)
	require.NoError(t, err)

	// 解析JSON以验证结构
	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	require.NoError(t, err)

	// 验证符合对接文档格式
	assert.Equal(t, "connector.status_changed", result["eventType"])
	assert.Equal(t, "CP-002", result["chargePointId"])
	assert.Equal(t, "gateway-pod-def", result["gatewayId"])

	// 验证载荷结构符合对接文档示例
	payload, ok := result["payload"].(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, float64(1), payload["connectorId"])
	assert.Equal(t, "Charging", payload["status"])
	assert.Equal(t, "Preparing", payload["previousStatus"])
	assert.Equal(t, "NoError", payload["errorCode"])

	t.Logf("Generated connector status changed event JSON:\n%s", string(jsonData))
}
