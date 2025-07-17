package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/connection"
	"github.com/charging-platform/charge-point-gateway/internal/domain/protocol"
	"github.com/charging-platform/charge-point-gateway/internal/gateway"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
	"github.com/charging-platform/charge-point-gateway/internal/message"
	"github.com/charging-platform/charge-point-gateway/internal/metrics"
	"github.com/charging-platform/charge-point-gateway/internal/protocol/ocpp16"
	"github.com/gorilla/websocket"
)

// getGlobalLogger 获取全局日志器
func getGlobalLogger() *logger.Logger {
	// 创建一个默认的日志器
	l, _ := logger.New(logger.DefaultConfig())
	return l
}

// Manager WebSocket连接管理器
type Manager struct {
	// 配置
	config *Config

	// WebSocket升级器
	upgrader *websocket.Upgrader

	// 连接存储
	connections map[string]*ConnectionWrapper
	mutex       sync.RWMutex

	// 事件通道
	eventChan chan ConnectionEvent

	// 消息分发器 - 按照架构设计添加
	dispatcher gateway.MessageDispatcher

	// 生命周期管理
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startTime time.Time

	// 日志器
	logger *logger.Logger
}

// Config WebSocket管理器配置
type Config struct {
	// 服务器配置
	Host string `json:"host"`
	Port int    `json:"port"`
	Path string `json:"path"`

	// WebSocket配置
	ReadBufferSize    int           `json:"read_buffer_size"`
	WriteBufferSize   int           `json:"write_buffer_size"`
	HandshakeTimeout  time.Duration `json:"handshake_timeout"`
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`
	PingInterval      time.Duration `json:"ping_interval"`
	PongTimeout       time.Duration `json:"pong_timeout"`
	MaxMessageSize    int64         `json:"max_message_size"`
	EnableCompression bool          `json:"enable_compression"`

	// 连接管理
	MaxConnections  int           `json:"max_connections"`
	IdleTimeout     time.Duration `json:"idle_timeout"`
	CleanupInterval time.Duration `json:"cleanup_interval"`

	// 安全配置
	CheckOrigin       bool     `json:"check_origin"`
	AllowedOrigins    []string `json:"allowed_origins"`
	EnableSubprotocol bool     `json:"enable_subprotocol"`
	Subprotocols      []string `json:"subprotocols"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Host: "0.0.0.0",
		Port: 8080,
		Path: "/ocpp",

		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		HandshakeTimeout:  10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      10 * time.Second,
		PingInterval:      30 * time.Second,
		PongTimeout:       10 * time.Second,
		MaxMessageSize:    1024 * 1024, // 1MB
		EnableCompression: false,

		MaxConnections:  1000,
		IdleTimeout:     5 * time.Minute,
		CleanupInterval: 1 * time.Minute,

		CheckOrigin:       false,
		AllowedOrigins:    []string{},
		EnableSubprotocol: true,
		Subprotocols:      protocol.GetSupportedVersions(),
	}
}

// ConnectionEvent 连接事件
type ConnectionEvent struct {
	Type          ConnectionEventType `json:"type"`
	ChargePointID string              `json:"charge_point_id"`
	Connection    *ConnectionWrapper  `json:"connection,omitempty"`
	Message       []byte              `json:"message,omitempty"`
	Error         error               `json:"error,omitempty"`
	Timestamp     time.Time           `json:"timestamp"`
}

// ConnectionEventType 连接事件类型
type ConnectionEventType string

const (
	EventTypeConnected    ConnectionEventType = "connected"
	EventTypeDisconnected ConnectionEventType = "disconnected"
	EventTypeError        ConnectionEventType = "error"
	EventTypeMessage      ConnectionEventType = "message"
	EventTypePing         ConnectionEventType = "ping"
	EventTypePong         ConnectionEventType = "pong"
)

// ConnectionWrapper 连接包装器
type ConnectionWrapper struct {
	// 基础连接信息
	conn          *websocket.Conn
	chargePointID string
	metadata      *connection.Connection

	// 消息通道
	sendChan chan []byte

	// 消息分发器 - 按照架构设计添加
	dispatcher gateway.MessageDispatcher

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc

	// 状态管理
	lastActivity time.Time
	mutex        sync.RWMutex

	// 配置
	config *Config
	logger *logger.Logger
}

// NewManager 创建新的WebSocket管理器
func NewManager(config *Config, dispatcher gateway.MessageDispatcher, log *logger.Logger) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	if log == nil {
		log = getGlobalLogger()
	}

	ctx, cancel := context.WithCancel(context.Background())

	upgrader := &websocket.Upgrader{
		ReadBufferSize:    config.ReadBufferSize,
		WriteBufferSize:   config.WriteBufferSize,
		HandshakeTimeout:  config.HandshakeTimeout,
		EnableCompression: config.EnableCompression,
		Subprotocols:      config.Subprotocols,
		CheckOrigin: func(r *http.Request) bool {
			if !config.CheckOrigin {
				return true
			}

			origin := r.Header.Get("Origin")
			if len(config.AllowedOrigins) == 0 {
				return true
			}

			for _, allowed := range config.AllowedOrigins {
				if origin == allowed {
					return true
				}
			}
			return false
		},
	}

	return &Manager{
		config:      config,
		upgrader:    upgrader,
		connections: make(map[string]*ConnectionWrapper),
		eventChan:   make(chan ConnectionEvent, 100),
		dispatcher:  dispatcher,
		ctx:         ctx,
		cancel:      cancel,
		startTime:   time.Now(),
		logger:      log,
	}
}

// Start 启动WebSocket管理器
func (m *Manager) Start() error {
	m.logger.Infof("Starting WebSocket manager on %s:%d%s",
		m.config.Host, m.config.Port, m.config.Path)

	// 启动清理协程
	m.wg.Add(1)
	go m.cleanupRoutine()

	// 启动HTTP服务器
	m.wg.Add(1)
	go m.startHTTPServer()

	return nil
}

// startHTTPServer 启动HTTP服务器
func (m *Manager) startHTTPServer() {
	defer m.wg.Done()

	mux := http.NewServeMux()

	// 注册WebSocket路由
	mux.HandleFunc(m.config.Path+"/", m.handleWebSocketUpgrade)

	// 注册健康检查路由
	mux.HandleFunc("/health", m.handleHealthCheck)

	// 注册连接状态路由
	mux.HandleFunc("/connections", m.handleConnectionsStatus)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", m.config.Host, m.config.Port),
		Handler: mux,
	}

	m.logger.Infof("HTTP server starting on %s", server.Addr)

	// 启动服务器
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		m.logger.Errorf("HTTP server failed: %v", err)
	}
}

// handleWebSocketUpgrade 处理WebSocket升级请求
func (m *Manager) handleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) {
	// 从URL路径中提取充电桩ID
	chargePointID := m.extractChargePointID(r.URL.Path)
	if chargePointID == "" {
		http.Error(w, "Invalid charge point ID", http.StatusBadRequest)
		return
	}

	// 处理WebSocket连接
	if err := m.HandleConnection(w, r, chargePointID); err != nil {
		m.logger.Errorf("Failed to handle WebSocket connection for %s: %v", chargePointID, err)
		// 错误已经在HandleConnection中处理，这里不需要再次写入响应
	}
}

// extractChargePointID 从URL路径中提取充电桩ID
func (m *Manager) extractChargePointID(path string) string {
	// 移除路径前缀，例如 "/ocpp/CP-001" -> "CP-001"
	prefix := m.config.Path + "/"
	if len(path) <= len(prefix) {
		return ""
	}

	chargePointID := path[len(prefix):]
	if chargePointID == "" {
		return ""
	}

	return chargePointID
}

// handleHealthCheck 处理健康检查请求
func (m *Manager) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":      "healthy",
		"timestamp":   time.Now().Format(time.RFC3339),
		"connections": m.GetConnectionCount(),
		"uptime":      time.Since(m.startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleConnectionsStatus 处理连接状态请求
func (m *Manager) handleConnectionsStatus(w http.ResponseWriter, r *http.Request) {
	connections := make(map[string]interface{})

	m.mutex.RLock()
	for chargePointID, wrapper := range m.connections {
		// 获取子协议信息
		subprotocol, _ := wrapper.metadata.GetMetadata("subprotocol")
		subprotocolStr := ""
		if subprotocol != nil {
			subprotocolStr = subprotocol.(string)
		}

		connections[chargePointID] = map[string]interface{}{
			"last_activity": wrapper.GetLastActivity().Format(time.RFC3339),
			"connected_at":  wrapper.metadata.ConnectedAt.Format(time.RFC3339),
			"remote_addr":   wrapper.metadata.NetworkInfo.RemoteAddr,
			"subprotocol":   subprotocolStr,
		}
	}
	m.mutex.RUnlock()

	status := map[string]interface{}{
		"total_connections": len(connections),
		"connections":       connections,
		"timestamp":         time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Shutdown 优雅关闭WebSocket管理器
func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger.Info("Shutting down WebSocket manager...")

	// 取消上下文
	m.cancel()

	// 关闭所有连接
	m.mutex.Lock()
	for chargePointID, wrapper := range m.connections {
		m.logger.Infof("Closing connection for charge point: %s", chargePointID)
		wrapper.Close()
	}
	m.connections = make(map[string]*ConnectionWrapper)
	m.mutex.Unlock()

	// 等待所有协程结束
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("WebSocket manager shutdown completed")
		return nil
	case <-ctx.Done():
		m.logger.Warn("WebSocket manager shutdown timeout")
		return ctx.Err()
	}
}

// Stop 停止WebSocket管理器
func (m *Manager) Stop() error {
	m.logger.Info("Stopping WebSocket manager")

	// 取消上下文
	m.cancel()

	// 关闭所有连接
	m.mutex.Lock()
	for chargePointID, wrapper := range m.connections {
		m.logger.Debugf("Closing connection for charge point: %s", chargePointID)
		wrapper.Close()
	}
	m.connections = make(map[string]*ConnectionWrapper)
	m.mutex.Unlock()

	// 等待所有协程结束
	m.wg.Wait()

	// 关闭事件通道
	close(m.eventChan)

	m.logger.Info("WebSocket manager stopped")
	return nil
}

// HandleConnection 处理WebSocket连接升级
func (m *Manager) HandleConnection(w http.ResponseWriter, r *http.Request, chargePointID string) error {
	// 检查连接数限制
	if m.GetConnectionCount() >= m.config.MaxConnections {
		http.Error(w, "Too many connections", http.StatusTooManyRequests)
		return fmt.Errorf("connection limit exceeded")
	}

	// 检查是否已存在连接
	if m.HasConnection(chargePointID) {
		http.Error(w, "Connection already exists", http.StatusConflict)
		return fmt.Errorf("connection already exists for charge point: %s", chargePointID)
	}

	// 升级到WebSocket
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		m.logger.Errorf("Failed to upgrade connection for %s: %v", chargePointID, err)
		return fmt.Errorf("failed to upgrade connection: %w", err)
	}

	// 创建连接包装器
	wrapper := m.createConnectionWrapper(conn, chargePointID, r)

	// 存储连接
	m.mutex.Lock()
	m.connections[chargePointID] = wrapper
	m.mutex.Unlock()

	// Update metrics
	metrics.ActiveConnections.Inc()

	// 启动连接处理
	m.wg.Add(1)
	go m.handleConnectionWrapper(wrapper)

	// 发送连接事件
	m.sendEvent(ConnectionEvent{
		Type:          EventTypeConnected,
		ChargePointID: chargePointID,
		Connection:    wrapper,
		Timestamp:     time.Now(),
	})

	m.logger.Infof("WebSocket connection established for %s from %s",
		chargePointID, r.RemoteAddr)

	return nil
}

// createConnectionWrapper 创建连接包装器
func (m *Manager) createConnectionWrapper(conn *websocket.Conn, chargePointID string, r *http.Request) *ConnectionWrapper {
	ctx, cancel := context.WithCancel(m.ctx)

	// 设置连接参数
	conn.SetReadLimit(m.config.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(m.config.ReadTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(m.config.ReadTimeout))
		return nil
	})

	// 获取协商的子协议
	subprotocol := conn.Subprotocol()
	normalizedVersion := protocol.NormalizeVersion(subprotocol)
	if normalizedVersion == "" {
		// 如果没有协商到有效的子协议，使用默认版本
		normalizedVersion = protocol.GetDefaultVersion()
		m.logger.Warnf("No valid subprotocol negotiated for %s, using default: %s", chargePointID, normalizedVersion)
	}

	// 转换为连接协议版本类型
	connectionProtocolVersion := protocol.ToConnectionProtocolVersion(normalizedVersion)

	// 创建连接元数据
	metadata := connection.NewConnection(
		fmt.Sprintf("ws-%s-%d", chargePointID, time.Now().Unix()),
		chargePointID,
		connection.ConnectionTypeWebSocket,
		connectionProtocolVersion,
	)

	// 更新网络信息
	metadata.UpdateNetworkInfo(r.RemoteAddr, r.Host)
	metadata.SetMetadata("user_agent", r.UserAgent())
	metadata.SetMetadata("origin", r.Header.Get("Origin"))
	metadata.SetMetadata("subprotocol", normalizedVersion)

	return &ConnectionWrapper{
		conn:          conn,
		chargePointID: chargePointID,
		metadata:      metadata,
		sendChan:      make(chan []byte, 100),
		dispatcher:    m.dispatcher,
		ctx:           ctx,
		cancel:        cancel,
		lastActivity:  time.Now(),
		config:        m.config,
		logger:        m.logger,
	}
}

// handleConnectionWrapper 处理连接包装器
func (m *Manager) handleConnectionWrapper(wrapper *ConnectionWrapper) {
	defer m.wg.Done()
	defer wrapper.Close()
	defer m.removeConnection(wrapper.chargePointID)

	// 启动发送协程
	go wrapper.sendRoutine()

	// 启动ping协程
	go wrapper.pingRoutine()

	// 处理接收消息
	wrapper.receiveRoutine(m.eventChan)
}

// GetConnection 获取连接
func (m *Manager) GetConnection(chargePointID string) (*ConnectionWrapper, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	wrapper, exists := m.connections[chargePointID]
	return wrapper, exists
}

// HasConnection 检查连接是否存在
func (m *Manager) HasConnection(chargePointID string) bool {
	_, exists := m.GetConnection(chargePointID)
	return exists
}

// GetConnectionCount 获取连接数
func (m *Manager) GetConnectionCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.connections)
}

// GetAllConnections 获取所有连接
func (m *Manager) GetAllConnections() map[string]*ConnectionWrapper {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*ConnectionWrapper)
	for k, v := range m.connections {
		result[k] = v
	}
	return result
}

// removeConnection 移除连接
func (m *Manager) removeConnection(chargePointID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if wrapper, exists := m.connections[chargePointID]; exists {
		delete(m.connections, chargePointID)

		// Update metrics
		metrics.ActiveConnections.Dec()

		// 发送断开连接事件
		m.sendEvent(ConnectionEvent{
			Type:          EventTypeDisconnected,
			ChargePointID: chargePointID,
			Connection:    wrapper,
			Timestamp:     time.Now(),
		})

		m.logger.Infof("Connection removed for charge point: %s", chargePointID)
	}
}

// SendMessage 发送消息
func (m *Manager) SendMessage(chargePointID string, message []byte) error {
	wrapper, exists := m.GetConnection(chargePointID)
	if !exists {
		return fmt.Errorf("connection not found for charge point: %s", chargePointID)
	}

	return wrapper.SendMessage(message)
}

// SendCommand 发送指令（用于下行指令处理）
func (m *Manager) SendCommand(chargePointID string, cmd interface{}) error {
	// 将通用的 Command 结构转换为 OCPP Call 格式
	// [2, messageId, action, payload]
	var ocppCall []interface{}

	if command, ok := cmd.(*message.Command); ok {
		ocppCall = []interface{}{
			2, // MessageType Call
			// 为下行指令动态生成唯一的MessageID
			fmt.Sprintf("gw-%d", time.Now().UnixNano()),
			command.CommandName,
			command.Payload,
		}
	} else {
		return fmt.Errorf("unsupported command type: %T", cmd)
	}

	// 将OCPP Call序列化为JSON
	messageBytes, err := json.Marshal(ocppCall)
	if err != nil {
		return fmt.Errorf("failed to marshal OCPP call: %w", err)
	}

	return m.SendMessage(chargePointID, messageBytes)
}

// GetEventChannel 获取事件通道
func (m *Manager) GetEventChannel() <-chan ConnectionEvent {
	return m.eventChan
}

// BroadcastMessage 广播消息
func (m *Manager) BroadcastMessage(message []byte) {
	connections := m.GetAllConnections()

	for chargePointID, wrapper := range connections {
		if err := wrapper.SendMessage(message); err != nil {
			m.logger.Errorf("Failed to send broadcast message to %s: %v",
				chargePointID, err)
		}
	}
}

// SendMessage 发送消息
func (w *ConnectionWrapper) SendMessage(message []byte) error {
	select {
	case w.sendChan <- message:
		return nil
	case <-w.ctx.Done():
		return fmt.Errorf("connection closed")
	default:
		return fmt.Errorf("send channel full")
	}
}

// Close 关闭连接
func (w *ConnectionWrapper) Close() {
	w.cancel()
	w.conn.Close()
	close(w.sendChan)
}

// GetLastActivity 获取最后活动时间
func (w *ConnectionWrapper) GetLastActivity() time.Time {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.lastActivity
}

// GetMetadata 获取连接元数据
func (w *ConnectionWrapper) GetMetadata() *connection.Connection {
	return w.metadata
}

// updateActivity 更新活动时间
func (w *ConnectionWrapper) updateActivity() {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.lastActivity = time.Now()
	w.metadata.UpdateLastActivity()
}

// sendRoutine 发送协程
func (w *ConnectionWrapper) sendRoutine() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case message, ok := <-w.sendChan:
			if !ok {
				return
			}

			w.conn.SetWriteDeadline(time.Now().Add(w.config.WriteTimeout))
			if err := w.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				w.logger.Errorf("Failed to send message to %s: %v", w.chargePointID, err)
				return
			}

			w.updateActivity()
			w.metadata.IncrementMessagesSent(int64(len(message)))
		}
	}
}

// receiveRoutine 接收协程
func (w *ConnectionWrapper) receiveRoutine(eventChan chan<- ConnectionEvent) {
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			messageType, message, err := w.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					w.logger.Errorf("WebSocket error for %s: %v", w.chargePointID, err)
				}
				return
			}

			w.updateActivity()
			w.metadata.IncrementMessagesReceived(int64(len(message)))

			if messageType == websocket.TextMessage {
				// 按照架构设计，直接调用分发器处理消息
				if w.dispatcher != nil {
					w.handleMessage(message)
				}

				// 仍然发送消息事件（用于监控和日志）
				select {
				case eventChan <- ConnectionEvent{
					Type:          EventTypeMessage,
					ChargePointID: w.chargePointID,
					Connection:    w,
					Message:       message,
					Timestamp:     time.Now(),
				}:
				default:
					w.logger.Warn("Event channel full, dropping message event")
				}
			}
		}
	}
}

// handleMessage 处理接收到的消息 - 按照架构设计实现
func (w *ConnectionWrapper) handleMessage(message []byte) {
	w.logger.Errorf("WEBSOCKET: Handling message from %s", w.chargePointID)

	// 从连接元数据获取协议版本
	protocolVersion := ""
	if w.metadata != nil {
		if subprotocol, exists := w.metadata.GetMetadata("subprotocol"); exists && subprotocol != nil {
			if rawVersion, ok := subprotocol.(string); ok {
				// 规范化协议版本，确保一致性
				protocolVersion = protocol.NormalizeVersion(rawVersion)
				w.logger.Errorf("WEBSOCKET: Using normalized protocol version from metadata: %s (raw: %s)", protocolVersion, rawVersion)
			}
		}
	}

	// 调用分发器处理消息
	ctx := context.Background()
	response, err := w.dispatcher.DispatchMessage(ctx, w.chargePointID, protocolVersion, message)
	if err != nil {
		w.logger.Errorf("WEBSOCKET: Failed to dispatch message from %s: %v", w.chargePointID, err)
		// 可以在这里根据错误类型决定是否关闭连接或发送错误响应
		return
	}

	w.logger.Errorf("WEBSOCKET: Successfully dispatched message from %s", w.chargePointID)

	// 如果有响应，则发送回客户端
	if response != nil {
		// 将内部响应转换为OCPP 1.6 CallResult格式: [3, messageId, payload]
		if procResponse, ok := response.(*ocpp16.ProcessorResponse); ok && procResponse.Success {
			ocppResponse := []interface{}{
				3, // MessageType CallResult
				procResponse.MessageID,
				procResponse.Payload,
			}

			responseBytes, err := json.Marshal(ocppResponse)
			if err != nil {
				w.logger.Errorf("WEBSOCKET: Failed to marshal OCPP response for %s: %v", w.chargePointID, err)
				return
			}

			// 发送响应
			if err := w.SendMessage(responseBytes); err != nil {
				w.logger.Errorf("WEBSOCKET: Failed to send OCPP response to %s: %v", w.chargePointID, err)
			} else {
				w.logger.Debugf("WEBSOCKET: Successfully sent OCPP response to %s", w.chargePointID)
			}
		} else if procResponse, ok := response.(*ocpp16.ProcessorResponse); ok && !procResponse.Success {
			// TODO: Handle CallError message creation
			w.logger.Warnf("WEBSOCKET: Received a failed processor response for %s, but error handling is not implemented.", w.chargePointID)
		} else {
			w.logger.Errorf("WEBSOCKET: Received an unexpected response type for %s: %T", w.chargePointID, response)
		}
	}
}

// pingRoutine ping协程
func (w *ConnectionWrapper) pingRoutine() {
	ticker := time.NewTicker(w.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.conn.SetWriteDeadline(time.Now().Add(w.config.WriteTimeout))
			if err := w.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				w.logger.Errorf("Failed to send ping to %s: %v", w.chargePointID, err)
				return
			}

			// 发送ping事件
			select {
			case w.sendChan <- nil: // 这里应该通过事件通道发送，但为了简化先这样处理
			default:
			}
		}
	}
}

// sendEvent 发送事件
func (m *Manager) sendEvent(event ConnectionEvent) {
	select {
	case m.eventChan <- event:
	default:
		m.logger.Warnf("Event channel full, dropping event type: %s", event.Type)
	}
}

// cleanupRoutine 清理协程
func (m *Manager) cleanupRoutine() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupIdleConnections()
		}
	}
}

// cleanupIdleConnections 清理空闲连接
func (m *Manager) cleanupIdleConnections() {
	now := time.Now()
	var toRemove []string

	m.mutex.RLock()
	for chargePointID, wrapper := range m.connections {
		if now.Sub(wrapper.GetLastActivity()) > m.config.IdleTimeout {
			toRemove = append(toRemove, chargePointID)
		}
	}
	m.mutex.RUnlock()

	for _, chargePointID := range toRemove {
		m.logger.Infof("Closing idle connection for charge point: %s", chargePointID)
		if wrapper, exists := m.GetConnection(chargePointID); exists {
			wrapper.Close()
		}
	}
}
