package router

import (
	"testing"
	"time"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/protocol/ocpp16"
	"github.com/charging-platform/charge-point-gateway/internal/transport/websocket"
	"github.com/stretchr/testify/assert"
)

func TestDefaultRouterConfig(t *testing.T) {
	config := DefaultRouterConfig()
	
	assert.Equal(t, 1000, config.MaxConcurrentMessages)
	assert.Equal(t, 30*time.Second, config.MessageTimeout)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.Equal(t, 1000, config.EventChannelSize)
	assert.True(t, config.EnableEvents)
	assert.Equal(t, 8, config.WorkerCount)
	assert.Equal(t, 1000, config.BufferSize)
	assert.Equal(t, 1*time.Minute, config.StatsInterval)
	assert.True(t, config.EnableMetrics)
	assert.True(t, config.EnableErrorRecovery)
	assert.Equal(t, 10, config.ErrorThreshold)
	assert.Equal(t, 5*time.Minute, config.CircuitBreakerDelay)
}

func TestNewRouter(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	config := DefaultRouterConfig()
	
	router := NewRouter(wsManager, processor, config)
	
	assert.NotNil(t, router)
	assert.Equal(t, wsManager, router.wsManager)
	assert.Equal(t, processor, router.processor)
	assert.Equal(t, config, router.config)
	assert.NotNil(t, router.eventChan)
	assert.NotNil(t, router.stats)
	assert.NotNil(t, router.logger)
}

func TestNewRouterWithNilConfig(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	
	router := NewRouter(wsManager, processor, nil)
	
	assert.NotNil(t, router)
	assert.NotNil(t, router.config)
	assert.Equal(t, DefaultRouterConfig().MaxConcurrentMessages, router.config.MaxConcurrentMessages)
}

func TestRouter_StartStop(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	// 测试启动
	err := router.Start()
	assert.NoError(t, err)
	
	// 验证初始统计
	stats := router.GetStats()
	assert.Equal(t, int64(0), stats.MessagesReceived)
	assert.Equal(t, int64(0), stats.MessagesProcessed)
	assert.Equal(t, int64(0), stats.MessagesFailed)
	assert.Equal(t, int64(0), stats.EventsGenerated)
	
	// 测试停止
	err = router.Stop()
	assert.NoError(t, err)
}

func TestRouter_GetEventChannel(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	eventChan := router.GetEventChannel()
	assert.NotNil(t, eventChan)
	
	// 测试通道类型
	assert.IsType(t, (<-chan events.Event)(nil), eventChan)
}

func TestRouter_GetStats(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	stats := router.GetStats()
	
	assert.Equal(t, int64(0), stats.MessagesReceived)
	assert.Equal(t, int64(0), stats.MessagesProcessed)
	assert.Equal(t, int64(0), stats.MessagesFailed)
	assert.Equal(t, int64(0), stats.EventsGenerated)
	assert.Equal(t, float64(0), stats.AverageProcessTime)
	assert.WithinDuration(t, time.Now(), stats.LastResetTime, time.Second)
}

func TestRouter_ResetStats(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	// 模拟一些统计数据
	router.incrementReceivedMessages()
	router.incrementProcessedMessages()
	router.incrementFailedMessages()
	router.incrementGeneratedEvents()
	
	// 验证统计数据
	stats := router.GetStats()
	assert.Equal(t, int64(1), stats.MessagesReceived)
	assert.Equal(t, int64(1), stats.MessagesProcessed)
	assert.Equal(t, int64(1), stats.MessagesFailed)
	assert.Equal(t, int64(1), stats.EventsGenerated)
	
	// 重置统计
	router.ResetStats()
	
	// 验证重置后的统计
	stats = router.GetStats()
	assert.Equal(t, int64(0), stats.MessagesReceived)
	assert.Equal(t, int64(0), stats.MessagesProcessed)
	assert.Equal(t, int64(0), stats.MessagesFailed)
	assert.Equal(t, int64(0), stats.EventsGenerated)
}

func TestRouter_StatisticsUpdate(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	// 测试各种统计更新
	router.incrementReceivedMessages()
	router.incrementReceivedMessages()
	assert.Equal(t, int64(2), router.GetStats().MessagesReceived)
	
	router.incrementProcessedMessages()
	assert.Equal(t, int64(1), router.GetStats().MessagesProcessed)
	
	router.incrementFailedMessages()
	assert.Equal(t, int64(1), router.GetStats().MessagesFailed)
	
	router.incrementGeneratedEvents()
	router.incrementGeneratedEvents()
	router.incrementGeneratedEvents()
	assert.Equal(t, int64(3), router.GetStats().EventsGenerated)
}

func TestRouter_ProcessingTimeUpdate(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	// 测试处理时间更新
	duration1 := 100 * time.Millisecond
	router.updateProcessingTime(duration1)
	
	stats := router.GetStats()
	assert.Equal(t, 100.0, stats.AverageProcessTime)
	
	// 测试第二次更新（移动平均）
	duration2 := 200 * time.Millisecond
	router.updateProcessingTime(duration2)
	
	stats = router.GetStats()
	assert.Equal(t, 150.0, stats.AverageProcessTime) // (100+200)/2
}

func TestRouter_BroadcastMessage(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	message := []byte("test broadcast message")
	
	// 测试广播（不应该panic）
	router.BroadcastMessage(message)
}

func TestRouter_SendMessageToChargePoint(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	chargePointID := "CP001"
	message := []byte("test message")
	
	// 测试发送消息到不存在的充电桩
	err := router.SendMessageToChargePoint(chargePointID, message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection not found")
}

func TestRouter_GetActiveConnections(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	// 测试获取活跃连接（应该为空）
	connections := router.GetActiveConnections()
	assert.Empty(t, connections)
}

func TestRouter_IsChargePointConnected(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	chargePointID := "CP001"
	
	// 测试检查不存在的连接
	connected := router.IsChargePointConnected(chargePointID)
	assert.False(t, connected)
}

func TestRouter_GetConnectionInfo(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	chargePointID := "CP001"
	
	// 测试获取不存在的连接信息
	conn, exists := router.GetConnectionInfo(chargePointID)
	assert.Nil(t, conn)
	assert.False(t, exists)
}

func TestRouter_GetHealthStatus(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	// 添加一些统计数据
	router.incrementReceivedMessages()
	router.incrementProcessedMessages()
	router.updateProcessingTime(50 * time.Millisecond)
	
	healthStatus := router.GetHealthStatus()
	
	assert.Equal(t, "healthy", healthStatus["status"])
	assert.Equal(t, 0, healthStatus["active_connections"])
	assert.Equal(t, int64(1), healthStatus["messages_processed"])
	assert.Equal(t, int64(0), healthStatus["messages_failed"])
	assert.Equal(t, int64(0), healthStatus["events_generated"])
	assert.Equal(t, 50.0, healthStatus["average_process_time"])
	assert.GreaterOrEqual(t, healthStatus["uptime_seconds"], 0.0)
}

func TestMessageContext(t *testing.T) {
	msgCtx := &MessageContext{
		ChargePointID: "CP001",
		MessageData:   []byte("test message"),
		ReceivedAt:    time.Now(),
		Attempts:      0,
	}
	
	assert.Equal(t, "CP001", msgCtx.ChargePointID)
	assert.Equal(t, []byte("test message"), msgCtx.MessageData)
	assert.Equal(t, 0, msgCtx.Attempts)
	assert.Nil(t, msgCtx.LastError)
	assert.WithinDuration(t, time.Now(), msgCtx.ReceivedAt, time.Second)
}

func TestRouterStats(t *testing.T) {
	stats := &RouterStats{
		MessagesReceived:   100,
		MessagesProcessed:  95,
		MessagesFailed:     5,
		EventsGenerated:    200,
		AverageProcessTime: 25.5,
		LastResetTime:      time.Now(),
		ActiveConnections:  10,
		PendingMessages:    3,
	}
	
	assert.Equal(t, int64(100), stats.MessagesReceived)
	assert.Equal(t, int64(95), stats.MessagesProcessed)
	assert.Equal(t, int64(5), stats.MessagesFailed)
	assert.Equal(t, int64(200), stats.EventsGenerated)
	assert.Equal(t, 25.5, stats.AverageProcessTime)
	assert.Equal(t, 10, stats.ActiveConnections)
	assert.Equal(t, 3, stats.PendingMessages)
}

func TestRouter_SerializeResponse(t *testing.T) {
	wsManager := websocket.NewManager(websocket.DefaultConfig())
	processor := ocpp16.NewProcessor(ocpp16.DefaultProcessorConfig())
	router := NewRouter(wsManager, processor, DefaultRouterConfig())
	
	// 测试成功响应序列化
	successResponse := &ocpp16.ProcessorResponse{
		MessageID:   "12345",
		Success:     true,
		Payload:     map[string]string{"status": "Accepted"},
		ProcessedAt: time.Now(),
	}
	
	data, err := router.serializeResponse(successResponse)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	
	// 测试错误响应序列化
	errorResponse := &ocpp16.ProcessorResponse{
		MessageID:   "12346",
		Success:     false,
		Error:       assert.AnError,
		ProcessedAt: time.Now(),
	}
	
	data, err = router.serializeResponse(errorResponse)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
}
