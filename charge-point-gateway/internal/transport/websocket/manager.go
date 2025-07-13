package websocket

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/charging-platform/charge-point-gateway/internal/domain/connection"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
	"github.com/charging-platform/charge-point-gateway/internal/metrics"
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
	
	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
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
	MaxConnections    int           `json:"max_connections"`
	IdleTimeout       time.Duration `json:"idle_timeout"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	
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
		Subprotocols:      []string{"ocpp1.6", "ocpp2.0"},
	}
}

// ConnectionEvent 连接事件
type ConnectionEvent struct {
	Type         ConnectionEventType `json:"type"`
	ChargePointID string             `json:"charge_point_id"`
	Connection   *ConnectionWrapper  `json:"connection,omitempty"`
	Error        error               `json:"error,omitempty"`
	Timestamp    time.Time           `json:"timestamp"`
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
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	upgrader := &websocket.Upgrader{
		ReadBufferSize:   config.ReadBufferSize,
		WriteBufferSize:  config.WriteBufferSize,
		HandshakeTimeout: config.HandshakeTimeout,
		EnableCompression: config.EnableCompression,
		Subprotocols:     config.Subprotocols,
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
		ctx:         ctx,
		cancel:      cancel,
		logger:      getGlobalLogger(),
	}
}

// Start 启动WebSocket管理器
func (m *Manager) Start() error {
	m.logger.Infof("Starting WebSocket manager on %s:%d%s",
		m.config.Host, m.config.Port, m.config.Path)
	
	// 启动清理协程
	m.wg.Add(1)
	go m.cleanupRoutine()
	
	return nil
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
	
	// 创建连接元数据
	metadata := connection.NewConnection(
		fmt.Sprintf("ws-%s-%d", chargePointID, time.Now().Unix()),
		chargePointID,
		connection.ConnectionTypeWebSocket,
		connection.ProtocolVersionOCPP16,
	)
	
	// 更新网络信息
	metadata.UpdateNetworkInfo(r.RemoteAddr, r.Host)
	metadata.SetMetadata("user_agent", r.UserAgent())
	metadata.SetMetadata("origin", r.Header.Get("Origin"))
	metadata.SetMetadata("subprotocol", conn.Subprotocol())
	
	return &ConnectionWrapper{
		conn:          conn,
		chargePointID: chargePointID,
		metadata:      metadata,
		sendChan:      make(chan []byte, 100),
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
				// 发送消息事件
				select {
				case eventChan <- ConnectionEvent{
					Type:          EventTypeMessage,
					ChargePointID: w.chargePointID,
					Connection:    w,
					Timestamp:     time.Now(),
				}:
				default:
					w.logger.Warn("Event channel full, dropping message event")
				}
			}
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

// GetEventChannel 获取事件通道
func (m *Manager) GetEventChannel() <-chan ConnectionEvent {
	return m.eventChan
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
