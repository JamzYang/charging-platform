package ocpp16

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDateTime_MarshalJSON(t *testing.T) {
	dt := DateTime{Time: time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC)}
	
	data, err := json.Marshal(dt)
	require.NoError(t, err)
	
	expected := `"2023-12-25T10:30:45Z"`
	assert.Equal(t, expected, string(data))
}

func TestDateTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "valid RFC3339 time",
			input:    `"2023-12-25T10:30:45Z"`,
			expected: time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "valid RFC3339 time with timezone",
			input:    `"2023-12-25T10:30:45+08:00"`,
			expected: time.Date(2023, 12, 25, 2, 30, 45, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:    "null value",
			input:   `null`,
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   `"invalid-time"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dt DateTime
			err := json.Unmarshal([]byte(tt.input), &dt)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.input != `null` {
					assert.True(t, tt.expected.Equal(dt.Time))
				}
			}
		})
	}
}

func TestBootNotificationRequest_JSON(t *testing.T) {
	req := BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
		FirmwareVersion:   stringPtr("1.0.0"),
	}

	// 测试序列化
	data, err := json.Marshal(req)
	require.NoError(t, err)

	// 测试反序列化
	var decoded BootNotificationRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, req.ChargePointVendor, decoded.ChargePointVendor)
	assert.Equal(t, req.ChargePointModel, decoded.ChargePointModel)
	assert.Equal(t, req.FirmwareVersion, decoded.FirmwareVersion)
}

func TestBootNotificationResponse_JSON(t *testing.T) {
	resp := BootNotificationResponse{
		Status:      RegistrationStatusAccepted,
		CurrentTime: DateTime{Time: time.Now().UTC()},
		Interval:    300,
	}

	// 测试序列化
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	// 测试反序列化
	var decoded BootNotificationResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.Status, decoded.Status)
	assert.Equal(t, resp.Interval, decoded.Interval)
	// 时间比较允许1秒误差
	assert.WithinDuration(t, resp.CurrentTime.Time, decoded.CurrentTime.Time, time.Second)
}

func TestStatusNotificationRequest_JSON(t *testing.T) {
	timestamp := DateTime{Time: time.Now().UTC()}
	req := StatusNotificationRequest{
		ConnectorId: 1,
		ErrorCode:   ChargePointErrorCodeNoError,
		Status:      ChargePointStatusAvailable,
		Timestamp:   &timestamp,
		Info:        stringPtr("Test info"),
	}

	// 测试序列化
	data, err := json.Marshal(req)
	require.NoError(t, err)

	// 测试反序列化
	var decoded StatusNotificationRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, req.ConnectorId, decoded.ConnectorId)
	assert.Equal(t, req.ErrorCode, decoded.ErrorCode)
	assert.Equal(t, req.Status, decoded.Status)
	assert.Equal(t, req.Info, decoded.Info)
	assert.NotNil(t, decoded.Timestamp)
}

func TestStartTransactionRequest_JSON(t *testing.T) {
	req := StartTransactionRequest{
		ConnectorId: 1,
		IdTag:       "RFID123456",
		MeterStart:  1000,
		Timestamp:   DateTime{Time: time.Now().UTC()},
	}

	// 测试序列化
	data, err := json.Marshal(req)
	require.NoError(t, err)

	// 测试反序列化
	var decoded StartTransactionRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, req.ConnectorId, decoded.ConnectorId)
	assert.Equal(t, req.IdTag, decoded.IdTag)
	assert.Equal(t, req.MeterStart, decoded.MeterStart)
}

func TestMeterValuesRequest_JSON(t *testing.T) {
	req := MeterValuesRequest{
		ConnectorId:   1,
		TransactionId: intPtr(12345),
		MeterValue: []MeterValue{
			{
				Timestamp: DateTime{Time: time.Now().UTC()},
				SampledValue: []SampledValue{
					{
						Value:     "1234.56",
						Measurand: measurandPtr(MeasurandEnergyActiveImportRegister),
						Unit:      unitPtr(UnitOfMeasureKWh),
					},
				},
			},
		},
	}

	// 测试序列化
	data, err := json.Marshal(req)
	require.NoError(t, err)

	// 测试反序列化
	var decoded MeterValuesRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, req.ConnectorId, decoded.ConnectorId)
	assert.Equal(t, req.TransactionId, decoded.TransactionId)
	assert.Len(t, decoded.MeterValue, 1)
	assert.Len(t, decoded.MeterValue[0].SampledValue, 1)
	assert.Equal(t, "1234.56", decoded.MeterValue[0].SampledValue[0].Value)
}

func TestCallMessage_JSON(t *testing.T) {
	payload := BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	}

	msg := CallMessage{
		MessageTypeID: Call,
		MessageID:     "12345",
		Action:        ActionBootNotification,
		Payload:       payload,
	}

	// 测试序列化
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// 验证JSON结构
	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, float64(2), decoded["messageTypeId"])
	assert.Equal(t, "12345", decoded["messageId"])
	assert.Equal(t, string(ActionBootNotification), decoded["action"])
	assert.NotNil(t, decoded["payload"])
}

func TestCallResultMessage_JSON(t *testing.T) {
	payload := BootNotificationResponse{
		Status:      RegistrationStatusAccepted,
		CurrentTime: DateTime{Time: time.Now().UTC()},
		Interval:    300,
	}

	msg := CallResultMessage{
		MessageTypeID: CallResult,
		MessageID:     "12345",
		Payload:       payload,
	}

	// 测试序列化
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// 验证JSON结构
	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, float64(3), decoded["messageTypeId"])
	assert.Equal(t, "12345", decoded["messageId"])
	assert.NotNil(t, decoded["payload"])
}

func TestCallErrorMessage_JSON(t *testing.T) {
	msg := CallErrorMessage{
		MessageTypeID:    CallError,
		MessageID:        "12345",
		ErrorCode:        "InternalError",
		ErrorDescription: "An internal error occurred",
		ErrorDetails:     map[string]string{"detail": "test"},
	}

	// 测试序列化
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// 测试反序列化
	var decoded CallErrorMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.MessageTypeID, decoded.MessageTypeID)
	assert.Equal(t, msg.MessageID, decoded.MessageID)
	assert.Equal(t, msg.ErrorCode, decoded.ErrorCode)
	assert.Equal(t, msg.ErrorDescription, decoded.ErrorDescription)
}

func TestChargingProfile_JSON(t *testing.T) {
	profile := ChargingProfile{
		ChargingProfileId:      1,
		StackLevel:             0,
		ChargingProfilePurpose: ChargingProfilePurposeTxProfile,
		ChargingProfileKind:    ChargingProfileKindAbsolute,
		ChargingSchedule: ChargingSchedule{
			ChargingRateUnit: ChargingRateUnitA,
			ChargingSchedulePeriod: []ChargingSchedulePeriod{
				{
					StartPeriod: 0,
					Limit:       32.0,
				},
			},
		},
	}

	// 测试序列化
	data, err := json.Marshal(profile)
	require.NoError(t, err)

	// 测试反序列化
	var decoded ChargingProfile
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, profile.ChargingProfileId, decoded.ChargingProfileId)
	assert.Equal(t, profile.StackLevel, decoded.StackLevel)
	assert.Equal(t, profile.ChargingProfilePurpose, decoded.ChargingProfilePurpose)
	assert.Equal(t, profile.ChargingProfileKind, decoded.ChargingProfileKind)
	assert.Equal(t, profile.ChargingSchedule.ChargingRateUnit, decoded.ChargingSchedule.ChargingRateUnit)
	assert.Len(t, decoded.ChargingSchedule.ChargingSchedulePeriod, 1)
}

// 辅助函数
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func measurandPtr(m Measurand) *Measurand {
	return &m
}

func unitPtr(u UnitOfMeasure) *UnitOfMeasure {
	return &u
}
