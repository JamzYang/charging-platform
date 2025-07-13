package transaction

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/business/chargepoint"
	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/domain/ocpp16"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
)

// Manager 交易管理器
type Manager struct {
	// 核心组件
	chargePointManager *chargepoint.Manager
	
	// 交易存储
	transactions    map[int]*Transaction
	activeTransactions map[string]*Transaction // chargePointID -> transaction
	transactionsByTag  map[string][]*Transaction // idTag -> transactions
	mutex           sync.RWMutex
	
	// 配置
	config *ManagerConfig
	
	// 事件系统
	eventChan chan events.Event
	
	// 统计信息
	stats *ManagerStats
	
	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// 日志器
	logger *logger.Logger
	
	// 交易ID生成器
	nextTransactionID int
	idMutex          sync.Mutex
}

// ManagerConfig 交易管理器配置
type ManagerConfig struct {
	// 交易管理配置
	MaxTransactions       int           `json:"max_transactions"`
	TransactionTimeout    time.Duration `json:"transaction_timeout"`
	MaxTransactionTime    time.Duration `json:"max_transaction_time"`
	IdleTimeout          time.Duration `json:"idle_timeout"`
	
	// 计费配置
	EnableBilling        bool    `json:"enable_billing"`
	DefaultEnergyRate    float64 `json:"default_energy_rate"`
	DefaultTimeRate      float64 `json:"default_time_rate"`
	MinimumCharge        float64 `json:"minimum_charge"`
	
	// 授权配置
	RequireAuthorization bool          `json:"require_authorization"`
	AuthorizationTimeout time.Duration `json:"authorization_timeout"`
	AllowLocalAuth       bool          `json:"allow_local_auth"`
	
	// 事件配置
	EventChannelSize int  `json:"event_channel_size"`
	EnableEvents     bool `json:"enable_events"`
	
	// 性能配置
	WorkerCount       int           `json:"worker_count"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	EnableMetrics     bool          `json:"enable_metrics"`
	StatsInterval     time.Duration `json:"stats_interval"`
}

// DefaultManagerConfig 默认交易管理器配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		MaxTransactions:    10000,
		TransactionTimeout: 30 * time.Second,
		MaxTransactionTime: 24 * time.Hour,
		IdleTimeout:        30 * time.Minute,
		
		EnableBilling:     true,
		DefaultEnergyRate: 0.25, // 0.25元/kWh
		DefaultTimeRate:   0.05, // 0.05元/分钟
		MinimumCharge:     1.0,  // 最低1元
		
		RequireAuthorization: true,
		AuthorizationTimeout: 30 * time.Second,
		AllowLocalAuth:       false,
		
		EventChannelSize: 1000,
		EnableEvents:     true,
		
		WorkerCount:     4,
		CleanupInterval: 5 * time.Minute,
		EnableMetrics:   true,
		StatsInterval:   1 * time.Minute,
	}
}

// Transaction 交易实体
type Transaction struct {
	// 基本信息
	ID            int       `json:"id"`
	ChargePointID string    `json:"charge_point_id"`
	ConnectorID   int       `json:"connector_id"`
	
	// 用户信息
	IdTag         string    `json:"id_tag"`
	ParentIdTag   *string   `json:"parent_id_tag,omitempty"`
	
	// 时间信息
	StartTime     time.Time  `json:"start_time"`
	StopTime      *time.Time `json:"stop_time,omitempty"`
	LastActivity  time.Time  `json:"last_activity"`
	
	// 电量信息
	MeterStart    int     `json:"meter_start"`
	MeterStop     *int    `json:"meter_stop,omitempty"`
	EnergyUsed    float64 `json:"energy_used"` // kWh
	
	// 状态信息
	Status        TransactionStatus `json:"status"`
	StopReason    *string          `json:"stop_reason,omitempty"`
	
	// 计费信息
	BillingInfo   *BillingInfo     `json:"billing_info,omitempty"`
	
	// 授权信息
	AuthInfo      *AuthorizationInfo `json:"auth_info,omitempty"`
	
	// 元数据
	Metadata      map[string]interface{} `json:"metadata"`
	Tags          []string               `json:"tags"`
	
	mutex sync.RWMutex
}

// TransactionStatus 交易状态
type TransactionStatus string

const (
	TransactionStatusPending    TransactionStatus = "pending"
	TransactionStatusAuthorized TransactionStatus = "authorized"
	TransactionStatusActive     TransactionStatus = "active"
	TransactionStatusSuspended  TransactionStatus = "suspended"
	TransactionStatusCompleted  TransactionStatus = "completed"
	TransactionStatusFailed     TransactionStatus = "failed"
	TransactionStatusCancelled  TransactionStatus = "cancelled"
)

// BillingInfo 计费信息
type BillingInfo struct {
	EnergyRate    float64   `json:"energy_rate"`    // 元/kWh
	TimeRate      float64   `json:"time_rate"`      // 元/分钟
	ServiceFee    float64   `json:"service_fee"`    // 服务费
	TotalCost     float64   `json:"total_cost"`     // 总费用
	Currency      string    `json:"currency"`       // 货币单位
	CalculatedAt  time.Time `json:"calculated_at"`  // 计算时间
}

// AuthorizationInfo 授权信息
type AuthorizationInfo struct {
	Status        AuthorizationStatus `json:"status"`
	AuthorizedAt  time.Time          `json:"authorized_at"`
	ExpiresAt     *time.Time         `json:"expires_at,omitempty"`
	AuthMethod    string             `json:"auth_method"`
	AuthSource    string             `json:"auth_source"`
	GroupIdTag    *string            `json:"group_id_tag,omitempty"`
}

// AuthorizationStatus 授权状态
type AuthorizationStatus string

const (
	AuthorizationStatusAccepted     AuthorizationStatus = "Accepted"
	AuthorizationStatusBlocked      AuthorizationStatus = "Blocked"
	AuthorizationStatusExpired      AuthorizationStatus = "Expired"
	AuthorizationStatusInvalid      AuthorizationStatus = "Invalid"
	AuthorizationStatusConcurrentTx AuthorizationStatus = "ConcurrentTx"
)

// ManagerStats 交易管理器统计信息
type ManagerStats struct {
	TotalTransactions     int64     `json:"total_transactions"`
	ActiveTransactions    int       `json:"active_transactions"`
	CompletedTransactions int64     `json:"completed_transactions"`
	FailedTransactions    int64     `json:"failed_transactions"`
	TotalEnergyDelivered  float64   `json:"total_energy_delivered"` // kWh
	TotalRevenue         float64   `json:"total_revenue"`          // 总收入
	AverageTransactionTime float64  `json:"average_transaction_time"` // 分钟
	LastResetTime        time.Time `json:"last_reset_time"`
}

// NewManager 创建新的交易管理器
func NewManager(chargePointManager *chargepoint.Manager, config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// 创建日志器
	l, _ := logger.New(logger.DefaultConfig())
	
	return &Manager{
		chargePointManager: chargePointManager,
		transactions:       make(map[int]*Transaction),
		activeTransactions: make(map[string]*Transaction),
		transactionsByTag:  make(map[string][]*Transaction),
		config:            config,
		eventChan:         make(chan events.Event, config.EventChannelSize),
		stats: &ManagerStats{
			LastResetTime: time.Now(),
		},
		ctx:               ctx,
		cancel:            cancel,
		logger:            l,
		nextTransactionID: 1,
	}
}

// Start 启动交易管理器
func (m *Manager) Start() error {
	m.logger.Info("Starting transaction manager")
	
	// 启动事件处理协程
	if m.config.EnableEvents {
		m.wg.Add(1)
		go m.eventRoutine()
	}
	
	// 启动清理协程
	m.wg.Add(1)
	go m.cleanupRoutine()
	
	// 启动统计协程
	if m.config.EnableMetrics {
		m.wg.Add(1)
		go m.statsRoutine()
	}
	
	// 启动工作协程
	for i := 0; i < m.config.WorkerCount; i++ {
		m.wg.Add(1)
		go m.workerRoutine(i)
	}
	
	m.logger.Infof("Transaction manager started with %d workers", m.config.WorkerCount)
	return nil
}

// Stop 停止交易管理器
func (m *Manager) Stop() error {
	m.logger.Info("Stopping transaction manager")
	
	// 取消上下文
	m.cancel()
	
	// 等待所有协程结束
	m.wg.Wait()
	
	// 关闭事件通道
	close(m.eventChan)
	
	m.logger.Info("Transaction manager stopped")
	return nil
}

// StartTransaction 开始交易
func (m *Manager) StartTransaction(req *StartTransactionRequest) (*Transaction, error) {
	m.logger.Infof("Starting transaction for %s on %s-%d", 
		req.IdTag, req.ChargePointID, req.ConnectorID)
	
	// 验证充电桩和连接器
	if err := m.validateChargePoint(req.ChargePointID, req.ConnectorID); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	
	// 检查是否已有活跃交易
	if err := m.checkActiveTransaction(req.ChargePointID, req.ConnectorID); err != nil {
		return nil, fmt.Errorf("active transaction check failed: %w", err)
	}
	
	// 授权检查
	authInfo, err := m.authorizeTransaction(req.IdTag, req.ChargePointID)
	if err != nil {
		return nil, fmt.Errorf("authorization failed: %w", err)
	}
	
	// 生成交易ID
	transactionID := m.generateTransactionID()
	
	// 创建交易
	transaction := &Transaction{
		ID:            transactionID,
		ChargePointID: req.ChargePointID,
		ConnectorID:   req.ConnectorID,
		IdTag:         req.IdTag,
		ParentIdTag:   req.ParentIdTag,
		StartTime:     time.Now().UTC(),
		LastActivity:  time.Now().UTC(),
		MeterStart:    req.MeterStart,
		Status:        TransactionStatusActive,
		AuthInfo:      authInfo,
		Metadata:      make(map[string]interface{}),
		Tags:          []string{},
	}
	
	// 初始化计费信息
	if m.config.EnableBilling {
		transaction.BillingInfo = &BillingInfo{
			EnergyRate: m.config.DefaultEnergyRate,
			TimeRate:   m.config.DefaultTimeRate,
			Currency:   "CNY",
		}
	}
	
	// 存储交易
	m.mutex.Lock()
	m.transactions[transactionID] = transaction
	m.activeTransactions[fmt.Sprintf("%s-%d", req.ChargePointID, req.ConnectorID)] = transaction
	m.transactionsByTag[req.IdTag] = append(m.transactionsByTag[req.IdTag], transaction)
	m.mutex.Unlock()
	
	// 更新统计
	m.updateStats(func(stats *ManagerStats) {
		stats.TotalTransactions++
		stats.ActiveTransactions++
	})
	
	// 发送事件
	if m.config.EnableEvents {
		m.sendTransactionEvent(events.EventTypeTransactionStarted, transaction)
	}
	
	m.logger.Infof("Transaction %d started successfully", transactionID)
	return transaction, nil
}

// StopTransaction 停止交易
func (m *Manager) StopTransaction(req *StopTransactionRequest) error {
	m.logger.Infof("Stopping transaction %d", req.TransactionID)

	// 查找交易
	m.mutex.RLock()
	transaction, exists := m.transactions[req.TransactionID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("transaction not found: %d", req.TransactionID)
	}

	// 检查交易状态
	transaction.mutex.RLock()
	if transaction.Status != TransactionStatusActive && transaction.Status != TransactionStatusSuspended {
		transaction.mutex.RUnlock()
		return fmt.Errorf("transaction %d is not active (status: %s)", req.TransactionID, transaction.Status)
	}
	transaction.mutex.RUnlock()

	// 更新交易信息
	transaction.mutex.Lock()
	stopTime := time.Now().UTC()
	transaction.StopTime = &stopTime
	transaction.MeterStop = &req.MeterStop
	transaction.EnergyUsed = float64(req.MeterStop-transaction.MeterStart) / 1000.0 // 转换为kWh
	transaction.Status = TransactionStatusCompleted
	if req.Reason != nil {
		reason := string(*req.Reason)
		transaction.StopReason = &reason
	}
	transaction.LastActivity = stopTime

	// 计算费用
	if m.config.EnableBilling && transaction.BillingInfo != nil {
		m.calculateBilling(transaction)
	}
	transaction.mutex.Unlock()

	// 从活跃交易中移除
	m.mutex.Lock()
	activeKey := fmt.Sprintf("%s-%d", transaction.ChargePointID, transaction.ConnectorID)
	delete(m.activeTransactions, activeKey)
	m.mutex.Unlock()

	// 更新统计
	m.updateStats(func(stats *ManagerStats) {
		stats.ActiveTransactions--
		stats.CompletedTransactions++
		stats.TotalEnergyDelivered += transaction.EnergyUsed
		if transaction.BillingInfo != nil {
			stats.TotalRevenue += transaction.BillingInfo.TotalCost
		}

		// 更新平均交易时间
		duration := stopTime.Sub(transaction.StartTime).Minutes()
		if stats.AverageTransactionTime == 0 {
			stats.AverageTransactionTime = duration
		} else {
			stats.AverageTransactionTime = (stats.AverageTransactionTime + duration) / 2
		}
	})

	// 发送事件
	if m.config.EnableEvents {
		m.sendTransactionEvent(events.EventTypeTransactionStopped, transaction)
	}

	m.logger.Infof("Transaction %d stopped successfully, energy: %.2f kWh",
		req.TransactionID, transaction.EnergyUsed)

	return nil
}

// SuspendTransaction 暂停交易
func (m *Manager) SuspendTransaction(transactionID int, reason string) error {
	m.logger.Infof("Suspending transaction %d: %s", transactionID, reason)

	m.mutex.RLock()
	transaction, exists := m.transactions[transactionID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("transaction not found: %d", transactionID)
	}

	transaction.mutex.Lock()
	if transaction.Status != TransactionStatusActive {
		transaction.mutex.Unlock()
		return fmt.Errorf("transaction %d is not active", transactionID)
	}

	transaction.Status = TransactionStatusSuspended
	transaction.LastActivity = time.Now().UTC()
	transaction.mutex.Unlock()

	// 发送事件
	if m.config.EnableEvents {
		m.sendTransactionEvent(events.EventTypeTransactionStarted, transaction)
	}

	return nil
}

// ResumeTransaction 恢复交易
func (m *Manager) ResumeTransaction(transactionID int) error {
	m.logger.Infof("Resuming transaction %d", transactionID)

	m.mutex.RLock()
	transaction, exists := m.transactions[transactionID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("transaction not found: %d", transactionID)
	}

	transaction.mutex.Lock()
	if transaction.Status != TransactionStatusSuspended {
		transaction.mutex.Unlock()
		return fmt.Errorf("transaction %d is not suspended", transactionID)
	}

	transaction.Status = TransactionStatusActive
	transaction.LastActivity = time.Now().UTC()
	transaction.mutex.Unlock()

	// 发送事件
	if m.config.EnableEvents {
		m.sendTransactionEvent(events.EventTypeTransactionStarted, transaction)
	}

	return nil
}

// CancelTransaction 取消交易
func (m *Manager) CancelTransaction(transactionID int, reason string) error {
	m.logger.Infof("Cancelling transaction %d: %s", transactionID, reason)

	m.mutex.RLock()
	transaction, exists := m.transactions[transactionID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("transaction not found: %d", transactionID)
	}

	transaction.mutex.Lock()
	if transaction.Status == TransactionStatusCompleted || transaction.Status == TransactionStatusCancelled {
		transaction.mutex.Unlock()
		return fmt.Errorf("transaction %d is already finished", transactionID)
	}

	stopTime := time.Now().UTC()
	transaction.StopTime = &stopTime
	transaction.Status = TransactionStatusCancelled
	transaction.StopReason = &reason
	transaction.LastActivity = stopTime
	transaction.mutex.Unlock()

	// 从活跃交易中移除
	m.mutex.Lock()
	activeKey := fmt.Sprintf("%s-%d", transaction.ChargePointID, transaction.ConnectorID)
	delete(m.activeTransactions, activeKey)
	m.mutex.Unlock()

	// 更新统计
	m.updateStats(func(stats *ManagerStats) {
		stats.ActiveTransactions--
		stats.FailedTransactions++
	})

	// 发送事件
	if m.config.EnableEvents {
		m.sendTransactionEvent(events.EventTypeTransactionStopped, transaction)
	}

	return nil
}

// GetTransaction 获取交易信息
func (m *Manager) GetTransaction(transactionID int) (*Transaction, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	transaction, exists := m.transactions[transactionID]
	return transaction, exists
}

// GetActiveTransaction 获取活跃交易
func (m *Manager) GetActiveTransaction(chargePointID string, connectorID int) (*Transaction, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	activeKey := fmt.Sprintf("%s-%d", chargePointID, connectorID)
	transaction, exists := m.activeTransactions[activeKey]
	return transaction, exists
}

// GetTransactionsByIdTag 根据IdTag获取交易
func (m *Manager) GetTransactionsByIdTag(idTag string) []*Transaction {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	transactions := m.transactionsByTag[idTag]
	result := make([]*Transaction, len(transactions))
	copy(result, transactions)
	return result
}

// GetActiveTransactions 获取所有活跃交易
func (m *Manager) GetActiveTransactions() []*Transaction {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var transactions []*Transaction
	for _, transaction := range m.activeTransactions {
		transactions = append(transactions, transaction)
	}
	return transactions
}

// GetAllTransactions 获取所有交易
func (m *Manager) GetAllTransactions() []*Transaction {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var transactions []*Transaction
	for _, transaction := range m.transactions {
		transactions = append(transactions, transaction)
	}
	return transactions
}

// GetStats 获取统计信息
func (m *Manager) GetStats() ManagerStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return *m.stats
}

// GetEventChannel 获取事件通道
func (m *Manager) GetEventChannel() <-chan events.Event {
	return m.eventChan
}

// 请求和响应结构体

// StartTransactionRequest 开始交易请求
type StartTransactionRequest struct {
	ChargePointID string  `json:"charge_point_id"`
	ConnectorID   int     `json:"connector_id"`
	IdTag         string  `json:"id_tag"`
	ParentIdTag   *string `json:"parent_id_tag,omitempty"`
	MeterStart    int     `json:"meter_start"`
}

// StopTransactionRequest 停止交易请求
type StopTransactionRequest struct {
	TransactionID int                    `json:"transaction_id"`
	MeterStop     int                    `json:"meter_stop"`
	Reason        *ocpp16.Reason         `json:"reason,omitempty"`
}

// 辅助方法

// validateChargePoint 验证充电桩和连接器
func (m *Manager) validateChargePoint(chargePointID string, connectorID int) error {
	// 检查充电桩是否存在
	cp, exists := m.chargePointManager.GetChargePoint(chargePointID)
	if !exists {
		return fmt.Errorf("charge point not found: %s", chargePointID)
	}

	// 检查充电桩状态
	status := cp.Status

	if status != chargepoint.ChargePointStatusRegistered && status != chargepoint.ChargePointStatusConnected {
		return fmt.Errorf("charge point %s is not available (status: %s)", chargePointID, status)
	}

	// 检查连接器是否存在
	connector, exists := m.chargePointManager.GetConnector(chargePointID, connectorID)
	if !exists {
		return fmt.Errorf("connector not found: %s-%d", chargePointID, connectorID)
	}

	// 检查连接器状态
	connectorStatus := connector.Status

	if connectorStatus != chargepoint.ConnectorStatusAvailable {
		return fmt.Errorf("connector %s-%d is not available (status: %s)",
			chargePointID, connectorID, connectorStatus)
	}

	return nil
}

// checkActiveTransaction 检查是否已有活跃交易
func (m *Manager) checkActiveTransaction(chargePointID string, connectorID int) error {
	activeKey := fmt.Sprintf("%s-%d", chargePointID, connectorID)

	m.mutex.RLock()
	_, exists := m.activeTransactions[activeKey]
	m.mutex.RUnlock()

	if exists {
		return fmt.Errorf("connector %s-%d already has an active transaction", chargePointID, connectorID)
	}

	return nil
}

// authorizeTransaction 授权交易
func (m *Manager) authorizeTransaction(idTag, chargePointID string) (*AuthorizationInfo, error) {
	if !m.config.RequireAuthorization {
		return &AuthorizationInfo{
			Status:       AuthorizationStatusAccepted,
			AuthorizedAt: time.Now().UTC(),
			AuthMethod:   "local",
			AuthSource:   "gateway",
		}, nil
	}

	// 这里应该调用实际的授权服务
	// 简化实现，直接返回授权成功
	authInfo := &AuthorizationInfo{
		Status:       AuthorizationStatusAccepted,
		AuthorizedAt: time.Now().UTC(),
		AuthMethod:   "rfid",
		AuthSource:   "central_system",
	}

	// 设置过期时间（如果需要）
	if m.config.AuthorizationTimeout > 0 {
		expiresAt := time.Now().UTC().Add(m.config.AuthorizationTimeout)
		authInfo.ExpiresAt = &expiresAt
	}

	return authInfo, nil
}

// generateTransactionID 生成交易ID
func (m *Manager) generateTransactionID() int {
	m.idMutex.Lock()
	defer m.idMutex.Unlock()

	id := m.nextTransactionID
	m.nextTransactionID++
	return id
}

// calculateBilling 计算费用
func (m *Manager) calculateBilling(transaction *Transaction) {
	if transaction.BillingInfo == nil {
		return
	}

	// 计算能耗费用
	energyCost := transaction.EnergyUsed * transaction.BillingInfo.EnergyRate

	// 计算时间费用
	duration := time.Now().UTC().Sub(transaction.StartTime).Minutes()
	timeCost := duration * transaction.BillingInfo.TimeRate

	// 计算总费用
	totalCost := energyCost + timeCost + transaction.BillingInfo.ServiceFee

	// 应用最低费用
	if totalCost < m.config.MinimumCharge {
		totalCost = m.config.MinimumCharge
	}

	transaction.BillingInfo.TotalCost = totalCost
	transaction.BillingInfo.CalculatedAt = time.Now().UTC()
}

// updateStats 更新统计信息
func (m *Manager) updateStats(updateFunc func(*ManagerStats)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	updateFunc(m.stats)
}

// sendTransactionEvent 发送交易事件
func (m *Manager) sendTransactionEvent(eventType events.EventType, transaction *Transaction) {
	transaction.mutex.RLock()
	transactionInfo := events.TransactionInfo{
		ID:            transaction.ID,
		ChargePointID: transaction.ChargePointID,
		ConnectorID:   transaction.ConnectorID,
		IdTag:         transaction.IdTag,
		Status:        convertTransactionStatus(transaction.Status),
		StartTime:     transaction.StartTime,
		MeterStart:    transaction.MeterStart,
	}

	if transaction.StopTime != nil {
		transactionInfo.EndTime = transaction.StopTime
	}
	if transaction.MeterStop != nil {
		transactionInfo.MeterStop = transaction.MeterStop
	}
	if transaction.StopReason != nil {
		transactionInfo.StopReason = transaction.StopReason
	}
	transaction.mutex.RUnlock()

	metadata := events.Metadata{
		Source:          "transaction-manager",
		ProtocolVersion: "1.6",
	}

	var event events.Event
	switch eventType {
	case events.EventTypeTransactionStarted:
		authInfo := events.AuthorizationInfo{
			IdTag:  transaction.IdTag,
			Result: events.AuthorizationResultAccepted,
		}
		if transaction.AuthInfo != nil {
			authInfo.Result = convertAuthorizationResult(transaction.AuthInfo.Status)
		}

		event = &events.TransactionStartedEvent{
			BaseEvent:         events.NewBaseEvent(eventType, transaction.ChargePointID, events.EventSeverityInfo, metadata),
			TransactionInfo:   transactionInfo,
			AuthorizationInfo: authInfo,
		}
	case events.EventTypeTransactionStopped:
		event = &events.TransactionStoppedEvent{
			BaseEvent:       events.NewBaseEvent(eventType, transaction.ChargePointID, events.EventSeverityInfo, metadata),
			TransactionInfo: transactionInfo,
		}
	// 注意：TransactionUpdatedEvent在当前events包中不存在，跳过
	// case events.EventTypeTransactionUpdated:
	}

	if event != nil {
		select {
		case m.eventChan <- event:
		default:
			m.logger.Warn("Event channel full, dropping transaction event")
		}
	}
}

// 状态转换函数

// convertTransactionStatus 转换交易状态
func convertTransactionStatus(status TransactionStatus) events.TransactionStatus {
	switch status {
	case TransactionStatusPending:
		return events.TransactionStatusStarting
	case TransactionStatusActive:
		return events.TransactionStatusActive
	case TransactionStatusCompleted:
		return events.TransactionStatusStopped
	case TransactionStatusFailed:
		return events.TransactionStatusFaulted
	case TransactionStatusCancelled:
		return events.TransactionStatusStopped
	default:
		return events.TransactionStatusStarting
	}
}

// convertAuthorizationResult 转换授权结果
func convertAuthorizationResult(status AuthorizationStatus) events.AuthorizationResult {
	switch status {
	case AuthorizationStatusAccepted:
		return events.AuthorizationResultAccepted
	case AuthorizationStatusBlocked:
		return events.AuthorizationResultBlocked
	case AuthorizationStatusExpired:
		return events.AuthorizationResultExpired
	case AuthorizationStatusInvalid:
		return events.AuthorizationResultInvalid
	case AuthorizationStatusConcurrentTx:
		return events.AuthorizationResultConcurrentTx
	default:
		return events.AuthorizationResultInvalid
	}
}

// 协程方法

// eventRoutine 事件处理协程
func (m *Manager) eventRoutine() {
	defer m.wg.Done()

	// 监听充电桩管理器事件
	chargePointEventChan := m.chargePointManager.GetEventChannel()

	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-chargePointEventChan:
			m.handleChargePointEvent(event)
		}
	}
}

// handleChargePointEvent 处理充电桩事件
func (m *Manager) handleChargePointEvent(event events.Event) {
	switch event.GetType() {
	case events.EventTypeChargePointDisconnected:
		if cpEvent, ok := event.(*events.ChargePointDisconnectedEvent); ok {
			m.handleChargePointDisconnected(cpEvent.GetChargePointID())
		}
	case events.EventTypeConnectorStatusChanged:
		if connEvent, ok := event.(*events.ConnectorStatusChangedEvent); ok {
			m.handleConnectorStatusChanged(connEvent)
		}
	}
}

// handleChargePointDisconnected 处理充电桩断开连接
func (m *Manager) handleChargePointDisconnected(chargePointID string) {
	m.logger.Infof("Handling charge point disconnected: %s", chargePointID)

	// 查找该充电桩的所有活跃交易
	var activeTransactions []*Transaction

	m.mutex.RLock()
	for _, transaction := range m.activeTransactions {
		if transaction.ChargePointID == chargePointID {
			activeTransactions = append(activeTransactions, transaction)
		}
	}
	m.mutex.RUnlock()

	// 取消所有活跃交易
	for _, transaction := range activeTransactions {
		reason := "Charge point disconnected"
		if err := m.CancelTransaction(transaction.ID, reason); err != nil {
			m.logger.Errorf("Failed to cancel transaction %d: %v", transaction.ID, err)
		}
	}
}

// handleConnectorStatusChanged 处理连接器状态变化
func (m *Manager) handleConnectorStatusChanged(event *events.ConnectorStatusChangedEvent) {
	chargePointID := event.GetChargePointID()
	connectorID := event.ConnectorInfo.ID
	newStatus := event.ConnectorInfo.Status

	m.logger.Debugf("Handling connector status changed: %s-%d -> %s",
		chargePointID, connectorID, newStatus)

	// 如果连接器变为不可用，取消相关交易
	if newStatus == events.ConnectorStatusUnavailable || newStatus == events.ConnectorStatusFaulted {
		if transaction, exists := m.GetActiveTransaction(chargePointID, connectorID); exists {
			reason := fmt.Sprintf("Connector status changed to %s", newStatus)
			if err := m.CancelTransaction(transaction.ID, reason); err != nil {
				m.logger.Errorf("Failed to cancel transaction %d: %v", transaction.ID, err)
			}
		}
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
			m.cleanupExpiredTransactions()
		}
	}
}

// cleanupExpiredTransactions 清理过期交易
func (m *Manager) cleanupExpiredTransactions() {
	now := time.Now().UTC()
	var expiredTransactions []*Transaction

	m.mutex.RLock()
	for _, transaction := range m.activeTransactions {
		transaction.mutex.RLock()

		// 检查交易是否超时
		if now.Sub(transaction.StartTime) > m.config.MaxTransactionTime {
			expiredTransactions = append(expiredTransactions, transaction)
		} else if now.Sub(transaction.LastActivity) > m.config.IdleTimeout {
			// 检查是否空闲超时
			expiredTransactions = append(expiredTransactions, transaction)
		}

		transaction.mutex.RUnlock()
	}
	m.mutex.RUnlock()

	// 取消过期交易
	for _, transaction := range expiredTransactions {
		reason := "Transaction timeout"
		if err := m.CancelTransaction(transaction.ID, reason); err != nil {
			m.logger.Errorf("Failed to cancel expired transaction %d: %v", transaction.ID, err)
		}
	}

	if len(expiredTransactions) > 0 {
		m.logger.Infof("Cleaned up %d expired transactions", len(expiredTransactions))
	}
}

// statsRoutine 统计协程
func (m *Manager) statsRoutine() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.StatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.logStats()
		}
	}
}

// logStats 记录统计信息
func (m *Manager) logStats() {
	stats := m.GetStats()

	m.logger.Infof("Transaction Stats - Total: %d, Active: %d, Completed: %d, Failed: %d, Energy: %.2f kWh, Revenue: %.2f CNY, Avg Time: %.1f min",
		stats.TotalTransactions,
		stats.ActiveTransactions,
		stats.CompletedTransactions,
		stats.FailedTransactions,
		stats.TotalEnergyDelivered,
		stats.TotalRevenue,
		stats.AverageTransactionTime)
}

// workerRoutine 工作协程
func (m *Manager) workerRoutine(workerID int) {
	defer m.wg.Done()

	m.logger.Debugf("Transaction worker %d started", workerID)

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Debugf("Transaction worker %d stopped", workerID)
			return
		default:
			// 工作协程可以在这里处理队列中的任务
			// 例如：批量计费、数据同步等
			time.Sleep(100 * time.Millisecond)
		}
	}
}
