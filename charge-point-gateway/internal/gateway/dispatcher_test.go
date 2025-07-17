package gateway

import (
	"context"
	"testing"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/domain/protocol"
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
	dispatcher := NewDefaultMessageDispatcher(nil, nil)
	assert.NotNil(t, dispatcher)
	assert.NotNil(t, dispatcher.config)
	assert.NotNil(t, dispatcher.handlers)
	assert.NotNil(t, dispatcher.eventChan)
	assert.NotNil(t, dispatcher.logger)
	assert.Equal(t, protocol.OCPP_VERSION_1_6, dispatcher.config.DefaultProtocolVersion)
}

func TestDefaultMessageDispatcher_RegisterHandler(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil, nil)
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
	dispatcher := NewDefaultMessageDispatcher(nil, nil)
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
	dispatcher := NewDefaultMessageDispatcher(nil, nil)

	// 初始状态应该为空
	versions := dispatcher.GetRegisteredVersions()
	assert.Empty(t, versions)

	// 注册处理器
	handler16 := NewMockProtocolHandler(protocol.OCPP_VERSION_1_6)
	handler20 := NewMockProtocolHandler(protocol.OCPP_VERSION_2_0)

	err := dispatcher.RegisterHandler(protocol.OCPP_VERSION_1_6, handler16)
	require.NoError(t, err)
	err = dispatcher.RegisterHandler(protocol.OCPP_VERSION_2_0, handler20)
	require.NoError(t, err)

	// 检查注册的版本
	versions = dispatcher.GetRegisteredVersions()
	assert.Len(t, versions, 2)
	assert.Contains(t, versions, protocol.OCPP_VERSION_1_6)
	assert.Contains(t, versions, protocol.OCPP_VERSION_2_0)
}

func TestDefaultMessageDispatcher_GetHandlerForVersion(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil, nil)
	handler := NewMockProtocolHandler(protocol.OCPP_VERSION_1_6)

	// 获取不存在的处理器
	_, exists := dispatcher.GetHandlerForVersion(protocol.OCPP_VERSION_1_6)
	assert.False(t, exists)

	// 注册处理器
	err := dispatcher.RegisterHandler(protocol.OCPP_VERSION_1_6, handler)
	require.NoError(t, err)

	// 获取存在的处理器
	retrievedHandler, exists := dispatcher.GetHandlerForVersion(protocol.OCPP_VERSION_1_6)
	assert.True(t, exists)
	assert.Equal(t, handler, retrievedHandler)
}

func TestDefaultMessageDispatcher_IdentifyProtocolVersion(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil, nil)

	// 测试版本识别
	version, err := dispatcher.IdentifyProtocolVersion("CP001", []byte(`{"action": "BootNotification"}`))
	assert.NoError(t, err)
	assert.Equal(t, protocol.OCPP_VERSION_1_6, version) // 使用常量

	// 测试禁用版本检测
	config := DefaultDispatcherConfig()
	config.EnableVersionDetection = false
	dispatcher = NewDefaultMessageDispatcher(config, nil)

	version, err = dispatcher.IdentifyProtocolVersion("CP001", []byte(`{"action": "BootNotification"}`))
	assert.NoError(t, err)
	assert.Equal(t, protocol.OCPP_VERSION_1_6, version) // 使用常量
}

func TestDefaultMessageDispatcher_DispatchMessage(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil, nil)
	handler := NewMockProtocolHandler(protocol.OCPP_VERSION_1_6)

	// 注册处理器
	err := dispatcher.RegisterHandler(protocol.OCPP_VERSION_1_6, handler)
	require.NoError(t, err)

	// 测试消息分发
	ctx := context.Background()
	response, err := dispatcher.DispatchMessage(ctx, "CP001", protocol.OCPP_VERSION_1_6, []byte(`{"action": "BootNotification"}`))
	assert.NoError(t, err)

	expectedResponse := map[string]interface{}{"status": "Accepted"}
	assert.Equal(t, expectedResponse, response)
}

func TestDefaultMessageDispatcher_DispatchMessage_NoHandler(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil, nil)

	// 测试没有注册处理器的情况
	ctx := context.Background()
	_, err := dispatcher.DispatchMessage(ctx, "CP001", "2.0", []byte(`{"action": "BootNotification"}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no handler registered")
}

func TestDefaultMessageDispatcher_DispatchMessage_AutoDetectVersion(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil, nil)
	handler := NewMockProtocolHandler(protocol.OCPP_VERSION_1_6)

	// 注册处理器 - 使用常量
	err := dispatcher.RegisterHandler(protocol.OCPP_VERSION_1_6, handler)
	require.NoError(t, err)

	// 测试自动版本检测（不指定版本）
	ctx := context.Background()
	response, err := dispatcher.DispatchMessage(ctx, "CP001", "", []byte(`{"action": "BootNotification"}`))
	assert.NoError(t, err)

	expectedResponse := map[string]interface{}{"status": "Accepted"}
	assert.Equal(t, expectedResponse, response)
}

func TestDefaultMessageDispatcher_VersionMismatch_Fixed(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil, nil)
	handler := NewMockProtocolHandler(protocol.OCPP_VERSION_1_6)

	// 注册处理器时使用常量
	err := dispatcher.RegisterHandler(protocol.OCPP_VERSION_1_6, handler)
	require.NoError(t, err)

	// 测试自动版本检测（不指定版本）- 现在应该成功，因为自动识别返回正确的常量
	ctx := context.Background()
	response, err := dispatcher.DispatchMessage(ctx, "CP001", "", []byte(`{"action": "BootNotification"}`))
	assert.NoError(t, err)

	expectedResponse := map[string]interface{}{"status": "Accepted"}
	assert.Equal(t, expectedResponse, response)
}

// TestProtocolVersionNormalization 测试协议版本规范化
func TestProtocolVersionNormalization(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil, nil)
	handler := NewMockProtocolHandler(protocol.OCPP_VERSION_1_6)

	// 注册处理器时使用标准格式
	err := dispatcher.RegisterHandler(protocol.OCPP_VERSION_1_6, handler)
	require.NoError(t, err)

	ctx := context.Background()
	testCases := []struct {
		name          string
		inputVersion  string
		shouldSucceed bool
		expectedError string
	}{
		{
			name:          "Standard format ocpp1.6",
			inputVersion:  "ocpp1.6",
			shouldSucceed: true,
		},
		{
			name:          "Short format 1.6",
			inputVersion:  "1.6",
			shouldSucceed: true,
		},
		{
			name:          "Uppercase OCPP1.6",
			inputVersion:  "OCPP1.6",
			shouldSucceed: true,
		},
		{
			name:          "Empty version (auto-detect)",
			inputVersion:  "",
			shouldSucceed: true,
		},
		{
			name:          "Unsupported version",
			inputVersion:  "1.5",
			shouldSucceed: false,
			expectedError: "no handler registered for protocol version 1.5",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := dispatcher.DispatchMessage(ctx, "CP001", tc.inputVersion, []byte(`{"action": "BootNotification"}`))

			if tc.shouldSucceed {
				assert.NoError(t, err, "Expected success for version: %s", tc.inputVersion)
				expectedResponse := map[string]interface{}{"status": "Accepted"}
				assert.Equal(t, expectedResponse, response)
			} else {
				assert.Error(t, err, "Expected error for version: %s", tc.inputVersion)
				if tc.expectedError != "" {
					assert.Contains(t, err.Error(), tc.expectedError)
				}
			}
		})
	}
}

// TestWebSocketProtocolVersionIntegration 测试WebSocket协议版本集成
func TestWebSocketProtocolVersionIntegration(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil, nil)
	handler := NewMockProtocolHandler(protocol.OCPP_VERSION_1_6)

	// 注册处理器
	err := dispatcher.RegisterHandler(protocol.OCPP_VERSION_1_6, handler)
	require.NoError(t, err)

	ctx := context.Background()

	// 模拟WebSocket子协议场景
	testCases := []struct {
		name                 string
		webSocketSubprotocol string
		expectedSuccess      bool
	}{
		{
			name:                 "Standard WebSocket subprotocol",
			webSocketSubprotocol: "ocpp1.6",
			expectedSuccess:      true,
		},
		{
			name:                 "Legacy format subprotocol",
			webSocketSubprotocol: "1.6",
			expectedSuccess:      true,
		},
		{
			name:                 "No subprotocol (auto-detect)",
			webSocketSubprotocol: "",
			expectedSuccess:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 模拟从WebSocket连接获取的协议版本
			protocolVersion := tc.webSocketSubprotocol
			if protocolVersion != "" {
				// 模拟协议版本规范化（就像主函数中做的那样）
				protocolVersion = protocol.NormalizeVersion(protocolVersion)
			}

			response, err := dispatcher.DispatchMessage(ctx, "CP001", protocolVersion, []byte(`{"action": "BootNotification"}`))

			if tc.expectedSuccess {
				assert.NoError(t, err, "Expected success for subprotocol: %s", tc.webSocketSubprotocol)
				expectedResponse := map[string]interface{}{"status": "Accepted"}
				assert.Equal(t, expectedResponse, response)
			} else {
				assert.Error(t, err, "Expected error for subprotocol: %s", tc.webSocketSubprotocol)
			}
		})
	}
}

// TestE2EScenarioReproduction 重现E2E测试中的问题
func TestE2EScenarioReproduction(t *testing.T) {
	// 模拟实际应用中的设置
	dispatcher := NewDefaultMessageDispatcher(nil, nil)
	handler := NewMockProtocolHandler(protocol.OCPP_VERSION_1_6)

	// 使用与实际应用相同的注册方式
	err := dispatcher.RegisterHandler(protocol.OCPP_VERSION_1_6, handler)
	require.NoError(t, err)

	ctx := context.Background()

	// 模拟E2E测试中的场景：
	// 1. WebSocket客户端发送 "ocpp1.6" 子协议
	// 2. 服务器应该能够处理消息

	t.Run("E2E_WebSocket_Client_Scenario", func(t *testing.T) {
		// 模拟测试客户端发送的子协议
		clientSubprotocol := "ocpp1.6"

		// 模拟主函数中的协议版本处理逻辑
		protocolVersion := protocol.NormalizeVersion(clientSubprotocol)

		// 发送BootNotification消息（E2E测试中的第一个消息）
		bootNotificationMsg := []byte(`[2,"1","BootNotification",{"chargePointVendor":"Test","chargePointModel":"TestModel"}]`)

		response, err := dispatcher.DispatchMessage(ctx, "CP-001", protocolVersion, bootNotificationMsg)

		assert.NoError(t, err, "BootNotification should succeed")
		assert.NotNil(t, response, "Response should not be nil")

		// 验证处理器被正确调用
		expectedResponse := map[string]interface{}{"status": "Accepted"}
		assert.Equal(t, expectedResponse, response)
	})

	t.Run("E2E_Auto_Detection_Scenario", func(t *testing.T) {
		// 模拟没有子协议的情况（依赖自动检测）
		protocolVersion := ""

		bootNotificationMsg := []byte(`[2,"1","BootNotification",{"chargePointVendor":"Test","chargePointModel":"TestModel"}]`)

		response, err := dispatcher.DispatchMessage(ctx, "CP-001", protocolVersion, bootNotificationMsg)

		assert.NoError(t, err, "Auto-detection should work")
		assert.NotNil(t, response, "Response should not be nil")

		expectedResponse := map[string]interface{}{"status": "Accepted"}
		assert.Equal(t, expectedResponse, response)
	})

	t.Run("E2E_Legacy_Client_Scenario", func(t *testing.T) {
		// 模拟发送 "1.6" 格式的客户端
		clientSubprotocol := "1.6"

		protocolVersion := protocol.NormalizeVersion(clientSubprotocol)

		bootNotificationMsg := []byte(`[2,"1","BootNotification",{"chargePointVendor":"Test","chargePointModel":"TestModel"}]`)

		response, err := dispatcher.DispatchMessage(ctx, "CP-001", protocolVersion, bootNotificationMsg)

		assert.NoError(t, err, "Legacy client should work")
		assert.NotNil(t, response, "Response should not be nil")

		expectedResponse := map[string]interface{}{"status": "Accepted"}
		assert.Equal(t, expectedResponse, response)
	})
}

func TestDefaultMessageDispatcher_StartStop(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil, nil)
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
	dispatcher := NewDefaultMessageDispatcher(nil, nil)

	// 获取初始统计信息
	stats := dispatcher.GetStats()
	assert.Equal(t, int64(0), stats.TotalMessages)
	assert.Equal(t, int64(0), stats.SuccessfulMessages)
	assert.Equal(t, int64(0), stats.FailedMessages)
	assert.NotNil(t, stats.MessagesByVersion)
	assert.True(t, stats.Uptime >= 0)
}

func TestDefaultMessageDispatcher_GetEventChannel(t *testing.T) {
	dispatcher := NewDefaultMessageDispatcher(nil, nil)

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
