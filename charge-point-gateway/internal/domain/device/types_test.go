package device

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDevice(t *testing.T) {
	id := "device-123"
	chargePointID := "CP001"
	vendor := "TestVendor"
	model := "TestModel"

	device := NewDevice(id, chargePointID, vendor, model)

	// 测试基本属性
	assert.Equal(t, id, device.ID)
	assert.Equal(t, chargePointID, device.ChargePointID)
	assert.Equal(t, vendor, device.Vendor)
	assert.Equal(t, model, device.Model)
	assert.Equal(t, DeviceStatusOffline, device.Status)
	assert.Equal(t, RegistrationStatusPending, device.RegistrationStatus)

	// 测试时间戳
	assert.WithinDuration(t, time.Now(), device.CreatedAt, time.Second)
	assert.WithinDuration(t, time.Now(), device.UpdatedAt, time.Second)
	assert.WithinDuration(t, time.Now(), device.LastSeenAt, time.Second)
	assert.WithinDuration(t, time.Now(), device.Metrics.LastRebootTime, time.Second)

	// 测试初始化的切片和映射
	assert.NotNil(t, device.Connectors)
	assert.Len(t, device.Connectors, 0)
	assert.NotNil(t, device.Configurations)
	assert.Len(t, device.Configurations, 0)
	assert.NotNil(t, device.Capabilities.SupportedProfiles)
	assert.Len(t, device.Capabilities.SupportedProfiles, 0)
	assert.NotNil(t, device.Metadata)
	assert.NotNil(t, device.Tags)

	// 测试初始状态
	assert.False(t, device.IsOnline())
	assert.False(t, device.IsRegistered())
	assert.Nil(t, device.RegisteredAt)
}

func TestDevice_StatusManagement(t *testing.T) {
	device := NewDevice("test", "CP001", "Vendor", "Model")

	// 测试初始状态
	assert.Equal(t, DeviceStatusOffline, device.GetStatus())
	assert.False(t, device.IsOnline())

	// 测试状态变更
	device.SetStatus(DeviceStatusOnline)
	assert.Equal(t, DeviceStatusOnline, device.GetStatus())
	assert.True(t, device.IsOnline())

	device.SetStatus(DeviceStatusMaintenance)
	assert.Equal(t, DeviceStatusMaintenance, device.GetStatus())
	assert.False(t, device.IsOnline())

	device.SetStatus(DeviceStatusFaulted)
	assert.Equal(t, DeviceStatusFaulted, device.GetStatus())
	assert.False(t, device.IsOnline())
}

func TestDevice_RegistrationStatusManagement(t *testing.T) {
	device := NewDevice("test", "CP001", "Vendor", "Model")

	// 测试初始注册状态
	assert.Equal(t, RegistrationStatusPending, device.GetRegistrationStatus())
	assert.False(t, device.IsRegistered())
	assert.Nil(t, device.RegisteredAt)

	// 测试注册被拒绝
	device.SetRegistrationStatus(RegistrationStatusRejected)
	assert.Equal(t, RegistrationStatusRejected, device.GetRegistrationStatus())
	assert.False(t, device.IsRegistered())
	assert.Nil(t, device.RegisteredAt)

	// 测试注册成功
	device.SetRegistrationStatus(RegistrationStatusAccepted)
	assert.Equal(t, RegistrationStatusAccepted, device.GetRegistrationStatus())
	assert.True(t, device.IsRegistered())
	assert.NotNil(t, device.RegisteredAt)
	assert.WithinDuration(t, time.Now(), *device.RegisteredAt, time.Second)
}

func TestDevice_ConnectorManagement(t *testing.T) {
	device := NewDevice("test", "CP001", "Vendor", "Model")

	// 测试添加连接器
	connector1 := Connector{
		ID:               1,
		Type:             ConnectorTypeType2,
		PowerType:        PowerTypeAC,
		MaxPower:         22000,
		MaxCurrent:       32,
		MaxVoltage:       400,
		Phases:           3,
		Status:           "Available",
		LastStatusUpdate: time.Now().UTC(),
	}

	device.AddConnector(connector1)
	assert.Len(t, device.Connectors, 1)

	// 测试获取连接器
	retrieved, exists := device.GetConnector(1)
	assert.True(t, exists)
	assert.Equal(t, connector1.ID, retrieved.ID)
	assert.Equal(t, connector1.Type, retrieved.Type)
	assert.Equal(t, connector1.MaxPower, retrieved.MaxPower)

	// 测试获取不存在的连接器
	_, exists = device.GetConnector(999)
	assert.False(t, exists)

	// 测试更新现有连接器
	connector1Updated := connector1
	connector1Updated.Status = "Charging"
	connector1Updated.MaxPower = 11000

	device.AddConnector(connector1Updated)
	assert.Len(t, device.Connectors, 1) // 应该还是1个
	retrieved, exists = device.GetConnector(1)
	assert.True(t, exists)
	assert.Equal(t, "Charging", retrieved.Status)
	assert.Equal(t, float64(11000), retrieved.MaxPower)

	// 测试添加第二个连接器
	connector2 := Connector{
		ID:        2,
		Type:      ConnectorTypeCCS2,
		PowerType: PowerTypeDC,
		MaxPower:  50000,
		Status:    "Available",
	}

	device.AddConnector(connector2)
	assert.Len(t, device.Connectors, 2)
}

func TestDevice_ConnectorStatusUpdate(t *testing.T) {
	device := NewDevice("test", "CP001", "Vendor", "Model")

	// 添加连接器
	connector := Connector{
		ID:     1,
		Status: "Available",
	}
	device.AddConnector(connector)

	// 测试状态更新
	device.UpdateConnectorStatus(1, "Charging", "NoError", "Charging session active")

	retrieved, exists := device.GetConnector(1)
	require.True(t, exists)
	assert.Equal(t, "Charging", retrieved.Status)
	assert.Equal(t, "NoError", retrieved.ErrorCode)
	assert.Equal(t, "Charging session active", retrieved.Info)
	assert.WithinDuration(t, time.Now(), retrieved.LastStatusUpdate, time.Second)

	// 测试更新不存在的连接器
	device.UpdateConnectorStatus(999, "Faulted", "Error", "Test")
	// 应该不会panic或出错，只是不会有任何效果
}

func TestDevice_ConfigurationManagement(t *testing.T) {
	device := NewDevice("test", "CP001", "Vendor", "Model")

	// 测试设置配置
	key := "HeartbeatInterval"
	value := "300"
	readonly := false

	device.SetConfiguration(key, value, readonly)
	assert.Len(t, device.Configurations, 1)

	// 测试获取配置
	config, exists := device.GetConfiguration(key)
	assert.True(t, exists)
	assert.Equal(t, key, config.Key)
	assert.Equal(t, value, config.Value)
	assert.Equal(t, readonly, config.Readonly)

	// 测试更新现有配置
	newValue := "600"
	newReadonly := true
	device.SetConfiguration(key, newValue, newReadonly)
	assert.Len(t, device.Configurations, 1) // 应该还是1个

	config, exists = device.GetConfiguration(key)
	assert.True(t, exists)
	assert.Equal(t, newValue, config.Value)
	assert.Equal(t, newReadonly, config.Readonly)

	// 测试获取不存在的配置
	_, exists = device.GetConfiguration("NonexistentKey")
	assert.False(t, exists)

	// 测试添加第二个配置
	device.SetConfiguration("MeterValueSampleInterval", "60", false)
	assert.Len(t, device.Configurations, 2)
}

func TestDevice_MetricsUpdate(t *testing.T) {
	device := NewDevice("test", "CP001", "Vendor", "Model")

	// 测试更新指标
	metrics := DeviceMetrics{
		TotalEnergyDelivered: 1234.56,
		TotalTransactions:    100,
		ActiveTransactions:   2,
		UptimeSeconds:        86400,
		ErrorCount:           5,
		WarningCount:         10,
		AverageSessionTime:   45.5,
		PeakPowerUsage:       22000,
		TemperatureC:         25.5,
		HumidityPercent:      60.0,
	}

	device.UpdateMetrics(metrics)
	assert.Equal(t, metrics.TotalEnergyDelivered, device.Metrics.TotalEnergyDelivered)
	assert.Equal(t, metrics.TotalTransactions, device.Metrics.TotalTransactions)
	assert.Equal(t, metrics.ActiveTransactions, device.Metrics.ActiveTransactions)

	// 测试增加交易计数
	initialCount := device.Metrics.TotalTransactions
	device.IncrementTransactionCount()
	assert.Equal(t, initialCount+1, device.Metrics.TotalTransactions)

	// 测试添加能量交付量
	initialEnergy := device.Metrics.TotalEnergyDelivered
	additionalEnergy := 50.25
	device.AddEnergyDelivered(additionalEnergy)
	assert.Equal(t, initialEnergy+additionalEnergy, device.Metrics.TotalEnergyDelivered)
}

func TestDevice_MetadataManagement(t *testing.T) {
	device := NewDevice("test", "CP001", "Vendor", "Model")

	// 测试设置和获取元数据
	key := "installation_date"
	value := "2023-01-15"
	device.SetMetadata(key, value)

	retrievedValue, exists := device.GetMetadata(key)
	assert.True(t, exists)
	assert.Equal(t, value, retrievedValue)

	// 测试不存在的键
	_, exists = device.GetMetadata("nonexistent_key")
	assert.False(t, exists)

	// 测试复杂类型的元数据
	complexValue := map[string]interface{}{
		"location": "Building A",
		"floor":    2,
		"room":     "201",
	}
	device.SetMetadata("location_info", complexValue)

	retrievedComplex, exists := device.GetMetadata("location_info")
	assert.True(t, exists)
	assert.Equal(t, complexValue, retrievedComplex)
}

func TestDevice_TagManagement(t *testing.T) {
	device := NewDevice("test", "CP001", "Vendor", "Model")

	// 测试添加标签
	tag1 := "fast-charging"
	tag2 := "public-access"

	device.AddTag(tag1)
	assert.True(t, device.HasTag(tag1))
	assert.Len(t, device.Tags, 1)

	device.AddTag(tag2)
	assert.True(t, device.HasTag(tag2))
	assert.Len(t, device.Tags, 2)

	// 测试重复添加标签
	device.AddTag(tag1)
	assert.Len(t, device.Tags, 2) // 应该还是2个

	// 测试检查不存在的标签
	assert.False(t, device.HasTag("nonexistent-tag"))
}

func TestDevice_TimeCalculations(t *testing.T) {
	device := NewDevice("test", "CP001", "Vendor", "Model")

	// 测试运行时间
	uptime := device.GetUptime()
	assert.True(t, uptime >= 0)
	assert.True(t, uptime < time.Second) // 应该很短

	// 测试空闲时间
	idleTime := device.GetIdleTime()
	assert.True(t, idleTime >= 0)
	assert.True(t, idleTime < time.Second) // 应该很短

	// 等待一小段时间后测试
	time.Sleep(1 * time.Millisecond)
	uptime2 := device.GetUptime()
	assert.True(t, uptime2 > uptime)
}

func TestDevice_LastSeenUpdate(t *testing.T) {
	device := NewDevice("test", "CP001", "Vendor", "Model")

	// 记录初始时间
	initialTime := device.LastSeenAt

	// 等待一小段时间
	time.Sleep(1 * time.Millisecond)

	// 更新最后见到时间
	device.UpdateLastSeen()

	// 验证时间已更新
	assert.True(t, device.LastSeenAt.After(initialTime))
	assert.Equal(t, device.LastSeenAt, device.UpdatedAt)
}

func TestDevice_ThreadSafety(t *testing.T) {
	device := NewDevice("test", "CP001", "Vendor", "Model")

	// 并发测试状态变更
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 10; i++ {
			device.SetStatus(DeviceStatusOnline)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			_ = device.GetStatus()
		}
		done <- true
	}()

	// 等待两个goroutine完成
	<-done
	<-done

	// 验证最终状态
	assert.Equal(t, DeviceStatusOnline, device.GetStatus())
}

func TestConnectorTypes(t *testing.T) {
	// 测试连接器类型常量
	types := []ConnectorType{
		ConnectorTypeType1,
		ConnectorTypeType2,
		ConnectorTypeCHAdeMO,
		ConnectorTypeCCS1,
		ConnectorTypeCCS2,
		ConnectorTypeTesla,
		ConnectorTypeGB,
		ConnectorTypeOther,
	}

	for _, connType := range types {
		assert.NotEmpty(t, string(connType))
	}
}

func TestPowerTypes(t *testing.T) {
	// 测试功率类型常量
	assert.Equal(t, "AC", string(PowerTypeAC))
	assert.Equal(t, "DC", string(PowerTypeDC))
}

func TestDeviceStatuses(t *testing.T) {
	// 测试设备状态常量
	statuses := []DeviceStatus{
		DeviceStatusOnline,
		DeviceStatusOffline,
		DeviceStatusMaintenance,
		DeviceStatusFaulted,
		DeviceStatusUnknown,
	}

	for _, status := range statuses {
		assert.NotEmpty(t, string(status))
	}
}

func TestRegistrationStatuses(t *testing.T) {
	// 测试注册状态常量
	statuses := []RegistrationStatus{
		RegistrationStatusAccepted,
		RegistrationStatusPending,
		RegistrationStatusRejected,
	}

	for _, status := range statuses {
		assert.NotEmpty(t, string(status))
	}
}

func TestLocation(t *testing.T) {
	location := Location{
		Latitude:   39.9042,
		Longitude:  116.4074,
		Address:    "Tiananmen Square",
		City:       "Beijing",
		State:      "Beijing",
		Country:    "China",
		PostalCode: "100006",
	}

	assert.Equal(t, 39.9042, location.Latitude)
	assert.Equal(t, 116.4074, location.Longitude)
	assert.Equal(t, "Beijing", location.City)
	assert.Equal(t, "China", location.Country)
}
