package ocpp16

import (
	"context"
	"fmt"
	"sync"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/gateway"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
)

// ProtocolHandler OCPP 1.6协议处理器适配器
// 实现gateway.ProtocolHandler接口，将现有的Processor适配为标准的协议处理器
type ProtocolHandler struct {
	// 核心组件
	processor *Processor
	converter gateway.ModelConverter
	
	// 配置
	config *ProtocolHandlerConfig
	
	// 事件系统
	eventChan chan events.Event
	
	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// 状态管理
	started    bool
	startMutex sync.Mutex
	
	// 日志器
	logger *logger.Logger
}

// ProtocolHandlerConfig 协议处理器配置
type ProtocolHandlerConfig struct {
	// 事件配置
	EventChannelSize int `json:"event_channel_size"`
	EnableEvents     bool `json:"enable_events"`
	
	// 转换配置
	EnableConversion bool `json:"enable_conversion"`
	
	// 性能配置
	EventBufferSize int `json:"event_buffer_size"`
	
	// 日志配置
	LogLevel string `json:"log_level"`
}

// DefaultProtocolHandlerConfig 默认协议处理器配置
func DefaultProtocolHandlerConfig() *ProtocolHandlerConfig {
	return &ProtocolHandlerConfig{
		EventChannelSize: 1000,
		EnableEvents:     true,
		EnableConversion: true,
		EventBufferSize:  100,
		LogLevel:         "info",
	}
}

// NewProtocolHandler 创建新的OCPP 1.6协议处理器
func NewProtocolHandler(processor *Processor, converter gateway.ModelConverter, config *ProtocolHandlerConfig) *ProtocolHandler {
	if config == nil {
		config = DefaultProtocolHandlerConfig()
	}
	
	if processor == nil {
		panic("processor cannot be nil")
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// 创建日志器
	l, _ := logger.New(logger.DefaultConfig())
	
	return &ProtocolHandler{
		processor: processor,
		converter: converter,
		config:    config,
		eventChan: make(chan events.Event, config.EventChannelSize),
		ctx:       ctx,
		cancel:    cancel,
		logger:    l,
	}
}

// ProcessMessage 处理协议消息
func (h *ProtocolHandler) ProcessMessage(ctx context.Context, chargePointID string, message []byte) (interface{}, error) {
	h.logger.Debugf("Processing OCPP 1.6 message from %s", chargePointID)
	
	// 使用现有的处理器处理消息
	response, err := h.processor.ProcessMessage(chargePointID, message)
	if err != nil {
		h.logger.Errorf("Failed to process OCPP message from %s: %v", chargePointID, err)
		return nil, fmt.Errorf("OCPP message processing failed: %w", err)
	}
	
	h.logger.Debugf("Successfully processed OCPP 1.6 message from %s", chargePointID)
	
	return response, nil
}

// GetSupportedActions 获取支持的动作列表
func (h *ProtocolHandler) GetSupportedActions() []string {
	// OCPP 1.6 Core Profile支持的动作
	return []string{
		// Core Profile - Charge Point Initiated
		"BootNotification",
		"Heartbeat",
		"StatusNotification",
		"MeterValues",
		"StartTransaction",
		"StopTransaction",
		"Authorize",
		"DataTransfer",
		
		// Core Profile - Central System Initiated
		"ChangeAvailability",
		"ChangeConfiguration",
		"ClearCache",
		"GetConfiguration",
		"RemoteStartTransaction",
		"RemoteStopTransaction",
		"Reset",
		"UnlockConnector",
		
		// Firmware Management Profile
		"GetDiagnostics",
		"UpdateFirmware",
		"FirmwareStatusNotification",
		"DiagnosticsStatusNotification",
		
		// Local Auth List Management Profile
		"GetLocalListVersion",
		"SendLocalList",
		
		// Reservation Profile
		"ReserveNow",
		"CancelReservation",
		
		// Smart Charging Profile
		"SetChargingProfile",
		"ClearChargingProfile",
		"GetCompositeSchedule",
		"TriggerMessage",
	}
}

// GetVersion 获取协议版本
func (h *ProtocolHandler) GetVersion() string {
	return "1.6"
}

// Start 启动处理器
func (h *ProtocolHandler) Start() error {
	h.startMutex.Lock()
	defer h.startMutex.Unlock()
	
	if h.started {
		return fmt.Errorf("protocol handler already started")
	}
	
	h.logger.Info("Starting OCPP 1.6 protocol handler")
	
	// 启动底层处理器
	if err := h.processor.Start(); err != nil {
		return fmt.Errorf("failed to start OCPP processor: %w", err)
	}
	
	// 启动事件转发协程
	if h.config.EnableEvents {
		h.wg.Add(1)
		go h.eventForwardingRoutine()
	}
	
	h.started = true
	
	h.logger.Info("OCPP 1.6 protocol handler started successfully")
	
	return nil
}

// Stop 停止处理器
func (h *ProtocolHandler) Stop() error {
	h.startMutex.Lock()
	defer h.startMutex.Unlock()
	
	if !h.started {
		return nil
	}
	
	h.logger.Info("Stopping OCPP 1.6 protocol handler")
	
	// 取消上下文
	h.cancel()
	
	// 停止底层处理器
	if err := h.processor.Stop(); err != nil {
		h.logger.Errorf("Error stopping OCPP processor: %v", err)
	}
	
	// 等待所有协程结束
	h.wg.Wait()
	
	// 关闭事件通道
	close(h.eventChan)
	
	h.started = false
	
	h.logger.Info("OCPP 1.6 protocol handler stopped")
	
	return nil
}

// GetEventChannel 获取事件通道
func (h *ProtocolHandler) GetEventChannel() <-chan events.Event {
	return h.eventChan
}

// eventForwardingRoutine 事件转发协程
func (h *ProtocolHandler) eventForwardingRoutine() {
	defer h.wg.Done()
	
	// 获取处理器的事件通道
	processorEventChan := h.processor.GetEventChannel()
	
	h.logger.Debug("Event forwarding routine started")
	
	for {
		select {
		case <-h.ctx.Done():
			h.logger.Debug("Event forwarding routine stopping")
			return
			
		case event, ok := <-processorEventChan:
			if !ok {
				h.logger.Debug("Processor event channel closed")
				return
			}
			
			h.forwardEvent(event)
		}
	}
}

// forwardEvent 转发事件
func (h *ProtocolHandler) forwardEvent(event events.Event) {
	// 如果启用了转换器，尝试转换事件
	if h.config.EnableConversion && h.converter != nil {
		convertedEvent, err := h.convertEvent(event)
		if err != nil {
			h.logger.Warnf("Failed to convert event %s: %v", event.GetType(), err)
			// 转换失败时仍然转发原始事件
		} else {
			event = convertedEvent
		}
	}
	
	// 转发事件到统一通道
	select {
	case h.eventChan <- event:
		h.logger.Debugf("Forwarded event %s from charge point %s", event.GetType(), event.GetChargePointID())
	case <-h.ctx.Done():
		return
	default:
		h.logger.Warn("Event channel full, dropping event")
	}
}

// convertEvent 转换事件（如果需要）
func (h *ProtocolHandler) convertEvent(event events.Event) (events.Event, error) {
	// 这里可以使用转换器进行事件转换
	// 目前简化实现，直接返回原始事件
	// 在实际实现中，可以根据事件类型调用相应的转换方法
	
	switch event.GetType() {
	case events.EventTypeChargePointConnected:
		// 可以调用converter.ConvertBootNotification等方法
		return event, nil
	case events.EventTypeConnectorStatusChanged:
		// 可以调用converter.ConvertStatusNotification等方法
		return event, nil
	case events.EventTypeMeterValuesReceived:
		// 可以调用converter.ConvertMeterValues等方法
		return event, nil
	default:
		// 对于不需要转换的事件，直接返回
		return event, nil
	}
}

// GetStats 获取处理器统计信息
func (h *ProtocolHandler) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"version":           h.GetVersion(),
		"started":           h.started,
		"supported_actions": len(h.GetSupportedActions()),
		"event_channel_size": cap(h.eventChan),
		"event_channel_len":  len(h.eventChan),
	}
	
	// 添加底层处理器的统计信息
	if h.processor != nil {
		stats["pending_requests"] = h.processor.GetPendingRequestCount()
	}
	
	return stats
}

// IsHealthy 检查处理器健康状态
func (h *ProtocolHandler) IsHealthy() bool {
	if !h.started {
		return false
	}
	
	// 检查事件通道是否阻塞
	if len(h.eventChan) >= cap(h.eventChan)*9/10 { // 90%满认为不健康
		return false
	}
	
	return true
}
