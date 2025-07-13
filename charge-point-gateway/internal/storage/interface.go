package storage

import (
	"context"
	"time"
)

// ConnectionStorage 定义了管理充电桩连接映射的接口
type ConnectionStorage interface {
	// SetConnection 注册或更新一个充电桩的连接信息
	// chargePointID: 充电桩的唯一标识
	// gatewayID: 当前处理该连接的 Gateway Pod 的唯一标识
	// ttl: 键的过期时间，用于自动清理僵尸连接
	SetConnection(ctx context.Context, chargePointID string, gatewayID string, ttl time.Duration) error

	// GetConnection 获取指定充电桩当前连接的 Gateway Pod ID
	// 如果键不存在，应返回 redis.Nil 错误
	GetConnection(ctx context.Context, chargePointID string) (string, error)

	// DeleteConnection 删除一个充电桩的连接信息（例如，充电桩正常断连时）
	DeleteConnection(ctx context.Context, chargePointID string) error

	// Close 关闭与存储后端的连接
	Close() error
}