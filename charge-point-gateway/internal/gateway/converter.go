package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/domain/ocpp16"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
)

// ModelConverter 统一业务模型转换器接口
type ModelConverter interface {
	// ConvertToUnifiedEvent 将OCPP消息转换为统一业务事件
	ConvertToUnifiedEvent(ctx context.Context, chargePointID string, action string, payload interface{}) (events.Event, error)
	
	// ConvertBootNotification 转换BootNotification消息
	ConvertBootNotification(chargePointID string, req *ocpp16.BootNotificationRequest) (*events.ChargePointConnectedEvent, error)
	
	// ConvertHeartbeat 转换Heartbeat消息
	ConvertHeartbeat(chargePointID string, req *ocpp16.HeartbeatRequest) (events.Event, error)

	// ConvertStatusNotification 转换StatusNotification消息
	ConvertStatusNotification(chargePointID string, req *ocpp16.StatusNotificationRequest) (*events.ConnectorStatusChangedEvent, error)

	// ConvertMeterValues 转换MeterValues消息
	ConvertMeterValues(chargePointID string, req *ocpp16.MeterValuesRequest) (*events.MeterValuesReceivedEvent, error)
	
	// ConvertStartTransaction 转换StartTransaction消息
	ConvertStartTransaction(chargePointID string, req *ocpp16.StartTransactionRequest) (*events.TransactionStartedEvent, error)
	
	// ConvertStopTransaction 转换StopTransaction消息
	ConvertStopTransaction(chargePointID string, req *ocpp16.StopTransactionRequest) (*events.TransactionStoppedEvent, error)
	
	// GetSupportedActions 获取支持的转换动作列表
	GetSupportedActions() []string
}

// UnifiedModelConverter 统一业务模型转换器实现
type UnifiedModelConverter struct {
	// 事件工厂
	eventFactory *events.EventFactory
	
	// 转换规则配置
	config *ConverterConfig
	
	// 日志器
	logger *logger.Logger
}

// ConverterConfig 转换器配置
type ConverterConfig struct {
	// 是否启用严格模式（严格验证字段）
	StrictMode bool `json:"strict_mode"`
	
	// 默认超时时间
	DefaultTimeout time.Duration `json:"default_timeout"`
	
	// 是否启用字段映射日志
	EnableFieldMapping bool `json:"enable_field_mapping"`
	
	// 支持的OCPP版本
	SupportedVersions []string `json:"supported_versions"`
}

// DefaultConverterConfig 默认转换器配置
func DefaultConverterConfig() *ConverterConfig {
	return &ConverterConfig{
		StrictMode:         false,
		DefaultTimeout:     30 * time.Second,
		EnableFieldMapping: false,
		SupportedVersions:  []string{"1.6"},
	}
}

// NewUnifiedModelConverter 创建新的统一业务模型转换器
func NewUnifiedModelConverter(config *ConverterConfig) *UnifiedModelConverter {
	if config == nil {
		config = DefaultConverterConfig()
	}
	
	// 创建日志器
	l, _ := logger.New(logger.DefaultConfig())
	
	return &UnifiedModelConverter{
		eventFactory: events.NewEventFactory(),
		config:       config,
		logger:       l,
	}
}

// ConvertToUnifiedEvent 将OCPP消息转换为统一业务事件
func (c *UnifiedModelConverter) ConvertToUnifiedEvent(ctx context.Context, chargePointID string, action string, payload interface{}) (events.Event, error) {
	c.logger.Debugf("Converting OCPP action %s to unified event for charge point %s", action, chargePointID)
	
	switch action {
	case "BootNotification":
		if req, ok := payload.(*ocpp16.BootNotificationRequest); ok {
			return c.ConvertBootNotification(chargePointID, req)
		}
		return nil, fmt.Errorf("invalid payload type for BootNotification: %T", payload)
		
	case "Heartbeat":
		if req, ok := payload.(*ocpp16.HeartbeatRequest); ok {
			return c.ConvertHeartbeat(chargePointID, req)
		}
		return nil, fmt.Errorf("invalid payload type for Heartbeat: %T", payload)
		
	case "StatusNotification":
		if req, ok := payload.(*ocpp16.StatusNotificationRequest); ok {
			return c.ConvertStatusNotification(chargePointID, req)
		}
		return nil, fmt.Errorf("invalid payload type for StatusNotification: %T", payload)
		
	case "MeterValues":
		if req, ok := payload.(*ocpp16.MeterValuesRequest); ok {
			return c.ConvertMeterValues(chargePointID, req)
		}
		return nil, fmt.Errorf("invalid payload type for MeterValues: %T", payload)
		
	case "StartTransaction":
		if req, ok := payload.(*ocpp16.StartTransactionRequest); ok {
			return c.ConvertStartTransaction(chargePointID, req)
		}
		return nil, fmt.Errorf("invalid payload type for StartTransaction: %T", payload)
		
	case "StopTransaction":
		if req, ok := payload.(*ocpp16.StopTransactionRequest); ok {
			return c.ConvertStopTransaction(chargePointID, req)
		}
		return nil, fmt.Errorf("invalid payload type for StopTransaction: %T", payload)
		
	default:
		return nil, fmt.Errorf("unsupported OCPP action: %s", action)
	}
}

// ConvertBootNotification 转换BootNotification消息
func (c *UnifiedModelConverter) ConvertBootNotification(chargePointID string, req *ocpp16.BootNotificationRequest) (*events.ChargePointConnectedEvent, error) {
	if req == nil {
		return nil, fmt.Errorf("BootNotificationRequest is nil")
	}

	c.logger.Debugf("Converting BootNotification for charge point %s", chargePointID)

	// 创建充电桩信息
	chargePointInfo := events.ChargePointInfo{
		ID:              chargePointID,
		Vendor:          req.ChargePointVendor,
		Model:           req.ChargePointModel,
		SerialNumber:    req.ChargePointSerialNumber,
		FirmwareVersion: req.FirmwareVersion,
		LastSeen:        time.Now(),
		ProtocolVersion: "1.6",
	}

	// 创建元数据
	metadata := events.Metadata{
		Source:          "gateway",
		ProtocolVersion: "1.6",
	}

	// 创建连接事件
	event := c.eventFactory.CreateChargePointConnectedEvent(
		chargePointID,
		chargePointInfo,
		metadata,
	)

	return event, nil
}

// HeartbeatEvent 心跳事件
type HeartbeatEvent struct {
	*events.BaseEvent
}

// GetPayload 实现Event接口
func (e *HeartbeatEvent) GetPayload() interface{} {
	return map[string]interface{}{
		"timestamp": e.Timestamp,
	}
}

// ToJSON 实现Event接口
func (e *HeartbeatEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ConvertHeartbeat 转换Heartbeat消息
func (c *UnifiedModelConverter) ConvertHeartbeat(chargePointID string, req *ocpp16.HeartbeatRequest) (events.Event, error) {
	c.logger.Debugf("Converting Heartbeat for charge point %s", chargePointID)

	// 创建元数据
	metadata := events.Metadata{
		Source:          "gateway",
		ProtocolVersion: "1.6",
	}

	// 创建心跳事件
	event := &HeartbeatEvent{
		BaseEvent: events.NewBaseEvent(events.EventTypeChargePointHeartbeat, chargePointID, events.EventSeverityInfo, metadata),
	}

	return event, nil
}

// ConvertStatusNotification 转换StatusNotification消息
func (c *UnifiedModelConverter) ConvertStatusNotification(chargePointID string, req *ocpp16.StatusNotificationRequest) (*events.ConnectorStatusChangedEvent, error) {
	if req == nil {
		return nil, fmt.Errorf("StatusNotificationRequest is nil")
	}

	c.logger.Debugf("Converting StatusNotification for charge point %s, connector %d", chargePointID, req.ConnectorId)

	// 转换连接器状态
	var connectorStatus events.ConnectorStatus
	switch req.Status {
	case ocpp16.ChargePointStatusAvailable:
		connectorStatus = events.ConnectorStatusAvailable
	case ocpp16.ChargePointStatusPreparing:
		connectorStatus = events.ConnectorStatusPreparing
	case ocpp16.ChargePointStatusCharging:
		connectorStatus = events.ConnectorStatusCharging
	case ocpp16.ChargePointStatusSuspendedEVSE:
		connectorStatus = events.ConnectorStatusSuspendedEVSE
	case ocpp16.ChargePointStatusSuspendedEV:
		connectorStatus = events.ConnectorStatusSuspendedEV
	case ocpp16.ChargePointStatusFinishing:
		connectorStatus = events.ConnectorStatusFinishing
	case ocpp16.ChargePointStatusReserved:
		connectorStatus = events.ConnectorStatusReserved
	case ocpp16.ChargePointStatusUnavailable:
		connectorStatus = events.ConnectorStatusUnavailable
	case ocpp16.ChargePointStatusFaulted:
		connectorStatus = events.ConnectorStatusFaulted
	default:
		connectorStatus = events.ConnectorStatusUnavailable
	}

	// 创建连接器信息
	errorCodeStr := string(req.ErrorCode)
	connectorInfo := events.ConnectorInfo{
		ID:              req.ConnectorId,
		ChargePointID:   chargePointID,
		Status:          connectorStatus,
		ErrorCode:       &errorCodeStr,
		ErrorDescription: req.Info,
		VendorErrorCode: req.VendorErrorCode,
	}

	// 创建元数据
	metadata := events.Metadata{
		Source:          "gateway",
		ProtocolVersion: "1.6",
	}

	// 创建连接器状态变化事件
	event := c.eventFactory.CreateConnectorStatusChangedEvent(
		chargePointID,
		connectorInfo,
		events.ConnectorStatusUnavailable, // 假设之前状态为不可用
		metadata,
	)

	return event, nil
}

// ConvertMeterValues 转换MeterValues消息
func (c *UnifiedModelConverter) ConvertMeterValues(chargePointID string, req *ocpp16.MeterValuesRequest) (*events.MeterValuesReceivedEvent, error) {
	if req == nil {
		return nil, fmt.Errorf("MeterValuesRequest is nil")
	}

	c.logger.Debugf("Converting MeterValues for charge point %s, connector %d", chargePointID, req.ConnectorId)

	// 转换电表数据 - 将OCPP的复杂结构转换为简化的统一格式
	var meterValues []events.MeterValue
	for _, mv := range req.MeterValue {
		// 为每个采样值创建一个MeterValue条目
		for _, sv := range mv.SampledValue {
			// 确定电表值类型
			var meterType events.MeterValueType
			if sv.Measurand != nil {
				switch *sv.Measurand {
				case ocpp16.MeasurandEnergyActiveImportRegister:
					meterType = events.MeterValueTypeEnergyActiveImport
				case ocpp16.MeasurandPowerActiveImport:
					meterType = events.MeterValueTypePowerActiveImport
				case ocpp16.MeasurandCurrentImport:
					meterType = events.MeterValueTypeCurrentImport
				case ocpp16.MeasurandVoltage:
					meterType = events.MeterValueTypeVoltage
				case ocpp16.MeasurandTemperature:
					meterType = events.MeterValueTypeTemperature
				default:
					meterType = events.MeterValueTypeEnergyActiveImport // 默认值
				}
			} else {
				meterType = events.MeterValueTypeEnergyActiveImport // 默认值
			}

			// 转换可选字段
			var unit, phase, location, context *string
			if sv.Unit != nil {
				unitStr := string(*sv.Unit)
				unit = &unitStr
			}
			if sv.Phase != nil {
				phaseStr := string(*sv.Phase)
				phase = &phaseStr
			}
			if sv.Location != nil {
				locationStr := string(*sv.Location)
				location = &locationStr
			}
			if sv.Context != nil {
				contextStr := string(*sv.Context)
				context = &contextStr
			}

			meterValue := events.MeterValue{
				Type:      meterType,
				Value:     sv.Value,
				Unit:      unit,
				Phase:     phase,
				Location:  location,
				Context:   context,
				Timestamp: mv.Timestamp.Time,
			}
			meterValues = append(meterValues, meterValue)
		}
	}

	// 创建元数据
	metadata := events.Metadata{
		Source:          "gateway",
		ProtocolVersion: "1.6",
	}

	// 创建电表数据事件
	event := &events.MeterValuesReceivedEvent{
		BaseEvent:     events.NewBaseEvent(events.EventTypeMeterValuesReceived, chargePointID, events.EventSeverityInfo, metadata),
		ConnectorID:   req.ConnectorId,
		TransactionID: req.TransactionId,
		MeterValues:   meterValues,
	}

	return event, nil
}

// ConvertStartTransaction 转换StartTransaction消息
func (c *UnifiedModelConverter) ConvertStartTransaction(chargePointID string, req *ocpp16.StartTransactionRequest) (*events.TransactionStartedEvent, error) {
	if req == nil {
		return nil, fmt.Errorf("StartTransactionRequest is nil")
	}

	c.logger.Debugf("Converting StartTransaction for charge point %s, connector %d", chargePointID, req.ConnectorId)

	// 创建交易信息
	transactionInfo := events.TransactionInfo{
		ID:            0, // 将由后端系统分配
		ChargePointID: chargePointID,
		ConnectorID:   req.ConnectorId,
		IdTag:         req.IdTag,
		StartTime:     req.Timestamp.Time,
		MeterStart:    req.MeterStart,
		ReservationID: req.ReservationId,
	}

	// 创建授权信息（简化版）
	authInfo := events.AuthorizationInfo{
		IdTag:  req.IdTag,
		Result: events.AuthorizationResultAccepted,
	}

	// 创建元数据
	metadata := events.Metadata{
		Source:          "gateway",
		ProtocolVersion: "1.6",
	}

	// 创建交易开始事件
	event := c.eventFactory.CreateTransactionStartedEvent(
		chargePointID,
		transactionInfo,
		authInfo,
		metadata,
	)

	return event, nil
}

// ConvertStopTransaction 转换StopTransaction消息
func (c *UnifiedModelConverter) ConvertStopTransaction(chargePointID string, req *ocpp16.StopTransactionRequest) (*events.TransactionStoppedEvent, error) {
	if req == nil {
		return nil, fmt.Errorf("StopTransactionRequest is nil")
	}

	c.logger.Debugf("Converting StopTransaction for charge point %s, transaction %d", chargePointID, req.TransactionId)

	// 转换停止原因 - 直接转换为字符串
	var reasonStr string
	if req.Reason != nil {
		switch *req.Reason {
		case ocpp16.ReasonEmergencyStop:
			reasonStr = "emergency_stop"
		case ocpp16.ReasonEVDisconnected:
			reasonStr = "ev_disconnected"
		case ocpp16.ReasonHardReset:
			reasonStr = "hard_reset"
		case ocpp16.ReasonLocal:
			reasonStr = "local"
		case ocpp16.ReasonOther:
			reasonStr = "other"
		case ocpp16.ReasonPowerLoss:
			reasonStr = "power_loss"
		case ocpp16.ReasonReboot:
			reasonStr = "reboot"
		case ocpp16.ReasonRemote:
			reasonStr = "remote"
		case ocpp16.ReasonSoftReset:
			reasonStr = "soft_reset"
		case ocpp16.ReasonUnlockCommand:
			reasonStr = "unlock_command"
		case ocpp16.ReasonDeAuthorized:
			reasonStr = "deauthorized"
		default:
			reasonStr = "other"
		}
	} else {
		reasonStr = "unknown"
	}

	// 转换交易数据（如果有） - 使用与MeterValues相同的转换逻辑
	var transactionData []events.MeterValue
	if req.TransactionData != nil {
		for _, mv := range req.TransactionData {
			// 为每个采样值创建一个MeterValue条目
			for _, sv := range mv.SampledValue {
				// 确定电表值类型
				var meterType events.MeterValueType
				if sv.Measurand != nil {
					switch *sv.Measurand {
					case ocpp16.MeasurandEnergyActiveImportRegister:
						meterType = events.MeterValueTypeEnergyActiveImport
					case ocpp16.MeasurandPowerActiveImport:
						meterType = events.MeterValueTypePowerActiveImport
					case ocpp16.MeasurandCurrentImport:
						meterType = events.MeterValueTypeCurrentImport
					case ocpp16.MeasurandVoltage:
						meterType = events.MeterValueTypeVoltage
					case ocpp16.MeasurandTemperature:
						meterType = events.MeterValueTypeTemperature
					default:
						meterType = events.MeterValueTypeEnergyActiveImport // 默认值
					}
				} else {
					meterType = events.MeterValueTypeEnergyActiveImport // 默认值
				}

				// 转换可选字段
				var unit, phase, location, context *string
				if sv.Unit != nil {
					unitStr := string(*sv.Unit)
					unit = &unitStr
				}
				if sv.Phase != nil {
					phaseStr := string(*sv.Phase)
					phase = &phaseStr
				}
				if sv.Location != nil {
					locationStr := string(*sv.Location)
					location = &locationStr
				}
				if sv.Context != nil {
					contextStr := string(*sv.Context)
					context = &contextStr
				}

				meterValue := events.MeterValue{
					Type:      meterType,
					Value:     sv.Value,
					Unit:      unit,
					Phase:     phase,
					Location:  location,
					Context:   context,
					Timestamp: mv.Timestamp.Time,
				}
				transactionData = append(transactionData, meterValue)
			}
		}
	}

	// 处理可选的IdTag
	var idTag string
	if req.IdTag != nil {
		idTag = *req.IdTag
	}

	// 创建交易信息
	transactionInfo := events.TransactionInfo{
		ID:            req.TransactionId,
		ChargePointID: chargePointID,
		IdTag:         idTag,
		EndTime:       &req.Timestamp.Time,
		MeterStop:     &req.MeterStop,
		StopReason:    &reasonStr,
	}

	// 创建元数据
	metadata := events.Metadata{
		Source:          "gateway",
		ProtocolVersion: "1.6",
	}

	// 创建交易停止事件
	event := &events.TransactionStoppedEvent{
		BaseEvent:       events.NewBaseEvent(events.EventTypeTransactionStopped, chargePointID, events.EventSeverityInfo, metadata),
		TransactionInfo: transactionInfo,
		MeterValues:     transactionData,
	}

	return event, nil
}

// GetSupportedActions 获取支持的转换动作列表
func (c *UnifiedModelConverter) GetSupportedActions() []string {
	return []string{
		"BootNotification",
		"Heartbeat",
		"StatusNotification",
		"MeterValues",
		"StartTransaction",
		"StopTransaction",
	}
}
