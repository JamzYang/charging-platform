package ocpp16

// Message OCPP消息基础结构
type Message struct {
	MessageTypeID MessageType `json:"messageTypeId"`
	MessageID     string      `json:"messageId"`
	Action        Action      `json:"action,omitempty"`
	Payload       interface{} `json:"payload,omitempty"`
}

// CallMessage 请求消息
type CallMessage struct {
	MessageTypeID MessageType `json:"messageTypeId"`
	MessageID     string      `json:"messageId"`
	Action        Action      `json:"action"`
	Payload       interface{} `json:"payload"`
}

// CallResultMessage 响应消息
type CallResultMessage struct {
	MessageTypeID MessageType `json:"messageTypeId"`
	MessageID     string      `json:"messageId"`
	Payload       interface{} `json:"payload"`
}

// CallErrorMessage 错误消息
type CallErrorMessage struct {
	MessageTypeID    MessageType `json:"messageTypeId"`
	MessageID        string      `json:"messageId"`
	ErrorCode        string      `json:"errorCode"`
	ErrorDescription string      `json:"errorDescription"`
	ErrorDetails     interface{} `json:"errorDetails,omitempty"`
}

// BootNotificationRequest 启动通知请求
type BootNotificationRequest struct {
	ChargePointVendor       string  `json:"chargePointVendor" validate:"required,max=20"`
	ChargePointModel        string  `json:"chargePointModel" validate:"required,max=20"`
	ChargePointSerialNumber *string `json:"chargePointSerialNumber,omitempty" validate:"omitempty,max=25"`
	ChargeBoxSerialNumber   *string `json:"chargeBoxSerialNumber,omitempty" validate:"omitempty,max=25"`
	FirmwareVersion         *string `json:"firmwareVersion,omitempty" validate:"omitempty,max=50"`
	Iccid                   *string `json:"iccid,omitempty" validate:"omitempty,max=20"`
	Imsi                    *string `json:"imsi,omitempty" validate:"omitempty,max=20"`
	MeterType               *string `json:"meterType,omitempty" validate:"omitempty,max=25"`
	MeterSerialNumber       *string `json:"meterSerialNumber,omitempty" validate:"omitempty,max=25"`
}

// BootNotificationResponse 启动通知响应
type BootNotificationResponse struct {
	Status      RegistrationStatus `json:"status" validate:"required"`
	CurrentTime DateTime           `json:"currentTime" validate:"required"`
	Interval    int                `json:"interval" validate:"required,min=0"`
}

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct{}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	CurrentTime DateTime `json:"currentTime" validate:"required"`
}

// StatusNotificationRequest 状态通知请求
type StatusNotificationRequest struct {
	ConnectorId     int                   `json:"connectorId" validate:"required,min=0"`
	ErrorCode       ChargePointErrorCode  `json:"errorCode" validate:"required"`
	Info            *string               `json:"info,omitempty" validate:"omitempty,max=50"`
	Status          ChargePointStatus     `json:"status" validate:"required"`
	Timestamp       *DateTime             `json:"timestamp,omitempty"`
	VendorId        *string               `json:"vendorId,omitempty" validate:"omitempty,max=255"`
	VendorErrorCode *string               `json:"vendorErrorCode,omitempty" validate:"omitempty,max=50"`
}

// StatusNotificationResponse 状态通知响应
type StatusNotificationResponse struct{}

// AuthorizeRequest 授权请求
type AuthorizeRequest struct {
	IdTag string `json:"idTag" validate:"required,max=20"`
}

// AuthorizeResponse 授权响应
type AuthorizeResponse struct {
	IdTagInfo IdTagInfo `json:"idTagInfo" validate:"required"`
}

// StartTransactionRequest 开始交易请求
type StartTransactionRequest struct {
	ConnectorId   int       `json:"connectorId" validate:"required,min=1"`
	IdTag         string    `json:"idTag" validate:"required,max=20"`
	MeterStart    int       `json:"meterStart" validate:"required,min=0"`
	ReservationId *int      `json:"reservationId,omitempty"`
	Timestamp     DateTime  `json:"timestamp" validate:"required"`
}

// StartTransactionResponse 开始交易响应
type StartTransactionResponse struct {
	IdTagInfo     IdTagInfo `json:"idTagInfo" validate:"required"`
	TransactionId int       `json:"transactionId" validate:"required"`
}

// StopTransactionRequest 停止交易请求
type StopTransactionRequest struct {
	IdTag             *string       `json:"idTag,omitempty" validate:"omitempty,max=20"`
	MeterStop         int           `json:"meterStop" validate:"required,min=0"`
	Timestamp         DateTime      `json:"timestamp" validate:"required"`
	TransactionId     int           `json:"transactionId" validate:"required"`
	Reason            *Reason       `json:"reason,omitempty"`
	TransactionData   []MeterValue  `json:"transactionData,omitempty"`
}

// StopTransactionResponse 停止交易响应
type StopTransactionResponse struct {
	IdTagInfo *IdTagInfo `json:"idTagInfo,omitempty"`
}

// MeterValuesRequest 电表值请求
type MeterValuesRequest struct {
	ConnectorId     int          `json:"connectorId" validate:"required,min=0"`
	TransactionId   *int         `json:"transactionId,omitempty"`
	MeterValue      []MeterValue `json:"meterValue" validate:"required,min=1"`
}

// MeterValuesResponse 电表值响应
type MeterValuesResponse struct{}

// DataTransferRequest 数据传输请求
type DataTransferRequest struct {
	VendorId  string      `json:"vendorId" validate:"required,max=255"`
	MessageId *string     `json:"messageId,omitempty" validate:"omitempty,max=50"`
	Data      interface{} `json:"data,omitempty"`
}

// DataTransferResponse 数据传输响应
type DataTransferResponse struct {
	Status DataTransferStatus `json:"status" validate:"required"`
	Data   interface{}        `json:"data,omitempty"`
}

// DataTransferStatus 数据传输状态
type DataTransferStatus string

const (
	DataTransferStatusAccepted         DataTransferStatus = "Accepted"
	DataTransferStatusRejected         DataTransferStatus = "Rejected"
	DataTransferStatusUnknownMessageId DataTransferStatus = "UnknownMessageId"
	DataTransferStatusUnknownVendorId  DataTransferStatus = "UnknownVendorId"
)

// ResetRequest 重置请求
type ResetRequest struct {
	Type ResetType `json:"type" validate:"required"`
}

// ResetResponse 重置响应
type ResetResponse struct {
	Status ResetStatus `json:"status" validate:"required"`
}

// ResetStatus 重置状态
type ResetStatus string

const (
	ResetStatusAccepted ResetStatus = "Accepted"
	ResetStatusRejected ResetStatus = "Rejected"
)

// ChangeAvailabilityRequest 改变可用性请求
type ChangeAvailabilityRequest struct {
	ConnectorId int              `json:"connectorId" validate:"required,min=0"`
	Type        AvailabilityType `json:"type" validate:"required"`
}

// ChangeAvailabilityResponse 改变可用性响应
type ChangeAvailabilityResponse struct {
	Status AvailabilityStatus `json:"status" validate:"required"`
}

// GetConfigurationRequest 获取配置请求
type GetConfigurationRequest struct {
	Key []string `json:"key,omitempty"`
}

// GetConfigurationResponse 获取配置响应
type GetConfigurationResponse struct {
	ConfigurationKey []KeyValue `json:"configurationKey,omitempty"`
	UnknownKey       []string   `json:"unknownKey,omitempty"`
}

// ChangeConfigurationRequest 改变配置请求
type ChangeConfigurationRequest struct {
	Key   string `json:"key" validate:"required,max=50"`
	Value string `json:"value" validate:"required,max=500"`
}

// ChangeConfigurationResponse 改变配置响应
type ChangeConfigurationResponse struct {
	Status ConfigurationStatus `json:"status" validate:"required"`
}

// ClearCacheRequest 清除缓存请求
type ClearCacheRequest struct{}

// ClearCacheResponse 清除缓存响应
type ClearCacheResponse struct {
	Status ClearCacheStatus `json:"status" validate:"required"`
}

// UnlockConnectorRequest 解锁连接器请求
type UnlockConnectorRequest struct {
	ConnectorId int `json:"connectorId" validate:"required,min=1"`
}

// UnlockConnectorResponse 解锁连接器响应
type UnlockConnectorResponse struct {
	Status UnlockStatus `json:"status" validate:"required"`
}

// RemoteStartTransactionRequest 远程开始交易请求
type RemoteStartTransactionRequest struct {
	ConnectorId   *int                `json:"connectorId,omitempty" validate:"omitempty,min=1"`
	IdTag         string              `json:"idTag" validate:"required,max=20"`
	ChargingProfile *ChargingProfile  `json:"chargingProfile,omitempty"`
}

// RemoteStartTransactionResponse 远程开始交易响应
type RemoteStartTransactionResponse struct {
	Status RemoteStartStopStatus `json:"status" validate:"required"`
}

// RemoteStopTransactionRequest 远程停止交易请求
type RemoteStopTransactionRequest struct {
	TransactionId int `json:"transactionId" validate:"required"`
}

// RemoteStopTransactionResponse 远程停止交易响应
type RemoteStopTransactionResponse struct {
	Status RemoteStartStopStatus `json:"status" validate:"required"`
}

// ChargingProfile 充电配置文件
type ChargingProfile struct {
	ChargingProfileId      int                    `json:"chargingProfileId" validate:"required"`
	TransactionId          *int                   `json:"transactionId,omitempty"`
	StackLevel             int                    `json:"stackLevel" validate:"required,min=0"`
	ChargingProfilePurpose ChargingProfilePurpose `json:"chargingProfilePurpose" validate:"required"`
	ChargingProfileKind    ChargingProfileKind    `json:"chargingProfileKind" validate:"required"`
	RecurrencyKind         *RecurrencyKind        `json:"recurrencyKind,omitempty"`
	ValidFrom              *DateTime              `json:"validFrom,omitempty"`
	ValidTo                *DateTime              `json:"validTo,omitempty"`
	ChargingSchedule       ChargingSchedule       `json:"chargingSchedule" validate:"required"`
}

// ChargingProfilePurpose 充电配置文件目的
type ChargingProfilePurpose string

const (
	ChargingProfilePurposeChargePointMaxProfile ChargingProfilePurpose = "ChargePointMaxProfile"
	ChargingProfilePurposeTxDefaultProfile      ChargingProfilePurpose = "TxDefaultProfile"
	ChargingProfilePurposeTxProfile             ChargingProfilePurpose = "TxProfile"
)

// ChargingProfileKind 充电配置文件类型
type ChargingProfileKind string

const (
	ChargingProfileKindAbsolute  ChargingProfileKind = "Absolute"
	ChargingProfileKindRecurring ChargingProfileKind = "Recurring"
	ChargingProfileKindRelative  ChargingProfileKind = "Relative"
)

// RecurrencyKind 重复类型
type RecurrencyKind string

const (
	RecurrencyKindDaily  RecurrencyKind = "Daily"
	RecurrencyKindWeekly RecurrencyKind = "Weekly"
)

// ChargingSchedule 充电计划
type ChargingSchedule struct {
	Duration               *int                     `json:"duration,omitempty" validate:"omitempty,min=0"`
	StartSchedule          *DateTime                `json:"startSchedule,omitempty"`
	ChargingRateUnit       ChargingRateUnit         `json:"chargingRateUnit" validate:"required"`
	ChargingSchedulePeriod []ChargingSchedulePeriod `json:"chargingSchedulePeriod" validate:"required,min=1"`
	MinChargingRate        *float64                 `json:"minChargingRate,omitempty"`
}

// ChargingRateUnit 充电速率单位
type ChargingRateUnit string

const (
	ChargingRateUnitW ChargingRateUnit = "W"
	ChargingRateUnitA ChargingRateUnit = "A"
)

// ChargingSchedulePeriod 充电计划周期
type ChargingSchedulePeriod struct {
	StartPeriod  int      `json:"startPeriod" validate:"required,min=0"`
	Limit        float64  `json:"limit" validate:"required"`
	NumberPhases *int     `json:"numberPhases,omitempty" validate:"omitempty,min=1,max=3"`
}
