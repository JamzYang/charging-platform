package validation

import (
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/ocpp16"
	"github.com/stretchr/testify/assert"
)

func TestNewValidator(t *testing.T) {
	validator := NewValidator()
	assert.NotNil(t, validator)
	assert.NotNil(t, validator.validate)
}

func TestValidator_ValidateJSON(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "valid JSON",
			json:    `{"key": "value"}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			json:    `{"key": "value"`,
			wantErr: true,
		},
		{
			name:    "empty JSON",
			json:    `{}`,
			wantErr: false,
		},
		{
			name:    "JSON array",
			json:    `[1, 2, 3]`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateJSON([]byte(tt.json))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateOCPPMessage(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		messageType int
		messageID   string
		action      string
		payload     interface{}
		wantErr     bool
	}{
		{
			name:        "valid Call message",
			messageType: 2,
			messageID:   "12345",
			action:      "BootNotification",
			payload:     nil,
			wantErr:     false,
		},
		{
			name:        "valid CallResult message",
			messageType: 3,
			messageID:   "12345",
			action:      "",
			payload:     nil,
			wantErr:     false,
		},
		{
			name:        "valid CallError message",
			messageType: 4,
			messageID:   "12345",
			action:      "",
			payload:     nil,
			wantErr:     false,
		},
		{
			name:        "invalid message type",
			messageType: 5,
			messageID:   "12345",
			action:      "BootNotification",
			payload:     nil,
			wantErr:     true,
		},
		{
			name:        "empty message ID",
			messageType: 2,
			messageID:   "",
			action:      "BootNotification",
			payload:     nil,
			wantErr:     true,
		},
		{
			name:        "message ID too long",
			messageType: 2,
			messageID:   "this-is-a-very-long-message-id-that-exceeds-the-limit",
			action:      "BootNotification",
			payload:     nil,
			wantErr:     true,
		},
		{
			name:        "Call message without action",
			messageType: 2,
			messageID:   "12345",
			action:      "",
			payload:     nil,
			wantErr:     true,
		},
		{
			name:        "Call message with invalid action",
			messageType: 2,
			messageID:   "12345",
			action:      "InvalidAction",
			payload:     nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateOCPPMessage(tt.messageType, tt.messageID, tt.action, tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateStruct(t *testing.T) {
	validator := NewValidator()

	// 测试BootNotificationRequest
	validRequest := ocpp16.BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	}

	err := validator.ValidateStruct(validRequest)
	assert.NoError(t, err)

	// 测试无效的请求
	invalidRequest := ocpp16.BootNotificationRequest{
		ChargePointVendor: "", // 必填字段为空
		ChargePointModel:  "TestModel",
	}

	err = validator.ValidateStruct(invalidRequest)
	assert.Error(t, err)

	// 检查错误类型
	if validationErrors, ok := err.(ValidationErrors); ok {
		assert.Len(t, validationErrors, 1)
		assert.Equal(t, "ChargePointVendor", validationErrors[0].Field)
		assert.Equal(t, "required", validationErrors[0].Tag)
	}
}

func TestValidator_ValidateChargePointID(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name          string
		chargePointID string
		wantErr       bool
	}{
		{
			name:          "valid charge point ID",
			chargePointID: "CP001",
			wantErr:       false,
		},
		{
			name:          "valid charge point ID with hyphen",
			chargePointID: "CP-001",
			wantErr:       false,
		},
		{
			name:          "empty charge point ID",
			chargePointID: "",
			wantErr:       true,
		},
		{
			name:          "charge point ID too long",
			chargePointID: "this-is-a-very-long-charge-point-id",
			wantErr:       true,
		},
		{
			name:          "charge point ID with invalid characters",
			chargePointID: "CP@001",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateChargePointID(tt.chargePointID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateProtocolVersion(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{
			name:    "valid OCPP 1.6",
			version: "ocpp1.6",
			wantErr: false,
		},
		{
			name:    "valid OCPP 2.0",
			version: "ocpp2.0",
			wantErr: false,
		},
		{
			name:    "valid OCPP 2.0.1",
			version: "ocpp2.0.1",
			wantErr: false,
		},
		{
			name:    "invalid version",
			version: "ocpp1.5",
			wantErr: true,
		},
		{
			name:    "empty version",
			version: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateProtocolVersion(tt.version)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateMessageSize(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		data    []byte
		maxSize int
		wantErr bool
	}{
		{
			name:    "message within size limit",
			data:    []byte("hello"),
			maxSize: 10,
			wantErr: false,
		},
		{
			name:    "message at size limit",
			data:    []byte("hello"),
			maxSize: 5,
			wantErr: false,
		},
		{
			name:    "message exceeds size limit",
			data:    []byte("hello world"),
			maxSize: 5,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateMessageSize(tt.data, tt.maxSize)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCustomValidations(t *testing.T) {
	validator := NewValidator()

	// 测试自定义验证规则的结构体
	type TestStruct struct {
		DateTime    string `validate:"ocpp_datetime"`
		IdToken     string `validate:"ocpp_id_token"`
		ConnectorID int    `validate:"ocpp_connector_id"`
		MeterValue  string `validate:"ocpp_meter_value"`
		Status      string `validate:"ocpp_status"`
	}

	tests := []struct {
		name    string
		data    TestStruct
		wantErr bool
	}{
		{
			name: "valid data",
			data: TestStruct{
				DateTime:    time.Now().Format(time.RFC3339),
				IdToken:     "RFID123456",
				ConnectorID: 1,
				MeterValue:  "1234.56",
				Status:      "Available",
			},
			wantErr: false,
		},
		{
			name: "invalid datetime",
			data: TestStruct{
				DateTime:    "invalid-datetime",
				IdToken:     "RFID123456",
				ConnectorID: 1,
				MeterValue:  "1234.56",
				Status:      "Available",
			},
			wantErr: true,
		},
		{
			name: "invalid id token",
			data: TestStruct{
				DateTime:    time.Now().Format(time.RFC3339),
				IdToken:     "RFID@123456", // 包含非法字符
				ConnectorID: 1,
				MeterValue:  "1234.56",
				Status:      "Available",
			},
			wantErr: true,
		},
		{
			name: "invalid connector id",
			data: TestStruct{
				DateTime:    time.Now().Format(time.RFC3339),
				IdToken:     "RFID123456",
				ConnectorID: -1, // 负数
				MeterValue:  "1234.56",
				Status:      "Available",
			},
			wantErr: true,
		},
		{
			name: "invalid meter value",
			data: TestStruct{
				DateTime:    time.Now().Format(time.RFC3339),
				IdToken:     "RFID123456",
				ConnectorID: 1,
				MeterValue:  "not-a-number",
				Status:      "Available",
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			data: TestStruct{
				DateTime:    time.Now().Format(time.RFC3339),
				IdToken:     "RFID123456",
				ConnectorID: 1,
				MeterValue:  "1234.56",
				Status:      "InvalidStatus",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateStruct(tt.data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := ValidationError{
		Field:   "testField",
		Tag:     "required",
		Value:   "",
		Message: "Field is required",
	}

	assert.Equal(t, "Field is required", err.Error())
}

func TestValidationErrors(t *testing.T) {
	errors := ValidationErrors{
		{Field: "field1", Message: "Error 1"},
		{Field: "field2", Message: "Error 2"},
	}

	expected := "Error 1; Error 2"
	assert.Equal(t, expected, errors.Error())
}

func TestIsValidAction(t *testing.T) {
	tests := []struct {
		name   string
		action string
		want   bool
	}{
		{"valid core action", "BootNotification", true},
		{"valid core action", "Heartbeat", true},
		{"valid firmware action", "UpdateFirmware", true},
		{"valid reservation action", "ReserveNow", true},
		{"invalid action", "InvalidAction", false},
		{"empty action", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidAction(tt.action)
			assert.Equal(t, tt.want, result)
		})
	}
}
