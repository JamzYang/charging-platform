package chargepoint

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/domain/ocpp16"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
	"github.com/charging-platform/charge-point-gateway/internal/transport/router"
)

// Manager 充电桩连接管理器
type Manager struct {
	// 核心组件
	router *router.Router
	
	// 充电桩状态管理
	chargePoints map[string]*ChargePoint
	connectors   map[string]*Connector
	mutex        sync.RWMutex
	
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
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	// 连接管理配置
	MaxChargePoints       int           `json:"max_charge_points"`
	ConnectionTimeout     time.Duration `json:"connection_timeout"`
	HeartbeatInterval     time.Duration `json:"heartbeat_interval"`
	RegistrationTimeout   time.Duration `json:"registration_timeout"`
	
	// 状态管理配置
	StatusUpdateInterval  time.Duration `json:"status_update_interval"`
	ConnectorCheckInterval time.Duration `json:"connector_check_interval"`
	EnableAutoReconnect   bool          `json:"enable_auto_reconnect"`
	ReconnectDelay        time.Duration `json:"reconnect_delay"`
	
	// 事件配置
	EventChannelSize int  `json:"event_channel_size"`
	EnableEvents     bool `json:"enable_events"`
	
	// 性能配置
	WorkerCount       int           `json:"worker_count"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	EnableMetrics     bool          `json:"enable_metrics"`
	StatsInterval     time.Duration `json:"stats_interval"`
}

// DefaultManagerConfig 默认管理器配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		MaxChargePoints:       1000,
		ConnectionTimeout:     30 * time.Second,
		HeartbeatInterval:     300 * time.Second, // 5分钟
		RegistrationTimeout:   60 * time.Second,
		
		StatusUpdateInterval:   10 * time.Second,
		ConnectorCheckInterval: 30 * time.Second,
		EnableAutoReconnect:    true,
		ReconnectDelay:         5 * time.Second,
		
		EventChannelSize: 1000,
		EnableEvents:     true,
		
		WorkerCount:     4,
		CleanupInterval: 5 * time.Minute,
		EnableMetrics:   true,
		StatsInterval:   1 * time.Minute,
	}
}

// ChargePoint 充电桩信息
type ChargePoint struct {
	// 基本信息
	ID              string                    `json:"id"`
	Vendor          string                    `json:"vendor"`
	Model           string                    `json:"model"`
	SerialNumber    string                    `json:"serial_number"`
	FirmwareVersion string                    `json:"firmware_version"`
	
	// 连接状态
	Status          ChargePointStatus         `json:"status"`
	ConnectedAt     time.Time                 `json:"connected_at"`
	LastSeen        time.Time                 `json:"last_seen"`
	LastHeartbeat   time.Time                 `json:"last_heartbeat"`
	
	// 配置信息
	ProtocolVersion string                    `json:"protocol_version"`
	ConnectorCount  int                       `json:"connector_count"`
	Connectors      map[int]*Connector        `json:"connectors"`
	
	// 运行时信息
	CurrentTransactions map[int]*Transaction  `json:"current_transactions"`
	TotalEnergy        float64               `json:"total_energy"`
	TotalTransactions  int64                 `json:"total_transactions"`
	
	// 元数据
	Metadata        map[string]interface{}    `json:"metadata"`
	Tags            []string                  `json:"tags"`
	Location        *Location                 `json:"location,omitempty"`
	
	mutex sync.RWMutex
}

// ChargePointStatus 充电桩状态
type ChargePointStatus string

const (
	ChargePointStatusUnknown      ChargePointStatus = "unknown"
	ChargePointStatusConnecting   ChargePointStatus = "connecting"
	ChargePointStatusConnected    ChargePointStatus = "connected"
	ChargePointStatusRegistered   ChargePointStatus = "registered"
	ChargePointStatusDisconnected ChargePointStatus = "disconnected"
	ChargePointStatusFaulted      ChargePointStatus = "faulted"
	ChargePointStatusMaintenance  ChargePointStatus = "maintenance"
)

// Connector 连接器信息
type Connector struct {
	// 基本信息
	ID            int                    `json:"id"`
	ChargePointID string                 `json:"charge_point_id"`
	
	// 状态信息
	Status        ConnectorStatus        `json:"status"`
	ErrorCode     string                 `json:"error_code,omitempty"`
	Info          string                 `json:"info,omitempty"`
	VendorId      string                 `json:"vendor_id,omitempty"`
	VendorErrorCode string               `json:"vendor_error_code,omitempty"`
	
	// 当前交易
	CurrentTransaction *Transaction        `json:"current_transaction,omitempty"`
	
	// 统计信息
	TotalEnergy       float64             `json:"total_energy"`
	TotalTransactions int64               `json:"total_transactions"`
	LastStatusUpdate  time.Time           `json:"last_status_update"`
	
	// 配置信息
	MaxPower          float64             `json:"max_power"`
	ConnectorType     string              `json:"connector_type"`
	
	mutex sync.RWMutex
}

// ConnectorStatus 连接器状态
type ConnectorStatus string

const (
	ConnectorStatusAvailable     ConnectorStatus = "Available"
	ConnectorStatusPreparing     ConnectorStatus = "Preparing"
	ConnectorStatusCharging      ConnectorStatus = "Charging"
	ConnectorStatusSuspendedEVSE ConnectorStatus = "SuspendedEVSE"
	ConnectorStatusSuspendedEV   ConnectorStatus = "SuspendedEV"
	ConnectorStatusFinishing     ConnectorStatus = "Finishing"
	ConnectorStatusReserved      ConnectorStatus = "Reserved"
	ConnectorStatusUnavailable   ConnectorStatus = "Unavailable"
	ConnectorStatusFaulted       ConnectorStatus = "Faulted"
)

// Transaction 交易信息
type Transaction struct {
	// 基本信息
	ID            int       `json:"id"`
	ChargePointID string    `json:"charge_point_id"`
	ConnectorID   int       `json:"connector_id"`
	
	// 用户信息
	IdTag         string    `json:"id_tag"`
	
	// 时间信息
	StartTime     time.Time `json:"start_time"`
	StopTime      *time.Time `json:"stop_time,omitempty"`
	
	// 电量信息
	MeterStart    int       `json:"meter_start"`
	MeterStop     *int      `json:"meter_stop,omitempty"`
	
	// 状态信息
	Status        TransactionStatus `json:"status"`
	StopReason    string    `json:"stop_reason,omitempty"`
	
	mutex sync.RWMutex
}

// TransactionStatus 交易状态
type TransactionStatus string

const (
	TransactionStatusActive    TransactionStatus = "active"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
)

// Location 位置信息
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Address   string  `json:"address,omitempty"`
}

// ManagerStats 管理器统计信息
type ManagerStats struct {
	TotalChargePoints     int       `json:"total_charge_points"`
	ConnectedChargePoints int       `json:"connected_charge_points"`
	RegisteredChargePoints int      `json:"registered_charge_points"`
	TotalConnectors       int       `json:"total_connectors"`
	ActiveTransactions    int       `json:"active_transactions"`
	TotalTransactions     int64     `json:"total_transactions"`
	TotalEnergy          float64   `json:"total_energy"`
	LastResetTime        time.Time `json:"last_reset_time"`
}

// NewManager 创建新的充电桩管理器
func NewManager(router *router.Router, config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// 创建日志器
	l, _ := logger.New(logger.DefaultConfig())
	
	return &Manager{
		router:       router,
		chargePoints: make(map[string]*ChargePoint),
		connectors:   make(map[string]*Connector),
		config:       config,
		eventChan:    make(chan events.Event, config.EventChannelSize),
		stats: &ManagerStats{
			LastResetTime: time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
		logger: l,
	}
}

// Start 启动充电桩管理器
func (m *Manager) Start() error {
	m.logger.Info("Starting charge point manager")
	
	// 启动事件处理协程
	if m.config.EnableEvents {
		m.wg.Add(1)
		go m.eventRoutine()
	}
	
	// 启动状态检查协程
	m.wg.Add(1)
	go m.statusCheckRoutine()
	
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
	
	m.logger.Infof("Charge point manager started with %d workers", m.config.WorkerCount)
	return nil
}

// Stop 停止充电桩管理器
func (m *Manager) Stop() error {
	m.logger.Info("Stopping charge point manager")
	
	// 取消上下文
	m.cancel()
	
	// 等待所有协程结束
	m.wg.Wait()
	
	// 关闭事件通道
	close(m.eventChan)
	
	m.logger.Info("Charge point manager stopped")
	return nil
}

// RegisterChargePoint 注册充电桩
func (m *Manager) RegisterChargePoint(req *ocpp16.BootNotificationRequest, chargePointID string) (*ChargePoint, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// 检查是否超过最大数量
	if len(m.chargePoints) >= m.config.MaxChargePoints {
		return nil, fmt.Errorf("maximum charge points limit reached: %d", m.config.MaxChargePoints)
	}
	
	// 创建或更新充电桩
	cp, exists := m.chargePoints[chargePointID]
	if !exists {
		cp = &ChargePoint{
			ID:                  chargePointID,
			Status:              ChargePointStatusConnecting,
			ConnectedAt:         time.Now(),
			Connectors:          make(map[int]*Connector),
			CurrentTransactions: make(map[int]*Transaction),
			Metadata:            make(map[string]interface{}),
			Tags:                []string{},
		}
		m.chargePoints[chargePointID] = cp
	}
	
	// 更新充电桩信息
	cp.mutex.Lock()
	cp.Vendor = req.ChargePointVendor
	cp.Model = req.ChargePointModel
	if req.ChargePointSerialNumber != nil {
		cp.SerialNumber = *req.ChargePointSerialNumber
	}
	if req.FirmwareVersion != nil {
		cp.FirmwareVersion = *req.FirmwareVersion
	}
	cp.ProtocolVersion = "1.6"
	cp.Status = ChargePointStatusRegistered
	cp.LastSeen = time.Now()
	cp.mutex.Unlock()
	
	m.logger.Infof("Charge point registered: %s (%s %s)", chargePointID, cp.Vendor, cp.Model)
	
	// 发送事件
	if m.config.EnableEvents {
		m.sendChargePointEvent(events.EventTypeChargePointConnected, cp)
	}
	
	return cp, nil
}

// UpdateChargePointStatus 更新充电桩状态
func (m *Manager) UpdateChargePointStatus(chargePointID string, status ChargePointStatus) error {
	m.mutex.RLock()
	cp, exists := m.chargePoints[chargePointID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("charge point not found: %s", chargePointID)
	}

	cp.mutex.Lock()
	oldStatus := cp.Status
	cp.Status = status
	cp.LastSeen = time.Now()
	cp.mutex.Unlock()

	m.logger.Infof("Charge point %s status changed: %s -> %s", chargePointID, oldStatus, status)

	// 发送事件
	if m.config.EnableEvents && oldStatus != status {
		m.sendChargePointEvent(events.EventTypeChargePointStatusChanged, cp)
	}

	return nil
}

// UpdateConnectorStatus 更新连接器状态
func (m *Manager) UpdateConnectorStatus(req *ocpp16.StatusNotificationRequest, chargePointID string) error {
	m.mutex.RLock()
	cp, exists := m.chargePoints[chargePointID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("charge point not found: %s", chargePointID)
	}

	connectorKey := fmt.Sprintf("%s-%d", chargePointID, req.ConnectorId)

	// 获取或创建连接器
	connector, exists := m.connectors[connectorKey]
	if !exists {
		connector = &Connector{
			ID:            req.ConnectorId,
			ChargePointID: chargePointID,
			Status:        ConnectorStatus(req.Status),
		}
		m.connectors[connectorKey] = connector

		// 添加到充电桩的连接器列表
		cp.mutex.Lock()
		cp.Connectors[req.ConnectorId] = connector
		if req.ConnectorId > cp.ConnectorCount {
			cp.ConnectorCount = req.ConnectorId
		}
		cp.mutex.Unlock()
	}

	// 更新连接器状态
	connector.mutex.Lock()
	oldStatus := connector.Status
	connector.Status = ConnectorStatus(req.Status)
	connector.ErrorCode = string(req.ErrorCode)
	if req.Info != nil {
		connector.Info = *req.Info
	}
	if req.VendorId != nil {
		connector.VendorId = *req.VendorId
	}
	if req.VendorErrorCode != nil {
		connector.VendorErrorCode = *req.VendorErrorCode
	}
	connector.LastStatusUpdate = time.Now()
	connector.mutex.Unlock()

	m.logger.Infof("Connector %s-%d status changed: %s -> %s",
		chargePointID, req.ConnectorId, oldStatus, connector.Status)

	// 发送事件
	if m.config.EnableEvents && oldStatus != connector.Status {
		m.sendConnectorEvent(events.EventTypeConnectorStatusChanged, connector)
	}

	return nil
}

// StartTransaction 开始交易
func (m *Manager) StartTransaction(req *ocpp16.StartTransactionRequest, chargePointID string) (*Transaction, error) {
	m.mutex.RLock()
	cp, exists := m.chargePoints[chargePointID]
	m.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("charge point not found: %s", chargePointID)
	}

	connectorKey := fmt.Sprintf("%s-%d", chargePointID, req.ConnectorId)
	connector, exists := m.connectors[connectorKey]
	if !exists {
		return nil, fmt.Errorf("connector not found: %s-%d", chargePointID, req.ConnectorId)
	}

	// 检查连接器状态
	connector.mutex.RLock()
	if connector.Status != ConnectorStatusAvailable && connector.Status != ConnectorStatusPreparing {
		connector.mutex.RUnlock()
		return nil, fmt.Errorf("connector not available for transaction: %s", connector.Status)
	}
	connector.mutex.RUnlock()

	// 生成交易ID
	transactionID := int(time.Now().Unix())

	// 创建交易
	transaction := &Transaction{
		ID:            transactionID,
		ChargePointID: chargePointID,
		ConnectorID:   req.ConnectorId,
		IdTag:         req.IdTag,
		StartTime:     req.Timestamp.Time,
		MeterStart:    req.MeterStart,
		Status:        TransactionStatusActive,
	}

	// 更新连接器
	connector.mutex.Lock()
	connector.CurrentTransaction = transaction
	connector.Status = ConnectorStatusCharging
	connector.TotalTransactions++
	connector.mutex.Unlock()

	// 更新充电桩
	cp.mutex.Lock()
	cp.CurrentTransactions[req.ConnectorId] = transaction
	cp.TotalTransactions++
	cp.mutex.Unlock()

	m.logger.Infof("Transaction started: %d on %s-%d by %s",
		transactionID, chargePointID, req.ConnectorId, req.IdTag)

	// 发送事件
	if m.config.EnableEvents {
		m.sendTransactionEvent(events.EventTypeTransactionStarted, transaction)
	}

	return transaction, nil
}

// StopTransaction 停止交易
func (m *Manager) StopTransaction(req *ocpp16.StopTransactionRequest, chargePointID string) error {
	// 查找交易
	var transaction *Transaction
	var connector *Connector

	m.mutex.RLock()
	cp, exists := m.chargePoints[chargePointID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("charge point not found: %s", chargePointID)
	}

	// 查找包含该交易的连接器
	cp.mutex.RLock()
	for connectorID, trans := range cp.CurrentTransactions {
		if trans.ID == req.TransactionId {
			transaction = trans
			connectorKey := fmt.Sprintf("%s-%d", chargePointID, connectorID)
			connector = m.connectors[connectorKey]
			break
		}
	}
	cp.mutex.RUnlock()

	if transaction == nil {
		return fmt.Errorf("transaction not found: %d", req.TransactionId)
	}

	// 更新交易
	transaction.mutex.Lock()
	stopTime := req.Timestamp.Time
	transaction.StopTime = &stopTime
	transaction.MeterStop = &req.MeterStop
	transaction.Status = TransactionStatusCompleted
	if req.Reason != nil {
		transaction.StopReason = string(*req.Reason)
	}
	transaction.mutex.Unlock()

	// 更新连接器
	if connector != nil {
		connector.mutex.Lock()
		connector.CurrentTransaction = nil
		connector.Status = ConnectorStatusAvailable
		energyUsed := float64(req.MeterStop - transaction.MeterStart)
		connector.TotalEnergy += energyUsed
		connector.mutex.Unlock()
	}

	// 更新充电桩
	cp.mutex.Lock()
	delete(cp.CurrentTransactions, transaction.ConnectorID)
	energyUsed := float64(req.MeterStop - transaction.MeterStart)
	cp.TotalEnergy += energyUsed
	cp.mutex.Unlock()

	m.logger.Infof("Transaction stopped: %d on %s, energy: %.2f kWh",
		req.TransactionId, chargePointID, energyUsed/1000)

	// 发送事件
	if m.config.EnableEvents {
		m.sendTransactionEvent(events.EventTypeTransactionStopped, transaction)
	}

	return nil
}

// UpdateHeartbeat 更新心跳
func (m *Manager) UpdateHeartbeat(chargePointID string) error {
	m.mutex.RLock()
	cp, exists := m.chargePoints[chargePointID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("charge point not found: %s", chargePointID)
	}

	cp.mutex.Lock()
	cp.LastHeartbeat = time.Now()
	cp.LastSeen = time.Now()
	cp.mutex.Unlock()

	m.logger.Debugf("Heartbeat received from %s", chargePointID)
	return nil
}

// DisconnectChargePoint 断开充电桩连接
func (m *Manager) DisconnectChargePoint(chargePointID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	cp, exists := m.chargePoints[chargePointID]
	if !exists {
		return fmt.Errorf("charge point not found: %s", chargePointID)
	}

	// 更新状态
	cp.mutex.Lock()
	cp.Status = ChargePointStatusDisconnected
	cp.mutex.Unlock()

	// 清理连接器
	for connectorID := range cp.Connectors {
		connectorKey := fmt.Sprintf("%s-%d", chargePointID, connectorID)
		if connector, exists := m.connectors[connectorKey]; exists {
			connector.mutex.Lock()
			connector.Status = ConnectorStatusUnavailable
			connector.mutex.Unlock()
		}
	}

	m.logger.Infof("Charge point disconnected: %s", chargePointID)

	// 发送事件
	if m.config.EnableEvents {
		m.sendChargePointEvent(events.EventTypeChargePointDisconnected, cp)
	}

	return nil
}

// GetChargePoint 获取充电桩信息
func (m *Manager) GetChargePoint(chargePointID string) (*ChargePoint, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	cp, exists := m.chargePoints[chargePointID]
	return cp, exists
}

// GetAllChargePoints 获取所有充电桩
func (m *Manager) GetAllChargePoints() map[string]*ChargePoint {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*ChargePoint)
	for id, cp := range m.chargePoints {
		result[id] = cp
	}
	return result
}

// GetConnector 获取连接器信息
func (m *Manager) GetConnector(chargePointID string, connectorID int) (*Connector, bool) {
	connectorKey := fmt.Sprintf("%s-%d", chargePointID, connectorID)
	connector, exists := m.connectors[connectorKey]
	return connector, exists
}

// GetActiveTransactions 获取活跃交易
func (m *Manager) GetActiveTransactions() []*Transaction {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var transactions []*Transaction
	for _, cp := range m.chargePoints {
		cp.mutex.RLock()
		for _, trans := range cp.CurrentTransactions {
			if trans.Status == TransactionStatusActive {
				transactions = append(transactions, trans)
			}
		}
		cp.mutex.RUnlock()
	}
	return transactions
}

// GetStats 获取统计信息
func (m *Manager) GetStats() ManagerStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := ManagerStats{
		TotalChargePoints: len(m.chargePoints),
		TotalConnectors:   len(m.connectors),
		LastResetTime:     m.stats.LastResetTime,
	}

	var totalEnergy float64
	var totalTransactions int64
	var activeTransactions int
	var connectedCount int
	var registeredCount int

	for _, cp := range m.chargePoints {
		cp.mutex.RLock()
		totalEnergy += cp.TotalEnergy
		totalTransactions += cp.TotalTransactions
		activeTransactions += len(cp.CurrentTransactions)

		switch cp.Status {
		case ChargePointStatusConnected, ChargePointStatusRegistered:
			connectedCount++
			if cp.Status == ChargePointStatusRegistered {
				registeredCount++
			}
		}
		cp.mutex.RUnlock()
	}

	stats.ConnectedChargePoints = connectedCount
	stats.RegisteredChargePoints = registeredCount
	stats.ActiveTransactions = activeTransactions
	stats.TotalTransactions = totalTransactions
	stats.TotalEnergy = totalEnergy

	return stats
}

// GetEventChannel 获取事件通道
func (m *Manager) GetEventChannel() <-chan events.Event {
	return m.eventChan
}

// 协程方法

// eventRoutine 事件处理协程
func (m *Manager) eventRoutine() {
	defer m.wg.Done()

	// 监听路由器事件
	routerEventChan := m.router.GetEventChannel()

	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-routerEventChan:
			m.handleRouterEvent(event)
		}
	}
}

// handleRouterEvent 处理路由器事件
func (m *Manager) handleRouterEvent(event events.Event) {
	switch event.GetType() {
	case events.EventTypeChargePointConnected:
		if cpEvent, ok := event.(*events.ChargePointConnectedEvent); ok {
			m.handleChargePointConnectedEvent(cpEvent)
		}
	case events.EventTypeChargePointDisconnected:
		if cpEvent, ok := event.(*events.ChargePointDisconnectedEvent); ok {
			m.handleChargePointDisconnectedEvent(cpEvent)
		}
	case events.EventTypeConnectorStatusChanged:
		if connEvent, ok := event.(*events.ConnectorStatusChangedEvent); ok {
			m.handleConnectorStatusChangedEvent(connEvent)
		}
	}
}

// handleChargePointConnectedEvent 处理充电桩连接事件
func (m *Manager) handleChargePointConnectedEvent(event *events.ChargePointConnectedEvent) {
	m.logger.Debugf("Handling charge point connected event: %s", event.ChargePointID)

	// 更新连接状态
	if err := m.UpdateChargePointStatus(event.ChargePointID, ChargePointStatusConnected); err != nil {
		m.logger.Errorf("Failed to update charge point status: %v", err)
	}
}

// handleChargePointDisconnectedEvent 处理充电桩断开事件
func (m *Manager) handleChargePointDisconnectedEvent(event *events.ChargePointDisconnectedEvent) {
	m.logger.Debugf("Handling charge point disconnected event: %s", event.ChargePointID)

	// 断开充电桩
	if err := m.DisconnectChargePoint(event.ChargePointID); err != nil {
		m.logger.Errorf("Failed to disconnect charge point: %v", err)
	}
}

// handleConnectorStatusChangedEvent 处理连接器状态变化事件
func (m *Manager) handleConnectorStatusChangedEvent(event *events.ConnectorStatusChangedEvent) {
	m.logger.Debugf("Handling connector status changed event: %s-%d",
		event.ChargePointID, event.ConnectorInfo.ID)

	// 这里可以添加额外的业务逻辑
}

// statusCheckRoutine 状态检查协程
func (m *Manager) statusCheckRoutine() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.StatusUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkChargePointStatus()
		}
	}
}

// checkChargePointStatus 检查充电桩状态
func (m *Manager) checkChargePointStatus() {
	now := time.Now()

	m.mutex.RLock()
	chargePoints := make([]*ChargePoint, 0, len(m.chargePoints))
	for _, cp := range m.chargePoints {
		chargePoints = append(chargePoints, cp)
	}
	m.mutex.RUnlock()

	for _, cp := range chargePoints {
		cp.mutex.RLock()
		lastSeen := cp.LastSeen
		status := cp.Status
		chargePointID := cp.ID
		cp.mutex.RUnlock()

		// 检查是否超时
		if status == ChargePointStatusConnected || status == ChargePointStatusRegistered {
			if now.Sub(lastSeen) > m.config.ConnectionTimeout {
				m.logger.Warnf("Charge point %s connection timeout", chargePointID)
				m.UpdateChargePointStatus(chargePointID, ChargePointStatusDisconnected)
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
			m.cleanupDisconnectedChargePoints()
		}
	}
}

// cleanupDisconnectedChargePoints 清理断开连接的充电桩
func (m *Manager) cleanupDisconnectedChargePoints() {
	now := time.Now()
	var toRemove []string

	m.mutex.RLock()
	for id, cp := range m.chargePoints {
		cp.mutex.RLock()
		if cp.Status == ChargePointStatusDisconnected &&
		   now.Sub(cp.LastSeen) > m.config.CleanupInterval {
			toRemove = append(toRemove, id)
		}
		cp.mutex.RUnlock()
	}
	m.mutex.RUnlock()

	if len(toRemove) > 0 {
		m.mutex.Lock()
		for _, id := range toRemove {
			// 清理连接器
			if cp, exists := m.chargePoints[id]; exists {
				for connectorID := range cp.Connectors {
					connectorKey := fmt.Sprintf("%s-%d", id, connectorID)
					delete(m.connectors, connectorKey)
				}
			}
			delete(m.chargePoints, id)
		}
		m.mutex.Unlock()

		m.logger.Infof("Cleaned up %d disconnected charge points", len(toRemove))
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

	m.logger.Infof("ChargePoint Stats - Total: %d, Connected: %d, Registered: %d, Connectors: %d, Active Transactions: %d, Total Energy: %.2f kWh",
		stats.TotalChargePoints,
		stats.ConnectedChargePoints,
		stats.RegisteredChargePoints,
		stats.TotalConnectors,
		stats.ActiveTransactions,
		stats.TotalEnergy/1000)
}

// workerRoutine 工作协程
func (m *Manager) workerRoutine(workerID int) {
	defer m.wg.Done()

	m.logger.Debugf("ChargePoint worker %d started", workerID)

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Debugf("ChargePoint worker %d stopped", workerID)
			return
		default:
			// 工作协程可以在这里处理队列中的任务
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// 事件发送方法

// sendChargePointEvent 发送充电桩事件
func (m *Manager) sendChargePointEvent(eventType events.EventType, cp *ChargePoint) {
	cp.mutex.RLock()
	chargePointInfo := events.ChargePointInfo{
		ID:              cp.ID,
		Vendor:          cp.Vendor,
		Model:           cp.Model,
		SerialNumber:    &cp.SerialNumber,
		FirmwareVersion: &cp.FirmwareVersion,
		LastSeen:        cp.LastSeen,
		ProtocolVersion: cp.ProtocolVersion,
	}
	cp.mutex.RUnlock()

	metadata := events.Metadata{
		Source:          "chargepoint-manager",
		ProtocolVersion: "1.6",
	}

	var event events.Event
	switch eventType {
	case events.EventTypeChargePointConnected:
		event = &events.ChargePointConnectedEvent{
			BaseEvent:       events.NewBaseEvent(eventType, cp.ID, events.EventSeverityInfo, metadata),
			ChargePointInfo: chargePointInfo,
		}
	case events.EventTypeChargePointDisconnected:
		event = &events.ChargePointDisconnectedEvent{
			BaseEvent: events.NewBaseEvent(eventType, cp.ID, events.EventSeverityInfo, metadata),
			Reason:    "Connection lost",
		}
	// 注意：ChargePointStatusChangedEvent在当前events包中不存在，跳过
	// case events.EventTypeChargePointStatusChanged:
	}

	if event != nil {
		select {
		case m.eventChan <- event:
		default:
			m.logger.Warn("Event channel full, dropping charge point event")
		}
	}
}

// sendConnectorEvent 发送连接器事件
func (m *Manager) sendConnectorEvent(eventType events.EventType, connector *Connector) {
	connector.mutex.RLock()
	connectorInfo := events.ConnectorInfo{
		ID:            connector.ID,
		ChargePointID: connector.ChargePointID,
		Status:        convertConnectorStatus(connector.Status),
		ErrorCode:     &connector.ErrorCode,
	}
	connector.mutex.RUnlock()

	metadata := events.Metadata{
		Source:          "chargepoint-manager",
		ProtocolVersion: "1.6",
	}

	event := &events.ConnectorStatusChangedEvent{
		BaseEvent:      events.NewBaseEvent(eventType, connector.ChargePointID, events.EventSeverityInfo, metadata),
		ConnectorInfo:  connectorInfo,
		PreviousStatus: events.ConnectorStatusUnavailable, // 简化处理
	}

	select {
	case m.eventChan <- event:
	default:
		m.logger.Warn("Event channel full, dropping connector event")
	}
}

// sendTransactionEvent 发送交易事件
func (m *Manager) sendTransactionEvent(eventType events.EventType, transaction *Transaction) {
	transaction.mutex.RLock()
	transactionInfo := events.TransactionInfo{
		ID:            transaction.ID,
		ChargePointID: transaction.ChargePointID,
		ConnectorID:   transaction.ConnectorID,
		IdTag:         transaction.IdTag,
		StartTime:     transaction.StartTime,
		MeterStart:    transaction.MeterStart,
	}
	if transaction.StopTime != nil {
		transactionInfo.EndTime = transaction.StopTime
	}
	if transaction.MeterStop != nil {
		transactionInfo.MeterStop = transaction.MeterStop
	}
	transaction.mutex.RUnlock()

	metadata := events.Metadata{
		Source:          "chargepoint-manager",
		ProtocolVersion: "1.6",
	}

	var event events.Event
	switch eventType {
	case events.EventTypeTransactionStarted:
		event = &events.TransactionStartedEvent{
			BaseEvent:       events.NewBaseEvent(eventType, transaction.ChargePointID, events.EventSeverityInfo, metadata),
			TransactionInfo: transactionInfo,
		}
	case events.EventTypeTransactionStopped:
		event = &events.TransactionStoppedEvent{
			BaseEvent:       events.NewBaseEvent(eventType, transaction.ChargePointID, events.EventSeverityInfo, metadata),
			TransactionInfo: transactionInfo,
		}
	}

	if event != nil {
		select {
		case m.eventChan <- event:
		default:
			m.logger.Warn("Event channel full, dropping transaction event")
		}
	}
}

// 状态转换辅助函数

// convertChargePointStatus 转换充电桩状态
func convertChargePointStatus(status ChargePointStatus) events.ChargePointStatus {
	switch status {
	case ChargePointStatusConnected:
		return events.ChargePointStatusOnline
	case ChargePointStatusRegistered:
		return events.ChargePointStatusRegistered
	case ChargePointStatusDisconnected:
		return events.ChargePointStatusOffline
	case ChargePointStatusMaintenance:
		return events.ChargePointStatusMaintenance
	default:
		return events.ChargePointStatusOffline
	}
}

// convertConnectorStatus 转换连接器状态
func convertConnectorStatus(status ConnectorStatus) events.ConnectorStatus {
	switch status {
	case ConnectorStatusAvailable:
		return events.ConnectorStatusAvailable
	case ConnectorStatusPreparing:
		return events.ConnectorStatusPreparing
	case ConnectorStatusCharging:
		return events.ConnectorStatusCharging
	case ConnectorStatusSuspendedEVSE:
		return events.ConnectorStatusSuspendedEVSE
	case ConnectorStatusSuspendedEV:
		return events.ConnectorStatusSuspendedEV
	case ConnectorStatusFinishing:
		return events.ConnectorStatusFinishing
	case ConnectorStatusReserved:
		return events.ConnectorStatusReserved
	case ConnectorStatusUnavailable:
		return events.ConnectorStatusUnavailable
	case ConnectorStatusFaulted:
		return events.ConnectorStatusFaulted
	default:
		return events.ConnectorStatusUnavailable
	}
}
