package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/domain/ocpp16"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUnifiedModelConverter(t *testing.T) {
	converter := NewUnifiedModelConverter(nil)
	assert.NotNil(t, converter)
	assert.NotNil(t, converter.eventFactory)
	assert.NotNil(t, converter.config)
	assert.NotNil(t, converter.logger)
}

func TestUnifiedModelConverter_GetSupportedActions(t *testing.T) {
	converter := NewUnifiedModelConverter(nil)
	actions := converter.GetSupportedActions()
	
	expected := []string{
		"BootNotification",
		"Heartbeat",
		"StatusNotification", 
		"MeterValues",
		"StartTransaction",
		"StopTransaction",
	}
	
	assert.Equal(t, expected, actions)
}

func TestUnifiedModelConverter_ConvertBootNotification(t *testing.T) {
	converter := NewUnifiedModelConverter(nil)
	chargePointID := "CP001"
	
	req := &ocpp16.BootNotificationRequest{
		ChargePointVendor:      "TestVendor",
		ChargePointModel:       "TestModel",
		ChargePointSerialNumber: stringPtr("SN123456"),
		FirmwareVersion:        stringPtr("1.0.0"),
	}
	
	event, err := converter.ConvertBootNotification(chargePointID, req)
	require.NoError(t, err)
	require.NotNil(t, event)
	
	assert.Equal(t, events.EventTypeChargePointConnected, event.GetType())
	assert.Equal(t, chargePointID, event.GetChargePointID())
	assert.Equal(t, "TestVendor", event.ChargePointInfo.Vendor)
	assert.Equal(t, "TestModel", event.ChargePointInfo.Model)
	if event.ChargePointInfo.SerialNumber != nil {
		assert.Equal(t, "SN123456", *event.ChargePointInfo.SerialNumber)
	}
	if event.ChargePointInfo.FirmwareVersion != nil {
		assert.Equal(t, "1.0.0", *event.ChargePointInfo.FirmwareVersion)
	}
	assert.Equal(t, "1.6", event.ChargePointInfo.ProtocolVersion)
}

func TestUnifiedModelConverter_ConvertBootNotification_NilRequest(t *testing.T) {
	converter := NewUnifiedModelConverter(nil)
	chargePointID := "CP001"
	
	event, err := converter.ConvertBootNotification(chargePointID, nil)
	assert.Error(t, err)
	assert.Nil(t, event)
	assert.Contains(t, err.Error(), "BootNotificationRequest is nil")
}

func TestUnifiedModelConverter_ConvertHeartbeat(t *testing.T) {
	converter := NewUnifiedModelConverter(nil)
	chargePointID := "CP001"
	
	req := &ocpp16.HeartbeatRequest{}
	
	event, err := converter.ConvertHeartbeat(chargePointID, req)
	require.NoError(t, err)
	require.NotNil(t, event)

	assert.Equal(t, events.EventTypeChargePointHeartbeat, event.GetType())
	assert.Equal(t, chargePointID, event.GetChargePointID())
	assert.WithinDuration(t, time.Now(), event.GetTimestamp(), time.Second)
}

func TestUnifiedModelConverter_ConvertStatusNotification(t *testing.T) {
	converter := NewUnifiedModelConverter(nil)
	chargePointID := "CP001"
	
	req := &ocpp16.StatusNotificationRequest{
		ConnectorId: 1,
		Status:      ocpp16.ChargePointStatusAvailable,
		ErrorCode:   ocpp16.ChargePointErrorCodeNoError,
		Info:        stringPtr("Test info"),
		VendorId:    stringPtr("TestVendor"),
	}
	
	event, err := converter.ConvertStatusNotification(chargePointID, req)
	require.NoError(t, err)
	require.NotNil(t, event)
	
	assert.Equal(t, events.EventTypeConnectorStatusChanged, event.GetType())
	assert.Equal(t, chargePointID, event.GetChargePointID())
	assert.Equal(t, 1, event.ConnectorInfo.ID)
	assert.Equal(t, events.ConnectorStatusAvailable, event.ConnectorInfo.Status)
	if event.ConnectorInfo.ErrorCode != nil {
		assert.Equal(t, "NoError", *event.ConnectorInfo.ErrorCode)
	}
	if event.ConnectorInfo.ErrorDescription != nil {
		assert.Equal(t, "Test info", *event.ConnectorInfo.ErrorDescription)
	}
	if event.ConnectorInfo.VendorErrorCode != nil {
		assert.Equal(t, "TestVendor", *event.ConnectorInfo.VendorErrorCode)
	}
}

func TestUnifiedModelConverter_ConvertStatusNotification_StatusMapping(t *testing.T) {
	converter := NewUnifiedModelConverter(nil)
	chargePointID := "CP001"
	
	testCases := []struct {
		ocppStatus     ocpp16.ChargePointStatus
		expectedStatus events.ConnectorStatus
	}{
		{ocpp16.ChargePointStatusAvailable, events.ConnectorStatusAvailable},
		{ocpp16.ChargePointStatusPreparing, events.ConnectorStatusPreparing},
		{ocpp16.ChargePointStatusCharging, events.ConnectorStatusCharging},
		{ocpp16.ChargePointStatusSuspendedEVSE, events.ConnectorStatusSuspendedEVSE},
		{ocpp16.ChargePointStatusSuspendedEV, events.ConnectorStatusSuspendedEV},
		{ocpp16.ChargePointStatusFinishing, events.ConnectorStatusFinishing},
		{ocpp16.ChargePointStatusReserved, events.ConnectorStatusReserved},
		{ocpp16.ChargePointStatusUnavailable, events.ConnectorStatusUnavailable},
		{ocpp16.ChargePointStatusFaulted, events.ConnectorStatusFaulted},
	}
	
	for _, tc := range testCases {
		req := &ocpp16.StatusNotificationRequest{
			ConnectorId: 1,
			Status:      tc.ocppStatus,
			ErrorCode:   ocpp16.ChargePointErrorCodeNoError,
		}
		
		event, err := converter.ConvertStatusNotification(chargePointID, req)
		require.NoError(t, err)
		assert.Equal(t, tc.expectedStatus, event.ConnectorInfo.Status, "Status mapping failed for %s", tc.ocppStatus)
	}
}

func TestUnifiedModelConverter_ConvertMeterValues(t *testing.T) {
	converter := NewUnifiedModelConverter(nil)
	chargePointID := "CP001"
	
	timestamp := ocpp16.DateTime{Time: time.Now()}
	req := &ocpp16.MeterValuesRequest{
		ConnectorId:   1,
		TransactionId: intPtr(123),
		MeterValue: []ocpp16.MeterValue{
			{
				Timestamp: timestamp,
				SampledValue: []ocpp16.SampledValue{
					{
						Value:     "100.5",
						Context:   &[]ocpp16.ReadingContext{ocpp16.ReadingContextSamplePeriodic}[0],
						Format:    &[]ocpp16.ValueFormat{ocpp16.ValueFormatRaw}[0],
						Measurand: &[]ocpp16.Measurand{ocpp16.MeasurandEnergyActiveImportRegister}[0],
						Unit:      &[]ocpp16.UnitOfMeasure{ocpp16.UnitOfMeasureWh}[0],
					},
				},
			},
		},
	}
	
	event, err := converter.ConvertMeterValues(chargePointID, req)
	require.NoError(t, err)
	require.NotNil(t, event)
	
	assert.Equal(t, events.EventTypeMeterValuesReceived, event.GetType())
	assert.Equal(t, chargePointID, event.GetChargePointID())
	assert.Equal(t, 1, event.ConnectorID)
	if event.TransactionID != nil {
		assert.Equal(t, 123, *event.TransactionID)
	}
	assert.Len(t, event.MeterValues, 1)

	meterValue := event.MeterValues[0]
	assert.Equal(t, timestamp.Time, meterValue.Timestamp)
	assert.Equal(t, events.MeterValueTypeEnergyActiveImport, meterValue.Type)
	assert.Equal(t, "100.5", meterValue.Value)
	if meterValue.Context != nil {
		assert.Equal(t, "Sample.Periodic", *meterValue.Context)
	}
	if meterValue.Unit != nil {
		assert.Equal(t, "Wh", *meterValue.Unit)
	}
}

func TestUnifiedModelConverter_ConvertToUnifiedEvent(t *testing.T) {
	converter := NewUnifiedModelConverter(nil)
	chargePointID := "CP001"
	ctx := context.Background()
	
	// 测试BootNotification
	bootReq := &ocpp16.BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	}
	
	event, err := converter.ConvertToUnifiedEvent(ctx, chargePointID, "BootNotification", bootReq)
	require.NoError(t, err)
	assert.Equal(t, events.EventTypeChargePointConnected, event.GetType())
	
	// 测试不支持的动作
	_, err = converter.ConvertToUnifiedEvent(ctx, chargePointID, "UnsupportedAction", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported OCPP action")
	
	// 测试错误的payload类型
	_, err = converter.ConvertToUnifiedEvent(ctx, chargePointID, "BootNotification", "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payload type")
}

// 辅助函数
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
