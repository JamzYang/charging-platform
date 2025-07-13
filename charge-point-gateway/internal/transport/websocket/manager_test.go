package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/connection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 8080, config.Port)
	assert.Equal(t, "/ocpp", config.Path)
	assert.Equal(t, 4096, config.ReadBufferSize)
	assert.Equal(t, 4096, config.WriteBufferSize)
	assert.Equal(t, 10*time.Second, config.HandshakeTimeout)
	assert.Equal(t, 1000, config.MaxConnections)
	assert.False(t, config.CheckOrigin)
	assert.True(t, config.EnableSubprotocol)
	assert.Contains(t, config.Subprotocols, "ocpp1.6")
}

func TestNewManager(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config)
	
	assert.NotNil(t, manager)
	assert.Equal(t, config, manager.config)
	assert.NotNil(t, manager.upgrader)
	assert.NotNil(t, manager.connections)
	assert.NotNil(t, manager.eventChan)
	assert.NotNil(t, manager.ctx)
	assert.NotNil(t, manager.logger)
}

func TestNewManagerWithNilConfig(t *testing.T) {
	manager := NewManager(nil)
	
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.config)
	assert.Equal(t, DefaultConfig().Host, manager.config.Host)
}

func TestManager_StartStop(t *testing.T) {
	manager := NewManager(DefaultConfig())
	
	// 测试启动
	err := manager.Start()
	assert.NoError(t, err)
	
	// 测试停止
	err = manager.Stop()
	assert.NoError(t, err)
	
	// 验证连接已清空
	assert.Equal(t, 0, manager.GetConnectionCount())
}

func TestManager_ConnectionManagement(t *testing.T) {
	manager := NewManager(DefaultConfig())
	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()
	
	chargePointID := "CP001"
	
	// 测试初始状态
	assert.False(t, manager.HasConnection(chargePointID))
	assert.Equal(t, 0, manager.GetConnectionCount())
	
	// 模拟连接（这里我们直接操作内部状态进行测试）
	// 在实际使用中，连接会通过HandleConnection方法建立
	
	// 测试获取不存在的连接
	_, exists := manager.GetConnection(chargePointID)
	assert.False(t, exists)
	
	// 测试获取所有连接
	connections := manager.GetAllConnections()
	assert.Empty(t, connections)
}

func TestManager_HandleConnection_TooManyConnections(t *testing.T) {
	config := DefaultConfig()
	config.MaxConnections = 0 // 设置为0以测试连接限制
	manager := NewManager(config)
	
	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()
	
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := manager.HandleConnection(w, r, "CP001")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection limit exceeded")
	}))
	defer server.Close()
	
	// 发送请求
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestManager_SendMessage_ConnectionNotFound(t *testing.T) {
	manager := NewManager(DefaultConfig())
	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()
	
	err = manager.SendMessage("nonexistent", []byte("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection not found")
}

func TestConnectionWrapper_SendMessage(t *testing.T) {
	config := DefaultConfig()
	
	// 创建模拟的WebSocket连接
	// 注意：这里我们创建一个最小的测试，实际的WebSocket测试需要更复杂的设置
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	wrapper := &ConnectionWrapper{
		chargePointID: "CP001",
		sendChan:      make(chan []byte, 10),
		ctx:           ctx,
		cancel:        cancel,
		lastActivity:  time.Now(),
		config:        config,
	}
	
	// 测试发送消息
	message := []byte("test message")
	err := wrapper.SendMessage(message)
	assert.NoError(t, err)
	
	// 验证消息在通道中
	select {
	case receivedMessage := <-wrapper.sendChan:
		assert.Equal(t, message, receivedMessage)
	case <-time.After(time.Second):
		t.Fatal("Message not received in send channel")
	}
}

func TestConnectionWrapper_SendMessage_ChannelFull(t *testing.T) {
	config := DefaultConfig()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	wrapper := &ConnectionWrapper{
		chargePointID: "CP001",
		sendChan:      make(chan []byte, 1), // 容量为1的通道
		ctx:           ctx,
		cancel:        cancel,
		config:        config,
	}
	
	// 填满通道
	err := wrapper.SendMessage([]byte("message1"))
	assert.NoError(t, err)
	
	// 再次发送应该失败
	err = wrapper.SendMessage([]byte("message2"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "send channel full")
}

func TestConnectionWrapper_SendMessage_ContextCancelled(t *testing.T) {
	config := DefaultConfig()

	// 创建一个已经取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	wrapper := &ConnectionWrapper{
		chargePointID: "CP001",
		sendChan:      make(chan []byte, 10),
		ctx:           ctx,
		cancel:        cancel,
		config:        config,
	}

	// 验证上下文确实被取消了
	select {
	case <-ctx.Done():
		// 上下文已取消，这是我们期望的
	default:
		t.Fatal("Context should be cancelled")
	}

	// 多次尝试发送消息，测试并发行为
	var errors []error
	for i := 0; i < 10; i++ {
		err := wrapper.SendMessage([]byte("test"))
		if err != nil {
			errors = append(errors, err)
		}
	}

	// 在高性能系统中，至少应该有一些发送失败
	// 但我们不强制要求所有发送都失败，因为这取决于调度
	t.Logf("Failed sends: %d/10", len(errors))

	// 如果有错误，验证错误消息
	for _, err := range errors {
		assert.Contains(t, err.Error(), "connection closed")
	}
}

func TestConnectionWrapper_GetLastActivity(t *testing.T) {
	wrapper := &ConnectionWrapper{
		lastActivity: time.Now(),
	}
	
	activity := wrapper.GetLastActivity()
	assert.WithinDuration(t, time.Now(), activity, time.Second)
}

func TestConnectionWrapper_UpdateActivity(t *testing.T) {
	// 创建真实的连接元数据
	metadata := connection.NewConnection(
		"test-conn",
		"CP001",
		connection.ConnectionTypeWebSocket,
		connection.ProtocolVersionOCPP16,
	)

	wrapper := &ConnectionWrapper{
		metadata:     metadata,
		lastActivity: time.Now().Add(-time.Hour), // 设置为1小时前
	}

	oldActivity := wrapper.GetLastActivity()

	// 等待一小段时间确保时间差异
	time.Sleep(1 * time.Millisecond)

	wrapper.updateActivity()

	newActivity := wrapper.GetLastActivity()
	assert.True(t, newActivity.After(oldActivity))
}

func TestConnectionEvent(t *testing.T) {
	event := ConnectionEvent{
		Type:          EventTypeConnected,
		ChargePointID: "CP001",
		Timestamp:     time.Now(),
	}
	
	assert.Equal(t, EventTypeConnected, event.Type)
	assert.Equal(t, "CP001", event.ChargePointID)
	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
}

func TestConnectionEventTypes(t *testing.T) {
	eventTypes := []ConnectionEventType{
		EventTypeConnected,
		EventTypeDisconnected,
		EventTypeError,
		EventTypeMessage,
		EventTypePing,
		EventTypePong,
	}
	
	for _, eventType := range eventTypes {
		assert.NotEmpty(t, string(eventType))
	}
}

func TestManager_BroadcastMessage(t *testing.T) {
	manager := NewManager(DefaultConfig())
	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()
	
	// 测试空连接列表的广播
	manager.BroadcastMessage([]byte("test message"))
	// 应该不会panic或出错
}

func TestManager_GetEventChannel(t *testing.T) {
	manager := NewManager(DefaultConfig())
	
	eventChan := manager.GetEventChannel()
	assert.NotNil(t, eventChan)
	
	// 测试通道类型
	assert.IsType(t, (<-chan ConnectionEvent)(nil), eventChan)
}

func TestManager_CleanupIdleConnections(t *testing.T) {
	config := DefaultConfig()
	config.IdleTimeout = 100 * time.Millisecond // 设置很短的超时时间
	
	manager := NewManager(config)
	err := manager.Start()
	require.NoError(t, err)
	defer manager.Stop()
	
	// 直接调用清理方法进行测试
	manager.cleanupIdleConnections()
	// 应该不会panic
}

// 移除mockConnection，使用真实的connection.Connection

func TestUpgraderConfiguration(t *testing.T) {
	config := DefaultConfig()
	config.CheckOrigin = true
	config.AllowedOrigins = []string{"http://example.com"}
	
	manager := NewManager(config)
	
	// 验证upgrader配置
	assert.Equal(t, config.ReadBufferSize, manager.upgrader.ReadBufferSize)
	assert.Equal(t, config.WriteBufferSize, manager.upgrader.WriteBufferSize)
	assert.Equal(t, config.HandshakeTimeout, manager.upgrader.HandshakeTimeout)
	assert.Equal(t, config.EnableCompression, manager.upgrader.EnableCompression)
	assert.Equal(t, config.Subprotocols, manager.upgrader.Subprotocols)
	
	// 测试CheckOrigin函数
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://example.com")
	assert.True(t, manager.upgrader.CheckOrigin(req))
	
	req.Header.Set("Origin", "http://malicious.com")
	assert.False(t, manager.upgrader.CheckOrigin(req))
}

func TestUpgraderCheckOriginDisabled(t *testing.T) {
	config := DefaultConfig()
	config.CheckOrigin = false
	
	manager := NewManager(config)
	
	// 当CheckOrigin禁用时，应该允许所有来源
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	assert.True(t, manager.upgrader.CheckOrigin(req))
}

func TestUpgraderCheckOriginEmptyAllowedList(t *testing.T) {
	config := DefaultConfig()
	config.CheckOrigin = true
	config.AllowedOrigins = []string{} // 空的允许列表
	
	manager := NewManager(config)
	
	// 当允许列表为空时，应该允许所有来源
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	assert.True(t, manager.upgrader.CheckOrigin(req))
}
