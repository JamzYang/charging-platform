package gateway

import (
	"context"
	"testing"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockProtocolHandler 简单的模拟协议处理器
type MockProtocolHandler struct {
	eventChan chan events.Event
	version   string
	started   bool

	// 用于测试的回调函数
	processMessageFunc func(ctx context.Context, chargePointID string, message []byte) (interface{}, error)
	supportedActions   []string
}

func NewMockProtocolHandler(version string) *MockProtocolHandler {
	return &MockProtocolHandler{
		eventChan:        make(chan events.Event, 10),
		version:          version,
		supportedActions: []string{"BootNotification", "Heartbeat"},
		processMessageFunc: func(ctx context.Context, chargePointID string, message []byte) (interface{}, error) {
			return map[string]interface{}{"status": "Accepted"}, nil
		},
	}
}

func (m *MockProtocolHandler) ProcessMessage(ctx context.Context, chargePointID string, message []byte) (interface{}, error) {
	if m.processMessageFunc != nil {
		return m.processMessageFunc(ctx, chargePointID, message)
	}
	return map[string]interface{}{"status": "Accepted"}, nil
}

func (m *MockProtocolHandler) GetSupportedActions() []string {
	return m.supportedActions
}

func (m *MockProtocolHandler) GetVersion() string {
	return m.version
}

func (m *MockProtocolHandler) Start() error {
	m.started = true
	return nil
}

func (m *MockProtocolHandler) Stop() error {
	m.started = false
	close(m.eventChan)
	return nil
}

func (m *MockProtocolHandler) GetEventChannel() <-chan events.Event {
	return m.eventChan
}

func TestNewDefaultMessageDispatcher(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)
	assert.NotNil(t, dispatcher)
	assert.NotNil(t, dispatcher.config)
	assert.NotNil(t, dispatcher.handlers)
	assert.NotNil(t, dispatcher.eventChan)
	assert.NotNil(t, dispatcher.logger)
	assert.Equal(t, "1.6", dispatcher.config.DefaultProtocolVersion)
}

func TestDefaultMessageDispatcher_RegisterHandler(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)
	handler := NewMockProtocolHandler("1.6")
	
	// 成功注册
	err := dispatcher.RegisterHandler("1.6", handler)
	assert.NoError(t, err)
	
	// 重复注册应该失败
	err = dispatcher.RegisterHandler("1.6", handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
	
	// 空版本应该失败
	err = dispatcher.RegisterHandler("", handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
	
	// nil处理器应该失败
	err = dispatcher.RegisterHandler("2.0", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestDefaultMessageDispatcher_UnregisterHandler(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)
	handler := NewMockProtocolHandler("1.6")
	
	// 注册处理器
	err := dispatcher.RegisterHandler("1.6", handler)
	require.NoError(t, err)
	
	// 成功注销
	err = dispatcher.UnregisterHandler("1.6")
	assert.NoError(t, err)
	
	// 注销不存在的处理器应该失败
	err = dispatcher.UnregisterHandler("1.6")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no handler registered")
}

func TestDefaultMessageDispatcher_GetRegisteredVersions(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)
	
	// 初始状态应该为空
	versions := dispatcher.GetRegisteredVersions()
	assert.Empty(t, versions)
	
	// 注册处理器
	handler16 := NewMockProtocolHandler("1.6")
	handler20 := NewMockProtocolHandler("2.0")
	
	err := dispatcher.RegisterHandler("1.6", handler16)
	require.NoError(t, err)
	err = dispatcher.RegisterHandler("2.0", handler20)
	require.NoError(t, err)
	
	// 检查注册的版本
	versions = dispatcher.GetRegisteredVersions()
	assert.Len(t, versions, 2)
	assert.Contains(t, versions, "1.6")
	assert.Contains(t, versions, "2.0")
}

func TestDefaultMessageDispatcher_GetHandlerForVersion(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)
	handler := NewMockProtocolHandler("1.6")
	
	// 获取不存在的处理器
	_, exists := dispatcher.GetHandlerForVersion("1.6")
	assert.False(t, exists)
	
	// 注册处理器
	err := dispatcher.RegisterHandler("1.6", handler)
	require.NoError(t, err)
	
	// 获取存在的处理器
	retrievedHandler, exists := dispatcher.GetHandlerForVersion("1.6")
	assert.True(t, exists)
	assert.Equal(t, handler, retrievedHandler)
}

func TestDefaultMessageDispatcher_IdentifyProtocolVersion(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)
	
	// 测试版本识别
	version, err := dispatcher.IdentifyProtocolVersion("CP001", []byte(`{"action": "BootNotification"}`))
	assert.NoError(t, err)
	assert.Equal(t, "1.6", version) // 目前总是返回1.6
	
	// 测试禁用版本检测
	config := DefaultDispatcherConfig()
	config.EnableVersionDetection = false
	dispatcher = NewDefaultMessageDispatcher(config)
	
	version, err = dispatcher.IdentifyProtocolVersion("CP001", []byte(`{"action": "BootNotification"}`))
	assert.NoError(t, err)
	assert.Equal(t, "1.6", version)
}

func TestDefaultMessageDispatcher_DispatchMessage(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)
	handler := NewMockProtocolHandler("1.6")

	// 注册处理器
	err := dispatcher.RegisterHandler("1.6", handler)
	require.NoError(t, err)

	// 测试消息分发
	ctx := context.Background()
	response, err := dispatcher.DispatchMessage(ctx, "CP001", "1.6", []byte(`{"action": "BootNotification"}`))
	assert.NoError(t, err)

	expectedResponse := map[string]interface{}{"status": "Accepted"}
	assert.Equal(t, expectedResponse, response)
}

func TestDefaultMessageDispatcher_DispatchMessage_NoHandler(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)
	
	// 测试没有注册处理器的情况
	ctx := context.Background()
	_, err := dispatcher.DispatchMessage(ctx, "CP001", "2.0", []byte(`{"action": "BootNotification"}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no handler registered")
}

func TestDefaultMessageDispatcher_DispatchMessage_AutoDetectVersion(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)
	handler := NewMockProtocolHandler("1.6")

	// 注册处理器
	err := dispatcher.RegisterHandler("1.6", handler)
	require.NoError(t, err)

	// 测试自动版本检测（不指定版本）
	ctx := context.Background()
	response, err := dispatcher.DispatchMessage(ctx, "CP001", "", []byte(`{"action": "BootNotification"}`))
	assert.NoError(t, err)

	expectedResponse := map[string]interface{}{"status": "Accepted"}
	assert.Equal(t, expectedResponse, response)
}

func TestDefaultMessageDispatcher_StartStop(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)
	handler := NewMockProtocolHandler("1.6")

	// 注册处理器
	err := dispatcher.RegisterHandler("1.6", handler)
	require.NoError(t, err)

	// 测试启动
	err = dispatcher.Start()
	assert.NoError(t, err)
	assert.True(t, handler.started)

	// 重复启动应该失败
	err = dispatcher.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	// 测试停止
	err = dispatcher.Stop()
	assert.NoError(t, err)
	assert.False(t, handler.started)

	// 重复停止应该成功
	err = dispatcher.Stop()
	assert.NoError(t, err)
}

func TestDefaultMessageDispatcher_GetStats(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)
	
	// 获取初始统计信息
	stats := dispatcher.GetStats()
	assert.Equal(t, int64(0), stats.TotalMessages)
	assert.Equal(t, int64(0), stats.SuccessfulMessages)
	assert.Equal(t, int64(0), stats.FailedMessages)
	assert.NotNil(t, stats.MessagesByVersion)
	assert.True(t, stats.Uptime >= 0)
}

func TestDefaultMessageDispatcher_GetEventChannel(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil)

	eventChan := dispatcher.GetEventChannel()
	assert.NotNil(t, eventChan)

	// 测试通道是否可读
	select {
	case <-eventChan:
		// 通道可读
	default:
		// 通道为空，这是正常的
	}
}
