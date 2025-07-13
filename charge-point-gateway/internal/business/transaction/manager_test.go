package transaction

import (
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/business/chargepoint"
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
	
	assert.Equal(t, 10000, config.MaxTransactions)
	assert.Equal(t, 30*time.Second, config.TransactionTimeout)
	assert.Equal(t, 24*time.Hour, config.MaxTransactionTime)
	assert.Equal(t, 30*time.Minute, config.IdleTimeout)
	assert.True(t, config.EnableBilling)
	assert.Equal(t, 0.25, config.DefaultEnergyRate)
	assert.Equal(t, 0.05, config.DefaultTimeRate)
	assert.Equal(t, 1.0, config.MinimumCharge)
	assert.True(t, config.RequireAuthorization)
	assert.Equal(t, 30*time.Second, config.AuthorizationTimeout)
	assert.False(t, config.AllowLocalAuth)
	assert.Equal(t, 1000, config.EventChannelSize)
	assert.True(t, config.EnableEvents)
	assert.Equal(t, 4, config.WorkerCount)
	assert.Equal(t, 5*time.Minute, config.CleanupInterval)
	assert.True(t, config.EnableMetrics)
	assert.Equal(t, 1*time.Minute, config.StatsInterval)
}

func TestNewManager(t *testing.T) {
	cpManager := createChargePointManager(t)
	config := DefaultManagerConfig()
	
	manager := NewManager(cpManager, config)
	
	assert.NotNil(t, manager)
	assert.Equal(t, cpManager, manager.chargePointManager)
	assert.Equal(t, config, manager.config)
	assert.NotNil(t, manager.transactions)
	assert.NotNil(t, manager.activeTransactions)
	assert.NotNil(t, manager.transactionsByTag)
	assert.NotNil(t, manager.eventChan)
	assert.NotNil(t, manager.stats)
	assert.NotNil(t, manager.logger)
	assert.Equal(t, 1, manager.nextTransactionID)
}

func TestNewManagerWithNilConfig(t *testing.T) {
	cpManager := createChargePointManager(t)
	
	manager := NewManager(cpManager, nil)
	
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.config)
	assert.Equal(t, DefaultManagerConfig().MaxTransactions, manager.config.MaxTransactions)
}

func TestManager_StartStop(t *testing.T) {
	cpManager := createChargePointManager(t)
	manager := NewManager(cpManager, DefaultManagerConfig())
	
	// 测试启动
	err := manager.Start()
	assert.NoError(t, err)
	
	// 验证初始统计
	stats := manager.GetStats()
	assert.Equal(t, int64(0), stats.TotalTransactions)
	assert.Equal(t, 0, stats.ActiveTransactions)
	assert.Equal(t, int64(0), stats.CompletedTransactions)
	assert.Equal(t, int64(0), stats.FailedTransactions)
	assert.Equal(t, float64(0), stats.TotalEnergyDelivered)
	assert.Equal(t, float64(0), stats.TotalRevenue)
	
	// 测试停止
	err = manager.Stop()
	assert.NoError(t, err)
}

func TestManager_StartTransaction(t *testing.T) {
	cpManager := createChargePointManager(t)
	manager := NewManager(cpManager, DefaultManagerConfig())
	
	// 设置充电桩和连接器
	chargePointID := "CP001"
	setupChargePointWithConnector(t, cpManager, chargePointID)
	
	// 开始交易
	req := &StartTransactionRequest{
		ChargePointID: chargePointID,
		ConnectorID:   1,
		IdTag:         "RFID123456",
		MeterStart:    1000,
	}
	
	transaction, err := manager.StartTransaction(req)
	require.NoError(t, err)
	require.NotNil(t, transaction)
	
	assert.Equal(t, 1, transaction.ID)
	assert.Equal(t, chargePointID, transaction.ChargePointID)
	assert.Equal(t, 1, transaction.ConnectorID)
	assert.Equal(t, "RFID123456", transaction.IdTag)
	assert.Equal(t, 1000, transaction.MeterStart)
	assert.Equal(t, TransactionStatusActive, transaction.Status)
	assert.WithinDuration(t, time.Now(), transaction.StartTime, time.Second)
	assert.WithinDuration(t, time.Now(), transaction.LastActivity, time.Second)
	assert.NotNil(t, transaction.AuthInfo)
	assert.NotNil(t, transaction.BillingInfo)
	
	// 验证交易已存储
	retrievedTx, exists := manager.GetTransaction(1)
	assert.True(t, exists)
	assert.Equal(t, transaction, retrievedTx)
	
	// 验证活跃交易
	activeTx, exists := manager.GetActiveTransaction(chargePointID, 1)
	assert.True(t, exists)
	assert.Equal(t, transaction, activeTx)
	
	// 验证按IdTag查询
	txsByTag := manager.GetTransactionsByIdTag("RFID123456")
	assert.Len(t, txsByTag, 1)
	assert.Equal(t, transaction, txsByTag[0])
	
	// 验证统计更新
	stats := manager.GetStats()
	assert.Equal(t, int64(1), stats.TotalTransactions)
	assert.Equal(t, 1, stats.ActiveTransactions)
}

func TestManager_StartTransaction_ValidationErrors(t *testing.T) {
	cpManager := createChargePointManager(t)
	manager := NewManager(cpManager, DefaultManagerConfig())
	
	// 测试充电桩不存在
	req := &StartTransactionRequest{
		ChargePointID: "CP999",
		ConnectorID:   1,
		IdTag:         "RFID123456",
		MeterStart:    1000,
	}
	
	_, err := manager.StartTransaction(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "charge point not found")
	
	// 设置充电桩但不设置连接器
	chargePointID := "CP001"
	setupChargePoint(t, cpManager, chargePointID)
	
	req.ChargePointID = chargePointID
	_, err = manager.StartTransaction(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connector not found")
}

func TestManager_StartTransaction_DuplicateTransaction(t *testing.T) {
	cpManager := createChargePointManager(t)
	manager := NewManager(cpManager, DefaultManagerConfig())
	
	// 设置充电桩和连接器
	chargePointID := "CP001"
	setupChargePointWithConnector(t, cpManager, chargePointID)
	
	// 开始第一个交易
	req := &StartTransactionRequest{
		ChargePointID: chargePointID,
		ConnectorID:   1,
		IdTag:         "RFID123456",
		MeterStart:    1000,
	}
	
	_, err := manager.StartTransaction(req)
	require.NoError(t, err)
	
	// 尝试在同一连接器上开始第二个交易
	_, err = manager.StartTransaction(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already has an active transaction")
}

func TestManager_StopTransaction(t *testing.T) {
	cpManager := createChargePointManager(t)
	manager := NewManager(cpManager, DefaultManagerConfig())
	
	// 先开始一个交易
	chargePointID := "CP001"
	transaction := setupActiveTransaction(t, cpManager, manager, chargePointID)
	
	// 停止交易
	reason := ocpp16.ReasonLocal
	req := &StopTransactionRequest{
		TransactionID: transaction.ID,
		MeterStop:     2000,
		Reason:        &reason,
	}
	
	err := manager.StopTransaction(req)
	assert.NoError(t, err)
	
	// 验证交易状态已更新
	assert.Equal(t, TransactionStatusCompleted, transaction.Status)
	assert.NotNil(t, transaction.StopTime)
	assert.Equal(t, 2000, *transaction.MeterStop)
	assert.Equal(t, "Local", *transaction.StopReason)
	assert.Equal(t, float64(1), transaction.EnergyUsed) // (2000-1000)/1000
	assert.NotNil(t, transaction.BillingInfo)
	assert.Greater(t, transaction.BillingInfo.TotalCost, 0.0)
	
	// 验证不再是活跃交易
	_, exists := manager.GetActiveTransaction(chargePointID, 1)
	assert.False(t, exists)
	
	// 验证统计更新
	stats := manager.GetStats()
	assert.Equal(t, 0, stats.ActiveTransactions)
	assert.Equal(t, int64(1), stats.CompletedTransactions)
	assert.Equal(t, float64(1), stats.TotalEnergyDelivered)
	assert.Greater(t, stats.TotalRevenue, 0.0)
	assert.Greater(t, stats.AverageTransactionTime, 0.0)
}

func TestManager_StopTransaction_NotFound(t *testing.T) {
	cpManager := createChargePointManager(t)
	manager := NewManager(cpManager, DefaultManagerConfig())
	
	req := &StopTransactionRequest{
		TransactionID: 999,
		MeterStop:     2000,
	}
	
	err := manager.StopTransaction(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction not found")
}

func TestManager_SuspendResumeTransaction(t *testing.T) {
	cpManager := createChargePointManager(t)
	manager := NewManager(cpManager, DefaultManagerConfig())
	
	// 先开始一个交易
	chargePointID := "CP001"
	transaction := setupActiveTransaction(t, cpManager, manager, chargePointID)
	
	// 暂停交易
	err := manager.SuspendTransaction(transaction.ID, "User request")
	assert.NoError(t, err)
	assert.Equal(t, TransactionStatusSuspended, transaction.Status)
	
	// 恢复交易
	err = manager.ResumeTransaction(transaction.ID)
	assert.NoError(t, err)
	assert.Equal(t, TransactionStatusActive, transaction.Status)
}

func TestManager_CancelTransaction(t *testing.T) {
	cpManager := createChargePointManager(t)
	manager := NewManager(cpManager, DefaultManagerConfig())
	
	// 先开始一个交易
	chargePointID := "CP001"
	transaction := setupActiveTransaction(t, cpManager, manager, chargePointID)
	
	// 取消交易
	err := manager.CancelTransaction(transaction.ID, "Emergency stop")
	assert.NoError(t, err)
	
	// 验证交易状态
	assert.Equal(t, TransactionStatusCancelled, transaction.Status)
	assert.NotNil(t, transaction.StopTime)
	assert.Equal(t, "Emergency stop", *transaction.StopReason)
	
	// 验证不再是活跃交易
	_, exists := manager.GetActiveTransaction(chargePointID, 1)
	assert.False(t, exists)
	
	// 验证统计更新
	stats := manager.GetStats()
	assert.Equal(t, 0, stats.ActiveTransactions)
	assert.Equal(t, int64(1), stats.FailedTransactions)
}

func TestManager_GetEventChannel(t *testing.T) {
	cpManager := createChargePointManager(t)
	manager := NewManager(cpManager, DefaultManagerConfig())
	
	eventChan := manager.GetEventChannel()
	assert.NotNil(t, eventChan)
	
	// 测试通道类型
	assert.IsType(t, (<-chan events.Event)(nil), eventChan)
}

func TestTransactionStatus(t *testing.T) {
	assert.Equal(t, "pending", string(TransactionStatusPending))
	assert.Equal(t, "authorized", string(TransactionStatusAuthorized))
	assert.Equal(t, "active", string(TransactionStatusActive))
	assert.Equal(t, "suspended", string(TransactionStatusSuspended))
	assert.Equal(t, "completed", string(TransactionStatusCompleted))
	assert.Equal(t, "failed", string(TransactionStatusFailed))
	assert.Equal(t, "cancelled", string(TransactionStatusCancelled))
}

func TestAuthorizationStatus(t *testing.T) {
	assert.Equal(t, "Accepted", string(AuthorizationStatusAccepted))
	assert.Equal(t, "Blocked", string(AuthorizationStatusBlocked))
	assert.Equal(t, "Expired", string(AuthorizationStatusExpired))
	assert.Equal(t, "Invalid", string(AuthorizationStatusInvalid))
	assert.Equal(t, "ConcurrentTx", string(AuthorizationStatusConcurrentTx))
}

// 辅助函数

func createChargePointManager(t *testing.T) *chargepoint.Manager {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16processor.NewProcessor(ocpp16processor.DefaultProcessorConfig())
	router := router.NewRouter(wsManager, processor, router.DefaultRouterConfig())
	return chargepoint.NewManager(router, chargepoint.DefaultManagerConfig())
}

func setupChargePoint(t *testing.T, cpManager *chargepoint.Manager, chargePointID string) {
	req := &ocpp16.BootNotificationRequest{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	}
	
	_, err := cpManager.RegisterChargePoint(req, chargePointID)
	require.NoError(t, err)
}

func setupChargePointWithConnector(t *testing.T, cpManager *chargepoint.Manager, chargePointID string) {
	setupChargePoint(t, cpManager, chargePointID)
	
	statusReq := &ocpp16.StatusNotificationRequest{
		ConnectorId: 1,
		ErrorCode:   ocpp16.ChargePointErrorCodeNoError,
		Status:      ocpp16.ChargePointStatusAvailable,
	}
	
	err := cpManager.UpdateConnectorStatus(statusReq, chargePointID)
	require.NoError(t, err)
}

func setupActiveTransaction(t *testing.T, cpManager *chargepoint.Manager, txManager *Manager, chargePointID string) *Transaction {
	setupChargePointWithConnector(t, cpManager, chargePointID)
	
	req := &StartTransactionRequest{
		ChargePointID: chargePointID,
		ConnectorID:   1,
		IdTag:         "RFID123456",
		MeterStart:    1000,
	}
	
	transaction, err := txManager.StartTransaction(req)
	require.NoError(t, err)
	return transaction
}
