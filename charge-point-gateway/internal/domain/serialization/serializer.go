package serialization

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/charging-platform/charge-point-gateway/internal/domain/ocpp16"
)

// SerializationFormat 序列化格式
type SerializationFormat string

const (
	FormatJSON SerializationFormat = "json"
	FormatXML  SerializationFormat = "xml"
)

// Serializer 消息序列化器
type Serializer struct {
	format SerializationFormat
}

// SerializationError 序列化错误
type SerializationError struct {
	Operation string
	Message   string
	Cause     error
}

// Error 实现error接口
func (e SerializationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s failed: %s (caused by: %v)", e.Operation, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s failed: %s", e.Operation, e.Message)
}

// NewSerializer 创建新的序列化器
func NewSerializer(format SerializationFormat) *Serializer {
	return &Serializer{
		format: format,
	}
}

// SerializeMessage 序列化OCPP消息
func (s *Serializer) SerializeMessage(messageType int, messageID string, action string, payload interface{}) ([]byte, error) {
	switch s.format {
	case FormatJSON:
		return s.serializeJSON(messageType, messageID, action, payload)
	case FormatXML:
		return nil, SerializationError{
			Operation: "SerializeMessage",
			Message:   "XML format not implemented",
		}
	default:
		return nil, SerializationError{
			Operation: "SerializeMessage",
			Message:   fmt.Sprintf("Unsupported format: %s", s.format),
		}
	}
}

// DeserializeMessage 反序列化OCPP消息
func (s *Serializer) DeserializeMessage(data []byte) (messageType int, messageID string, action string, payload json.RawMessage, err error) {
	switch s.format {
	case FormatJSON:
		return s.deserializeJSON(data)
	case FormatXML:
		return 0, "", "", nil, SerializationError{
			Operation: "DeserializeMessage",
			Message:   "XML format not implemented",
		}
	default:
		return 0, "", "", nil, SerializationError{
			Operation: "DeserializeMessage",
			Message:   fmt.Sprintf("Unsupported format: %s", s.format),
		}
	}
}

// serializeJSON 序列化为JSON格式
func (s *Serializer) serializeJSON(messageType int, messageID string, action string, payload interface{}) ([]byte, error) {
	var message []interface{}
	
	switch messageType {
	case 2: // Call
		message = []interface{}{messageType, messageID, action, payload}
	case 3: // CallResult
		message = []interface{}{messageType, messageID, payload}
	case 4: // CallError
		if errorPayload, ok := payload.(map[string]interface{}); ok {
			errorCode := errorPayload["errorCode"]
			errorDescription := errorPayload["errorDescription"]
			errorDetails := errorPayload["errorDetails"]
			message = []interface{}{messageType, messageID, errorCode, errorDescription, errorDetails}
		} else {
			return nil, SerializationError{
				Operation: "serializeJSON",
				Message:   "Invalid CallError payload format",
			}
		}
	default:
		return nil, SerializationError{
			Operation: "serializeJSON",
			Message:   fmt.Sprintf("Invalid message type: %d", messageType),
		}
	}
	
	data, err := json.Marshal(message)
	if err != nil {
		return nil, SerializationError{
			Operation: "serializeJSON",
			Message:   "Failed to marshal JSON",
			Cause:     err,
		}
	}
	
	return data, nil
}

// deserializeJSON 从JSON格式反序列化
func (s *Serializer) deserializeJSON(data []byte) (messageType int, messageID string, action string, payload json.RawMessage, err error) {
	var message []json.RawMessage
	
	if err := json.Unmarshal(data, &message); err != nil {
		return 0, "", "", nil, SerializationError{
			Operation: "deserializeJSON",
			Message:   "Failed to unmarshal JSON array",
			Cause:     err,
		}
	}
	
	if len(message) < 3 {
		return 0, "", "", nil, SerializationError{
			Operation: "deserializeJSON",
			Message:   "Message array too short",
		}
	}
	
	// 解析消息类型
	var msgType int
	if err := json.Unmarshal(message[0], &msgType); err != nil {
		return 0, "", "", nil, SerializationError{
			Operation: "deserializeJSON",
			Message:   "Failed to parse message type",
			Cause:     err,
		}
	}
	
	// 解析消息ID
	var msgID string
	if err := json.Unmarshal(message[1], &msgID); err != nil {
		return 0, "", "", nil, SerializationError{
			Operation: "deserializeJSON",
			Message:   "Failed to parse message ID",
			Cause:     err,
		}
	}
	
	switch msgType {
	case 2: // Call
		if len(message) != 4 {
			return 0, "", "", nil, SerializationError{
				Operation: "deserializeJSON",
				Message:   "Call message must have exactly 4 elements",
			}
		}
		
		var act string
		if err := json.Unmarshal(message[2], &act); err != nil {
			return 0, "", "", nil, SerializationError{
				Operation: "deserializeJSON",
				Message:   "Failed to parse action",
				Cause:     err,
			}
		}
		
		return msgType, msgID, act, message[3], nil
		
	case 3: // CallResult
		if len(message) != 3 {
			return 0, "", "", nil, SerializationError{
				Operation: "deserializeJSON",
				Message:   "CallResult message must have exactly 3 elements",
			}
		}
		
		return msgType, msgID, "", message[2], nil
		
	case 4: // CallError
		if len(message) < 4 || len(message) > 5 {
			return 0, "", "", nil, SerializationError{
				Operation: "deserializeJSON",
				Message:   "CallError message must have 4 or 5 elements",
			}
		}
		
		// 构造错误payload
		errorPayload := map[string]interface{}{}
		
		var errorCode string
		if err := json.Unmarshal(message[2], &errorCode); err != nil {
			return 0, "", "", nil, SerializationError{
				Operation: "deserializeJSON",
				Message:   "Failed to parse error code",
				Cause:     err,
			}
		}
		errorPayload["errorCode"] = errorCode
		
		var errorDescription string
		if err := json.Unmarshal(message[3], &errorDescription); err != nil {
			return 0, "", "", nil, SerializationError{
				Operation: "deserializeJSON",
				Message:   "Failed to parse error description",
				Cause:     err,
			}
		}
		errorPayload["errorDescription"] = errorDescription
		
		if len(message) == 5 {
			var errorDetails interface{}
			if err := json.Unmarshal(message[4], &errorDetails); err != nil {
				return 0, "", "", nil, SerializationError{
					Operation: "deserializeJSON",
					Message:   "Failed to parse error details",
					Cause:     err,
				}
			}
			errorPayload["errorDetails"] = errorDetails
		}
		
		payloadData, _ := json.Marshal(errorPayload)
		return msgType, msgID, "", payloadData, nil
		
	default:
		return 0, "", "", nil, SerializationError{
			Operation: "deserializeJSON",
			Message:   fmt.Sprintf("Invalid message type: %d", msgType),
		}
	}
}

// SerializePayload 序列化载荷到指定类型
func (s *Serializer) SerializePayload(payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, SerializationError{
			Operation: "SerializePayload",
			Message:   "Failed to marshal payload",
			Cause:     err,
		}
	}
	return data, nil
}

// DeserializePayload 反序列化载荷到指定类型
func (s *Serializer) DeserializePayload(data []byte, target interface{}) error {
	if err := json.Unmarshal(data, target); err != nil {
		return SerializationError{
			Operation: "DeserializePayload",
			Message:   "Failed to unmarshal payload",
			Cause:     err,
		}
	}
	return nil
}

// GetPayloadType 根据action获取对应的payload类型
func (s *Serializer) GetPayloadType(action string, isRequest bool) reflect.Type {
	payloadTypes := map[string]map[bool]reflect.Type{
		"BootNotification": {
			true:  reflect.TypeOf(ocpp16.BootNotificationRequest{}),
			false: reflect.TypeOf(ocpp16.BootNotificationResponse{}),
		},
		"Heartbeat": {
			true:  reflect.TypeOf(ocpp16.HeartbeatRequest{}),
			false: reflect.TypeOf(ocpp16.HeartbeatResponse{}),
		},
		"StatusNotification": {
			true:  reflect.TypeOf(ocpp16.StatusNotificationRequest{}),
			false: reflect.TypeOf(ocpp16.StatusNotificationResponse{}),
		},
		"Authorize": {
			true:  reflect.TypeOf(ocpp16.AuthorizeRequest{}),
			false: reflect.TypeOf(ocpp16.AuthorizeResponse{}),
		},
		"StartTransaction": {
			true:  reflect.TypeOf(ocpp16.StartTransactionRequest{}),
			false: reflect.TypeOf(ocpp16.StartTransactionResponse{}),
		},
		"StopTransaction": {
			true:  reflect.TypeOf(ocpp16.StopTransactionRequest{}),
			false: reflect.TypeOf(ocpp16.StopTransactionResponse{}),
		},
		"MeterValues": {
			true:  reflect.TypeOf(ocpp16.MeterValuesRequest{}),
			false: reflect.TypeOf(ocpp16.MeterValuesResponse{}),
		},
		"DataTransfer": {
			true:  reflect.TypeOf(ocpp16.DataTransferRequest{}),
			false: reflect.TypeOf(ocpp16.DataTransferResponse{}),
		},
		"Reset": {
			true:  reflect.TypeOf(ocpp16.ResetRequest{}),
			false: reflect.TypeOf(ocpp16.ResetResponse{}),
		},
		"ChangeAvailability": {
			true:  reflect.TypeOf(ocpp16.ChangeAvailabilityRequest{}),
			false: reflect.TypeOf(ocpp16.ChangeAvailabilityResponse{}),
		},
		"GetConfiguration": {
			true:  reflect.TypeOf(ocpp16.GetConfigurationRequest{}),
			false: reflect.TypeOf(ocpp16.GetConfigurationResponse{}),
		},
		"ChangeConfiguration": {
			true:  reflect.TypeOf(ocpp16.ChangeConfigurationRequest{}),
			false: reflect.TypeOf(ocpp16.ChangeConfigurationResponse{}),
		},
		"ClearCache": {
			true:  reflect.TypeOf(ocpp16.ClearCacheRequest{}),
			false: reflect.TypeOf(ocpp16.ClearCacheResponse{}),
		},
		"UnlockConnector": {
			true:  reflect.TypeOf(ocpp16.UnlockConnectorRequest{}),
			false: reflect.TypeOf(ocpp16.UnlockConnectorResponse{}),
		},
		"RemoteStartTransaction": {
			true:  reflect.TypeOf(ocpp16.RemoteStartTransactionRequest{}),
			false: reflect.TypeOf(ocpp16.RemoteStartTransactionResponse{}),
		},
		"RemoteStopTransaction": {
			true:  reflect.TypeOf(ocpp16.RemoteStopTransactionRequest{}),
			false: reflect.TypeOf(ocpp16.RemoteStopTransactionResponse{}),
		},
	}
	
	if actionTypes, exists := payloadTypes[action]; exists {
		if payloadType, exists := actionTypes[isRequest]; exists {
			return payloadType
		}
	}
	
	return nil
}

// CreatePayloadInstance 创建payload实例
func (s *Serializer) CreatePayloadInstance(action string, isRequest bool) interface{} {
	payloadType := s.GetPayloadType(action, isRequest)
	if payloadType == nil {
		return nil
	}
	
	return reflect.New(payloadType).Interface()
}

// PrettyPrint 格式化打印JSON
func (s *Serializer) PrettyPrint(data []byte) ([]byte, error) {
	var temp interface{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return nil, SerializationError{
			Operation: "PrettyPrint",
			Message:   "Failed to parse JSON",
			Cause:     err,
		}
	}
	
	prettyData, err := json.MarshalIndent(temp, "", "  ")
	if err != nil {
		return nil, SerializationError{
			Operation: "PrettyPrint",
			Message:   "Failed to format JSON",
			Cause:     err,
		}
	}
	
	return prettyData, nil
}

// CompactJSON 压缩JSON
func (s *Serializer) CompactJSON(data []byte) ([]byte, error) {
	var temp interface{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return nil, SerializationError{
			Operation: "CompactJSON",
			Message:   "Failed to parse JSON",
			Cause:     err,
		}
	}
	
	compactData, err := json.Marshal(temp)
	if err != nil {
		return nil, SerializationError{
			Operation: "CompactJSON",
			Message:   "Failed to compact JSON",
			Cause:     err,
		}
	}
	
	return compactData, nil
}
