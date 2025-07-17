package router

import (
	"context"
	"testing"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/domain/protocol"
	"github.com/charging-platform/charge-point-gateway/internal/gateway"
	"github.com/charging-platform/charge-point-gateway/internal/transport/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMessageDispatcher 模拟消息分发器
type MockMessageDispatcher struct {
	started     bool
	eventChan   chan events.Event
	handlers    map[string]gateway.ProtocolHandler
	processFunc func(ctx context.Context, chargePointID string, protocolVersion string, message []byte) (interface{}, error)
}

func NewMockMessageDispatcher() *MockMessageDispatcher {
	return &MockMessageDispatcher{
		eventChan: make(chan events.Event, 10),
		handlers:  make(map[string]gateway.ProtocolHandler),
		processFunc: func(ctx context.Context, chargePointID string, protocolVersion string, message []byte) (interface{}, error) {
			return map[string]interface{}{"status": "Accepted"}, nil
		},
	}
}

func (m *MockMessageDispatcher) RegisterHandler(version string, handler gateway.ProtocolHandler) error {
	m.handlers[version] = handler
	return nil
}

func (m *MockMessageDispatcher) UnregisterHandler(version string) error {
	delete(m.handlers, version)
	return nil
}

func (m *MockMessageDispatcher) DispatchMessage(ctx context.Context, chargePointID string, protocolVersion string, message []byte) (interface{}, error) {
	return m.processFunc(ctx, chargePointID, protocolVersion, message)
}

func (m *MockMessageDispatcher) IdentifyProtocolVersion(chargePointID string, message []byte) (string, error) {
	return protocol.OCPP_VERSION_1_6, nil
}

func (m *MockMessageDispatcher) GetRegisteredVersions() []string {
	versions := make([]string, 0, len(m.handlers))
	for version := range m.handlers {
		versions = append(versions, version)
	}
	return versions
}

func (m *MockMessageDispatcher) GetHandlerForVersion(version string) (gateway.ProtocolHandler, bool) {
	handler, exists := m.handlers[version]
	return handler, exists
}

func (m *MockMessageDispatcher) Start() error {
	m.started = true
	return nil
}

func (m *MockMessageDispatcher) Stop() error {
	m.started = false
	close(m.eventChan)
	return nil
}

func (m *MockMessageDispatcher) GetEventChannel() <-chan events.Event {
	return m.eventChan
}

func (m *MockMessageDispatcher) GetStats() gateway.DispatcherStats {
	return gateway.DispatcherStats{}
}

func TestNewDefaultMessageRouter(t *testing.T) {
	router := NewDefaultMessageRouter(nil)

	assert.NotNil(t, router)
	assert.NotNil(t, router.config)
	assert.NotNil(t, router.connections)
	assert.NotNil(t, router.eventChan)
	assert.NotNil(t, router.logger)
	assert.False(t, router.started)
}

func TestDefaultMessageRouter_SetMessageDispatcher(t *testing.T) {
	router := NewDefaultMessageRouter(nil)
	dispatcher := NewMockMessageDispatcher()

	// 测试设置分发器
	err := router.SetMessageDispatcher(dispatcher)
	assert.NoError(t, err)
	assert.Equal(t, dispatcher, router.dispatcher)

	// 测试设置nil分发器
	err = router.SetMessageDispatcher(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestDefaultMessageRouter_SetWebSocketManager(t *testing.T) {
	router := NewDefaultMessageRouter(nil)
	dispatcher := NewMockMessageDispatcher()
	wsManager := websocket.NewManager(websocket.DefaultConfig(), dispatcher, nil)

	// 测试设置WebSocket管理器
	err := router.SetWebSocketManager(wsManager)
	assert.NoError(t, err)
	assert.Equal(t, wsManager, router.wsManager)

	// 测试设置nil管理器
	err = router.SetWebSocketManager(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestDefaultMessageRouter_StartStop(t *testing.T) {
	router := NewDefaultMessageRouter(nil)
	dispatcher := NewMockMessageDispatcher()
	wsManager := websocket.NewManager(websocket.DefaultConfig(), dispatcher, nil)

	// 设置必要组件
	err := router.SetMessageDispatcher(dispatcher)
	require.NoError(t, err)
	err = router.SetWebSocketManager(wsManager)
	require.NoError(t, err)

	// 测试启动
	err = router.Start()
	assert.NoError(t, err)
	assert.True(t, router.started)
	assert.True(t, dispatcher.started)

	// 测试重复启动
	err = router.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	// 测试停止
	err = router.Stop()
	assert.NoError(t, err)
	assert.False(t, router.started)
	assert.False(t, dispatcher.started)

	// 测试重复停止
	err = router.Stop()
	assert.NoError(t, err)
}

func TestDefaultMessageRouter_StartWithoutComponents(t *testing.T) {
	router := NewDefaultMessageRouter(nil)

	// 测试没有分发器的启动
	err := router.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dispatcher must be set")

	// 设置分发器但没有WebSocket管理器
	dispatcher := NewMockMessageDispatcher()
	err = router.SetMessageDispatcher(dispatcher)
	require.NoError(t, err)

	err = router.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "websocket manager must be set")
}

func TestDefaultMessageRouter_RouteMessage(t *testing.T) {
	router := NewDefaultMessageRouter(nil)
	dispatcher := NewMockMessageDispatcher()

	// 设置分发器
	err := router.SetMessageDispatcher(dispatcher)
	require.NoError(t, err)

	// 测试路由消息
	ctx := context.Background()
	message := []byte(`{"action": "BootNotification"}`)

	err = router.RouteMessage(ctx, "CP001", message)
	assert.NoError(t, err)

	// 验证统计信息
	stats := router.GetStats()
	assert.Equal(t, int64(1), stats.MessagesReceived)
	assert.Equal(t, int64(1), stats.MessagesRouted)
	assert.Equal(t, int64(0), stats.MessagesFailed)
}

func TestDefaultMessageRouter_RouteMessageWithoutDispatcher(t *testing.T) {
	router := NewDefaultMessageRouter(nil)

	// 测试没有分发器的消息路由
	ctx := context.Background()
	message := []byte(`{"action": "BootNotification"}`)

	err := router.RouteMessage(ctx, "CP001", message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dispatcher not set")

	// 验证统计信息
	stats := router.GetStats()
	assert.Equal(t, int64(1), stats.MessagesReceived)
	assert.Equal(t, int64(0), stats.MessagesRouted)
	assert.Equal(t, int64(1), stats.MessagesFailed)
}

func TestDefaultMessageRouter_RegisterUnregisterConnection(t *testing.T) {
	router := NewDefaultMessageRouter(nil)

	// 创建模拟连接
	conn := &websocket.ConnectionWrapper{}

	// 测试注册连接
	err := router.RegisterConnection("CP001", conn)
	assert.NoError(t, err)

	// 验证连接存在
	assert.True(t, router.IsChargePointConnected("CP001"))

	// 获取活跃连接
	connections := router.GetActiveConnections()
	assert.Contains(t, connections, "CP001")

	// 测试注销连接
	err = router.UnregisterConnection("CP001")
	assert.NoError(t, err)

	// 验证连接不存在
	assert.False(t, router.IsChargePointConnected("CP001"))

	// 测试注销不存在的连接
	err = router.UnregisterConnection("CP002")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection not found")
}

func TestDefaultMessageRouter_ConnectionLimit(t *testing.T) {
	config := DefaultRouterConfig()
	config.MaxConnections = 2
	router := NewDefaultMessageRouter(config)

	conn := &websocket.ConnectionWrapper{}

	// 注册最大数量的连接
	err := router.RegisterConnection("CP001", conn)
	assert.NoError(t, err)
	err = router.RegisterConnection("CP002", conn)
	assert.NoError(t, err)

	// 尝试注册超过限制的连接
	err = router.RegisterConnection("CP003", conn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection limit exceeded")
}

func TestDefaultMessageRouter_GetStats(t *testing.T) {
	router := NewDefaultMessageRouter(nil)

	// 获取初始统计信息
	stats := router.GetStats()
	assert.Equal(t, int64(0), stats.MessagesReceived)
	assert.Equal(t, int64(0), stats.MessagesRouted)
	assert.Equal(t, int64(0), stats.MessagesFailed)
	assert.Equal(t, int64(0), stats.EventsForwarded)
	assert.True(t, stats.Uptime >= 0)

	// 更新一些统计信息
	router.incrementReceivedMessages()
	router.incrementRoutedMessages()
	router.incrementFailedMessages()
	router.incrementForwardedEvents()

	// 验证统计信息更新
	stats = router.GetStats()
	assert.Equal(t, int64(1), stats.MessagesReceived)
	assert.Equal(t, int64(1), stats.MessagesRouted)
	assert.Equal(t, int64(1), stats.MessagesFailed)
	assert.Equal(t, int64(1), stats.EventsForwarded)
}

func TestDefaultMessageRouter_ResetStats(t *testing.T) {
	router := NewDefaultMessageRouter(nil)

	// 更新一些统计信息
	router.incrementReceivedMessages()
	router.incrementRoutedMessages()

	// 验证统计信息
	stats := router.GetStats()
	assert.Equal(t, int64(1), stats.MessagesReceived)
	assert.Equal(t, int64(1), stats.MessagesRouted)

	// 重置统计信息
	router.ResetStats()

	// 验证统计信息已重置
	stats = router.GetStats()
	assert.Equal(t, int64(0), stats.MessagesReceived)
	assert.Equal(t, int64(0), stats.MessagesRouted)
}

func TestDefaultMessageRouter_GetHealthStatus(t *testing.T) {
	router := NewDefaultMessageRouter(nil)
	dispatcher := NewMockMessageDispatcher()
	wsManager := websocket.NewManager(websocket.DefaultConfig(), dispatcher, nil)

	// 设置组件
	err := router.SetMessageDispatcher(dispatcher)
	require.NoError(t, err)
	err = router.SetWebSocketManager(wsManager)
	require.NoError(t, err)

	// 获取健康状态
	health := router.GetHealthStatus()

	assert.Equal(t, "healthy", health["status"])
	assert.True(t, health["dispatcher_set"].(bool))
	assert.True(t, health["websocket_manager_set"].(bool))
	assert.Equal(t, int64(0), health["messages_received"])
	assert.Equal(t, float64(0), health["error_rate_percent"])
}

func TestDefaultMessageRouter_GetEventChannel(t *testing.T) {
	router := NewDefaultMessageRouter(nil)

	eventChan := router.GetEventChannel()
	assert.NotNil(t, eventChan)

	// 测试通道是否可读
	select {
	case <-eventChan:
		// 通道可读
	default:
		// 通道为空，这是正常的
	}
}
