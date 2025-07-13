package events

import (
	"time"
)

// EventType 事件类型
type EventType string

const (
	// 充电桩生命周期事件
	EventTypeChargePointConnected    EventType = "charge_point.connected"
	EventTypeChargePointDisconnected EventType = "charge_point.disconnected"
	EventTypeChargePointRegistered   EventType = "charge_point.registered"
	EventTypeChargePointRejected     EventType = "charge_point.rejected"

	// 充电桩状态事件
	EventTypeChargePointStatusChanged EventType = "charge_point.status_changed"
	EventTypeConnectorStatusChanged   EventType = "connector.status_changed"
	EventTypeChargePointHeartbeat     EventType = "charge_point.heartbeat"

	// 交易事件
	EventTypeTransactionStarted EventType = "transaction.started"
	EventTypeTransactionStopped EventType = "transaction.stopped"
	EventTypeTransactionUpdated EventType = "transaction.updated"

	// 授权事件
	EventTypeAuthorizationRequested EventType = "authorization.requested"
	EventTypeAuthorizationGranted    EventType = "authorization.granted"
	EventTypeAuthorizationDenied     EventType = "authorization.denied"

	// 电表数据事件
	EventTypeMeterValuesReceived EventType = "meter_values.received"

	// 配置事件
	EventTypeConfigurationChanged EventType = "configuration.changed"
	EventTypeConfigurationRequested EventType = "configuration.requested"

	// 远程指令事件
	EventTypeRemoteCommandReceived EventType = "remote_command.received"
	EventTypeRemoteCommandExecuted EventType = "remote_command.executed"
	EventTypeRemoteCommandFailed   EventType = "remote_command.failed"

	// 固件和诊断事件
	EventTypeFirmwareUpdateStarted   EventType = "firmware.update_started"
	EventTypeFirmwareUpdateCompleted EventType = "firmware.update_completed"
	EventTypeDiagnosticsRequested    EventType = "diagnostics.requested"
	EventTypeDiagnosticsUploaded     EventType = "diagnostics.uploaded"

	// 数据传输事件
	EventTypeDataTransferReceived EventType = "data_transfer.received"

	// 错误事件
	EventTypeProtocolError EventType = "protocol.error"
	EventTypeSystemError   EventType = "system.error"
)

// EventSeverity 事件严重程度
type EventSeverity string

const (
	EventSeverityInfo     EventSeverity = "info"
	EventSeverityWarning  EventSeverity = "warning"
	EventSeverityError    EventSeverity = "error"
	EventSeverityCritical EventSeverity = "critical"
)

// ChargePointStatus 统一的充电桩状态
type ChargePointStatus string

const (
	ChargePointStatusOnline      ChargePointStatus = "online"
	ChargePointStatusOffline     ChargePointStatus = "offline"
	ChargePointStatusRegistered  ChargePointStatus = "registered"
	ChargePointStatusRejected    ChargePointStatus = "rejected"
	ChargePointStatusMaintenance ChargePointStatus = "maintenance"
)

// ConnectorStatus 统一的连接器状态
type ConnectorStatus string

const (
	ConnectorStatusAvailable     ConnectorStatus = "available"
	ConnectorStatusPreparing     ConnectorStatus = "preparing"
	ConnectorStatusCharging      ConnectorStatus = "charging"
	ConnectorStatusSuspendedEVSE ConnectorStatus = "suspended_evse"
	ConnectorStatusSuspendedEV   ConnectorStatus = "suspended_ev"
	ConnectorStatusFinishing     ConnectorStatus = "finishing"
	ConnectorStatusReserved      ConnectorStatus = "reserved"
	ConnectorStatusUnavailable   ConnectorStatus = "unavailable"
	ConnectorStatusFaulted       ConnectorStatus = "faulted"
)

// TransactionStatus 统一的交易状态
type TransactionStatus string

const (
	TransactionStatusStarting TransactionStatus = "starting"
	TransactionStatusActive   TransactionStatus = "active"
	TransactionStatusStopping TransactionStatus = "stopping"
	TransactionStatusStopped  TransactionStatus = "stopped"
	TransactionStatusFaulted  TransactionStatus = "faulted"
)

// AuthorizationResult 统一的授权结果
type AuthorizationResult string

const (
	AuthorizationResultAccepted     AuthorizationResult = "accepted"
	AuthorizationResultBlocked      AuthorizationResult = "blocked"
	AuthorizationResultExpired      AuthorizationResult = "expired"
	AuthorizationResultInvalid      AuthorizationResult = "invalid"
	AuthorizationResultConcurrentTx AuthorizationResult = "concurrent_tx"
	AuthorizationResultUnknown      AuthorizationResult = "unknown"
)

// MeterValueType 电表值类型
type MeterValueType string

const (
	MeterValueTypeEnergyActiveImport   MeterValueType = "energy_active_import"
	MeterValueTypeEnergyActiveExport   MeterValueType = "energy_active_export"
	MeterValueTypePowerActiveImport    MeterValueType = "power_active_import"
	MeterValueTypePowerActiveExport    MeterValueType = "power_active_export"
	MeterValueTypeCurrentImport        MeterValueType = "current_import"
	MeterValueTypeCurrentExport        MeterValueType = "current_export"
	MeterValueTypeVoltage              MeterValueType = "voltage"
	MeterValueTypeFrequency            MeterValueType = "frequency"
	MeterValueTypeTemperature          MeterValueType = "temperature"
	MeterValueTypeSoC                  MeterValueType = "soc"
	MeterValueTypePowerFactor          MeterValueType = "power_factor"
)

// CommandType 远程指令类型
type CommandType string

const (
	CommandTypeReset                  CommandType = "reset"
	CommandTypeChangeAvailability     CommandType = "change_availability"
	CommandTypeRemoteStartTransaction CommandType = "remote_start_transaction"
	CommandTypeRemoteStopTransaction  CommandType = "remote_stop_transaction"
	CommandTypeUnlockConnector        CommandType = "unlock_connector"
	CommandTypeGetConfiguration       CommandType = "get_configuration"
	CommandTypeChangeConfiguration    CommandType = "change_configuration"
	CommandTypeClearCache             CommandType = "clear_cache"
	CommandTypeUpdateFirmware         CommandType = "update_firmware"
	CommandTypeGetDiagnostics         CommandType = "get_diagnostics"
	CommandTypeDataTransfer           CommandType = "data_transfer"
)

// CommandStatus 指令执行状态
type CommandStatus string

const (
	CommandStatusPending   CommandStatus = "pending"
	CommandStatusExecuting CommandStatus = "executing"
	CommandStatusCompleted CommandStatus = "completed"
	CommandStatusFailed    CommandStatus = "failed"
	CommandStatusTimeout   CommandStatus = "timeout"
	CommandStatusRejected  CommandStatus = "rejected"
)

// ErrorCode 统一错误代码
type ErrorCode string

const (
	ErrorCodeProtocolError        ErrorCode = "protocol_error"
	ErrorCodeFormatViolation      ErrorCode = "format_violation"
	ErrorCodePropertyConstraint   ErrorCode = "property_constraint"
	ErrorCodeOccurrenceConstraint ErrorCode = "occurrence_constraint"
	ErrorCodeTypeConstraint       ErrorCode = "type_constraint"
	ErrorCodeGenericError         ErrorCode = "generic_error"
	ErrorCodeInternalError        ErrorCode = "internal_error"
	ErrorCodeNotImplemented       ErrorCode = "not_implemented"
	ErrorCodeNotSupported         ErrorCode = "not_supported"
	ErrorCodeSecurityError        ErrorCode = "security_error"
	ErrorCodeMessageTypeNotSupported ErrorCode = "message_type_not_supported"
)

// Location 位置信息
type Location struct {
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Address   *string  `json:"address,omitempty"`
}

// ChargePointInfo 充电桩基本信息
type ChargePointInfo struct {
	ID                      string    `json:"id"`
	Vendor                  string    `json:"vendor"`
	Model                   string    `json:"model"`
	SerialNumber            *string   `json:"serial_number,omitempty"`
	FirmwareVersion         *string   `json:"firmware_version,omitempty"`
	ConnectorCount          int       `json:"connector_count"`
	Location                *Location `json:"location,omitempty"`
	LastSeen                time.Time `json:"last_seen"`
	ProtocolVersion         string    `json:"protocol_version"`
	SupportedFeatureProfiles []string  `json:"supported_feature_profiles,omitempty"`
}

// ConnectorInfo 连接器信息
type ConnectorInfo struct {
	ID               int             `json:"id"`
	ChargePointID    string          `json:"charge_point_id"`
	Status           ConnectorStatus `json:"status"`
	ErrorCode        *string         `json:"error_code,omitempty"`
	ErrorDescription *string         `json:"error_description,omitempty"`
	VendorErrorCode  *string         `json:"vendor_error_code,omitempty"`
	MaxPower         *float64        `json:"max_power,omitempty"`
	ConnectorType    *string         `json:"connector_type,omitempty"`
}

// TransactionInfo 交易信息
type TransactionInfo struct {
	ID            int               `json:"id"`
	ChargePointID string            `json:"charge_point_id"`
	ConnectorID   int               `json:"connector_id"`
	IdTag         string            `json:"id_tag"`
	Status        TransactionStatus `json:"status"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       *time.Time        `json:"end_time,omitempty"`
	MeterStart    int               `json:"meter_start"`
	MeterStop     *int              `json:"meter_stop,omitempty"`
	StopReason    *string           `json:"stop_reason,omitempty"`
	ReservationID *int              `json:"reservation_id,omitempty"`
}

// MeterValue 统一的电表值
type MeterValue struct {
	Type      MeterValueType `json:"type"`
	Value     string         `json:"value"`
	Unit      *string        `json:"unit,omitempty"`
	Phase     *string        `json:"phase,omitempty"`
	Location  *string        `json:"location,omitempty"`
	Context   *string        `json:"context,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// AuthorizationInfo 授权信息
type AuthorizationInfo struct {
	IdTag       string              `json:"id_tag"`
	Result      AuthorizationResult `json:"result"`
	ExpiryDate  *time.Time          `json:"expiry_date,omitempty"`
	ParentIdTag *string             `json:"parent_id_tag,omitempty"`
	GroupIdTag  *string             `json:"group_id_tag,omitempty"`
}

// RemoteCommand 远程指令
type RemoteCommand struct {
	ID            string                 `json:"id"`
	ChargePointID string                 `json:"charge_point_id"`
	Type          CommandType            `json:"type"`
	Status        CommandStatus          `json:"status"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	ExecutedAt    *time.Time             `json:"executed_at,omitempty"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
	ErrorCode     *string                `json:"error_code,omitempty"`
	ErrorMessage  *string                `json:"error_message,omitempty"`
	Result        map[string]interface{} `json:"result,omitempty"`
}

// ConfigurationItem 配置项
type ConfigurationItem struct {
	Key      string  `json:"key"`
	Value    *string `json:"value,omitempty"`
	Readonly bool    `json:"readonly"`
}

// DataTransferInfo 数据传输信息
type DataTransferInfo struct {
	VendorID  string                 `json:"vendor_id"`
	MessageID *string                `json:"message_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Status    string                 `json:"status"`
}

// ErrorInfo 错误信息
type ErrorInfo struct {
	Code        ErrorCode `json:"code"`
	Description string    `json:"description"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Metadata 事件元数据
type Metadata struct {
	Source          string                 `json:"source"`           // 事件源标识
	CorrelationID   *string                `json:"correlation_id,omitempty"` // 关联ID
	CausationID     *string                `json:"causation_id,omitempty"`   // 因果ID
	UserID          *string                `json:"user_id,omitempty"`        // 用户ID
	SessionID       *string                `json:"session_id,omitempty"`     // 会话ID
	RequestID       *string                `json:"request_id,omitempty"`     // 请求ID
	ProtocolVersion string                 `json:"protocol_version"`         // 协议版本
	MessageID       *string                `json:"message_id,omitempty"`     // 原始消息ID
	Custom          map[string]interface{} `json:"custom,omitempty"`         // 自定义字段
}
