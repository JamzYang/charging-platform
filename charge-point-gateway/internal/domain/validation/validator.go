package validation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// Validator OCPP消息验证器
type Validator struct {
	validate *validator.Validate
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// Error 实现error接口
func (e ValidationError) Error() string {
	return e.Message
}

// ValidationErrors 验证错误集合
type ValidationErrors []ValidationError

// Error 实现error接口
func (e ValidationErrors) Error() string {
	var messages []string
	for _, err := range e {
		messages = append(messages, err.Message)
	}
	return strings.Join(messages, "; ")
}

// NewValidator 创建新的验证器
func NewValidator() *Validator {
	validate := validator.New()
	
	// 注册自定义验证规则
	registerCustomValidations(validate)
	
	return &Validator{
		validate: validate,
	}
}

// ValidateStruct 验证结构体
func (v *Validator) ValidateStruct(s interface{}) error {
	err := v.validate.Struct(s)
	if err == nil {
		return nil
	}
	
	var validationErrors ValidationErrors
	
	if validatorErrors, ok := err.(validator.ValidationErrors); ok {
		for _, validatorError := range validatorErrors {
			validationError := ValidationError{
				Field:   validatorError.Field(),
				Tag:     validatorError.Tag(),
				Value:   fmt.Sprintf("%v", validatorError.Value()),
				Message: getErrorMessage(validatorError),
			}
			validationErrors = append(validationErrors, validationError)
		}
	}
	
	return validationErrors
}

// ValidateJSON 验证JSON格式
func (v *Validator) ValidateJSON(data []byte) error {
	var temp interface{}
	return json.Unmarshal(data, &temp)
}

// ValidateOCPPMessage 验证OCPP消息格式
func (v *Validator) ValidateOCPPMessage(messageType int, messageID string, action string, payload interface{}) error {
	// 验证消息类型
	if messageType < 2 || messageType > 4 {
		return ValidationError{
			Field:   "messageType",
			Tag:     "range",
			Value:   strconv.Itoa(messageType),
			Message: "Message type must be 2 (Call), 3 (CallResult), or 4 (CallError)",
		}
	}
	
	// 验证消息ID
	if messageID == "" {
		return ValidationError{
			Field:   "messageId",
			Tag:     "required",
			Value:   "",
			Message: "Message ID is required",
		}
	}
	
	if len(messageID) > 36 {
		return ValidationError{
			Field:   "messageId",
			Tag:     "max",
			Value:   messageID,
			Message: "Message ID must not exceed 36 characters",
		}
	}
	
	// 对于Call消息，验证action
	if messageType == 2 {
		if action == "" {
			return ValidationError{
				Field:   "action",
				Tag:     "required",
				Value:   "",
				Message: "Action is required for Call messages",
			}
		}
		
		if !isValidAction(action) {
			return ValidationError{
				Field:   "action",
				Tag:     "invalid",
				Value:   action,
				Message: "Invalid OCPP action",
			}
		}
	}
	
	// 验证payload
	if payload != nil {
		return v.ValidateStruct(payload)
	}
	
	return nil
}

// registerCustomValidations 注册自定义验证规则
func registerCustomValidations(validate *validator.Validate) {
	// 注册OCPP特定的验证规则
	validate.RegisterValidation("ocpp_datetime", validateOCPPDateTime)
	validate.RegisterValidation("ocpp_id_token", validateOCPPIdToken)
	validate.RegisterValidation("ocpp_connector_id", validateOCPPConnectorId)
	validate.RegisterValidation("ocpp_meter_value", validateOCPPMeterValue)
	validate.RegisterValidation("ocpp_status", validateOCPPStatus)
}

// validateOCPPDateTime 验证OCPP日期时间格式
func validateOCPPDateTime(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return true // 允许空值，required标签会处理必填验证
	}
	
	// OCPP使用RFC3339格式
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

// validateOCPPIdToken 验证OCPP ID令牌
func validateOCPPIdToken(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return true
	}
	
	// ID令牌长度限制
	if len(value) > 20 {
		return false
	}
	
	// 只允许字母数字字符
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9]+$`, value)
	return matched
}

// validateOCPPConnectorId 验证连接器ID
func validateOCPPConnectorId(fl validator.FieldLevel) bool {
	value := fl.Field().Int()
	// 连接器ID必须大于等于0
	return value >= 0
}

// validateOCPPMeterValue 验证电表值
func validateOCPPMeterValue(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return false
	}
	
	// 尝试解析为数字
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

// validateOCPPStatus 验证OCPP状态值
func validateOCPPStatus(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return true
	}
	
	// 定义有效的状态值
	validStatuses := map[string]bool{
		"Available":     true,
		"Preparing":     true,
		"Charging":      true,
		"SuspendedEVSE": true,
		"SuspendedEV":   true,
		"Finishing":     true,
		"Reserved":      true,
		"Unavailable":   true,
		"Faulted":       true,
	}
	
	return validStatuses[value]
}

// isValidAction 检查是否为有效的OCPP动作
func isValidAction(action string) bool {
	validActions := map[string]bool{
		// Core Profile
		"Authorize":              true,
		"BootNotification":       true,
		"ChangeAvailability":     true,
		"ChangeConfiguration":    true,
		"ClearCache":             true,
		"DataTransfer":           true,
		"GetConfiguration":       true,
		"Heartbeat":              true,
		"MeterValues":            true,
		"RemoteStartTransaction": true,
		"RemoteStopTransaction":  true,
		"Reset":                  true,
		"StartTransaction":       true,
		"StatusNotification":     true,
		"StopTransaction":        true,
		"UnlockConnector":        true,
		
		// Firmware Management Profile
		"GetDiagnostics":                   true,
		"DiagnosticsStatusNotification":    true,
		"FirmwareStatusNotification":       true,
		"UpdateFirmware":                   true,
		
		// Local Auth List Management Profile
		"GetLocalListVersion": true,
		"SendLocalList":       true,
		
		// Reservation Profile
		"CancelReservation": true,
		"ReserveNow":        true,
		
		// Smart Charging Profile
		"ClearChargingProfile": true,
		"GetCompositeSchedule": true,
		"SetChargingProfile":   true,
		
		// Trigger Message Profile
		"TriggerMessage": true,
	}
	
	return validActions[action]
}

// getErrorMessage 获取友好的错误消息
func getErrorMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("Field '%s' is required", fe.Field())
	case "min":
		return fmt.Sprintf("Field '%s' must be at least %s", fe.Field(), fe.Param())
	case "max":
		return fmt.Sprintf("Field '%s' must not exceed %s", fe.Field(), fe.Param())
	case "email":
		return fmt.Sprintf("Field '%s' must be a valid email", fe.Field())
	case "url":
		return fmt.Sprintf("Field '%s' must be a valid URL", fe.Field())
	case "ocpp_datetime":
		return fmt.Sprintf("Field '%s' must be a valid RFC3339 datetime", fe.Field())
	case "ocpp_id_token":
		return fmt.Sprintf("Field '%s' must be a valid ID token (max 20 alphanumeric characters)", fe.Field())
	case "ocpp_connector_id":
		return fmt.Sprintf("Field '%s' must be a valid connector ID (>= 0)", fe.Field())
	case "ocpp_meter_value":
		return fmt.Sprintf("Field '%s' must be a valid numeric meter value", fe.Field())
	case "ocpp_status":
		return fmt.Sprintf("Field '%s' must be a valid OCPP status", fe.Field())
	default:
		return fmt.Sprintf("Field '%s' failed validation for tag '%s'", fe.Field(), fe.Tag())
	}
}

// ValidateMessageSize 验证消息大小
func (v *Validator) ValidateMessageSize(data []byte, maxSize int) error {
	if len(data) > maxSize {
		return ValidationError{
			Field:   "message",
			Tag:     "max_size",
			Value:   fmt.Sprintf("%d bytes", len(data)),
			Message: fmt.Sprintf("Message size %d bytes exceeds maximum allowed size %d bytes", len(data), maxSize),
		}
	}
	return nil
}

// ValidateChargePointID 验证充电桩ID
func (v *Validator) ValidateChargePointID(chargePointID string) error {
	if chargePointID == "" {
		return ValidationError{
			Field:   "chargePointId",
			Tag:     "required",
			Value:   "",
			Message: "Charge point ID is required",
		}
	}
	
	if len(chargePointID) > 20 {
		return ValidationError{
			Field:   "chargePointId",
			Tag:     "max",
			Value:   chargePointID,
			Message: "Charge point ID must not exceed 20 characters",
		}
	}
	
	// 只允许字母数字字符和连字符
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9\-]+$`, chargePointID)
	if !matched {
		return ValidationError{
			Field:   "chargePointId",
			Tag:     "format",
			Value:   chargePointID,
			Message: "Charge point ID can only contain alphanumeric characters and hyphens",
		}
	}
	
	return nil
}

// ValidateProtocolVersion 验证协议版本
func (v *Validator) ValidateProtocolVersion(version string) error {
	validVersions := map[string]bool{
		"ocpp1.6": true,
		"ocpp2.0": true,
		"ocpp2.0.1": true,
	}
	
	if !validVersions[version] {
		return ValidationError{
			Field:   "protocolVersion",
			Tag:     "invalid",
			Value:   version,
			Message: "Unsupported protocol version",
		}
	}
	
	return nil
}
