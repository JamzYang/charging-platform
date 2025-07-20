package router

import (
	"context"
	"fmt"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/gateway"
	"github.com/charging-platform/charge-point-gateway/internal/transport/websocket"
)

// MessageRouter 新的消息路由器接口
// 专注于路由职责，不直接处理协议逻辑
type MessageRouter interface {
	// RouteMessage 路由消息到分发器
	RouteMessage(ctx context.Context, chargePointID string, message []byte) error

	// RegisterConnection 注册新连接
	RegisterConnection(chargePointID string, conn *websocket.ConnectionWrapper) error

	// UnregisterConnection 注销连接
	UnregisterConnection(chargePointID string) error

	// GetConnectionInfo 获取连接信息
	GetConnectionInfo(chargePointID string) (*websocket.ConnectionWrapper, bool)

	// SetMessageDispatcher 设置消息分发器
	SetMessageDispatcher(dispatcher gateway.MessageDispatcher) error

	// SetWebSocketManager 设置WebSocket管理器
	SetWebSocketManager(manager *websocket.Manager) error

	// Start 启动路由器
	Start() error

	// Stop 停止路由器
	Stop() error

	// GetEventChannel 获取事件通道
	GetEventChannel() <-chan events.Event

	// GetStats 获取路由统计信息
	GetStats() RouterStats

	// ResetStats 重置统计信息
	ResetStats()

	// SendMessageToChargePoint 发送消息到指定充电桩
	SendMessageToChargePoint(chargePointID string, message []byte) error

	// BroadcastMessage 广播消息到所有连接
	BroadcastMessage(message []byte) error

	// GetActiveConnections 获取活跃连接列表
	GetActiveConnections() []string

	// IsChargePointConnected 检查充电桩是否连接
	IsChargePointConnected(chargePointID string) bool

	// GetHealthStatus 获取健康状态
	GetHealthStatus() map[string]interface{}
}

// RouterConfig 新的路由器配置
type RouterConfig struct {
	// 消息处理配置
	MaxConcurrentMessages int           `json:"max_concurrent_messages"`
	MessageTimeout        time.Duration `json:"message_timeout"`
	RetryAttempts         int           `json:"retry_attempts"`
	RetryDelay            time.Duration `json:"retry_delay"`

	// 连接管理配置
	MaxConnections          int           `json:"max_connections"`
	ConnectionTimeout       time.Duration `json:"connection_timeout"`
	ConnectionCheckInterval time.Duration `json:"connection_check_interval"`

	// 事件配置
	EventChannelSize int  `json:"event_channel_size"`
	EnableEvents     bool `json:"enable_events"`

	// 性能配置
	WorkerCount   int           `json:"worker_count"`
	BufferSize    int           `json:"buffer_size"`
	StatsInterval time.Duration `json:"stats_interval"`
	EnableMetrics bool          `json:"enable_metrics"`

	// 错误处理配置
	EnableErrorRecovery bool          `json:"enable_error_recovery"`
	ErrorThreshold      int           `json:"error_threshold"`
	CircuitBreakerDelay time.Duration `json:"circuit_breaker_delay"`

	// 日志配置
	EnableMessageLogging bool   `json:"enable_message_logging"`
	LogLevel             string `json:"log_level"`
}

// DefaultRouterConfig 默认路由器配置
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		MaxConcurrentMessages: 1000,
		MessageTimeout:        30 * time.Second,
		RetryAttempts:         3,
		RetryDelay:            1 * time.Second,

		MaxConnections:          10000,
		ConnectionTimeout:       5 * time.Minute,
		ConnectionCheckInterval: 30 * time.Second,

		EventChannelSize: 50000, // 增加事件通道容量以支持高并发
		EnableEvents:     true,

		WorkerCount:   8,
		BufferSize:    1000,
		StatsInterval: 1 * time.Minute,
		EnableMetrics: true,

		EnableErrorRecovery: true,
		ErrorThreshold:      10,
		CircuitBreakerDelay: 5 * time.Minute,

		EnableMessageLogging: false,
		LogLevel:             "info",
	}
}

// RouterStats 路由器统计信息
type RouterStats struct {
	// 消息统计
	MessagesReceived int64 `json:"messages_received"`
	MessagesRouted   int64 `json:"messages_routed"`
	MessagesFailed   int64 `json:"messages_failed"`
	MessagesDropped  int64 `json:"messages_dropped"`

	// 连接统计
	ActiveConnections   int   `json:"active_connections"`
	TotalConnections    int64 `json:"total_connections"`
	ConnectionsAccepted int64 `json:"connections_accepted"`
	ConnectionsRejected int64 `json:"connections_rejected"`

	// 事件统计
	EventsForwarded int64 `json:"events_forwarded"`
	EventsDropped   int64 `json:"events_dropped"`

	// 性能统计
	AverageRouteTime float64 `json:"average_route_time_ms"`
	MaxRouteTime     float64 `json:"max_route_time_ms"`

	// 错误统计
	RoutingErrors    int64 `json:"routing_errors"`
	ConnectionErrors int64 `json:"connection_errors"`
	DispatcherErrors int64 `json:"dispatcher_errors"`

	// 时间信息
	StartTime     time.Time     `json:"start_time"`
	LastResetTime time.Time     `json:"last_reset_time"`
	Uptime        time.Duration `json:"uptime"`
}

// MessageContext 消息上下文
type MessageContext struct {
	// 基本信息
	ChargePointID string    `json:"charge_point_id"`
	MessageData   []byte    `json:"message_data"`
	ReceivedAt    time.Time `json:"received_at"`

	// 处理信息
	Attempts       int           `json:"attempts"`
	LastError      error         `json:"last_error,omitempty"`
	ProcessingTime time.Duration `json:"processing_time"`

	// 路由信息
	ProtocolVersion string `json:"protocol_version,omitempty"`
	MessageType     string `json:"message_type,omitempty"`
	MessageID       string `json:"message_id,omitempty"`

	// 连接信息
	Connection *websocket.ConnectionWrapper `json:"-"`
	RemoteAddr string                       `json:"remote_addr"`
	UserAgent  string                       `json:"user_agent,omitempty"`
}

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	// 基本信息
	ChargePointID string    `json:"charge_point_id"`
	ConnectedAt   time.Time `json:"connected_at"`
	LastActivity  time.Time `json:"last_activity"`

	// 网络信息
	RemoteAddr string `json:"remote_addr"`
	UserAgent  string `json:"user_agent,omitempty"`

	// 协议信息
	ProtocolVersion  string   `json:"protocol_version,omitempty"`
	SupportedActions []string `json:"supported_actions,omitempty"`

	// 统计信息
	MessagesReceived int64 `json:"messages_received"`
	MessagesSent     int64 `json:"messages_sent"`
	ErrorCount       int64 `json:"error_count"`

	// 状态信息
	Status        string    `json:"status"`
	IsHealthy     bool      `json:"is_healthy"`
	LastError     string    `json:"last_error,omitempty"`
	LastErrorTime time.Time `json:"last_error_time,omitempty"`
}

// RouterEvent 路由器事件
type RouterEvent struct {
	Type          RouterEventType `json:"type"`
	ChargePointID string          `json:"charge_point_id"`
	Timestamp     time.Time       `json:"timestamp"`
	Data          interface{}     `json:"data,omitempty"`
	Error         error           `json:"error,omitempty"`
}

// RouterEventType 路由器事件类型
type RouterEventType string

const (
	// 连接事件
	RouterEventConnectionAccepted RouterEventType = "connection.accepted"
	RouterEventConnectionRejected RouterEventType = "connection.rejected"
	RouterEventConnectionClosed   RouterEventType = "connection.closed"
	RouterEventConnectionError    RouterEventType = "connection.error"

	// 消息事件
	RouterEventMessageReceived RouterEventType = "message.received"
	RouterEventMessageRouted   RouterEventType = "message.routed"
	RouterEventMessageFailed   RouterEventType = "message.failed"
	RouterEventMessageDropped  RouterEventType = "message.dropped"

	// 系统事件
	RouterEventStarted       RouterEventType = "router.started"
	RouterEventStopped       RouterEventType = "router.stopped"
	RouterEventDispatcherSet RouterEventType = "dispatcher.set"
	RouterEventHealthCheck   RouterEventType = "health.check"
)

// HealthStatus 健康状态
type HealthStatus struct {
	Status            string                 `json:"status"`
	Timestamp         time.Time              `json:"timestamp"`
	Uptime            time.Duration          `json:"uptime"`
	ActiveConnections int                    `json:"active_connections"`
	MessageRate       float64                `json:"message_rate_per_second"`
	ErrorRate         float64                `json:"error_rate_percent"`
	MemoryUsage       int64                  `json:"memory_usage_bytes"`
	Details           map[string]interface{} `json:"details,omitempty"`
}

// RouterError 路由器错误类型
type RouterError struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

func (e *RouterError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// 预定义错误代码
const (
	ErrCodeDispatcherNotSet       = "DISPATCHER_NOT_SET"
	ErrCodeWebSocketManagerNotSet = "WEBSOCKET_MANAGER_NOT_SET"
	ErrCodeConnectionNotFound     = "CONNECTION_NOT_FOUND"
	ErrCodeMessageTimeout         = "MESSAGE_TIMEOUT"
	ErrCodeRoutingFailed          = "ROUTING_FAILED"
	ErrCodeConnectionLimit        = "CONNECTION_LIMIT_EXCEEDED"
	ErrCodeInvalidMessage         = "INVALID_MESSAGE"
	ErrCodeDispatcherError        = "DISPATCHER_ERROR"
)
