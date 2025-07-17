package router

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/gateway"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
	"github.com/charging-platform/charge-point-gateway/internal/transport/websocket"
)

// DefaultMessageRouter 默认消息路由器实现
type DefaultMessageRouter struct {
	// 核心组件
	dispatcher gateway.MessageDispatcher
	wsManager  *websocket.Manager

	// 配置
	config *RouterConfig

	// 连接管理
	connections map[string]*ConnectionInfo
	connMutex   sync.RWMutex

	// 事件系统
	eventChan chan events.Event

	// 统计信息
	stats      RouterStats
	statsMutex sync.RWMutex

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

// NewDefaultMessageRouter 创建新的默认消息路由器
func NewDefaultMessageRouter(config *RouterConfig) *DefaultMessageRouter {
	if config == nil {
		config = DefaultRouterConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 创建日志器
	l, _ := logger.New(logger.DefaultConfig())

	return &DefaultMessageRouter{
		config:      config,
		connections: make(map[string]*ConnectionInfo),
		eventChan:   make(chan events.Event, config.EventChannelSize),
		stats: RouterStats{
			StartTime:     time.Now(),
			LastResetTime: time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
		logger: l,
	}
}

// SetMessageDispatcher 设置消息分发器
func (r *DefaultMessageRouter) SetMessageDispatcher(dispatcher gateway.MessageDispatcher) error {
	if dispatcher == nil {
		return &RouterError{
			Code:      ErrCodeDispatcherNotSet,
			Message:   "message dispatcher cannot be nil",
			Timestamp: time.Now(),
		}
	}

	r.dispatcher = dispatcher
	r.logger.Info("Message dispatcher set successfully")

	return nil
}

// SetWebSocketManager 设置WebSocket管理器
func (r *DefaultMessageRouter) SetWebSocketManager(manager *websocket.Manager) error {
	if manager == nil {
		return &RouterError{
			Code:      ErrCodeWebSocketManagerNotSet,
			Message:   "websocket manager cannot be nil",
			Timestamp: time.Now(),
		}
	}

	r.wsManager = manager
	r.logger.Info("WebSocket manager set successfully")

	return nil
}

// Start 启动路由器
func (r *DefaultMessageRouter) Start() error {
	r.startMutex.Lock()
	defer r.startMutex.Unlock()

	if r.started {
		return &RouterError{
			Code:      "ALREADY_STARTED",
			Message:   "router is already started",
			Timestamp: time.Now(),
		}
	}

	// 检查必要组件
	if r.dispatcher == nil {
		return &RouterError{
			Code:      ErrCodeDispatcherNotSet,
			Message:   "message dispatcher must be set before starting",
			Timestamp: time.Now(),
		}
	}

	if r.wsManager == nil {
		return &RouterError{
			Code:      ErrCodeWebSocketManagerNotSet,
			Message:   "websocket manager must be set before starting",
			Timestamp: time.Now(),
		}
	}

	r.logger.Info("Starting message router")

	// 启动WebSocket管理器
	if err := r.wsManager.Start(); err != nil {
		return fmt.Errorf("failed to start WebSocket manager: %w", err)
	}

	// 启动消息分发器
	if err := r.dispatcher.Start(); err != nil {
		return fmt.Errorf("failed to start message dispatcher: %w", err)
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

	// 启动连接清理协程
	r.wg.Add(1)
	go r.connectionCleanupRoutine()

	// 启动工作协程
	for i := 0; i < r.config.WorkerCount; i++ {
		r.wg.Add(1)
		go r.workerRoutine(i)
	}

	r.started = true
	r.stats.StartTime = time.Now()

	r.logger.Infof("Message router started with %d workers", r.config.WorkerCount)

	return nil
}

// Stop 停止路由器
func (r *DefaultMessageRouter) Stop() error {
	r.startMutex.Lock()
	defer r.startMutex.Unlock()

	if !r.started {
		return nil
	}

	r.logger.Info("Stopping message router")

	// 取消上下文
	r.cancel()

	// 停止组件
	if r.dispatcher != nil {
		if err := r.dispatcher.Stop(); err != nil {
			r.logger.Errorf("Error stopping message dispatcher: %v", err)
		}
	}

	if r.wsManager != nil {
		if err := r.wsManager.Stop(); err != nil {
			r.logger.Errorf("Error stopping WebSocket manager: %v", err)
		}
	}

	// 等待所有协程结束
	r.wg.Wait()

	// 关闭事件通道
	close(r.eventChan)

	r.started = false

	r.logger.Info("Message router stopped")

	return nil
}

// RouteMessage 路由消息到分发器
func (r *DefaultMessageRouter) RouteMessage(ctx context.Context, chargePointID string, message []byte) error {
	startTime := time.Now()

	// 更新统计信息
	r.incrementReceivedMessages()

	// 检查分发器
	if r.dispatcher == nil {
		r.incrementFailedMessages()
		return &RouterError{
			Code:      ErrCodeDispatcherNotSet,
			Message:   "message dispatcher not set",
			Timestamp: time.Now(),
			Context: map[string]interface{}{
				"charge_point_id": chargePointID,
			},
		}
	}

	// 创建带超时的上下文
	routeCtx, cancel := context.WithTimeout(ctx, r.config.MessageTimeout)
	defer cancel()

	// 记录消息日志
	if r.config.EnableMessageLogging {
		r.logger.Debugf("Routing message from %s: %s", chargePointID, string(message))
	}

	// 分发消息（不指定协议版本，让分发器自动识别）
	response, err := r.dispatcher.DispatchMessage(routeCtx, chargePointID, "", message)

	processingTime := time.Since(startTime)
	r.updateProcessingTime(processingTime)

	if err != nil {
		r.incrementFailedMessages()
		r.logger.Errorf("Failed to route message from %s: %v", chargePointID, err)

		return &RouterError{
			Code:      ErrCodeRoutingFailed,
			Message:   fmt.Sprintf("failed to route message: %v", err),
			Timestamp: time.Now(),
			Context: map[string]interface{}{
				"charge_point_id": chargePointID,
				"processing_time": processingTime,
			},
		}
	}

	r.incrementRoutedMessages()

	// 如果有响应，发送回充电桩
	if response != nil {
		if err := r.sendResponse(chargePointID, response); err != nil {
			r.logger.Errorf("Failed to send response to %s: %v", chargePointID, err)
		}
	}

	r.logger.Debugf("Successfully routed message from %s in %v", chargePointID, processingTime)

	return nil
}

// RegisterConnection 注册新连接
func (r *DefaultMessageRouter) RegisterConnection(chargePointID string, conn *websocket.ConnectionWrapper) error {
	r.connMutex.Lock()
	defer r.connMutex.Unlock()

	// 检查连接限制
	if len(r.connections) >= r.config.MaxConnections {
		r.incrementRejectedConnections()
		return &RouterError{
			Code:      ErrCodeConnectionLimit,
			Message:   "maximum connection limit exceeded",
			Timestamp: time.Now(),
			Context: map[string]interface{}{
				"charge_point_id":     chargePointID,
				"current_connections": len(r.connections),
				"max_connections":     r.config.MaxConnections,
			},
		}
	}

	// 创建连接信息
	connInfo := &ConnectionInfo{
		ChargePointID:    chargePointID,
		ConnectedAt:      time.Now(),
		LastActivity:     time.Now(),
		MessagesReceived: 0,
		MessagesSent:     0,
		ErrorCount:       0,
		Status:           "connected",
		IsHealthy:        true,
	}

	// 如果连接已存在，更新信息
	if existing, exists := r.connections[chargePointID]; exists {
		r.logger.Warnf("Connection %s already exists, updating", chargePointID)
		connInfo.MessagesReceived = existing.MessagesReceived
		connInfo.MessagesSent = existing.MessagesSent
		connInfo.ErrorCount = existing.ErrorCount
	}

	r.connections[chargePointID] = connInfo
	r.incrementAcceptedConnections()

	r.logger.Infof("Connection registered: %s", chargePointID)

	return nil
}

// UnregisterConnection 注销连接
func (r *DefaultMessageRouter) UnregisterConnection(chargePointID string) error {
	r.connMutex.Lock()
	defer r.connMutex.Unlock()

	if _, exists := r.connections[chargePointID]; !exists {
		return &RouterError{
			Code:      ErrCodeConnectionNotFound,
			Message:   "connection not found",
			Timestamp: time.Now(),
			Context: map[string]interface{}{
				"charge_point_id": chargePointID,
			},
		}
	}

	delete(r.connections, chargePointID)

	r.logger.Infof("Connection unregistered: %s", chargePointID)

	return nil
}

// GetConnectionInfo 获取连接信息
func (r *DefaultMessageRouter) GetConnectionInfo(chargePointID string) (*websocket.ConnectionWrapper, bool) {
	// 委托给WebSocket管理器
	if r.wsManager == nil {
		return nil, false
	}

	return r.wsManager.GetConnection(chargePointID)
}

// GetEventChannel 获取事件通道
func (r *DefaultMessageRouter) GetEventChannel() <-chan events.Event {
	return r.eventChan
}

// GetStats 获取路由统计信息
func (r *DefaultMessageRouter) GetStats() RouterStats {
	r.statsMutex.RLock()
	defer r.statsMutex.RUnlock()

	// 复制统计信息
	stats := r.stats
	stats.Uptime = time.Since(r.stats.StartTime)
	stats.ActiveConnections = len(r.connections)

	return stats
}

// ResetStats 重置统计信息
func (r *DefaultMessageRouter) ResetStats() {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()

	r.stats = RouterStats{
		StartTime:     r.stats.StartTime,
		LastResetTime: time.Now(),
	}

	r.logger.Info("Router statistics reset")
}

// SendMessageToChargePoint 发送消息到指定充电桩
func (r *DefaultMessageRouter) SendMessageToChargePoint(chargePointID string, message []byte) error {
	if r.wsManager == nil {
		return &RouterError{
			Code:      ErrCodeWebSocketManagerNotSet,
			Message:   "websocket manager not set",
			Timestamp: time.Now(),
		}
	}

	// 更新连接统计
	r.updateConnectionMessageSent(chargePointID)

	return r.wsManager.SendMessage(chargePointID, message)
}

// BroadcastMessage 广播消息到所有连接
func (r *DefaultMessageRouter) BroadcastMessage(message []byte) error {
	if r.wsManager == nil {
		return &RouterError{
			Code:      ErrCodeWebSocketManagerNotSet,
			Message:   "websocket manager not set",
			Timestamp: time.Now(),
		}
	}

	r.logger.Debugf("Broadcasting message to all connections")
	r.wsManager.BroadcastMessage(message)

	return nil
}

// GetActiveConnections 获取活跃连接列表
func (r *DefaultMessageRouter) GetActiveConnections() []string {
	r.connMutex.RLock()
	defer r.connMutex.RUnlock()

	connections := make([]string, 0, len(r.connections))
	for chargePointID := range r.connections {
		connections = append(connections, chargePointID)
	}

	return connections
}

// IsChargePointConnected 检查充电桩是否连接
func (r *DefaultMessageRouter) IsChargePointConnected(chargePointID string) bool {
	r.connMutex.RLock()
	defer r.connMutex.RUnlock()

	_, exists := r.connections[chargePointID]
	return exists
}

// GetHealthStatus 获取健康状态
func (r *DefaultMessageRouter) GetHealthStatus() map[string]interface{} {
	stats := r.GetStats()

	// 计算错误率
	errorRate := float64(0)
	if stats.MessagesReceived > 0 {
		errorRate = float64(stats.MessagesFailed) / float64(stats.MessagesReceived) * 100
	}

	// 计算消息处理率
	messageRate := float64(0)
	if stats.Uptime.Seconds() > 0 {
		messageRate = float64(stats.MessagesReceived) / stats.Uptime.Seconds()
	}

	status := "healthy"
	if errorRate > 10 { // 错误率超过10%认为不健康
		status = "unhealthy"
	} else if errorRate > 5 { // 错误率超过5%认为警告
		status = "warning"
	}

	return map[string]interface{}{
		"status":                  status,
		"timestamp":               time.Now(),
		"uptime_seconds":          stats.Uptime.Seconds(),
		"active_connections":      stats.ActiveConnections,
		"messages_received":       stats.MessagesReceived,
		"messages_routed":         stats.MessagesRouted,
		"messages_failed":         stats.MessagesFailed,
		"events_forwarded":        stats.EventsForwarded,
		"error_rate_percent":      errorRate,
		"message_rate_per_second": messageRate,
		"average_route_time_ms":   stats.AverageRouteTime,
		"max_route_time_ms":       stats.MaxRouteTime,
		"dispatcher_set":          r.dispatcher != nil,
		"websocket_manager_set":   r.wsManager != nil,
	}
}

// messageRoutine 消息处理协程
func (r *DefaultMessageRouter) messageRoutine() {
	defer r.wg.Done()

	if r.wsManager == nil {
		r.logger.Error("WebSocket manager not set, message routine exiting")
		return
	}

	// 获取WebSocket事件通道
	wsEventChan := r.wsManager.GetEventChannel()

	r.logger.Debug("Message routine started")

	for {
		select {
		case <-r.ctx.Done():
			r.logger.Debug("Message routine stopping")
			return

		case wsEvent := <-wsEventChan:
			r.handleWebSocketEvent(wsEvent)
		}
	}
}

// handleWebSocketEvent 处理WebSocket事件
func (r *DefaultMessageRouter) handleWebSocketEvent(wsEvent websocket.ConnectionEvent) {
	switch wsEvent.Type {
	case websocket.EventTypeConnected:
		r.logger.Infof("Charge point connected: %s", wsEvent.ChargePointID)
		if err := r.RegisterConnection(wsEvent.ChargePointID, wsEvent.Connection); err != nil {
			r.logger.Errorf("Failed to register connection %s: %v", wsEvent.ChargePointID, err)
		}

	case websocket.EventTypeDisconnected:
		r.logger.Infof("Charge point disconnected: %s", wsEvent.ChargePointID)
		if err := r.UnregisterConnection(wsEvent.ChargePointID); err != nil {
			r.logger.Errorf("Failed to unregister connection %s: %v", wsEvent.ChargePointID, err)
		}

	case websocket.EventTypeMessage:
		// 处理接收到的消息
		r.logger.Debugf("Message event received from %s", wsEvent.ChargePointID)
		if len(wsEvent.Message) > 0 {
			r.handleMessage(wsEvent.ChargePointID, wsEvent.Message)
		}

	case websocket.EventTypeError:
		r.logger.Errorf("WebSocket error for %s: %v", wsEvent.ChargePointID, wsEvent.Error)
		r.incrementConnectionErrors()
		r.updateConnectionError(wsEvent.ChargePointID, wsEvent.Error)

	default:
		r.logger.Debugf("Unhandled WebSocket event type: %s", wsEvent.Type)
	}
}

// handleMessage 处理消息
func (r *DefaultMessageRouter) handleMessage(chargePointID string, messageData []byte) {
	// 更新连接活动时间
	r.updateConnectionActivity(chargePointID)

	// 异步路由消息
	go func() {
		ctx := context.Background()
		if err := r.RouteMessage(ctx, chargePointID, messageData); err != nil {
			r.logger.Errorf("Failed to route message from %s: %v", chargePointID, err)
		}
	}()
}

// TODO: 这个方法需要在WebSocket管理器支持消息数据后重新实现
// handleMessageEvent 处理消息事件（当WebSocket管理器支持消息数据时使用）
func (r *DefaultMessageRouter) handleMessageEvent(chargePointID string) {
	// 更新连接活动时间
	r.updateConnectionActivity(chargePointID)

	// 这里需要从WebSocket连接中读取实际的消息数据
	// 目前WebSocket事件不包含消息数据，这是一个架构限制
	r.logger.Debugf("Message event received from %s, but message data not available in event", chargePointID)
}

// eventRoutine 事件处理协程
func (r *DefaultMessageRouter) eventRoutine() {
	defer r.wg.Done()

	if r.dispatcher == nil {
		r.logger.Error("Message dispatcher not set, event routine exiting")
		return
	}

	// 获取分发器事件通道
	dispatcherEventChan := r.dispatcher.GetEventChannel()

	r.logger.Debug("Event routine started")

	for {
		select {
		case <-r.ctx.Done():
			r.logger.Debug("Event routine stopping")
			return

		case event := <-dispatcherEventChan:
			r.forwardEvent(event)
		}
	}
}

// forwardEvent 转发事件
func (r *DefaultMessageRouter) forwardEvent(event events.Event) {
	select {
	case r.eventChan <- event:
		r.incrementForwardedEvents()
		r.logger.Debugf("Forwarded event %s from charge point %s", event.GetType(), event.GetChargePointID())
	default:
		r.incrementDroppedEvents()
		r.logger.Warn("Event channel full, dropping event")
	}
}

// statsRoutine 统计协程
func (r *DefaultMessageRouter) statsRoutine() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.config.StatsInterval)
	defer ticker.Stop()

	r.logger.Debug("Stats routine started")

	for {
		select {
		case <-r.ctx.Done():
			r.logger.Debug("Stats routine stopping")
			return

		case <-ticker.C:
			r.logStats()
		}
	}
}

// connectionCleanupRoutine 连接清理协程
func (r *DefaultMessageRouter) connectionCleanupRoutine() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.config.ConnectionCheckInterval)
	defer ticker.Stop()

	r.logger.Debug("Connection cleanup routine started")

	for {
		select {
		case <-r.ctx.Done():
			r.logger.Debug("Connection cleanup routine stopping")
			return

		case <-ticker.C:
			r.cleanupStaleConnections()
		}
	}
}

// workerRoutine 工作协程
func (r *DefaultMessageRouter) workerRoutine(workerID int) {
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

// sendResponse 发送响应
func (r *DefaultMessageRouter) sendResponse(chargePointID string, response interface{}) error {
	// 这里需要将响应序列化为字节数组
	// 简化实现，实际应该根据协议格式序列化
	responseData := []byte(fmt.Sprintf("%v", response))

	return r.SendMessageToChargePoint(chargePointID, responseData)
}

// cleanupStaleConnections 清理过期连接
func (r *DefaultMessageRouter) cleanupStaleConnections() {
	r.connMutex.Lock()
	defer r.connMutex.Unlock()

	now := time.Now()
	staleConnections := make([]string, 0)

	for chargePointID, connInfo := range r.connections {
		if now.Sub(connInfo.LastActivity) > r.config.ConnectionTimeout {
			staleConnections = append(staleConnections, chargePointID)
		}
	}

	for _, chargePointID := range staleConnections {
		delete(r.connections, chargePointID)
		r.logger.Warnf("Cleaned up stale connection: %s", chargePointID)
	}

	if len(staleConnections) > 0 {
		r.logger.Infof("Cleaned up %d stale connections", len(staleConnections))
	}
}

// logStats 记录统计信息
func (r *DefaultMessageRouter) logStats() {
	stats := r.GetStats()

	r.logger.Infof("Router Stats - Received: %d, Routed: %d, Failed: %d, Events: %d, Connections: %d, Avg Time: %.2fms",
		stats.MessagesReceived,
		stats.MessagesRouted,
		stats.MessagesFailed,
		stats.EventsForwarded,
		stats.ActiveConnections,
		stats.AverageRouteTime)
}

// 统计更新方法
func (r *DefaultMessageRouter) incrementReceivedMessages() {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()
	r.stats.MessagesReceived++
}

func (r *DefaultMessageRouter) incrementRoutedMessages() {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()
	r.stats.MessagesRouted++
}

func (r *DefaultMessageRouter) incrementFailedMessages() {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()
	r.stats.MessagesFailed++
}

func (r *DefaultMessageRouter) incrementDroppedMessages() {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()
	r.stats.MessagesDropped++
}

func (r *DefaultMessageRouter) incrementAcceptedConnections() {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()
	r.stats.ConnectionsAccepted++
	r.stats.TotalConnections++
}

func (r *DefaultMessageRouter) incrementRejectedConnections() {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()
	r.stats.ConnectionsRejected++
}

func (r *DefaultMessageRouter) incrementForwardedEvents() {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()
	r.stats.EventsForwarded++
}

func (r *DefaultMessageRouter) incrementDroppedEvents() {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()
	r.stats.EventsDropped++
}

func (r *DefaultMessageRouter) incrementConnectionErrors() {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()
	r.stats.ConnectionErrors++
}

func (r *DefaultMessageRouter) updateProcessingTime(duration time.Duration) {
	r.statsMutex.Lock()
	defer r.statsMutex.Unlock()

	durationMs := float64(duration.Nanoseconds()) / 1e6

	if durationMs > r.stats.MaxRouteTime {
		r.stats.MaxRouteTime = durationMs
	}

	if r.stats.AverageRouteTime == 0 {
		r.stats.AverageRouteTime = durationMs
	} else {
		// 简单移动平均
		r.stats.AverageRouteTime = (r.stats.AverageRouteTime + durationMs) / 2
	}
}

// 连接信息更新方法
func (r *DefaultMessageRouter) updateConnectionActivity(chargePointID string) {
	r.connMutex.Lock()
	defer r.connMutex.Unlock()

	if connInfo, exists := r.connections[chargePointID]; exists {
		connInfo.LastActivity = time.Now()
		connInfo.MessagesReceived++
	}
}

func (r *DefaultMessageRouter) updateConnectionMessageSent(chargePointID string) {
	r.connMutex.Lock()
	defer r.connMutex.Unlock()

	if connInfo, exists := r.connections[chargePointID]; exists {
		connInfo.MessagesSent++
	}
}

func (r *DefaultMessageRouter) updateConnectionError(chargePointID string, err error) {
	r.connMutex.Lock()
	defer r.connMutex.Unlock()

	if connInfo, exists := r.connections[chargePointID]; exists {
		connInfo.ErrorCount++
		connInfo.LastError = err.Error()
		connInfo.LastErrorTime = time.Now()
		connInfo.IsHealthy = false
	}
}
