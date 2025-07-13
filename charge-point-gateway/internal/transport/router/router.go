package router

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
	"github.com/charging-platform/charge-point-gateway/internal/protocol/ocpp16"
	"github.com/charging-platform/charge-point-gateway/internal/transport/websocket"
)

// Router 消息路由器
type Router struct {
	// 核心组件
	wsManager *websocket.Manager
	processor *ocpp16.Processor
	
	// 路由配置
	config *RouterConfig
	
	// 事件系统
	eventChan chan events.Event
	
	// 消息统计
	stats *RouterStats
	mutex sync.RWMutex
	
	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// 日志器
	logger *logger.Logger
}

// RouterConfig 路由器配置
type RouterConfig struct {
	// 消息处理配置
	MaxConcurrentMessages int           `json:"max_concurrent_messages"`
	MessageTimeout         time.Duration `json:"message_timeout"`
	RetryAttempts          int           `json:"retry_attempts"`
	RetryDelay             time.Duration `json:"retry_delay"`
	
	// 事件配置
	EventChannelSize int  `json:"event_channel_size"`
	EnableEvents     bool `json:"enable_events"`
	
	// 性能配置
	WorkerCount       int           `json:"worker_count"`
	BufferSize        int           `json:"buffer_size"`
	StatsInterval     time.Duration `json:"stats_interval"`
	EnableMetrics     bool          `json:"enable_metrics"`
	
	// 错误处理配置
	EnableErrorRecovery bool          `json:"enable_error_recovery"`
	ErrorThreshold      int           `json:"error_threshold"`
	CircuitBreakerDelay time.Duration `json:"circuit_breaker_delay"`
}

// DefaultRouterConfig 默认路由器配置
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		MaxConcurrentMessages: 1000,
		MessageTimeout:         30 * time.Second,
		RetryAttempts:          3,
		RetryDelay:             1 * time.Second,
		
		EventChannelSize: 1000,
		EnableEvents:     true,
		
		WorkerCount:   8,
		BufferSize:    1000,
		StatsInterval: 1 * time.Minute,
		EnableMetrics: true,
		
		EnableErrorRecovery: true,
		ErrorThreshold:      10,
		CircuitBreakerDelay: 5 * time.Minute,
	}
}

// RouterStats 路由器统计信息
type RouterStats struct {
	MessagesReceived    int64     `json:"messages_received"`
	MessagesProcessed   int64     `json:"messages_processed"`
	MessagesFailed      int64     `json:"messages_failed"`
	EventsGenerated     int64     `json:"events_generated"`
	AverageProcessTime  float64   `json:"average_process_time_ms"`
	LastResetTime       time.Time `json:"last_reset_time"`
	ActiveConnections   int       `json:"active_connections"`
	PendingMessages     int       `json:"pending_messages"`
}

// MessageContext 消息上下文
type MessageContext struct {
	ChargePointID string    `json:"charge_point_id"`
	MessageData   []byte    `json:"message_data"`
	ReceivedAt    time.Time `json:"received_at"`
	Attempts      int       `json:"attempts"`
	LastError     error     `json:"last_error,omitempty"`
}

// NewRouter 创建新的消息路由器
func NewRouter(wsManager *websocket.Manager, processor *ocpp16.Processor, config *RouterConfig) *Router {
	if config == nil {
		config = DefaultRouterConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// 创建日志器
	l, _ := logger.New(logger.DefaultConfig())
	
	return &Router{
		wsManager: wsManager,
		processor: processor,
		config:    config,
		eventChan: make(chan events.Event, config.EventChannelSize),
		stats: &RouterStats{
			LastResetTime: time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
		logger: l,
	}
}

// Start 启动消息路由器
func (r *Router) Start() error {
	r.logger.Info("Starting message router")
	
	// 启动WebSocket管理器
	if err := r.wsManager.Start(); err != nil {
		return fmt.Errorf("failed to start WebSocket manager: %w", err)
	}
	
	// 启动OCPP处理器
	if err := r.processor.Start(); err != nil {
		return fmt.Errorf("failed to start OCPP processor: %w", err)
	}
	
	// 启动消息处理协程
	r.wg.Add(1)
	go r.messageRoutine()
	
	// 启动事件处理协程
	if r.config.EnableEvents {
		r.wg.Add(1)
		go r.eventRoutine()
	}
	
	// 启动统计协程
	if r.config.EnableMetrics {
		r.wg.Add(1)
		go r.statsRoutine()
	}
	
	// 启动工作协程
	for i := 0; i < r.config.WorkerCount; i++ {
		r.wg.Add(1)
		go r.workerRoutine(i)
	}
	
	r.logger.Infof("Message router started with %d workers", r.config.WorkerCount)
	return nil
}

// Stop 停止消息路由器
func (r *Router) Stop() error {
	r.logger.Info("Stopping message router")
	
	// 取消上下文
	r.cancel()
	
	// 停止组件
	if err := r.processor.Stop(); err != nil {
		r.logger.Errorf("Error stopping OCPP processor: %v", err)
	}
	
	if err := r.wsManager.Stop(); err != nil {
		r.logger.Errorf("Error stopping WebSocket manager: %v", err)
	}
	
	// 等待所有协程结束
	r.wg.Wait()
	
	// 关闭事件通道
	close(r.eventChan)
	
	r.logger.Info("Message router stopped")
	return nil
}

// messageRoutine 消息处理协程
func (r *Router) messageRoutine() {
	defer r.wg.Done()
	
	// 获取WebSocket事件通道
	wsEventChan := r.wsManager.GetEventChannel()
	
	for {
		select {
		case <-r.ctx.Done():
			return
		case wsEvent := <-wsEventChan:
			r.handleWebSocketEvent(wsEvent)
		}
	}
}

// handleWebSocketEvent 处理WebSocket事件
func (r *Router) handleWebSocketEvent(wsEvent websocket.ConnectionEvent) {
	switch wsEvent.Type {
	case websocket.EventTypeConnected:
		r.logger.Infof("Charge point connected: %s", wsEvent.ChargePointID)
		r.updateConnectionCount()
		
	case websocket.EventTypeDisconnected:
		r.logger.Infof("Charge point disconnected: %s", wsEvent.ChargePointID)
		r.updateConnectionCount()
		
	case websocket.EventTypeMessage:
		r.handleMessage(wsEvent.ChargePointID, wsEvent.Connection)
		
	case websocket.EventTypeError:
		r.logger.Errorf("WebSocket error for %s: %v", wsEvent.ChargePointID, wsEvent.Error)
		r.incrementFailedMessages()
		
	default:
		r.logger.Debugf("Unhandled WebSocket event type: %s", wsEvent.Type)
	}
}

// handleMessage 处理消息
func (r *Router) handleMessage(chargePointID string, conn *websocket.ConnectionWrapper) {
	// 这里需要从连接中读取消息数据
	// 由于WebSocket连接包装器的设计，我们需要一个消息队列或者回调机制
	// 为了简化，我们假设消息数据通过某种方式传递过来
	
	r.logger.Debugf("Processing message from %s", chargePointID)
	r.incrementReceivedMessages()
	
	// 创建消息上下文
	msgCtx := &MessageContext{
		ChargePointID: chargePointID,
		ReceivedAt:    time.Now(),
		Attempts:      0,
	}
	
	// 异步处理消息
	go r.processMessageAsync(msgCtx)
}

// processMessageAsync 异步处理消息
func (r *Router) processMessageAsync(msgCtx *MessageContext) {
	startTime := time.Now()
	
	// 处理消息
	response, err := r.processor.ProcessMessage(msgCtx.ChargePointID, msgCtx.MessageData)
	
	processingTime := time.Since(startTime)
	r.updateProcessingTime(processingTime)
	
	if err != nil {
		r.logger.Errorf("Failed to process message from %s: %v", msgCtx.ChargePointID, err)
		r.incrementFailedMessages()
		
		// 重试逻辑
		if r.config.EnableErrorRecovery && msgCtx.Attempts < r.config.RetryAttempts {
			msgCtx.Attempts++
			msgCtx.LastError = err
			
			// 延迟重试
			time.Sleep(r.config.RetryDelay)
			go r.processMessageAsync(msgCtx)
			return
		}
		
		// 发送错误响应
		r.sendErrorResponse(msgCtx.ChargePointID, err)
		return
	}
	
	r.incrementProcessedMessages()
	
	// 发送响应
	if response != nil {
		r.sendResponse(msgCtx.ChargePointID, response)
	}
}

// sendResponse 发送响应
func (r *Router) sendResponse(chargePointID string, response *ocpp16.ProcessorResponse) {
	// 序列化响应
	responseData, err := r.serializeResponse(response)
	if err != nil {
		r.logger.Errorf("Failed to serialize response for %s: %v", chargePointID, err)
		return
	}
	
	// 通过WebSocket发送
	if err := r.wsManager.SendMessage(chargePointID, responseData); err != nil {
		r.logger.Errorf("Failed to send response to %s: %v", chargePointID, err)
	}
}

// sendErrorResponse 发送错误响应
func (r *Router) sendErrorResponse(chargePointID string, err error) {
	// 创建错误响应
	errorResponse := map[string]interface{}{
		"error": err.Error(),
		"timestamp": time.Now().UTC(),
	}
	
	// 序列化错误响应
	responseData, serErr := r.serializeErrorResponse(errorResponse)
	if serErr != nil {
		r.logger.Errorf("Failed to serialize error response for %s: %v", chargePointID, serErr)
		return
	}
	
	// 通过WebSocket发送
	if err := r.wsManager.SendMessage(chargePointID, responseData); err != nil {
		r.logger.Errorf("Failed to send error response to %s: %v", chargePointID, err)
	}
}

// eventRoutine 事件处理协程
func (r *Router) eventRoutine() {
	defer r.wg.Done()
	
	// 获取处理器事件通道
	processorEventChan := r.processor.GetEventChannel()
	
	for {
		select {
		case <-r.ctx.Done():
			return
		case event := <-processorEventChan:
			r.forwardEvent(event)
		}
	}
}

// forwardEvent 转发事件
func (r *Router) forwardEvent(event events.Event) {
	select {
	case r.eventChan <- event:
		r.incrementGeneratedEvents()
	default:
		r.logger.Warn("Event channel full, dropping event")
	}
}

// statsRoutine 统计协程
func (r *Router) statsRoutine() {
	defer r.wg.Done()
	
	ticker := time.NewTicker(r.config.StatsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.logStats()
		}
	}
}

// workerRoutine 工作协程
func (r *Router) workerRoutine(workerID int) {
	defer r.wg.Done()
	
	r.logger.Debugf("Router worker %d started", workerID)
	
	for {
		select {
		case <-r.ctx.Done():
			r.logger.Debugf("Router worker %d stopped", workerID)
			return
		default:
			// 工作协程可以在这里处理队列中的任务
			// 目前主要用于保持协程活跃
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// GetEventChannel 获取事件通道
func (r *Router) GetEventChannel() <-chan events.Event {
	return r.eventChan
}

// GetStats 获取统计信息
func (r *Router) GetStats() RouterStats {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	stats := *r.stats
	stats.ActiveConnections = r.wsManager.GetConnectionCount()
	stats.PendingMessages = r.processor.GetPendingRequestCount()
	
	return stats
}

// ResetStats 重置统计信息
func (r *Router) ResetStats() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.stats = &RouterStats{
		LastResetTime: time.Now(),
	}
}

// 统计更新方法
func (r *Router) incrementReceivedMessages() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.stats.MessagesReceived++
}

func (r *Router) incrementProcessedMessages() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.stats.MessagesProcessed++
}

func (r *Router) incrementFailedMessages() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.stats.MessagesFailed++
}

func (r *Router) incrementGeneratedEvents() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.stats.EventsGenerated++
}

func (r *Router) updateConnectionCount() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.stats.ActiveConnections = r.wsManager.GetConnectionCount()
}

func (r *Router) updateProcessingTime(duration time.Duration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	durationMs := float64(duration.Nanoseconds()) / 1e6

	if r.stats.AverageProcessTime == 0 {
		r.stats.AverageProcessTime = durationMs
	} else {
		// 简单移动平均
		r.stats.AverageProcessTime = (r.stats.AverageProcessTime + durationMs) / 2
	}
}

// logStats 记录统计信息
func (r *Router) logStats() {
	stats := r.GetStats()

	r.logger.Infof("Router Stats - Received: %d, Processed: %d, Failed: %d, Events: %d, Connections: %d, Avg Time: %.2fms",
		stats.MessagesReceived,
		stats.MessagesProcessed,
		stats.MessagesFailed,
		stats.EventsGenerated,
		stats.ActiveConnections,
		stats.AverageProcessTime)
}

// serializeResponse 序列化响应
func (r *Router) serializeResponse(response *ocpp16.ProcessorResponse) ([]byte, error) {
	// 这里需要根据OCPP协议格式序列化响应
	// 简化实现，实际应该使用serialization包

	if response.Success {
		// 创建CallResult消息
		message := []interface{}{3, response.MessageID, response.Payload}
		return r.marshalJSON(message)
	} else {
		// 创建CallError消息
		errorCode := "InternalError"
		errorDescription := "Processing failed"
		if response.Error != nil {
			errorDescription = response.Error.Error()
		}

		message := []interface{}{4, response.MessageID, errorCode, errorDescription, map[string]interface{}{}}
		return r.marshalJSON(message)
	}
}

// serializeErrorResponse 序列化错误响应
func (r *Router) serializeErrorResponse(errorResponse map[string]interface{}) ([]byte, error) {
	return r.marshalJSON(errorResponse)
}

// marshalJSON JSON序列化辅助方法
func (r *Router) marshalJSON(data interface{}) ([]byte, error) {
	// 这里应该使用serialization包，简化实现
	return []byte(fmt.Sprintf("%v", data)), nil
}

// BroadcastMessage 广播消息
func (r *Router) BroadcastMessage(message []byte) {
	r.logger.Debugf("Broadcasting message to all connections")
	r.wsManager.BroadcastMessage(message)
}

// SendMessageToChargePoint 发送消息到指定充电桩
func (r *Router) SendMessageToChargePoint(chargePointID string, message []byte) error {
	return r.wsManager.SendMessage(chargePointID, message)
}

// GetActiveConnections 获取活跃连接列表
func (r *Router) GetActiveConnections() []string {
	connections := r.wsManager.GetAllConnections()
	var chargePointIDs []string

	for chargePointID := range connections {
		chargePointIDs = append(chargePointIDs, chargePointID)
	}

	return chargePointIDs
}

// IsChargePointConnected 检查充电桩是否连接
func (r *Router) IsChargePointConnected(chargePointID string) bool {
	return r.wsManager.HasConnection(chargePointID)
}

// GetConnectionInfo 获取连接信息
func (r *Router) GetConnectionInfo(chargePointID string) (*websocket.ConnectionWrapper, bool) {
	return r.wsManager.GetConnection(chargePointID)
}

// HandleHTTPUpgrade 处理HTTP升级到WebSocket
func (r *Router) HandleHTTPUpgrade(w interface{}, req interface{}, chargePointID string) error {
	// 这里需要类型断言，简化实现
	return fmt.Errorf("HTTP upgrade handling not implemented in this simplified version")
}

// GetHealthStatus 获取健康状态
func (r *Router) GetHealthStatus() map[string]interface{} {
	stats := r.GetStats()

	return map[string]interface{}{
		"status":              "healthy",
		"active_connections":  stats.ActiveConnections,
		"messages_processed":  stats.MessagesProcessed,
		"messages_failed":     stats.MessagesFailed,
		"events_generated":    stats.EventsGenerated,
		"average_process_time": stats.AverageProcessTime,
		"uptime_seconds":      time.Since(stats.LastResetTime).Seconds(),
	}
}
