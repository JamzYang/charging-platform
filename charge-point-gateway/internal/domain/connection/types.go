package connection

import (
	"net"
	"sync"
	"time"
)

// ConnectionState 连接状态
type ConnectionState string

const (
	ConnectionStateConnecting    ConnectionState = "connecting"
	ConnectionStateConnected     ConnectionState = "connected"
	ConnectionStateAuthenticated ConnectionState = "authenticated"
	ConnectionStateRegistered    ConnectionState = "registered"
	ConnectionStateDisconnecting ConnectionState = "disconnecting"
	ConnectionStateDisconnected  ConnectionState = "disconnected"
	ConnectionStateFaulted       ConnectionState = "faulted"
)

// ProtocolVersion 协议版本
type ProtocolVersion string

const (
	ProtocolVersionOCPP16  ProtocolVersion = "ocpp1.6"
	ProtocolVersionOCPP20  ProtocolVersion = "ocpp2.0"
	ProtocolVersionOCPP201 ProtocolVersion = "ocpp2.0.1"
)

// ConnectionType 连接类型
type ConnectionType string

const (
	ConnectionTypeWebSocket ConnectionType = "websocket"
	ConnectionTypeHTTP      ConnectionType = "http"
	ConnectionTypeTCP       ConnectionType = "tcp"
)

// SecurityProfile 安全配置文件
type SecurityProfile struct {
	SecurityProfile int    `json:"security_profile"` // 0, 1, 2, 3
	TLSEnabled      bool   `json:"tls_enabled"`
	CertificateAuth bool   `json:"certificate_auth"`
	BasicAuth       bool   `json:"basic_auth"`
	Username        string `json:"username,omitempty"`
	Password        string `json:"password,omitempty"`
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	RemoteAddr    string    `json:"remote_addr"`
	LocalAddr     string    `json:"local_addr"`
	UserAgent     string    `json:"user_agent,omitempty"`
	Origin        string    `json:"origin,omitempty"`
	ForwardedFor  string    `json:"forwarded_for,omitempty"`
	ConnectedAt   time.Time `json:"connected_at"`
	LastActivity  time.Time `json:"last_activity"`
	BytesSent     int64     `json:"bytes_sent"`
	BytesReceived int64     `json:"bytes_received"`
	MessagesSent  int64     `json:"messages_sent"`
	MessagesReceived int64  `json:"messages_received"`
}

// ConnectionMetrics 连接指标
type ConnectionMetrics struct {
	ConnectDuration    time.Duration `json:"connect_duration"`
	LastPingTime       time.Time     `json:"last_ping_time"`
	LastPongTime       time.Time     `json:"last_pong_time"`
	PingInterval       time.Duration `json:"ping_interval"`
	PongTimeout        time.Duration `json:"pong_timeout"`
	ReconnectCount     int           `json:"reconnect_count"`
	ErrorCount         int           `json:"error_count"`
	LastErrorTime      *time.Time    `json:"last_error_time,omitempty"`
	LastErrorMessage   string        `json:"last_error_message,omitempty"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	MaxResponseTime    time.Duration `json:"max_response_time"`
	MinResponseTime    time.Duration `json:"min_response_time"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	MaxMessageSize    int           `json:"max_message_size"`
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`
	PingInterval      time.Duration `json:"ping_interval"`
	PongTimeout       time.Duration `json:"pong_timeout"`
	MaxReconnectCount int           `json:"max_reconnect_count"`
	ReconnectInterval time.Duration `json:"reconnect_interval"`
	EnableCompression bool          `json:"enable_compression"`
	BufferSize        int           `json:"buffer_size"`
}

// Connection 连接模型
type Connection struct {
	// 基本信息
	ID              string            `json:"id"`
	ChargePointID   string            `json:"charge_point_id"`
	State           ConnectionState   `json:"state"`
	Type            ConnectionType    `json:"type"`
	ProtocolVersion ProtocolVersion   `json:"protocol_version"`
	
	// 网络信息
	NetworkInfo NetworkInfo `json:"network_info"`
	
	// 安全信息
	Security SecurityProfile `json:"security"`
	
	// 配置信息
	Config ConnectionConfig `json:"config"`
	
	// 指标信息
	Metrics ConnectionMetrics `json:"metrics"`
	
	// 时间戳
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ConnectedAt time.Time  `json:"connected_at"`
	LastSeenAt  time.Time  `json:"last_seen_at"`
	
	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Tags     []string               `json:"tags,omitempty"`
	
	// 内部字段 (不序列化)
	conn   net.Conn    `json:"-"`
	mutex  sync.RWMutex `json:"-"`
	closed bool        `json:"-"`
}

// NewConnection 创建新连接
func NewConnection(id, chargePointID string, connType ConnectionType, protocolVersion ProtocolVersion) *Connection {
	now := time.Now().UTC()
	return &Connection{
		ID:              id,
		ChargePointID:   chargePointID,
		State:           ConnectionStateConnecting,
		Type:            connType,
		ProtocolVersion: protocolVersion,
		NetworkInfo: NetworkInfo{
			ConnectedAt:      now,
			LastActivity:     now,
			BytesSent:        0,
			BytesReceived:    0,
			MessagesSent:     0,
			MessagesReceived: 0,
		},
		Security: SecurityProfile{
			SecurityProfile: 0,
			TLSEnabled:      false,
			CertificateAuth: false,
			BasicAuth:       false,
		},
		Config: ConnectionConfig{
			MaxMessageSize:    1024 * 1024, // 1MB
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			PingInterval:      30 * time.Second,
			PongTimeout:       10 * time.Second,
			MaxReconnectCount: 3,
			ReconnectInterval: 5 * time.Second,
			EnableCompression: false,
			BufferSize:        4096,
		},
		Metrics: ConnectionMetrics{
			ConnectDuration:     0,
			PingInterval:        30 * time.Second,
			PongTimeout:         10 * time.Second,
			ReconnectCount:      0,
			ErrorCount:          0,
			AverageResponseTime: 0,
			MaxResponseTime:     0,
			MinResponseTime:     time.Duration(^uint64(0) >> 1), // Max duration
		},
		CreatedAt:   now,
		UpdatedAt:   now,
		ConnectedAt: now,
		LastSeenAt:  now,
		Metadata:    make(map[string]interface{}),
		Tags:        make([]string, 0),
		closed:      false,
	}
}

// GetState 获取连接状态 (线程安全)
func (c *Connection) GetState() ConnectionState {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.State
}

// SetState 设置连接状态 (线程安全)
func (c *Connection) SetState(state ConnectionState) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.State = state
	c.UpdatedAt = time.Now().UTC()
}

// IsConnected 检查是否已连接
func (c *Connection) IsConnected() bool {
	state := c.GetState()
	return state == ConnectionStateConnected || 
		   state == ConnectionStateAuthenticated || 
		   state == ConnectionStateRegistered
}

// IsClosed 检查是否已关闭
func (c *Connection) IsClosed() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.closed
}

// Close 关闭连接
func (c *Connection) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	if c.closed {
		return nil
	}
	
	c.closed = true
	c.State = ConnectionStateDisconnected
	c.UpdatedAt = time.Now().UTC()
	
	if c.conn != nil {
		return c.conn.Close()
	}
	
	return nil
}

// UpdateLastActivity 更新最后活动时间
func (c *Connection) UpdateLastActivity() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.NetworkInfo.LastActivity = time.Now().UTC()
	c.LastSeenAt = c.NetworkInfo.LastActivity
	c.UpdatedAt = c.NetworkInfo.LastActivity
}

// UpdateNetworkInfo 更新网络信息
func (c *Connection) UpdateNetworkInfo(remoteAddr, localAddr string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.NetworkInfo.RemoteAddr = remoteAddr
	c.NetworkInfo.LocalAddr = localAddr
	c.UpdatedAt = time.Now().UTC()
}

// IncrementMessagesSent 增加发送消息计数
func (c *Connection) IncrementMessagesSent(bytes int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.NetworkInfo.MessagesSent++
	c.NetworkInfo.BytesSent += bytes
	// 直接更新时间，避免重复加锁
	now := time.Now().UTC()
	c.NetworkInfo.LastActivity = now
	c.LastSeenAt = now
	c.UpdatedAt = now
}

// IncrementMessagesReceived 增加接收消息计数
func (c *Connection) IncrementMessagesReceived(bytes int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.NetworkInfo.MessagesReceived++
	c.NetworkInfo.BytesReceived += bytes
	// 直接更新时间，避免重复加锁
	now := time.Now().UTC()
	c.NetworkInfo.LastActivity = now
	c.LastSeenAt = now
	c.UpdatedAt = now
}

// UpdateResponseTime 更新响应时间指标
func (c *Connection) UpdateResponseTime(responseTime time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// 更新最大响应时间
	if responseTime > c.Metrics.MaxResponseTime {
		c.Metrics.MaxResponseTime = responseTime
	}
	
	// 更新最小响应时间
	if responseTime < c.Metrics.MinResponseTime {
		c.Metrics.MinResponseTime = responseTime
	}
	
	// 计算平均响应时间 (简单移动平均)
	if c.Metrics.AverageResponseTime == 0 {
		c.Metrics.AverageResponseTime = responseTime
	} else {
		c.Metrics.AverageResponseTime = (c.Metrics.AverageResponseTime + responseTime) / 2
	}
	
	c.UpdatedAt = time.Now().UTC()
}

// RecordError 记录错误
func (c *Connection) RecordError(errorMessage string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.Metrics.ErrorCount++
	now := time.Now().UTC()
	c.Metrics.LastErrorTime = &now
	c.Metrics.LastErrorMessage = errorMessage
	c.UpdatedAt = now
}

// IncrementReconnectCount 增加重连计数
func (c *Connection) IncrementReconnectCount() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.Metrics.ReconnectCount++
	c.UpdatedAt = time.Now().UTC()
}

// SetMetadata 设置元数据
func (c *Connection) SetMetadata(key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.Metadata == nil {
		c.Metadata = make(map[string]interface{})
	}
	c.Metadata[key] = value
	c.UpdatedAt = time.Now().UTC()
}

// GetMetadata 获取元数据
func (c *Connection) GetMetadata(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if c.Metadata == nil {
		return nil, false
	}
	value, exists := c.Metadata[key]
	return value, exists
}

// AddTag 添加标签
func (c *Connection) AddTag(tag string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// 检查标签是否已存在
	for _, existingTag := range c.Tags {
		if existingTag == tag {
			return
		}
	}
	
	c.Tags = append(c.Tags, tag)
	c.UpdatedAt = time.Now().UTC()
}

// RemoveTag 移除标签
func (c *Connection) RemoveTag(tag string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	for i, existingTag := range c.Tags {
		if existingTag == tag {
			c.Tags = append(c.Tags[:i], c.Tags[i+1:]...)
			c.UpdatedAt = time.Now().UTC()
			break
		}
	}
}

// HasTag 检查是否有指定标签
func (c *Connection) HasTag(tag string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	for _, existingTag := range c.Tags {
		if existingTag == tag {
			return true
		}
	}
	return false
}

// GetConnectionDuration 获取连接持续时间
func (c *Connection) GetConnectionDuration() time.Duration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return time.Since(c.ConnectedAt)
}

// GetIdleDuration 获取空闲时间
func (c *Connection) GetIdleDuration() time.Duration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return time.Since(c.NetworkInfo.LastActivity)
}
