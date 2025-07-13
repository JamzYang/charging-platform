package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event 统一业务事件接口
type Event interface {
	// GetID 获取事件ID
	GetID() string
	// GetType 获取事件类型
	GetType() EventType
	// GetChargePointID 获取充电桩ID
	GetChargePointID() string
	// GetTimestamp 获取事件时间戳
	GetTimestamp() time.Time
	// GetSeverity 获取事件严重程度
	GetSeverity() EventSeverity
	// GetMetadata 获取事件元数据
	GetMetadata() Metadata
	// GetPayload 获取事件载荷
	GetPayload() interface{}
	// ToJSON 序列化为JSON
	ToJSON() ([]byte, error)
}

// BaseEvent 基础事件结构
type BaseEvent struct {
	ID            string        `json:"id"`
	Type          EventType     `json:"type"`
	ChargePointID string        `json:"charge_point_id"`
	Timestamp     time.Time     `json:"timestamp"`
	Severity      EventSeverity `json:"severity"`
	Metadata      Metadata      `json:"metadata"`
}

// GetID 实现Event接口
func (e *BaseEvent) GetID() string {
	return e.ID
}

// GetType 实现Event接口
func (e *BaseEvent) GetType() EventType {
	return e.Type
}

// GetChargePointID 实现Event接口
func (e *BaseEvent) GetChargePointID() string {
	return e.ChargePointID
}

// GetTimestamp 实现Event接口
func (e *BaseEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// GetSeverity 实现Event接口
func (e *BaseEvent) GetSeverity() EventSeverity {
	return e.Severity
}

// GetMetadata 实现Event接口
func (e *BaseEvent) GetMetadata() Metadata {
	return e.Metadata
}

// NewBaseEvent 创建基础事件
func NewBaseEvent(eventType EventType, chargePointID string, severity EventSeverity, metadata Metadata) *BaseEvent {
	return &BaseEvent{
		ID:            uuid.New().String(),
		Type:          eventType,
		ChargePointID: chargePointID,
		Timestamp:     time.Now().UTC(),
		Severity:      severity,
		Metadata:      metadata,
	}
}

// ChargePointConnectedEvent 充电桩连接事件
type ChargePointConnectedEvent struct {
	*BaseEvent
	ChargePointInfo ChargePointInfo `json:"charge_point_info"`
}

// GetPayload 实现Event接口
func (e *ChargePointConnectedEvent) GetPayload() interface{} {
	return e.ChargePointInfo
}

// ToJSON 实现Event接口
func (e *ChargePointConnectedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ChargePointDisconnectedEvent 充电桩断开连接事件
type ChargePointDisconnectedEvent struct {
	*BaseEvent
	Reason string `json:"reason"`
}

// GetPayload 实现Event接口
func (e *ChargePointDisconnectedEvent) GetPayload() interface{} {
	return map[string]interface{}{
		"reason": e.Reason,
	}
}

// ToJSON 实现Event接口
func (e *ChargePointDisconnectedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ChargePointHeartbeatEvent 充电桩心跳事件
type ChargePointHeartbeatEvent struct {
	*BaseEvent
}

// GetPayload 实现Event接口
func (e *ChargePointHeartbeatEvent) GetPayload() interface{} {
	return nil
}

// ToJSON 实现Event接口
func (e *ChargePointHeartbeatEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ChargePointRegisteredEvent 充电桩注册事件
type ChargePointRegisteredEvent struct {
	*BaseEvent
	ChargePointInfo ChargePointInfo `json:"charge_point_info"`
	Interval        int             `json:"interval"`
}

// GetPayload 实现Event接口
func (e *ChargePointRegisteredEvent) GetPayload() interface{} {
	return map[string]interface{}{
		"charge_point_info": e.ChargePointInfo,
		"interval":          e.Interval,
	}
}

// ToJSON 实现Event接口
func (e *ChargePointRegisteredEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ConnectorStatusChangedEvent 连接器状态变更事件
type ConnectorStatusChangedEvent struct {
	*BaseEvent
	ConnectorInfo ConnectorInfo     `json:"connector_info"`
	PreviousStatus ConnectorStatus  `json:"previous_status"`
}

// GetPayload 实现Event接口
func (e *ConnectorStatusChangedEvent) GetPayload() interface{} {
	return map[string]interface{}{
		"connector_info":   e.ConnectorInfo,
		"previous_status": e.PreviousStatus,
	}
}

// ToJSON 实现Event接口
func (e *ConnectorStatusChangedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// TransactionStartedEvent 交易开始事件
type TransactionStartedEvent struct {
	*BaseEvent
	TransactionInfo   TransactionInfo   `json:"transaction_info"`
	AuthorizationInfo AuthorizationInfo `json:"authorization_info"`
}

// GetPayload 实现Event接口
func (e *TransactionStartedEvent) GetPayload() interface{} {
	return map[string]interface{}{
		"transaction_info":   e.TransactionInfo,
		"authorization_info": e.AuthorizationInfo,
	}
}

// ToJSON 实现Event接口
func (e *TransactionStartedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// TransactionStoppedEvent 交易停止事件
type TransactionStoppedEvent struct {
	*BaseEvent
	TransactionInfo TransactionInfo `json:"transaction_info"`
	MeterValues     []MeterValue    `json:"meter_values,omitempty"`
}

// GetPayload 实现Event接口
func (e *TransactionStoppedEvent) GetPayload() interface{} {
	return map[string]interface{}{
		"transaction_info": e.TransactionInfo,
		"meter_values":     e.MeterValues,
	}
}

// ToJSON 实现Event接口
func (e *TransactionStoppedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// MeterValuesReceivedEvent 电表值接收事件
type MeterValuesReceivedEvent struct {
	*BaseEvent
	ConnectorID     int           `json:"connector_id"`
	TransactionID   *int          `json:"transaction_id,omitempty"`
	MeterValues     []MeterValue  `json:"meter_values"`
}

// GetPayload 实现Event接口
func (e *MeterValuesReceivedEvent) GetPayload() interface{} {
	return map[string]interface{}{
		"connector_id":   e.ConnectorID,
		"transaction_id": e.TransactionID,
		"meter_values":   e.MeterValues,
	}
}

// ToJSON 实现Event接口
func (e *MeterValuesReceivedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// AuthorizationRequestedEvent 授权请求事件
type AuthorizationRequestedEvent struct {
	*BaseEvent
	IdTag string `json:"id_tag"`
}

// GetPayload 实现Event接口
func (e *AuthorizationRequestedEvent) GetPayload() interface{} {
	return map[string]interface{}{
		"id_tag": e.IdTag,
	}
}

// ToJSON 实现Event接口
func (e *AuthorizationRequestedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// AuthorizationGrantedEvent 授权通过事件
type AuthorizationGrantedEvent struct {
	*BaseEvent
	AuthorizationInfo AuthorizationInfo `json:"authorization_info"`
}

// GetPayload 实现Event接口
func (e *AuthorizationGrantedEvent) GetPayload() interface{} {
	return e.AuthorizationInfo
}

// ToJSON 实现Event接口
func (e *AuthorizationGrantedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// RemoteCommandReceivedEvent 远程指令接收事件
type RemoteCommandReceivedEvent struct {
	*BaseEvent
	Command RemoteCommand `json:"command"`
}

// GetPayload 实现Event接口
func (e *RemoteCommandReceivedEvent) GetPayload() interface{} {
	return e.Command
}

// ToJSON 实现Event接口
func (e *RemoteCommandReceivedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// RemoteCommandExecutedEvent 远程指令执行事件
type RemoteCommandExecutedEvent struct {
	*BaseEvent
	Command RemoteCommand `json:"command"`
}

// GetPayload 实现Event接口
func (e *RemoteCommandExecutedEvent) GetPayload() interface{} {
	return e.Command
}

// ToJSON 实现Event接口
func (e *RemoteCommandExecutedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// DataTransferReceivedEvent 数据传输接收事件
type DataTransferReceivedEvent struct {
	*BaseEvent
	DataTransferInfo DataTransferInfo `json:"data_transfer_info"`
}

// GetPayload 实现Event接口
func (e *DataTransferReceivedEvent) GetPayload() interface{} {
	return e.DataTransferInfo
}

// ToJSON 实现Event接口
func (e *DataTransferReceivedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ProtocolErrorEvent 协议错误事件
type ProtocolErrorEvent struct {
	*BaseEvent
	ErrorInfo     ErrorInfo `json:"error_info"`
	OriginalMessage interface{} `json:"original_message,omitempty"`
}

// GetPayload 实现Event接口
func (e *ProtocolErrorEvent) GetPayload() interface{} {
	return map[string]interface{}{
		"error_info":       e.ErrorInfo,
		"original_message": e.OriginalMessage,
	}
}

// ToJSON 实现Event接口
func (e *ProtocolErrorEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// EventFactory 事件工厂
type EventFactory struct{}

// NewEventFactory 创建事件工厂
func NewEventFactory() *EventFactory {
	return &EventFactory{}
}

// CreateChargePointConnectedEvent 创建充电桩连接事件
func (f *EventFactory) CreateChargePointConnectedEvent(chargePointID string, info ChargePointInfo, metadata Metadata) *ChargePointConnectedEvent {
	return &ChargePointConnectedEvent{
		BaseEvent:       NewBaseEvent(EventTypeChargePointConnected, chargePointID, EventSeverityInfo, metadata),
		ChargePointInfo: info,
	}
}

// CreateConnectorStatusChangedEvent 创建连接器状态变更事件
func (f *EventFactory) CreateConnectorStatusChangedEvent(chargePointID string, connectorInfo ConnectorInfo, previousStatus ConnectorStatus, metadata Metadata) *ConnectorStatusChangedEvent {
	return &ConnectorStatusChangedEvent{
		BaseEvent:      NewBaseEvent(EventTypeConnectorStatusChanged, chargePointID, EventSeverityInfo, metadata),
		ConnectorInfo:  connectorInfo,
		PreviousStatus: previousStatus,
	}
}

// CreateTransactionStartedEvent 创建交易开始事件
func (f *EventFactory) CreateTransactionStartedEvent(chargePointID string, transactionInfo TransactionInfo, authInfo AuthorizationInfo, metadata Metadata) *TransactionStartedEvent {
	return &TransactionStartedEvent{
		BaseEvent:         NewBaseEvent(EventTypeTransactionStarted, chargePointID, EventSeverityInfo, metadata),
		TransactionInfo:   transactionInfo,
		AuthorizationInfo: authInfo,
	}
}

// CreateProtocolErrorEvent 创建协议错误事件
func (f *EventFactory) CreateProtocolErrorEvent(chargePointID string, errorInfo ErrorInfo, originalMessage interface{}, metadata Metadata) *ProtocolErrorEvent {
	return &ProtocolErrorEvent{
		BaseEvent:       NewBaseEvent(EventTypeProtocolError, chargePointID, EventSeverityError, metadata),
		ErrorInfo:       errorInfo,
		OriginalMessage: originalMessage,
	}
}
