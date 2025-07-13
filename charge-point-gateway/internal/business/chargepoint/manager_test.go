package chargepoint

import (
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/domain/ocpp16"
	ocpp16processor "github.com/charging-platform/charge-point-gateway/internal/protocol/ocpp16"
	"github.com/charging-platform/charge-point-gateway/internal/transport/router"
	"github.com/charging-platform/charge-point-gateway/internal/transport/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()
	
	assert.Equal(t, 1000, config.MaxChargePoints)
	assert.Equal(t, 30*time.Second, config.ConnectionTimeout)
	assert.Equal(t, 300*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 60*time.Second, config.RegistrationTimeout)
	assert.Equal(t, 10*time.Second, config.StatusUpdateInterval)
	assert.Equal(t, 30*time.Second, config.ConnectorCheckInterval)
	assert.True(t, config.EnableAutoReconnect)
	assert.Equal(t, 5*time.Second, config.ReconnectDelay)
	assert.Equal(t, 1000, config.EventChannelSize)
	assert.True(t, config.EnableEvents)
	assert.Equal(t, 4, config.WorkerCount)
	assert.Equal(t, 5*time.Minute, config.CleanupInterval)
	assert.True(t, config.EnableMetrics)
	assert.Equal(t, 1*time.Minute, config.StatsInterval)
}

func TestNewManager(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16processor.NewProcessor(ocpp16processor.DefaultProcessorConfig())
	router := router.NewRouter(wsManager, processor, router.DefaultRouterConfig())
	config := DefaultManagerConfig()
	
	manager := NewManager(router, config)
	
	assert.NotNil(t, manager)
	assert.Equal(t, router, manager.router)
	assert.Equal(t, config, manager.config)
	assert.NotNil(t, manager.chargePoints)
	assert.NotNil(t, manager.connectors)
	assert.NotNil(t, manager.eventChan)
	assert.NotNil(t, manager.stats)
	assert.NotNil(t, manager.logger)
}

func TestNewManagerWithNilConfig(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16processor.NewProcessor(ocpp16processor.DefaultProcessorConfig())
	router := router.NewRouter(wsManager, processor, router.DefaultRouterConfig())
	
	manager := NewManager(router, nil)
	
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.config)
	assert.Equal(t, DefaultManagerConfig().MaxChargePoints, manager.config.MaxChargePoints)
}

func TestManager_StartStop(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16processor.NewProcessor(ocpp16processor.DefaultProcessorConfig())
	router := router.NewRouter(wsManager, processor, router.DefaultRouterConfig())
	manager := NewManager(router, DefaultManagerConfig())
	
	// 测试启动
	err := manager.Start()
	assert.NoError(t, err)
	
	// 验证初始统计
	stats := manager.GetStats()
	assert.Equal(t, 0, stats.TotalChargePoints)
	assert.Equal(t, 0, stats.ConnectedChargePoints)
	assert.Equal(t, 0, stats.RegisteredChargePoints)
	assert.Equal(t, 0, stats.TotalConnectors)
	assert.Equal(t, 0, stats.ActiveTransactions)
	
	// 测试停止
	err = manager.Stop()
	assert.NoError(t, err)
}

func TestManager_RegisterChargePoint(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16processor.NewProcessor(ocpp16processor.DefaultProcessorConfig())
	router := router.NewRouter(wsManager, processor, router.DefaultRouterConfig())
	manager := NewManager(router, DefaultManagerConfig())
	
	// 创建BootNotification请求
	serialNumber := "SN123456"
	firmwareVersion := "1.0.0"
	req := &ocpp16.BootNotificationRequest{
		ChargePointVendor:       "TestVendor",
		ChargePointModel:        "TestModel",
		ChargePointSerialNumber: &serialNumber,
		FirmwareVersion:         &firmwareVersion,
	}
	
	chargePointID := "CP001"
	
	// 注册充电桩
	cp, err := manager.RegisterChargePoint(req, chargePointID)
	require.NoError(t, err)
	require.NotNil(t, cp)
	
	assert.Equal(t, chargePointID, cp.ID)
	assert.Equal(t, "TestVendor", cp.Vendor)
	assert.Equal(t, "TestModel", cp.Model)
	assert.Equal(t, "SN123456", cp.SerialNumber)
	assert.Equal(t, "1.0.0", cp.FirmwareVersion)
	assert.Equal(t, "1.6", cp.ProtocolVersion)
	assert.Equal(t, ChargePointStatusRegistered, cp.Status)
	assert.WithinDuration(t, time.Now(), cp.ConnectedAt, time.Second)
	assert.WithinDuration(t, time.Now(), cp.LastSeen, time.Second)
	
	// 验证充电桩已添加到管理器
	retrievedCP, exists := manager.GetChargePoint(chargePointID)
	assert.True(t, exists)
	assert.Equal(t, cp, retrievedCP)
}

func TestManager_RegisterChargePoint_MaxLimit(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16processor.NewProcessor(ocpp16processor.DefaultProcessorConfig())
	router := router.NewRouter(wsManager, processor, router.DefaultRouterConfig())
	
	config := DefaultManagerConfig()
	config.MaxChargePoints = 1 // 设置最大数量为1
	manager := NewManager(router, config)
	
	req := &ocpp16.BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	}
	
	// 注册第一个充电桩应该成功
	_, err := manager.RegisterChargePoint(req, "CP001")
	assert.NoError(t, err)
	
	// 注册第二个充电桩应该失败
	_, err = manager.RegisterChargePoint(req, "CP002")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum charge points limit reached")
}

func TestManager_UpdateChargePointStatus(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16processor.NewProcessor(ocpp16processor.DefaultProcessorConfig())
	router := router.NewRouter(wsManager, processor, router.DefaultRouterConfig())
	manager := NewManager(router, DefaultManagerConfig())
	
	// 先注册一个充电桩
	req := &ocpp16.BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	}
	chargePointID := "CP001"
	_, err := manager.RegisterChargePoint(req, chargePointID)
	require.NoError(t, err)
	
	// 更新状态
	err = manager.UpdateChargePointStatus(chargePointID, ChargePointStatusConnected)
	assert.NoError(t, err)
	
	// 验证状态已更新
	cp, exists := manager.GetChargePoint(chargePointID)
	assert.True(t, exists)
	assert.Equal(t, ChargePointStatusConnected, cp.Status)
	assert.WithinDuration(t, time.Now(), cp.LastSeen, time.Second)
}

func TestManager_UpdateConnectorStatus(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16processor.NewProcessor(ocpp16processor.DefaultProcessorConfig())
	router := router.NewRouter(wsManager, processor, router.DefaultRouterConfig())
	manager := NewManager(router, DefaultManagerConfig())
	
	// 先注册一个充电桩
	req := &ocpp16.BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	}
	chargePointID := "CP001"
	_, err := manager.RegisterChargePoint(req, chargePointID)
	require.NoError(t, err)
	
	// 更新连接器状态
	statusReq := &ocpp16.StatusNotificationRequest{
		ConnectorId: 1,
		ErrorCode:   ocpp16.ChargePointErrorCodeNoError,
		Status:      ocpp16.ChargePointStatusAvailable,
	}
	
	err = manager.UpdateConnectorStatus(statusReq, chargePointID)
	assert.NoError(t, err)
	
	// 验证连接器已创建和更新
	connector, exists := manager.GetConnector(chargePointID, 1)
	assert.True(t, exists)
	assert.Equal(t, 1, connector.ID)
	assert.Equal(t, chargePointID, connector.ChargePointID)
	assert.Equal(t, ConnectorStatusAvailable, connector.Status)
	assert.Equal(t, "NoError", connector.ErrorCode)
	assert.WithinDuration(t, time.Now(), connector.LastStatusUpdate, time.Second)
	
	// 验证充电桩的连接器计数已更新
	cp, _ := manager.GetChargePoint(chargePointID)
	assert.Equal(t, 1, cp.ConnectorCount)
	assert.Contains(t, cp.Connectors, 1)
}

func TestManager_GetEventChannel(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16processor.NewProcessor(ocpp16processor.DefaultProcessorConfig())
	router := router.NewRouter(wsManager, processor, router.DefaultRouterConfig())
	manager := NewManager(router, DefaultManagerConfig())
	
	eventChan := manager.GetEventChannel()
	assert.NotNil(t, eventChan)
	
	// 测试通道类型
	assert.IsType(t, (<-chan events.Event)(nil), eventChan)
}

func TestManager_GetStats(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16processor.NewProcessor(ocpp16processor.DefaultProcessorConfig())
	router := router.NewRouter(wsManager, processor, router.DefaultRouterConfig())
	manager := NewManager(router, DefaultManagerConfig())
	
	stats := manager.GetStats()
	
	assert.Equal(t, 0, stats.TotalChargePoints)
	assert.Equal(t, 0, stats.ConnectedChargePoints)
	assert.Equal(t, 0, stats.RegisteredChargePoints)
	assert.Equal(t, 0, stats.TotalConnectors)
	assert.Equal(t, 0, stats.ActiveTransactions)
	assert.Equal(t, int64(0), stats.TotalTransactions)
	assert.Equal(t, float64(0), stats.TotalEnergy)
	assert.WithinDuration(t, time.Now(), stats.LastResetTime, time.Second)
}

func TestChargePointStatus(t *testing.T) {
	assert.Equal(t, "unknown", string(ChargePointStatusUnknown))
	assert.Equal(t, "connecting", string(ChargePointStatusConnecting))
	assert.Equal(t, "connected", string(ChargePointStatusConnected))
	assert.Equal(t, "registered", string(ChargePointStatusRegistered))
	assert.Equal(t, "disconnected", string(ChargePointStatusDisconnected))
	assert.Equal(t, "faulted", string(ChargePointStatusFaulted))
	assert.Equal(t, "maintenance", string(ChargePointStatusMaintenance))
}

func TestConnectorStatus(t *testing.T) {
	assert.Equal(t, "Available", string(ConnectorStatusAvailable))
	assert.Equal(t, "Preparing", string(ConnectorStatusPreparing))
	assert.Equal(t, "Charging", string(ConnectorStatusCharging))
	assert.Equal(t, "SuspendedEVSE", string(ConnectorStatusSuspendedEVSE))
	assert.Equal(t, "SuspendedEV", string(ConnectorStatusSuspendedEV))
	assert.Equal(t, "Finishing", string(ConnectorStatusFinishing))
	assert.Equal(t, "Reserved", string(ConnectorStatusReserved))
	assert.Equal(t, "Unavailable", string(ConnectorStatusUnavailable))
	assert.Equal(t, "Faulted", string(ConnectorStatusFaulted))
}

func TestTransactionStatus(t *testing.T) {
	assert.Equal(t, "active", string(TransactionStatusActive))
	assert.Equal(t, "completed", string(TransactionStatusCompleted))
	assert.Equal(t, "failed", string(TransactionStatusFailed))
}

// 辅助函数

func setupChargePoint(t *testing.T, manager *Manager, chargePointID string) *ChargePoint {
	req := &ocpp16.BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	}
	
	cp, err := manager.RegisterChargePoint(req, chargePointID)
	require.NoError(t, err)
	return cp
}
