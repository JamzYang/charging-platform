package ocpp16

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/domain/ocpp16"
	"github.com/charging-platform/charge-point-gateway/internal/domain/serialization"
	"github.com/charging-platform/charge-point-gateway/internal/domain/validation"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
	"github.com/charging-platform/charge-point-gateway/internal/storage"
)

// Processor OCPP 1.6消息处理器
type Processor struct {
	// 核心组件
	serializer *serialization.Serializer
	validator  *validation.Validator

	// 事件系统
	eventFactory *events.EventFactory
	eventChan    chan events.Event

	// 消息跟踪
	pendingRequests map[string]*PendingRequest
	requestMutex    sync.RWMutex

	// 配置
	config *ProcessorConfig

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 日志器
	logger *logger.Logger

	// 故障转移相关
	podID   string
	storage storage.ConnectionStorage // 引入 storage 接口
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	// 消息处理配置
	MaxMessageSize     int           `json:"max_message_size"`
	RequestTimeout     time.Duration `json:"request_timeout"`
	MaxPendingRequests int           `json:"max_pending_requests"`

	// 验证配置
	EnableValidation    bool `json:"enable_validation"`
	StrictValidation    bool `json:"strict_validation"`
	ValidateMessageSize bool `json:"validate_message_size"`

	// 事件配置
	EventChannelSize int  `json:"event_channel_size"`
	EnableEvents     bool `json:"enable_events"`

	// 性能配置
	WorkerCount     int           `json:"worker_count"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	EnableMetrics   bool          `json:"enable_metrics"`
}

// DefaultProcessorConfig 默认处理器配置
func DefaultProcessorConfig() *ProcessorConfig {
	return &ProcessorConfig{
		MaxMessageSize:     1024 * 1024, // 1MB
		RequestTimeout:     30 * time.Second,
		MaxPendingRequests: 1000,

		EnableValidation:    true,
		StrictValidation:    false,
		ValidateMessageSize: true,

		EventChannelSize: 1000,
		EnableEvents:     true,

		WorkerCount:     100, // 从4增加到100，支持高并发消息处理
		CleanupInterval: 1 * time.Minute,
		EnableMetrics:   true,
	}
}

// PendingRequest 待处理请求
type PendingRequest struct {
	MessageID     string                  `json:"message_id"`
	ChargePointID string                  `json:"charge_point_id"`
	Action        string                  `json:"action"`
	Payload       interface{}             `json:"payload"`
	ResponseChan  chan *ProcessorResponse `json:"-"`
	CreatedAt     time.Time               `json:"created_at"`
	Timeout       time.Duration           `json:"timeout"`
}

// ProcessorResponse 处理器响应
type ProcessorResponse struct {
	MessageID   string      `json:"message_id"`
	Success     bool        `json:"success"`
	Payload     interface{} `json:"payload,omitempty"`
	Error       string      `json:"error,omitempty"`
	ProcessedAt time.Time   `json:"processed_at"`
}

// ProcessorRequest 处理器请求
type ProcessorRequest struct {
	ChargePointID string    `json:"charge_point_id"`
	MessageData   []byte    `json:"message_data"`
	ReceivedAt    time.Time `json:"received_at"`
}

// NewProcessor 创建新的OCPP消息处理器
func NewProcessor(config *ProcessorConfig, podID string, storage storage.ConnectionStorage, log *logger.Logger) *Processor {
	if config == nil {
		config = DefaultProcessorConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 使用传入的日志器，如果为空则创建默认的
	if log == nil {
		log, _ = logger.New(logger.DefaultConfig())
	}

	return &Processor{
		serializer:      serialization.NewSerializer(serialization.FormatJSON),
		validator:       validation.NewValidator(),
		eventFactory:    events.NewEventFactory(),
		eventChan:       make(chan events.Event, config.EventChannelSize),
		pendingRequests: make(map[string]*PendingRequest),
		config:          config,
		ctx:             ctx,
		cancel:          cancel,
		logger:          log,
		podID:           podID,
		storage:         storage,
	}
}

// Start 启动消息处理器
func (p *Processor) Start() error {
	p.logger.Info("Starting OCPP message processor")

	// 启动清理协程
	p.wg.Add(1)
	go p.cleanupRoutine()

	// 启动工作协程
	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.workerRoutine(i)
	}

	p.logger.Infof("OCPP message processor started with %d workers", p.config.WorkerCount)
	return nil
}

// Stop 停止消息处理器
func (p *Processor) Stop() error {
	p.logger.Info("Stopping OCPP message processor")

	// 取消上下文
	p.cancel()

	// 清理待处理请求
	p.requestMutex.Lock()
	for messageID, req := range p.pendingRequests {
		close(req.ResponseChan)
		delete(p.pendingRequests, messageID)
	}
	p.requestMutex.Unlock()

	// 等待所有协程结束
	p.wg.Wait()

	// 关闭事件通道
	close(p.eventChan)

	p.logger.Info("OCPP message processor stopped")
	return nil
}

// ProcessMessage 处理OCPP消息
func (p *Processor) ProcessMessage(chargePointID string, messageData []byte) (*ProcessorResponse, error) {
	startTime := time.Now()

	// 验证消息大小
	if p.config.ValidateMessageSize {
		if err := p.validator.ValidateMessageSize(messageData, p.config.MaxMessageSize); err != nil {
			return nil, fmt.Errorf("message size validation failed: %w", err)
		}
	}

	// 验证JSON格式
	if err := p.validator.ValidateJSON(messageData); err != nil {
		return nil, fmt.Errorf("JSON validation failed: %w", err)
	}

	// 反序列化消息
	messageType, messageID, action, payload, err := p.serializer.DeserializeMessage(messageData)
	if err != nil {
		return nil, fmt.Errorf("message deserialization failed: %w", err)
	}

	// 验证OCPP消息格式（不包括payload，payload在具体处理时验证）
	if p.config.EnableValidation {
		if err := p.validator.ValidateOCPPMessage(messageType, messageID, action, nil); err != nil {
			return nil, fmt.Errorf("OCPP message validation failed: %w", err)
		}
	}

	// 根据消息类型处理
	switch messageType {
	case 2: // Call
		return p.processCallMessage(chargePointID, messageID, action, payload, startTime)
	case 3: // CallResult
		return p.processCallResultMessage(chargePointID, messageID, payload, startTime)
	case 4: // CallError
		return p.processCallErrorMessage(chargePointID, messageID, payload, startTime)
	default:
		return nil, fmt.Errorf("unsupported message type: %d", messageType)
	}
}

// processCallMessage 处理Call消息
func (p *Processor) processCallMessage(chargePointID, messageID, action string, payload json.RawMessage, startTime time.Time) (*ProcessorResponse, error) {
	p.logger.Debugf("Processing Call message: %s from %s", action, chargePointID)

	// 创建payload实例
	payloadInstance := p.serializer.CreatePayloadInstance(action, true)
	if payloadInstance == nil {
		return nil, fmt.Errorf("unsupported action: %s", action)
	}

	// 反序列化payload
	if err := p.serializer.DeserializePayload(payload, payloadInstance); err != nil {
		return nil, fmt.Errorf("payload deserialization failed: %w", err)
	}

	// 验证payload
	if p.config.EnableValidation {
		if err := p.validator.ValidateStruct(payloadInstance); err != nil {
			return nil, fmt.Errorf("payload validation failed: %w", err)
		}
	}

	// 处理具体的action
	responsePayload, err := p.handleAction(chargePointID, action, payloadInstance)
	if err != nil {
		return nil, fmt.Errorf("action handling failed: %w", err)
	}

	// 发送事件
	if p.config.EnableEvents {
		p.sendActionEvent(chargePointID, action, payloadInstance)
	}

	return &ProcessorResponse{
		MessageID:   messageID,
		Success:     true,
		Payload:     responsePayload,
		ProcessedAt: time.Now(),
	}, nil
}

// processCallResultMessage 处理CallResult消息
func (p *Processor) processCallResultMessage(chargePointID, messageID string, payload json.RawMessage, startTime time.Time) (*ProcessorResponse, error) {
	p.logger.Debugf("Processing CallResult message: %s from %s", messageID, chargePointID)

	// 查找待处理请求
	p.requestMutex.RLock()
	pendingReq, exists := p.pendingRequests[messageID]
	p.requestMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no pending request found for message ID: %s", messageID)
	}

	// 创建响应payload实例
	payloadInstance := p.serializer.CreatePayloadInstance(pendingReq.Action, false)
	if payloadInstance != nil {
		if err := p.serializer.DeserializePayload(payload, payloadInstance); err != nil {
			p.logger.Warnf("Failed to deserialize CallResult payload: %v", err)
			payloadInstance = payload
		}
	} else {
		payloadInstance = payload
	}

	// 发送响应到等待的协程
	response := &ProcessorResponse{
		MessageID:   messageID,
		Success:     true,
		Payload:     payloadInstance,
		ProcessedAt: time.Now(),
	}

	select {
	case pendingReq.ResponseChan <- response:
	default:
		p.logger.Warn("Response channel full, dropping response")
	}

	// 移除待处理请求
	p.requestMutex.Lock()
	delete(p.pendingRequests, messageID)
	p.requestMutex.Unlock()

	return response, nil
}

// processCallErrorMessage 处理CallError消息
func (p *Processor) processCallErrorMessage(chargePointID, messageID string, payload json.RawMessage, startTime time.Time) (*ProcessorResponse, error) {
	p.logger.Debugf("Processing CallError message: %s from %s", messageID, chargePointID)

	// 解析错误payload
	var errorPayload map[string]interface{}
	if err := json.Unmarshal(payload, &errorPayload); err != nil {
		return nil, fmt.Errorf("failed to parse error payload: %w", err)
	}

	// 查找待处理请求
	p.requestMutex.RLock()
	pendingReq, exists := p.pendingRequests[messageID]
	p.requestMutex.RUnlock()

	if exists {
		// 发送错误响应到等待的协程
		response := &ProcessorResponse{
			MessageID:   messageID,
			Success:     false,
			Error:       fmt.Sprintf("OCPP error: %v", errorPayload["errorDescription"]),
			ProcessedAt: time.Now(),
		}

		select {
		case pendingReq.ResponseChan <- response:
		default:
			p.logger.Warn("Response channel full, dropping error response")
		}

		// 移除待处理请求
		p.requestMutex.Lock()
		delete(p.pendingRequests, messageID)
		p.requestMutex.Unlock()
	}

	return &ProcessorResponse{
		MessageID:   messageID,
		Success:     false,
		Error:       fmt.Sprintf("OCPP error: %v", errorPayload["errorDescription"]),
		ProcessedAt: time.Now(),
	}, nil
}

// GetEventChannel 获取事件通道
func (p *Processor) GetEventChannel() <-chan events.Event {
	return p.eventChan
}

// GetPendingRequestCount 获取待处理请求数量
func (p *Processor) GetPendingRequestCount() int {
	p.requestMutex.RLock()
	defer p.requestMutex.RUnlock()
	return len(p.pendingRequests)
}

// handleAction 处理具体的OCPP动作
func (p *Processor) handleAction(chargePointID, action string, payload interface{}) (interface{}, error) {
	switch action {
	case "BootNotification":
		return p.handleBootNotification(chargePointID, payload.(*ocpp16.BootNotificationRequest))
	case "Heartbeat":
		return p.handleHeartbeat(chargePointID, payload.(*ocpp16.HeartbeatRequest))
	case "StatusNotification":
		return p.handleStatusNotification(chargePointID, payload.(*ocpp16.StatusNotificationRequest))
	case "Authorize":
		return p.handleAuthorize(chargePointID, payload.(*ocpp16.AuthorizeRequest))
	case "StartTransaction":
		return p.handleStartTransaction(chargePointID, payload.(*ocpp16.StartTransactionRequest))
	case "StopTransaction":
		return p.handleStopTransaction(chargePointID, payload.(*ocpp16.StopTransactionRequest))
	case "MeterValues":
		return p.handleMeterValues(chargePointID, payload.(*ocpp16.MeterValuesRequest))
	case "DataTransfer":
		return p.handleDataTransfer(chargePointID, payload.(*ocpp16.DataTransferRequest))
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// handleBootNotification 处理BootNotification
func (p *Processor) handleBootNotification(chargePointID string, req *ocpp16.BootNotificationRequest) (*ocpp16.BootNotificationResponse, error) {
	p.logger.Infof("BootNotification from %s: %s %s", chargePointID, req.ChargePointVendor, req.ChargePointModel)

	// 【关键步骤】: 更新 Redis 连接映射
	err := p.storage.SetConnection(context.Background(), chargePointID, p.podID, 5*time.Minute)
	if err != nil {
		// 记录严重错误，但这不应中断 BootNotification 的正常响应
		p.logger.Errorf("Failed to set connection mapping in Redis for charge point %s: %v", chargePointID, err)
	}

	// 创建响应
	response := &ocpp16.BootNotificationResponse{
		Status:      ocpp16.RegistrationStatusAccepted,
		CurrentTime: ocpp16.DateTime{Time: time.Now().UTC()},
		Interval:    300, // 5分钟心跳间隔
	}

	return response, nil
}

// handleHeartbeat 处理Heartbeat
func (p *Processor) handleHeartbeat(chargePointID string, req *ocpp16.HeartbeatRequest) (*ocpp16.HeartbeatResponse, error) {
	p.logger.Debugf("Heartbeat from %s", chargePointID)

	response := &ocpp16.HeartbeatResponse{
		CurrentTime: ocpp16.DateTime{Time: time.Now().UTC()},
	}

	return response, nil
}

// handleStatusNotification 处理StatusNotification
func (p *Processor) handleStatusNotification(chargePointID string, req *ocpp16.StatusNotificationRequest) (*ocpp16.StatusNotificationResponse, error) {
	p.logger.Infof("StatusNotification from %s: connector %d status %s",
		chargePointID, req.ConnectorId, req.Status)

	response := &ocpp16.StatusNotificationResponse{}
	return response, nil
}

// handleAuthorize 处理Authorize
func (p *Processor) handleAuthorize(chargePointID string, req *ocpp16.AuthorizeRequest) (*ocpp16.AuthorizeResponse, error) {
	p.logger.Infof("Authorize from %s: idTag %s", chargePointID, req.IdTag)

	// 简单的授权逻辑 - 在实际应用中应该查询授权数据库
	response := &ocpp16.AuthorizeResponse{
		IdTagInfo: ocpp16.IdTagInfo{
			Status: ocpp16.AuthorizationStatusAccepted,
		},
	}

	return response, nil
}

// handleStartTransaction 处理StartTransaction
func (p *Processor) handleStartTransaction(chargePointID string, req *ocpp16.StartTransactionRequest) (*ocpp16.StartTransactionResponse, error) {
	p.logger.Infof("StartTransaction from %s: connector %d, idTag %s",
		chargePointID, req.ConnectorId, req.IdTag)

	// 生成交易ID - 在实际应用中应该从数据库获取
	transactionID := int(time.Now().Unix())

	response := &ocpp16.StartTransactionResponse{
		IdTagInfo: ocpp16.IdTagInfo{
			Status: ocpp16.AuthorizationStatusAccepted,
		},
		TransactionId: transactionID,
	}

	return response, nil
}

// handleStopTransaction 处理StopTransaction
func (p *Processor) handleStopTransaction(chargePointID string, req *ocpp16.StopTransactionRequest) (*ocpp16.StopTransactionResponse, error) {
	p.logger.Infof("StopTransaction from %s: transaction %d",
		chargePointID, req.TransactionId)

	response := &ocpp16.StopTransactionResponse{}
	return response, nil
}

// handleMeterValues 处理MeterValues
func (p *Processor) handleMeterValues(chargePointID string, req *ocpp16.MeterValuesRequest) (*ocpp16.MeterValuesResponse, error) {
	p.logger.Debugf("MeterValues from %s: connector %d, %d values",
		chargePointID, req.ConnectorId, len(req.MeterValue))

	response := &ocpp16.MeterValuesResponse{}
	return response, nil
}

// handleDataTransfer 处理DataTransfer
func (p *Processor) handleDataTransfer(chargePointID string, req *ocpp16.DataTransferRequest) (*ocpp16.DataTransferResponse, error) {
	p.logger.Infof("DataTransfer from %s: vendor %s", chargePointID, req.VendorId)

	response := &ocpp16.DataTransferResponse{
		Status: ocpp16.DataTransferStatusAccepted,
	}

	return response, nil
}

// sendActionEvent 发送动作事件
func (p *Processor) sendActionEvent(chargePointID, action string, payload interface{}) {
	metadata := events.Metadata{
		Source:          "ocpp16-processor",
		ProtocolVersion: "1.6",
	}

	var event events.Event

	switch action {
	case "BootNotification":
		if req, ok := payload.(*ocpp16.BootNotificationRequest); ok {
			chargePointInfo := events.ChargePointInfo{
				ID:              chargePointID,
				Vendor:          req.ChargePointVendor,
				Model:           req.ChargePointModel,
				SerialNumber:    req.ChargePointSerialNumber,
				FirmwareVersion: req.FirmwareVersion,
				LastSeen:        time.Now().UTC(),
				ProtocolVersion: "1.6",
			}
			event = p.eventFactory.CreateChargePointConnectedEvent(chargePointID, chargePointInfo, metadata)
		}
	case "StatusNotification":
		if req, ok := payload.(*ocpp16.StatusNotificationRequest); ok {
			connectorInfo := events.ConnectorInfo{
				ID:            req.ConnectorId,
				ChargePointID: chargePointID,
				Status:        convertOCPPStatusToEventStatus(req.Status),
				ErrorCode:     stringPtr(string(req.ErrorCode)),
			}
			// 这里需要获取之前的状态，简化处理
			event = p.eventFactory.CreateConnectorStatusChangedEvent(chargePointID, connectorInfo, events.ConnectorStatusAvailable, metadata)
		}
	}

	if event != nil {
		select {
		case p.eventChan <- event:
		default:
			p.logger.Warn("Event channel full, dropping event")
		}
	}
}

// cleanupRoutine 清理协程
func (p *Processor) cleanupRoutine() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.cleanupExpiredRequests()
		}
	}
}

// cleanupExpiredRequests 清理过期请求
func (p *Processor) cleanupExpiredRequests() {
	now := time.Now()
	var expiredIDs []string

	p.requestMutex.RLock()
	for messageID, req := range p.pendingRequests {
		if now.Sub(req.CreatedAt) > req.Timeout {
			expiredIDs = append(expiredIDs, messageID)
		}
	}
	p.requestMutex.RUnlock()

	if len(expiredIDs) > 0 {
		p.requestMutex.Lock()
		for _, messageID := range expiredIDs {
			if req, exists := p.pendingRequests[messageID]; exists {
				// 发送超时响应
				select {
				case req.ResponseChan <- &ProcessorResponse{
					MessageID:   messageID,
					Success:     false,
					Error:       "request timeout",
					ProcessedAt: now,
				}:
				default:
				}
				delete(p.pendingRequests, messageID)
			}
		}
		p.requestMutex.Unlock()

		p.logger.Warnf("Cleaned up %d expired requests", len(expiredIDs))
	}
}

// workerRoutine 工作协程
func (p *Processor) workerRoutine(workerID int) {
	defer p.wg.Done()

	p.logger.Debugf("Worker %d started", workerID)

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Debugf("Worker %d stopped", workerID)
			return
		default:
			// 工作协程可以在这里处理异步任务
			// 目前主要用于保持协程活跃，实际处理在ProcessMessage中同步进行
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// stringPtr 辅助函数
func stringPtr(s string) *string {
	return &s
}

// convertOCPPStatusToEventStatus 转换OCPP状态到事件状态
func convertOCPPStatusToEventStatus(ocppStatus ocpp16.ChargePointStatus) events.ConnectorStatus {
	switch ocppStatus {
	case ocpp16.ChargePointStatusAvailable:
		return events.ConnectorStatusAvailable
	case ocpp16.ChargePointStatusPreparing:
		return events.ConnectorStatusPreparing
	case ocpp16.ChargePointStatusCharging:
		return events.ConnectorStatusCharging
	case ocpp16.ChargePointStatusSuspendedEVSE:
		return events.ConnectorStatusSuspendedEVSE
	case ocpp16.ChargePointStatusSuspendedEV:
		return events.ConnectorStatusSuspendedEV
	case ocpp16.ChargePointStatusFinishing:
		return events.ConnectorStatusFinishing
	case ocpp16.ChargePointStatusReserved:
		return events.ConnectorStatusReserved
	case ocpp16.ChargePointStatusUnavailable:
		return events.ConnectorStatusUnavailable
	case ocpp16.ChargePointStatusFaulted:
		return events.ConnectorStatusFaulted
	default:
		return events.ConnectorStatusUnavailable
	}
}
