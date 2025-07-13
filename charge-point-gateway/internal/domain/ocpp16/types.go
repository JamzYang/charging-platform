package ocpp16

import (
	"time"
)

// MessageType OCPP消息类型
type MessageType int

const (
	// Call 请求消息
	Call MessageType = 2
	// CallResult 响应消息
	CallResult MessageType = 3
	// CallError 错误消息
	CallError MessageType = 4
)

// Action OCPP动作类型
type Action string

const (
	// Core Profile Actions
	ActionAuthorize              Action = "Authorize"
	ActionBootNotification       Action = "BootNotification"
	ActionChangeAvailability     Action = "ChangeAvailability"
	ActionChangeConfiguration    Action = "ChangeConfiguration"
	ActionClearCache             Action = "ClearCache"
	ActionDataTransfer           Action = "DataTransfer"
	ActionGetConfiguration       Action = "GetConfiguration"
	ActionHeartbeat              Action = "Heartbeat"
	ActionMeterValues            Action = "MeterValues"
	ActionRemoteStartTransaction Action = "RemoteStartTransaction"
	ActionRemoteStopTransaction  Action = "RemoteStopTransaction"
	ActionReset                  Action = "Reset"
	ActionStartTransaction       Action = "StartTransaction"
	ActionStatusNotification     Action = "StatusNotification"
	ActionStopTransaction        Action = "StopTransaction"
	ActionUnlockConnector        Action = "UnlockConnector"

	// Firmware Management Profile Actions
	ActionGetDiagnostics    Action = "GetDiagnostics"
	ActionDiagnosticsStatusNotification Action = "DiagnosticsStatusNotification"
	ActionFirmwareStatusNotification    Action = "FirmwareStatusNotification"
	ActionUpdateFirmware    Action = "UpdateFirmware"

	// Local Auth List Management Profile Actions
	ActionGetLocalListVersion Action = "GetLocalListVersion"
	ActionSendLocalList       Action = "SendLocalList"

	// Reservation Profile Actions
	ActionCancelReservation Action = "CancelReservation"
	ActionReserveNow        Action = "ReserveNow"

	// Smart Charging Profile Actions
	ActionClearChargingProfile Action = "ClearChargingProfile"
	ActionGetCompositeSchedule Action = "GetCompositeSchedule"
	ActionSetChargingProfile   Action = "SetChargingProfile"

	// Trigger Message Profile Actions
	ActionTriggerMessage Action = "TriggerMessage"
)

// ChargePointStatus 充电桩状态
type ChargePointStatus string

const (
	ChargePointStatusAvailable     ChargePointStatus = "Available"
	ChargePointStatusPreparing     ChargePointStatus = "Preparing"
	ChargePointStatusCharging      ChargePointStatus = "Charging"
	ChargePointStatusSuspendedEVSE ChargePointStatus = "SuspendedEVSE"
	ChargePointStatusSuspendedEV   ChargePointStatus = "SuspendedEV"
	ChargePointStatusFinishing     ChargePointStatus = "Finishing"
	ChargePointStatusReserved      ChargePointStatus = "Reserved"
	ChargePointStatusUnavailable   ChargePointStatus = "Unavailable"
	ChargePointStatusFaulted       ChargePointStatus = "Faulted"
)

// ChargePointErrorCode 充电桩错误代码
type ChargePointErrorCode string

const (
	ChargePointErrorCodeConnectorLockFailure         ChargePointErrorCode = "ConnectorLockFailure"
	ChargePointErrorCodeEVCommunicationError         ChargePointErrorCode = "EVCommunicationError"
	ChargePointErrorCodeGroundFailure                ChargePointErrorCode = "GroundFailure"
	ChargePointErrorCodeHighTemperature              ChargePointErrorCode = "HighTemperature"
	ChargePointErrorCodeInternalError                ChargePointErrorCode = "InternalError"
	ChargePointErrorCodeLocalListConflict            ChargePointErrorCode = "LocalListConflict"
	ChargePointErrorCodeNoError                      ChargePointErrorCode = "NoError"
	ChargePointErrorCodeOtherError                   ChargePointErrorCode = "OtherError"
	ChargePointErrorCodeOverCurrentFailure           ChargePointErrorCode = "OverCurrentFailure"
	ChargePointErrorCodeOverVoltage                  ChargePointErrorCode = "OverVoltage"
	ChargePointErrorCodePowerMeterFailure            ChargePointErrorCode = "PowerMeterFailure"
	ChargePointErrorCodePowerSwitchFailure           ChargePointErrorCode = "PowerSwitchFailure"
	ChargePointErrorCodeReaderFailure                ChargePointErrorCode = "ReaderFailure"
	ChargePointErrorCodeResetFailure                 ChargePointErrorCode = "ResetFailure"
	ChargePointErrorCodeUnderVoltage                 ChargePointErrorCode = "UnderVoltage"
	ChargePointErrorCodeWeakSignal                   ChargePointErrorCode = "WeakSignal"
)

// RegistrationStatus 注册状态
type RegistrationStatus string

const (
	RegistrationStatusAccepted RegistrationStatus = "Accepted"
	RegistrationStatusPending  RegistrationStatus = "Pending"
	RegistrationStatusRejected RegistrationStatus = "Rejected"
)

// AuthorizationStatus 授权状态
type AuthorizationStatus string

const (
	AuthorizationStatusAccepted     AuthorizationStatus = "Accepted"
	AuthorizationStatusBlocked      AuthorizationStatus = "Blocked"
	AuthorizationStatusExpired      AuthorizationStatus = "Expired"
	AuthorizationStatusInvalid      AuthorizationStatus = "Invalid"
	AuthorizationStatusConcurrentTx AuthorizationStatus = "ConcurrentTx"
)

// ResetType 重置类型
type ResetType string

const (
	ResetTypeHard ResetType = "Hard"
	ResetTypeSoft ResetType = "Soft"
)

// AvailabilityType 可用性类型
type AvailabilityType string

const (
	AvailabilityTypeInoperative AvailabilityType = "Inoperative"
	AvailabilityTypeOperative   AvailabilityType = "Operative"
)

// AvailabilityStatus 可用性状态
type AvailabilityStatus string

const (
	AvailabilityStatusAccepted  AvailabilityStatus = "Accepted"
	AvailabilityStatusRejected  AvailabilityStatus = "Rejected"
	AvailabilityStatusScheduled AvailabilityStatus = "Scheduled"
)

// ConfigurationStatus 配置状态
type ConfigurationStatus string

const (
	ConfigurationStatusAccepted       ConfigurationStatus = "Accepted"
	ConfigurationStatusRejected       ConfigurationStatus = "Rejected"
	ConfigurationStatusRebootRequired ConfigurationStatus = "RebootRequired"
	ConfigurationStatusNotSupported   ConfigurationStatus = "NotSupported"
)

// ClearCacheStatus 清除缓存状态
type ClearCacheStatus string

const (
	ClearCacheStatusAccepted ClearCacheStatus = "Accepted"
	ClearCacheStatusRejected ClearCacheStatus = "Rejected"
)

// UnlockStatus 解锁状态
type UnlockStatus string

const (
	UnlockStatusUnlocked         UnlockStatus = "Unlocked"
	UnlockStatusUnlockFailed     UnlockStatus = "UnlockFailed"
	UnlockStatusNotSupported     UnlockStatus = "NotSupported"
	UnlockStatusOngoingAuthorizedTransaction UnlockStatus = "OngoingAuthorizedTransaction"
)

// Reason 停止原因
type Reason string

const (
	ReasonEmergencyStop     Reason = "EmergencyStop"
	ReasonEVDisconnected    Reason = "EVDisconnected"
	ReasonHardReset         Reason = "HardReset"
	ReasonLocal             Reason = "Local"
	ReasonOther             Reason = "Other"
	ReasonPowerLoss         Reason = "PowerLoss"
	ReasonReboot            Reason = "Reboot"
	ReasonRemote            Reason = "Remote"
	ReasonSoftReset         Reason = "SoftReset"
	ReasonUnlockCommand     Reason = "UnlockCommand"
	ReasonDeAuthorized      Reason = "DeAuthorized"
)

// RemoteStartStopStatus 远程启动停止状态
type RemoteStartStopStatus string

const (
	RemoteStartStopStatusAccepted RemoteStartStopStatus = "Accepted"
	RemoteStartStopStatusRejected RemoteStartStopStatus = "Rejected"
)

// DateTime 自定义时间类型，用于JSON序列化
type DateTime struct {
	time.Time
}

// MarshalJSON 实现JSON序列化
func (dt DateTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + dt.Time.Format(time.RFC3339) + `"`), nil
}

// UnmarshalJSON 实现JSON反序列化
func (dt *DateTime) UnmarshalJSON(data []byte) error {
	str := string(data)
	if str == "null" {
		return nil
	}
	str = str[1 : len(str)-1] // 去掉引号
	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return err
	}
	dt.Time = t
	return nil
}

// IdToken ID令牌
type IdToken struct {
	IdToken string `json:"idToken" validate:"required,max=20"`
}

// IdTagInfo ID标签信息
type IdTagInfo struct {
	ExpiryDate  *DateTime            `json:"expiryDate,omitempty"`
	ParentIdTag *string              `json:"parentIdTag,omitempty" validate:"omitempty,max=20"`
	Status      AuthorizationStatus  `json:"status" validate:"required"`
}

// KeyValue 键值对
type KeyValue struct {
	Key      string  `json:"key" validate:"required,max=50"`
	Readonly bool    `json:"readonly"`
	Value    *string `json:"value,omitempty" validate:"omitempty,max=500"`
}

// MeterValue 电表值
type MeterValue struct {
	Timestamp    DateTime      `json:"timestamp" validate:"required"`
	SampledValue []SampledValue `json:"sampledValue" validate:"required,min=1"`
}

// SampledValue 采样值
type SampledValue struct {
	Value     string     `json:"value" validate:"required"`
	Context   *ReadingContext `json:"context,omitempty"`
	Format    *ValueFormat    `json:"format,omitempty"`
	Measurand *Measurand      `json:"measurand,omitempty"`
	Phase     *Phase          `json:"phase,omitempty"`
	Location  *Location       `json:"location,omitempty"`
	Unit      *UnitOfMeasure  `json:"unit,omitempty"`
}

// ReadingContext 读数上下文
type ReadingContext string

const (
	ReadingContextInterruptionBegin ReadingContext = "Interruption.Begin"
	ReadingContextInterruptionEnd   ReadingContext = "Interruption.End"
	ReadingContextSampleClock       ReadingContext = "Sample.Clock"
	ReadingContextSamplePeriodic    ReadingContext = "Sample.Periodic"
	ReadingContextTransactionBegin  ReadingContext = "Transaction.Begin"
	ReadingContextTransactionEnd    ReadingContext = "Transaction.End"
	ReadingContextTrigger           ReadingContext = "Trigger"
	ReadingContextOther             ReadingContext = "Other"
)

// ValueFormat 值格式
type ValueFormat string

const (
	ValueFormatRaw       ValueFormat = "Raw"
	ValueFormatSignedData ValueFormat = "SignedData"
)

// Measurand 测量值类型
type Measurand string

const (
	MeasurandCurrentExport                Measurand = "Current.Export"
	MeasurandCurrentImport                Measurand = "Current.Import"
	MeasurandCurrentOffered               Measurand = "Current.Offered"
	MeasurandEnergyActiveExportRegister   Measurand = "Energy.Active.Export.Register"
	MeasurandEnergyActiveImportRegister   Measurand = "Energy.Active.Import.Register"
	MeasurandEnergyReactiveExportRegister Measurand = "Energy.Reactive.Export.Register"
	MeasurandEnergyReactiveImportRegister Measurand = "Energy.Reactive.Import.Register"
	MeasurandEnergyActiveExportInterval   Measurand = "Energy.Active.Export.Interval"
	MeasurandEnergyActiveImportInterval   Measurand = "Energy.Active.Import.Interval"
	MeasurandEnergyReactiveExportInterval Measurand = "Energy.Reactive.Export.Interval"
	MeasurandEnergyReactiveImportInterval Measurand = "Energy.Reactive.Import.Interval"
	MeasurandFrequency                    Measurand = "Frequency"
	MeasurandPowerActiveExport            Measurand = "Power.Active.Export"
	MeasurandPowerActiveImport            Measurand = "Power.Active.Import"
	MeasurandPowerFactor                  Measurand = "Power.Factor"
	MeasurandPowerOffered                 Measurand = "Power.Offered"
	MeasurandPowerReactiveExport          Measurand = "Power.Reactive.Export"
	MeasurandPowerReactiveImport          Measurand = "Power.Reactive.Import"
	MeasurandRPM                          Measurand = "RPM"
	MeasurandSoC                          Measurand = "SoC"
	MeasurandTemperature                  Measurand = "Temperature"
	MeasurandVoltage                      Measurand = "Voltage"
)

// Phase 相位
type Phase string

const (
	PhaseL1   Phase = "L1"
	PhaseL2   Phase = "L2"
	PhaseL3   Phase = "L3"
	PhaseN    Phase = "N"
	PhaseL1N  Phase = "L1-N"
	PhaseL2N  Phase = "L2-N"
	PhaseL3N  Phase = "L3-N"
	PhaseL1L2 Phase = "L1-L2"
	PhaseL2L3 Phase = "L2-L3"
	PhaseL3L1 Phase = "L3-L1"
)

// Location 位置
type Location string

const (
	LocationBody   Location = "Body"
	LocationCable  Location = "Cable"
	LocationEV     Location = "EV"
	LocationInlet  Location = "Inlet"
	LocationOutlet Location = "Outlet"
)

// UnitOfMeasure 测量单位
type UnitOfMeasure string

const (
	UnitOfMeasureWh       UnitOfMeasure = "Wh"
	UnitOfMeasureKWh      UnitOfMeasure = "kWh"
	UnitOfMeasureVarh     UnitOfMeasure = "varh"
	UnitOfMeasureKvarh    UnitOfMeasure = "kvarh"
	UnitOfMeasureW        UnitOfMeasure = "W"
	UnitOfMeasureKW       UnitOfMeasure = "kW"
	UnitOfMeasureVA       UnitOfMeasure = "VA"
	UnitOfMeasureKVA      UnitOfMeasure = "kVA"
	UnitOfMeasureVar      UnitOfMeasure = "var"
	UnitOfMeasureKvar     UnitOfMeasure = "kvar"
	UnitOfMeasureA        UnitOfMeasure = "A"
	UnitOfMeasureV        UnitOfMeasure = "V"
	UnitOfMeasureCelsius  UnitOfMeasure = "Celsius"
	UnitOfMeasureFahrenheit UnitOfMeasure = "Fahrenheit"
	UnitOfMeasureK        UnitOfMeasure = "K"
	UnitOfMeasurePercent  UnitOfMeasure = "Percent"
)
