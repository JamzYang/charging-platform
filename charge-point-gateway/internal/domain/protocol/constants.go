package protocol

import "github.com/charging-platform/charge-point-gateway/internal/domain/connection"

// OCPP协议版本常量
const (
	// 标准OCPP版本
	OCPP_VERSION_1_6   = "ocpp1.6"
	OCPP_VERSION_2_0   = "ocpp2.0"
	OCPP_VERSION_2_0_1 = "ocpp2.0.1"

	// 默认版本
	DEFAULT_VERSION = OCPP_VERSION_1_6
)

// 支持的协议版本列表
var SupportedVersions = []string{
	OCPP_VERSION_1_6,
	OCPP_VERSION_2_0,
	OCPP_VERSION_2_0_1,
}

// 版本映射表 - 处理各种格式的版本号
var VersionMapping = map[string]string{
	// OCPP 1.6 的各种表示方式
	"1.6":     OCPP_VERSION_1_6,
	"ocpp1.6": OCPP_VERSION_1_6,
	"OCPP1.6": OCPP_VERSION_1_6,

	// OCPP 2.0 的各种表示方式
	"2.0":     OCPP_VERSION_2_0,
	"ocpp2.0": OCPP_VERSION_2_0,
	"OCPP2.0": OCPP_VERSION_2_0,

	// OCPP 2.0.1 的各种表示方式
	"2.0.1":     OCPP_VERSION_2_0_1,
	"ocpp2.0.1": OCPP_VERSION_2_0_1,
	"OCPP2.0.1": OCPP_VERSION_2_0_1,
}

// NormalizeVersion 规范化协议版本
func NormalizeVersion(version string) string {
	if normalized, exists := VersionMapping[version]; exists {
		return normalized
	}
	return ""
}

// IsVersionSupported 检查版本是否支持
func IsVersionSupported(version string) bool {
	normalized := NormalizeVersion(version)
	if normalized == "" {
		return false
	}

	for _, supported := range SupportedVersions {
		if normalized == supported {
			return true
		}
	}
	return false
}

// GetDefaultVersion 获取默认版本
func GetDefaultVersion() string {
	return DEFAULT_VERSION
}

// GetSupportedVersions 获取支持的版本列表
func GetSupportedVersions() []string {
	// 返回副本，避免外部修改
	result := make([]string, len(SupportedVersions))
	copy(result, SupportedVersions)
	return result
}

// ToConnectionProtocolVersion 将字符串版本转换为连接协议版本类型
func ToConnectionProtocolVersion(version string) connection.ProtocolVersion {
	normalized := NormalizeVersion(version)
	switch normalized {
	case OCPP_VERSION_1_6:
		return connection.ProtocolVersionOCPP16
	case OCPP_VERSION_2_0:
		return connection.ProtocolVersionOCPP20
	case OCPP_VERSION_2_0_1:
		return connection.ProtocolVersionOCPP201
	default:
		return connection.ProtocolVersionOCPP16 // 默认版本
	}
}
