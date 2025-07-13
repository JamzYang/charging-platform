package serialization

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/charging-platform/charge-point-gateway/internal/domain/ocpp16"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSerializer(t *testing.T) {
	serializer := NewSerializer(FormatJSON)
	assert.NotNil(t, serializer)
	assert.Equal(t, FormatJSON, serializer.format)
}

func TestSerializer_SerializeMessage(t *testing.T) {
	serializer := NewSerializer(FormatJSON)

	tests := []struct {
		name        string
		messageType int
		messageID   string
		action      string
		payload     interface{}
		wantErr     bool
	}{
		{
			name:        "Call message",
			messageType: 2,
			messageID:   "12345",
			action:      "BootNotification",
			payload:     map[string]string{"vendor": "test"},
			wantErr:     false,
		},
		{
			name:        "CallResult message",
			messageType: 3,
			messageID:   "12345",
			action:      "",
			payload:     map[string]string{"status": "Accepted"},
			wantErr:     false,
		},
		{
			name:        "CallError message",
			messageType: 4,
			messageID:   "12345",
			action:      "",
			payload: map[string]interface{}{
				"errorCode":        "InternalError",
				"errorDescription": "An error occurred",
				"errorDetails":     map[string]string{"detail": "test"},
			},
			wantErr: false,
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
			name:        "invalid CallError payload",
			messageType: 4,
			messageID:   "12345",
			action:      "",
			payload:     "invalid payload",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := serializer.SerializeMessage(tt.messageType, tt.messageID, tt.action, tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)
				
				// 验证生成的JSON是有效的
				var temp interface{}
				err = json.Unmarshal(data, &temp)
				assert.NoError(t, err)
			}
		})
	}
}

func TestSerializer_DeserializeMessage(t *testing.T) {
	serializer := NewSerializer(FormatJSON)

	tests := []struct {
		name            string
		data            string
		wantMessageType int
		wantMessageID   string
		wantAction      string
		wantErr         bool
	}{
		{
			name:            "Call message",
			data:            `[2, "12345", "BootNotification", {"vendor": "test"}]`,
			wantMessageType: 2,
			wantMessageID:   "12345",
			wantAction:      "BootNotification",
			wantErr:         false,
		},
		{
			name:            "CallResult message",
			data:            `[3, "12345", {"status": "Accepted"}]`,
			wantMessageType: 3,
			wantMessageID:   "12345",
			wantAction:      "",
			wantErr:         false,
		},
		{
			name:            "CallError message",
			data:            `[4, "12345", "InternalError", "An error occurred", {"detail": "test"}]`,
			wantMessageType: 4,
			wantMessageID:   "12345",
			wantAction:      "",
			wantErr:         false,
		},
		{
			name:    "invalid JSON",
			data:    `[2, "12345", "BootNotification"`,
			wantErr: true,
		},
		{
			name:    "array too short",
			data:    `[2, "12345"]`,
			wantErr: true,
		},
		{
			name:    "invalid message type",
			data:    `[5, "12345", "BootNotification", {}]`,
			wantErr: true,
		},
		{
			name:    "Call message wrong length",
			data:    `[2, "12345", "BootNotification"]`,
			wantErr: true,
		},
		{
			name:    "CallResult message wrong length",
			data:    `[3, "12345"]`,
			wantErr: true,
		},
		{
			name:    "CallError message wrong length",
			data:    `[4, "12345", "InternalError"]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messageType, messageID, action, payload, err := serializer.DeserializeMessage([]byte(tt.data))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantMessageType, messageType)
				assert.Equal(t, tt.wantMessageID, messageID)
				assert.Equal(t, tt.wantAction, action)
				assert.NotNil(t, payload)
			}
		})
	}
}

func TestSerializer_SerializeDeserializeRoundTrip(t *testing.T) {
	serializer := NewSerializer(FormatJSON)

	// 测试Call消息的往返序列化
	originalPayload := map[string]interface{}{
		"chargePointVendor": "TestVendor",
		"chargePointModel":  "TestModel",
	}

	// 序列化
	data, err := serializer.SerializeMessage(2, "test-123", "BootNotification", originalPayload)
	require.NoError(t, err)

	// 反序列化
	messageType, messageID, action, payload, err := serializer.DeserializeMessage(data)
	require.NoError(t, err)

	// 验证结果
	assert.Equal(t, 2, messageType)
	assert.Equal(t, "test-123", messageID)
	assert.Equal(t, "BootNotification", action)

	// 验证payload
	var deserializedPayload map[string]interface{}
	err = json.Unmarshal(payload, &deserializedPayload)
	require.NoError(t, err)
	assert.Equal(t, originalPayload, deserializedPayload)
}

func TestSerializer_SerializePayload(t *testing.T) {
	serializer := NewSerializer(FormatJSON)

	payload := ocpp16.BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	}

	data, err := serializer.SerializePayload(payload)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	// 验证JSON有效性
	var temp interface{}
	err = json.Unmarshal(data, &temp)
	assert.NoError(t, err)
}

func TestSerializer_DeserializePayload(t *testing.T) {
	serializer := NewSerializer(FormatJSON)

	data := []byte(`{"chargePointVendor": "TestVendor", "chargePointModel": "TestModel"}`)
	var target ocpp16.BootNotificationRequest

	err := serializer.DeserializePayload(data, &target)
	assert.NoError(t, err)
	assert.Equal(t, "TestVendor", target.ChargePointVendor)
	assert.Equal(t, "TestModel", target.ChargePointModel)

	// 测试无效JSON
	invalidData := []byte(`{"chargePointVendor": "TestVendor"`)
	err = serializer.DeserializePayload(invalidData, &target)
	assert.Error(t, err)
}

func TestSerializer_GetPayloadType(t *testing.T) {
	serializer := NewSerializer(FormatJSON)

	tests := []struct {
		name      string
		action    string
		isRequest bool
		wantType  reflect.Type
	}{
		{
			name:      "BootNotification request",
			action:    "BootNotification",
			isRequest: true,
			wantType:  reflect.TypeOf(ocpp16.BootNotificationRequest{}),
		},
		{
			name:      "BootNotification response",
			action:    "BootNotification",
			isRequest: false,
			wantType:  reflect.TypeOf(ocpp16.BootNotificationResponse{}),
		},
		{
			name:      "Heartbeat request",
			action:    "Heartbeat",
			isRequest: true,
			wantType:  reflect.TypeOf(ocpp16.HeartbeatRequest{}),
		},
		{
			name:      "unknown action",
			action:    "UnknownAction",
			isRequest: true,
			wantType:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializer.GetPayloadType(tt.action, tt.isRequest)
			assert.Equal(t, tt.wantType, result)
		})
	}
}

func TestSerializer_CreatePayloadInstance(t *testing.T) {
	serializer := NewSerializer(FormatJSON)

	// 测试已知action
	instance := serializer.CreatePayloadInstance("BootNotification", true)
	assert.NotNil(t, instance)
	assert.IsType(t, &ocpp16.BootNotificationRequest{}, instance)

	// 测试未知action
	instance = serializer.CreatePayloadInstance("UnknownAction", true)
	assert.Nil(t, instance)
}

func TestSerializer_PrettyPrint(t *testing.T) {
	serializer := NewSerializer(FormatJSON)

	compactJSON := []byte(`{"key":"value","number":123}`)
	prettyJSON, err := serializer.PrettyPrint(compactJSON)
	assert.NoError(t, err)
	assert.Contains(t, string(prettyJSON), "\n")
	assert.Contains(t, string(prettyJSON), "  ")

	// 测试无效JSON
	invalidJSON := []byte(`{"key":"value"`)
	_, err = serializer.PrettyPrint(invalidJSON)
	assert.Error(t, err)
}

func TestSerializer_CompactJSON(t *testing.T) {
	serializer := NewSerializer(FormatJSON)

	prettyJSON := []byte(`{
  "key": "value",
  "number": 123
}`)
	compactJSON, err := serializer.CompactJSON(prettyJSON)
	assert.NoError(t, err)
	assert.NotContains(t, string(compactJSON), "\n")
	assert.NotContains(t, string(compactJSON), "  ")

	// 测试无效JSON
	invalidJSON := []byte(`{"key":"value"`)
	_, err = serializer.CompactJSON(invalidJSON)
	assert.Error(t, err)
}

func TestSerializer_UnsupportedFormat(t *testing.T) {
	serializer := NewSerializer(FormatXML)

	// 测试序列化
	_, err := serializer.SerializeMessage(2, "test", "BootNotification", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "XML format not implemented")

	// 测试反序列化
	_, _, _, _, err = serializer.DeserializeMessage([]byte(`<xml></xml>`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "XML format not implemented")
}

func TestSerializationError(t *testing.T) {
	// 测试没有cause的错误
	err := SerializationError{
		Operation: "TestOperation",
		Message:   "Test message",
	}
	expected := "TestOperation failed: Test message"
	assert.Equal(t, expected, err.Error())

	// 测试有cause的错误
	causeErr := assert.AnError
	errWithCause := SerializationError{
		Operation: "TestOperation",
		Message:   "Test message",
		Cause:     causeErr,
	}
	expectedWithCause := "TestOperation failed: Test message (caused by: assert.AnError general error for testing)"
	assert.Equal(t, expectedWithCause, errWithCause.Error())
}

func TestSerializer_CallErrorSerialization(t *testing.T) {
	serializer := NewSerializer(FormatJSON)

	// 测试CallError消息的序列化和反序列化
	errorPayload := map[string]interface{}{
		"errorCode":        "InternalError",
		"errorDescription": "An internal error occurred",
		"errorDetails":     map[string]string{"detail": "test detail"},
	}

	// 序列化
	data, err := serializer.SerializeMessage(4, "error-123", "", errorPayload)
	require.NoError(t, err)

	// 反序列化
	messageType, messageID, action, payload, err := serializer.DeserializeMessage(data)
	require.NoError(t, err)

	assert.Equal(t, 4, messageType)
	assert.Equal(t, "error-123", messageID)
	assert.Equal(t, "", action)

	// 验证错误payload
	var deserializedPayload map[string]interface{}
	err = json.Unmarshal(payload, &deserializedPayload)
	require.NoError(t, err)
	assert.Equal(t, "InternalError", deserializedPayload["errorCode"])
	assert.Equal(t, "An internal error occurred", deserializedPayload["errorDescription"])
	assert.NotNil(t, deserializedPayload["errorDetails"])
}
