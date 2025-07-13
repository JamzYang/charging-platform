package device

import (
	"sync"
	"time"
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline      DeviceStatus = "online"
	DeviceStatusOffline     DeviceStatus = "offline"
	DeviceStatusMaintenance DeviceStatus = "maintenance"
	DeviceStatusFaulted     DeviceStatus = "faulted"
	DeviceStatusUnknown     DeviceStatus = "unknown"
)

// RegistrationStatus 注册状态
type RegistrationStatus string

const (
	RegistrationStatusAccepted RegistrationStatus = "accepted"
	RegistrationStatusPending  RegistrationStatus = "pending"
	RegistrationStatusRejected RegistrationStatus = "rejected"
)

// ConnectorType 连接器类型
type ConnectorType string

const (
	ConnectorTypeType1     ConnectorType = "Type1"     // SAE J1772
	ConnectorTypeType2     ConnectorType = "Type2"     // IEC 62196-2
	ConnectorTypeCHAdeMO   ConnectorType = "CHAdeMO"   // CHAdeMO
	ConnectorTypeCCS1      ConnectorType = "CCS1"      // CCS Type 1
	ConnectorTypeCCS2      ConnectorType = "CCS2"      // CCS Type 2
	ConnectorTypeTesla     ConnectorType = "Tesla"     // Tesla Supercharger
	ConnectorTypeGB        ConnectorType = "GB"        // GB/T (China)
	ConnectorTypeOther     ConnectorType = "Other"
)

// PowerType 功率类型
type PowerType string

const (
	PowerTypeAC PowerType = "AC"
	PowerTypeDC PowerType = "DC"
)

// Location 地理位置
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Address   string  `json:"address,omitempty"`
	City      string  `json:"city,omitempty"`
	State     string  `json:"state,omitempty"`
	Country   string  `json:"country,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
}

// Connector 连接器信息
type Connector struct {
	ID               int           `json:"id"`
	Type             ConnectorType `json:"type"`
	PowerType        PowerType     `json:"power_type"`
	MaxPower         float64       `json:"max_power"`         // 最大功率 (W)
	MaxCurrent       float64       `json:"max_current"`       // 最大电流 (A)
	MaxVoltage       float64       `json:"max_voltage"`       // 最大电压 (V)
	Phases           int           `json:"phases"`            // 相数 (1 or 3)
	Status           string        `json:"status"`            // 连接器状态
	ErrorCode        string        `json:"error_code,omitempty"`
	Info             string        `json:"info,omitempty"`
	VendorErrorCode  string        `json:"vendor_error_code,omitempty"`
	LastStatusUpdate time.Time     `json:"last_status_update"`
}

// FirmwareInfo 固件信息
type FirmwareInfo struct {
	Version         string     `json:"version"`
	UpdateStatus    string     `json:"update_status,omitempty"`
	UpdateProgress  int        `json:"update_progress,omitempty"` // 0-100
	LastUpdateTime  *time.Time `json:"last_update_time,omitempty"`
	NextUpdateTime  *time.Time `json:"next_update_time,omitempty"`
	UpdateURL       string     `json:"update_url,omitempty"`
	UpdateChecksum  string     `json:"update_checksum,omitempty"`
}

// DiagnosticsInfo 诊断信息
type DiagnosticsInfo struct {
	Status         string     `json:"status,omitempty"`
	LastUploadTime *time.Time `json:"last_upload_time,omitempty"`
	UploadURL      string     `json:"upload_url,omitempty"`
	FileName       string     `json:"file_name,omitempty"`
	FileSize       int64      `json:"file_size,omitempty"`
}

// Configuration 配置项
type Configuration struct {
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
	Readonly bool   `json:"readonly"`
}

// FeatureProfile 功能配置文件
type FeatureProfile struct {
	Name        string   `json:"name"`
	Supported   bool     `json:"supported"`
	Version     string   `json:"version,omitempty"`
	Features    []string `json:"features,omitempty"`
	LastChecked time.Time `json:"last_checked"`
}

// DeviceCapabilities 设备能力
type DeviceCapabilities struct {
	SupportedProfiles    []FeatureProfile `json:"supported_profiles"`
	MaxConnectors        int              `json:"max_connectors"`
	SupportsReservation  bool             `json:"supports_reservation"`
	SupportsRemoteStart  bool             `json:"supports_remote_start"`
	SupportsSmartCharging bool            `json:"supports_smart_charging"`
	SupportsFirmwareUpdate bool           `json:"supports_firmware_update"`
	SupportsLocalAuth    bool             `json:"supports_local_auth"`
	MaxChargingProfiles  int              `json:"max_charging_profiles,omitempty"`
	ChargingScheduleMaxPeriods int        `json:"charging_schedule_max_periods,omitempty"`
}

// DeviceMetrics 设备指标
type DeviceMetrics struct {
	TotalEnergyDelivered float64   `json:"total_energy_delivered"` // kWh
	TotalTransactions    int64     `json:"total_transactions"`
	ActiveTransactions   int       `json:"active_transactions"`
	UptimeSeconds        int64     `json:"uptime_seconds"`
	LastRebootTime       time.Time `json:"last_reboot_time"`
	ErrorCount           int64     `json:"error_count"`
	WarningCount         int64     `json:"warning_count"`
	AverageSessionTime   float64   `json:"average_session_time"` // minutes
	PeakPowerUsage       float64   `json:"peak_power_usage"`     // W
	TemperatureC         float64   `json:"temperature_c,omitempty"`
	HumidityPercent      float64   `json:"humidity_percent,omitempty"`
}

// MaintenanceInfo 维护信息
type MaintenanceInfo struct {
	LastMaintenanceDate  *time.Time `json:"last_maintenance_date,omitempty"`
	NextMaintenanceDate  *time.Time `json:"next_maintenance_date,omitempty"`
	MaintenanceInterval  int        `json:"maintenance_interval,omitempty"` // days
	MaintenanceNotes     string     `json:"maintenance_notes,omitempty"`
	WarrantyExpiry       *time.Time `json:"warranty_expiry,omitempty"`
	ServiceProvider      string     `json:"service_provider,omitempty"`
	ServiceContact       string     `json:"service_contact,omitempty"`
}

// Device 设备模型
type Device struct {
	// 基本信息
	ID                   string              `json:"id"`
	ChargePointID        string              `json:"charge_point_id"`
	Vendor               string              `json:"vendor"`
	Model                string              `json:"model"`
	SerialNumber         string              `json:"serial_number,omitempty"`
	ChargeBoxSerialNumber string             `json:"charge_box_serial_number,omitempty"`
	
	// 状态信息
	Status               DeviceStatus        `json:"status"`
	RegistrationStatus   RegistrationStatus  `json:"registration_status"`
	
	// 网络信息
	ICCID                string              `json:"iccid,omitempty"`  // SIM卡ID
	IMSI                 string              `json:"imsi,omitempty"`   // 移动用户识别码
	MeterType            string              `json:"meter_type,omitempty"`
	MeterSerialNumber    string              `json:"meter_serial_number,omitempty"`
	
	// 位置信息
	Location             *Location           `json:"location,omitempty"`
	
	// 连接器信息
	Connectors           []Connector         `json:"connectors"`
	
	// 固件和诊断
	Firmware             FirmwareInfo        `json:"firmware"`
	Diagnostics          DiagnosticsInfo     `json:"diagnostics"`
	
	// 配置信息
	Configurations       []Configuration     `json:"configurations"`
	
	// 设备能力
	Capabilities         DeviceCapabilities  `json:"capabilities"`
	
	// 指标信息
	Metrics              DeviceMetrics       `json:"metrics"`
	
	// 维护信息
	Maintenance          MaintenanceInfo     `json:"maintenance"`
	
	// 时间戳
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
	LastSeenAt           time.Time           `json:"last_seen_at"`
	RegisteredAt         *time.Time          `json:"registered_at,omitempty"`
	
	// 元数据
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
	Tags                 []string               `json:"tags,omitempty"`
	
	// 内部字段 (不序列化)
	mutex                sync.RWMutex        `json:"-"`
}

// NewDevice 创建新设备
func NewDevice(id, chargePointID, vendor, model string) *Device {
	now := time.Now().UTC()
	return &Device{
		ID:                 id,
		ChargePointID:      chargePointID,
		Vendor:             vendor,
		Model:              model,
		Status:             DeviceStatusOffline,
		RegistrationStatus: RegistrationStatusPending,
		Connectors:         make([]Connector, 0),
		Configurations:     make([]Configuration, 0),
		Capabilities: DeviceCapabilities{
			SupportedProfiles: make([]FeatureProfile, 0),
		},
		Metrics: DeviceMetrics{
			LastRebootTime: now,
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSeenAt: now,
		Metadata:   make(map[string]interface{}),
		Tags:       make([]string, 0),
	}
}

// GetStatus 获取设备状态 (线程安全)
func (d *Device) GetStatus() DeviceStatus {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.Status
}

// SetStatus 设置设备状态 (线程安全)
func (d *Device) SetStatus(status DeviceStatus) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Status = status
	d.UpdatedAt = time.Now().UTC()
	d.LastSeenAt = d.UpdatedAt
}

// GetRegistrationStatus 获取注册状态 (线程安全)
func (d *Device) GetRegistrationStatus() RegistrationStatus {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.RegistrationStatus
}

// SetRegistrationStatus 设置注册状态 (线程安全)
func (d *Device) SetRegistrationStatus(status RegistrationStatus) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.RegistrationStatus = status
	d.UpdatedAt = time.Now().UTC()
	
	if status == RegistrationStatusAccepted {
		now := time.Now().UTC()
		d.RegisteredAt = &now
	}
}

// IsOnline 检查设备是否在线
func (d *Device) IsOnline() bool {
	return d.GetStatus() == DeviceStatusOnline
}

// IsRegistered 检查设备是否已注册
func (d *Device) IsRegistered() bool {
	return d.GetRegistrationStatus() == RegistrationStatusAccepted
}

// UpdateLastSeen 更新最后见到时间
func (d *Device) UpdateLastSeen() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.LastSeenAt = time.Now().UTC()
	d.UpdatedAt = d.LastSeenAt
}

// AddConnector 添加连接器
func (d *Device) AddConnector(connector Connector) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	// 检查连接器ID是否已存在
	for i, existing := range d.Connectors {
		if existing.ID == connector.ID {
			d.Connectors[i] = connector
			d.UpdatedAt = time.Now().UTC()
			return
		}
	}
	
	d.Connectors = append(d.Connectors, connector)
	d.UpdatedAt = time.Now().UTC()
}

// GetConnector 获取连接器
func (d *Device) GetConnector(connectorID int) (*Connector, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	for _, connector := range d.Connectors {
		if connector.ID == connectorID {
			return &connector, true
		}
	}
	return nil, false
}

// UpdateConnectorStatus 更新连接器状态
func (d *Device) UpdateConnectorStatus(connectorID int, status, errorCode, info string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	for i, connector := range d.Connectors {
		if connector.ID == connectorID {
			d.Connectors[i].Status = status
			d.Connectors[i].ErrorCode = errorCode
			d.Connectors[i].Info = info
			d.Connectors[i].LastStatusUpdate = time.Now().UTC()
			d.UpdatedAt = d.Connectors[i].LastStatusUpdate
			break
		}
	}
}

// SetConfiguration 设置配置项
func (d *Device) SetConfiguration(key, value string, readonly bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	// 检查配置项是否已存在
	for i, config := range d.Configurations {
		if config.Key == key {
			d.Configurations[i].Value = value
			d.Configurations[i].Readonly = readonly
			d.UpdatedAt = time.Now().UTC()
			return
		}
	}
	
	// 添加新配置项
	d.Configurations = append(d.Configurations, Configuration{
		Key:      key,
		Value:    value,
		Readonly: readonly,
	})
	d.UpdatedAt = time.Now().UTC()
}

// GetConfiguration 获取配置项
func (d *Device) GetConfiguration(key string) (*Configuration, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	for _, config := range d.Configurations {
		if config.Key == key {
			return &config, true
		}
	}
	return nil, false
}

// UpdateMetrics 更新设备指标
func (d *Device) UpdateMetrics(metrics DeviceMetrics) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Metrics = metrics
	d.UpdatedAt = time.Now().UTC()
}

// IncrementTransactionCount 增加交易计数
func (d *Device) IncrementTransactionCount() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Metrics.TotalTransactions++
	d.UpdatedAt = time.Now().UTC()
}

// AddEnergyDelivered 添加能量交付量
func (d *Device) AddEnergyDelivered(energy float64) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Metrics.TotalEnergyDelivered += energy
	d.UpdatedAt = time.Now().UTC()
}

// SetMetadata 设置元数据
func (d *Device) SetMetadata(key string, value interface{}) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if d.Metadata == nil {
		d.Metadata = make(map[string]interface{})
	}
	d.Metadata[key] = value
	d.UpdatedAt = time.Now().UTC()
}

// GetMetadata 获取元数据
func (d *Device) GetMetadata(key string) (interface{}, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	if d.Metadata == nil {
		return nil, false
	}
	value, exists := d.Metadata[key]
	return value, exists
}

// AddTag 添加标签
func (d *Device) AddTag(tag string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	// 检查标签是否已存在
	for _, existingTag := range d.Tags {
		if existingTag == tag {
			return
		}
	}
	
	d.Tags = append(d.Tags, tag)
	d.UpdatedAt = time.Now().UTC()
}

// HasTag 检查是否有指定标签
func (d *Device) HasTag(tag string) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	for _, existingTag := range d.Tags {
		if existingTag == tag {
			return true
		}
	}
	return false
}

// GetUptime 获取设备运行时间
func (d *Device) GetUptime() time.Duration {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return time.Since(d.Metrics.LastRebootTime)
}

// GetIdleTime 获取设备空闲时间
func (d *Device) GetIdleTime() time.Duration {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return time.Since(d.LastSeenAt)
}
