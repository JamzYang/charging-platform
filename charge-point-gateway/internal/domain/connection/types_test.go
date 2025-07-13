package connection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewConnection(t *testing.T) {
	id := "conn-123"
	chargePointID := "CP001"
	connType := ConnectionTypeWebSocket
	protocolVersion := ProtocolVersionOCPP16

	conn := NewConnection(id, chargePointID, connType, protocolVersion)

	// 测试基本属性
	assert.Equal(t, id, conn.ID)
	assert.Equal(t, chargePointID, conn.ChargePointID)
	assert.Equal(t, connType, conn.Type)
	assert.Equal(t, protocolVersion, conn.ProtocolVersion)
	assert.Equal(t, ConnectionStateConnecting, conn.State)

	// 测试时间戳
	assert.WithinDuration(t, time.Now(), conn.CreatedAt, time.Second)
	assert.WithinDuration(t, time.Now(), conn.UpdatedAt, time.Second)
	assert.WithinDuration(t, time.Now(), conn.ConnectedAt, time.Second)
	assert.WithinDuration(t, time.Now(), conn.LastSeenAt, time.Second)

	// 测试默认配置
	assert.Equal(t, 1024*1024, conn.Config.MaxMessageSize)
	assert.Equal(t, 30*time.Second, conn.Config.ReadTimeout)
	assert.Equal(t, 30*time.Second, conn.Config.WriteTimeout)

	// 测试初始指标
	assert.Equal(t, int64(0), conn.NetworkInfo.BytesSent)
	assert.Equal(t, int64(0), conn.NetworkInfo.BytesReceived)
	assert.Equal(t, int64(0), conn.NetworkInfo.MessagesSent)
	assert.Equal(t, int64(0), conn.NetworkInfo.MessagesReceived)

	// 测试初始状态
	assert.False(t, conn.IsClosed())
	assert.NotNil(t, conn.Metadata)
	assert.NotNil(t, conn.Tags)
}

func TestConnection_StateManagement(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 测试初始状态
	assert.Equal(t, ConnectionStateConnecting, conn.GetState())
	assert.False(t, conn.IsConnected())

	// 测试状态变更
	conn.SetState(ConnectionStateConnected)
	assert.Equal(t, ConnectionStateConnected, conn.GetState())
	assert.True(t, conn.IsConnected())

	conn.SetState(ConnectionStateAuthenticated)
	assert.Equal(t, ConnectionStateAuthenticated, conn.GetState())
	assert.True(t, conn.IsConnected())

	conn.SetState(ConnectionStateRegistered)
	assert.Equal(t, ConnectionStateRegistered, conn.GetState())
	assert.True(t, conn.IsConnected())

	conn.SetState(ConnectionStateDisconnected)
	assert.Equal(t, ConnectionStateDisconnected, conn.GetState())
	assert.False(t, conn.IsConnected())
}

func TestConnection_NetworkInfoUpdate(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 测试网络信息更新
	remoteAddr := "192.168.1.100:12345"
	localAddr := "192.168.1.1:8080"
	conn.UpdateNetworkInfo(remoteAddr, localAddr)

	assert.Equal(t, remoteAddr, conn.NetworkInfo.RemoteAddr)
	assert.Equal(t, localAddr, conn.NetworkInfo.LocalAddr)
}

func TestConnection_MessageCounting(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 测试发送消息计数
	conn.IncrementMessagesSent(100)
	assert.Equal(t, int64(1), conn.NetworkInfo.MessagesSent)
	assert.Equal(t, int64(100), conn.NetworkInfo.BytesSent)

	conn.IncrementMessagesSent(200)
	assert.Equal(t, int64(2), conn.NetworkInfo.MessagesSent)
	assert.Equal(t, int64(300), conn.NetworkInfo.BytesSent)

	// 测试接收消息计数
	conn.IncrementMessagesReceived(150)
	assert.Equal(t, int64(1), conn.NetworkInfo.MessagesReceived)
	assert.Equal(t, int64(150), conn.NetworkInfo.BytesReceived)

	conn.IncrementMessagesReceived(250)
	assert.Equal(t, int64(2), conn.NetworkInfo.MessagesReceived)
	assert.Equal(t, int64(400), conn.NetworkInfo.BytesReceived)
}

func TestConnection_ResponseTimeTracking(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 测试第一次响应时间
	responseTime1 := 100 * time.Millisecond
	conn.UpdateResponseTime(responseTime1)
	assert.Equal(t, responseTime1, conn.Metrics.AverageResponseTime)
	assert.Equal(t, responseTime1, conn.Metrics.MaxResponseTime)
	assert.Equal(t, responseTime1, conn.Metrics.MinResponseTime)

	// 测试第二次响应时间
	responseTime2 := 200 * time.Millisecond
	conn.UpdateResponseTime(responseTime2)
	assert.Equal(t, 150*time.Millisecond, conn.Metrics.AverageResponseTime) // (100+200)/2
	assert.Equal(t, responseTime2, conn.Metrics.MaxResponseTime)
	assert.Equal(t, responseTime1, conn.Metrics.MinResponseTime)

	// 测试更小的响应时间
	responseTime3 := 50 * time.Millisecond
	conn.UpdateResponseTime(responseTime3)
	assert.Equal(t, responseTime3, conn.Metrics.MinResponseTime)
	assert.Equal(t, responseTime2, conn.Metrics.MaxResponseTime)
}

func TestConnection_ErrorTracking(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 测试错误记录
	errorMessage := "Connection timeout"
	conn.RecordError(errorMessage)

	assert.Equal(t, 1, conn.Metrics.ErrorCount)
	assert.Equal(t, errorMessage, conn.Metrics.LastErrorMessage)
	assert.NotNil(t, conn.Metrics.LastErrorTime)
	assert.WithinDuration(t, time.Now(), *conn.Metrics.LastErrorTime, time.Second)

	// 测试多次错误
	conn.RecordError("Another error")
	assert.Equal(t, 2, conn.Metrics.ErrorCount)
	assert.Equal(t, "Another error", conn.Metrics.LastErrorMessage)
}

func TestConnection_ReconnectTracking(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 测试重连计数
	assert.Equal(t, 0, conn.Metrics.ReconnectCount)

	conn.IncrementReconnectCount()
	assert.Equal(t, 1, conn.Metrics.ReconnectCount)

	conn.IncrementReconnectCount()
	assert.Equal(t, 2, conn.Metrics.ReconnectCount)
}

func TestConnection_MetadataManagement(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 测试设置和获取元数据
	key := "test_key"
	value := "test_value"
	conn.SetMetadata(key, value)

	retrievedValue, exists := conn.GetMetadata(key)
	assert.True(t, exists)
	assert.Equal(t, value, retrievedValue)

	// 测试不存在的键
	_, exists = conn.GetMetadata("nonexistent_key")
	assert.False(t, exists)

	// 测试覆盖元数据
	newValue := "new_test_value"
	conn.SetMetadata(key, newValue)
	retrievedValue, exists = conn.GetMetadata(key)
	assert.True(t, exists)
	assert.Equal(t, newValue, retrievedValue)
}

func TestConnection_TagManagement(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 测试添加标签
	tag1 := "production"
	tag2 := "high-priority"

	conn.AddTag(tag1)
	assert.True(t, conn.HasTag(tag1))
	assert.Len(t, conn.Tags, 1)

	conn.AddTag(tag2)
	assert.True(t, conn.HasTag(tag2))
	assert.Len(t, conn.Tags, 2)

	// 测试重复添加标签
	conn.AddTag(tag1)
	assert.Len(t, conn.Tags, 2) // 应该还是2个

	// 测试移除标签
	conn.RemoveTag(tag1)
	assert.False(t, conn.HasTag(tag1))
	assert.True(t, conn.HasTag(tag2))
	assert.Len(t, conn.Tags, 1)

	// 测试移除不存在的标签
	conn.RemoveTag("nonexistent")
	assert.Len(t, conn.Tags, 1) // 应该还是1个
}

func TestConnection_ActivityTracking(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 记录初始时间
	initialTime := conn.NetworkInfo.LastActivity

	// 等待一小段时间
	time.Sleep(1 * time.Millisecond)

	// 更新活动时间
	conn.UpdateLastActivity()

	// 验证时间已更新
	assert.True(t, conn.NetworkInfo.LastActivity.After(initialTime))
	assert.Equal(t, conn.NetworkInfo.LastActivity, conn.LastSeenAt)
}

func TestConnection_DurationCalculations(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 测试连接持续时间
	duration := conn.GetConnectionDuration()
	assert.True(t, duration >= 0)
	assert.True(t, duration < time.Second) // 应该很短

	// 测试空闲时间
	idleDuration := conn.GetIdleDuration()
	assert.True(t, idleDuration >= 0)
	assert.True(t, idleDuration < time.Second) // 应该很短
}

func TestConnection_Close(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 测试初始状态
	assert.False(t, conn.IsClosed())

	// 测试关闭连接
	err := conn.Close()
	assert.NoError(t, err)
	assert.True(t, conn.IsClosed())
	assert.Equal(t, ConnectionStateDisconnected, conn.GetState())

	// 测试重复关闭
	err = conn.Close()
	assert.NoError(t, err) // 应该不报错
}

func TestConnection_ThreadSafety(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	// 并发测试状态变更 - 减少循环次数和移除sleep
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 10; i++ {
			conn.SetState(ConnectionStateConnected)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			_ = conn.GetState()
		}
		done <- true
	}()

	// 等待两个goroutine完成
	<-done
	<-done

	// 验证最终状态
	assert.Equal(t, ConnectionStateConnected, conn.GetState())
}

func TestConnectionConfig_Defaults(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	config := conn.Config

	// 验证默认配置值
	assert.Equal(t, 1024*1024, config.MaxMessageSize)
	assert.Equal(t, 30*time.Second, config.ReadTimeout)
	assert.Equal(t, 30*time.Second, config.WriteTimeout)
	assert.Equal(t, 30*time.Second, config.PingInterval)
	assert.Equal(t, 10*time.Second, config.PongTimeout)
	assert.Equal(t, 3, config.MaxReconnectCount)
	assert.Equal(t, 5*time.Second, config.ReconnectInterval)
	assert.False(t, config.EnableCompression)
	assert.Equal(t, 4096, config.BufferSize)
}

func TestSecurityProfile_Defaults(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)

	security := conn.Security

	// 验证默认安全配置
	assert.Equal(t, 0, security.SecurityProfile)
	assert.False(t, security.TLSEnabled)
	assert.False(t, security.CertificateAuth)
	assert.False(t, security.BasicAuth)
	assert.Empty(t, security.Username)
	assert.Empty(t, security.Password)
}
