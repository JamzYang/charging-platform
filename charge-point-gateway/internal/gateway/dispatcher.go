package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
	"github.com/charging-platform/charge-point-gateway/internal/metrics"
)

// ProtocolHandler 协议处理器接口
type ProtocolHandler interface {
	// ProcessMessage 处理协议消息
	ProcessMessage(ctx context.Context, chargePointID string, message []byte) (interface{}, error)
	
	// GetSupportedActions 获取支持的动作列表
	GetSupportedActions() []string
	
	// GetVersion 获取协议版本
	GetVersion() string
	
	// Start 启动处理器
	Start() error
	
	// Stop 停止处理器
	Stop() error
	
	// GetEventChannel 获取事件通道
	GetEventChannel() <-chan events.Event
}

// MessageDispatcher 中央消息分发器接口
type MessageDispatcher interface {
	// RegisterHandler 注册协议处理器
	RegisterHandler(version string, handler ProtocolHandler) error
	
	// UnregisterHandler 注销协议处理器
	UnregisterHandler(version string) error
	
	// DispatchMessage 分发消息到对应的协议处理器
	DispatchMessage(ctx context.Context, chargePointID string, protocolVersion string, message []byte) (interface{}, error)
	
	// IdentifyProtocolVersion 识别协议版本
	IdentifyProtocolVersion(chargePointID string, message []byte) (string, error)
	
	// GetRegisteredVersions 获取已注册的协议版本列表
	GetRegisteredVersions() []string
	
	// GetHandlerForVersion 获取指定版本的处理器
	GetHandlerForVersion(version string) (ProtocolHandler, bool)
	
	// Start 启动分发器
	Start() error
	
	// Stop 停止分发器
	Stop() error
	
	// GetEventChannel 获取统一事件通道
	GetEventChannel() <-chan events.Event
	
	// GetStats 获取分发器统计信息
	GetStats() DispatcherStats
}

// DispatcherConfig 分发器配置
type DispatcherConfig struct {
	// 默认协议版本
	DefaultProtocolVersion string `json:"default_protocol_version"`
	
	// 事件通道缓冲区大小
	EventChannelBuffer int `json:"event_channel_buffer"`
	
	// 是否启用版本自动识别
	EnableVersionDetection bool `json:"enable_version_detection"`
	
	// 消息处理超时时间
	MessageTimeout time.Duration `json:"message_timeout"`
	
	// 是否启用统计信息收集
	EnableStats bool `json:"enable_stats"`
}

// DefaultDispatcherConfig 默认分发器配置
func DefaultDispatcherConfig() *DispatcherConfig {
	return &DispatcherConfig{
		DefaultProtocolVersion: "1.6",
		EventChannelBuffer:     1000,
		EnableVersionDetection: true,
		MessageTimeout:         30 * time.Second,
		EnableStats:            true,
	}
}

// DispatcherStats 分发器统计信息
type DispatcherStats struct {
	// 总消息数
	TotalMessages int64 `json:"total_messages"`
	
	// 成功处理的消息数
	SuccessfulMessages int64 `json:"successful_messages"`
	
	// 失败的消息数
	FailedMessages int64 `json:"failed_messages"`
	
	// 按版本分组的消息统计
	MessagesByVersion map[string]int64 `json:"messages_by_version"`
	
	// 平均处理时间
	AverageProcessingTime time.Duration `json:"average_processing_time"`
	
	// 最大处理时间
	MaxProcessingTime time.Duration `json:"max_processing_time"`
	
	// 启动时间
	StartTime time.Time `json:"start_time"`
	
	// 运行时间
	Uptime time.Duration `json:"uptime"`
}

// DefaultMessageDispatcher 默认消息分发器实现
type DefaultMessageDispatcher struct {
	// 配置
	config *DispatcherConfig
	
	// 协议处理器映射 (版本 -> 处理器)
	handlers map[string]ProtocolHandler
	
	// 读写锁保护handlers映射
	handlersMutex sync.RWMutex
	
	// 统一事件通道
	eventChan chan events.Event
	
	// 统计信息
	stats DispatcherStats
	
	// 统计信息锁
	statsMutex sync.RWMutex
	
	// 日志器
	logger *logger.Logger
	
	// 上下文和取消函数
	ctx    context.Context
	cancel context.CancelFunc
	
	// 等待组
	wg sync.WaitGroup
	
	// 是否已启动
	started bool
	
	// 启动锁
	startMutex sync.Mutex
}

// NewDefaultMessageDispatcher 创建新的默认消息分发器
func NewDefaultMessageDispatcher(config *DispatcherConfig) *DefaultMessageDispatcher {
	if config == nil {
		config = DefaultDispatcherConfig()
	}
	
	// 创建日志器
	l, _ := logger.New(logger.DefaultConfig())
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &DefaultMessageDispatcher{
		config:    config,
		handlers:  make(map[string]ProtocolHandler),
		eventChan: make(chan events.Event, config.EventChannelBuffer),
		stats: DispatcherStats{
			MessagesByVersion: make(map[string]int64),
			StartTime:         time.Now(),
		},
		logger: l,
		ctx:    ctx,
		cancel: cancel,
	}
}

// RegisterHandler 注册协议处理器
func (d *DefaultMessageDispatcher) RegisterHandler(version string, handler ProtocolHandler) error {
	if version == "" {
		return fmt.Errorf("protocol version cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}
	
	d.handlersMutex.Lock()
	defer d.handlersMutex.Unlock()
	
	if _, exists := d.handlers[version]; exists {
		return fmt.Errorf("handler for version %s already registered", version)
	}
	
	d.handlers[version] = handler
	d.logger.Infof("Registered protocol handler for version %s", version)
	
	return nil
}

// UnregisterHandler 注销协议处理器
func (d *DefaultMessageDispatcher) UnregisterHandler(version string) error {
	d.handlersMutex.Lock()
	defer d.handlersMutex.Unlock()
	
	if _, exists := d.handlers[version]; !exists {
		return fmt.Errorf("no handler registered for version %s", version)
	}
	
	delete(d.handlers, version)
	d.logger.Infof("Unregistered protocol handler for version %s", version)
	
	return nil
}

// GetRegisteredVersions 获取已注册的协议版本列表
func (d *DefaultMessageDispatcher) GetRegisteredVersions() []string {
	d.handlersMutex.RLock()
	defer d.handlersMutex.RUnlock()
	
	versions := make([]string, 0, len(d.handlers))
	for version := range d.handlers {
		versions = append(versions, version)
	}
	
	return versions
}

// GetHandlerForVersion 获取指定版本的处理器
func (d *DefaultMessageDispatcher) GetHandlerForVersion(version string) (ProtocolHandler, bool) {
	d.handlersMutex.RLock()
	defer d.handlersMutex.RUnlock()

	handler, exists := d.handlers[version]
	return handler, exists
}

// IdentifyProtocolVersion 识别协议版本
func (d *DefaultMessageDispatcher) IdentifyProtocolVersion(chargePointID string, message []byte) (string, error) {
	if !d.config.EnableVersionDetection {
		return d.config.DefaultProtocolVersion, nil
	}

	// 简单的版本识别逻辑 - 基于消息内容
	// 在实际实现中，可以通过解析消息头或特定字段来识别版本

	// 尝试解析为JSON并查找版本相关信息
	// 这里简化处理，直接返回默认版本
	// TODO: 实现更复杂的版本识别逻辑

	d.logger.Debugf("Identifying protocol version for charge point %s", chargePointID)

	// 目前只支持OCPP 1.6，直接返回
	return "1.6", nil
}

// DispatchMessage 分发消息到对应的协议处理器
func (d *DefaultMessageDispatcher) DispatchMessage(ctx context.Context, chargePointID string, protocolVersion string, message []byte) (interface{}, error) {
	startTime := time.Now()

	// 更新统计信息
	d.updateStats(protocolVersion, startTime, true)

	// 如果没有指定版本，尝试自动识别
	if protocolVersion == "" {
		var err error
		protocolVersion, err = d.IdentifyProtocolVersion(chargePointID, message)
		if err != nil {
			d.updateStats(protocolVersion, startTime, false)
			return nil, fmt.Errorf("failed to identify protocol version: %w", err)
		}
	}

	// 获取对应版本的处理器
	handler, exists := d.GetHandlerForVersion(protocolVersion)
	if !exists {
		d.updateStats(protocolVersion, startTime, false)
		return nil, fmt.Errorf("no handler registered for protocol version %s", protocolVersion)
	}

	// 创建带超时的上下文
	msgCtx, cancel := context.WithTimeout(ctx, d.config.MessageTimeout)
	defer cancel()

	// 处理消息
	response, err := handler.ProcessMessage(msgCtx, chargePointID, message)
	if err != nil {
		d.updateStats(protocolVersion, startTime, false)
		return nil, fmt.Errorf("handler failed to process message: %w", err)
	}

	d.updateStats(protocolVersion, startTime, true)
	// TODO: The messageType is not available at this level of abstraction.
	// For now, we use a placeholder. This could be improved by having the
	// protocol handler return the parsed message type.
	metrics.MessagesReceived.WithLabelValues(protocolVersion, "unknown").Inc()
	metrics.MessageProcessingDuration.WithLabelValues("unknown").Observe(time.Since(startTime).Seconds())
	d.logger.Debugf("Successfully dispatched message for charge point %s using protocol %s", chargePointID, protocolVersion)

	return response, nil
}

// Start 启动分发器
func (d *DefaultMessageDispatcher) Start() error {
	d.startMutex.Lock()
	defer d.startMutex.Unlock()

	if d.started {
		return fmt.Errorf("dispatcher already started")
	}

	d.logger.Info("Starting message dispatcher")

	// 启动所有注册的处理器
	d.handlersMutex.RLock()
	for version, handler := range d.handlers {
		if err := handler.Start(); err != nil {
			d.handlersMutex.RUnlock()
			return fmt.Errorf("failed to start handler for version %s: %w", version, err)
		}
		d.logger.Debugf("Started handler for protocol version %s", version)
	}
	d.handlersMutex.RUnlock()

	// 启动事件聚合器
	d.wg.Add(1)
	go d.eventAggregator()

	d.started = true
	d.stats.StartTime = time.Now()

	d.logger.Infof("Message dispatcher started with %d registered handlers", len(d.handlers))

	return nil
}

// Stop 停止分发器
func (d *DefaultMessageDispatcher) Stop() error {
	d.startMutex.Lock()
	defer d.startMutex.Unlock()

	if !d.started {
		return nil
	}

	d.logger.Info("Stopping message dispatcher")

	// 取消上下文
	d.cancel()

	// 停止所有处理器
	d.handlersMutex.RLock()
	for version, handler := range d.handlers {
		if err := handler.Stop(); err != nil {
			d.logger.Errorf("Failed to stop handler for version %s: %v", version, err)
		} else {
			d.logger.Debugf("Stopped handler for protocol version %s", version)
		}
	}
	d.handlersMutex.RUnlock()

	// 等待事件聚合器停止
	d.wg.Wait()

	// 关闭事件通道
	close(d.eventChan)

	d.started = false

	d.logger.Info("Message dispatcher stopped")

	return nil
}

// GetEventChannel 获取统一事件通道
func (d *DefaultMessageDispatcher) GetEventChannel() <-chan events.Event {
	return d.eventChan
}

// GetStats 获取分发器统计信息
func (d *DefaultMessageDispatcher) GetStats() DispatcherStats {
	d.statsMutex.RLock()
	defer d.statsMutex.RUnlock()

	// 复制统计信息
	stats := d.stats
	stats.Uptime = time.Since(d.stats.StartTime)

	// 复制版本统计映射
	stats.MessagesByVersion = make(map[string]int64)
	for version, count := range d.stats.MessagesByVersion {
		stats.MessagesByVersion[version] = count
	}

	return stats
}

// updateStats 更新统计信息
func (d *DefaultMessageDispatcher) updateStats(version string, startTime time.Time, success bool) {
	if !d.config.EnableStats {
		return
	}

	d.statsMutex.Lock()
	defer d.statsMutex.Unlock()

	processingTime := time.Since(startTime)

	d.stats.TotalMessages++
	if success {
		d.stats.SuccessfulMessages++
	} else {
		d.stats.FailedMessages++
	}

	// 更新版本统计
	d.stats.MessagesByVersion[version]++

	// 更新处理时间统计
	if processingTime > d.stats.MaxProcessingTime {
		d.stats.MaxProcessingTime = processingTime
	}

	// 计算平均处理时间
	if d.stats.TotalMessages > 0 {
		totalTime := time.Duration(d.stats.AverageProcessingTime.Nanoseconds()*int64(d.stats.TotalMessages-1)) + processingTime
		d.stats.AverageProcessingTime = totalTime / time.Duration(d.stats.TotalMessages)
	}
}

// eventAggregator 事件聚合器 - 从所有处理器收集事件并转发到统一通道
func (d *DefaultMessageDispatcher) eventAggregator() {
	defer d.wg.Done()

	d.logger.Debug("Starting event aggregator")

	// 收集所有处理器的事件通道
	var eventChannels []<-chan events.Event

	d.handlersMutex.RLock()
	for _, handler := range d.handlers {
		eventChannels = append(eventChannels, handler.GetEventChannel())
	}
	d.handlersMutex.RUnlock()

	// 使用select语句监听所有事件通道
	for {
		select {
		case <-d.ctx.Done():
			d.logger.Debug("Event aggregator stopping")
			return

		default:
			// 轮询所有事件通道
			for _, eventChan := range eventChannels {
				select {
				case event, ok := <-eventChan:
					if !ok {
						continue
					}

					// 转发事件到统一通道
					select {
					case d.eventChan <- event:
						d.logger.Debugf("Forwarded event %s from charge point %s", event.GetType(), event.GetChargePointID())
					case <-d.ctx.Done():
						return
					default:
						d.logger.Warn("Event channel full, dropping event")
					}

				default:
					// 没有事件，继续下一个通道
				}
			}

			// 短暂休眠避免CPU占用过高
			time.Sleep(10 * time.Millisecond)
		}
	}
}
