package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/charging-platform/charge-point-gateway/internal/config"
)

// RedisStorage 使用 Redis 来存储连接映射
type RedisStorage struct {
	Client *redis.Client // 将 client 字段改为公共字段，以便测试访问
	Prefix string        // 将 prefix 字段改为公共字段，以便测试访问
}

// NewRedisStorage 创建一个新的 RedisStorage 实例
func NewRedisStorage(cfg config.RedisConfig) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 尝试 ping Redis 以验证连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		// 包装原始错误，提供更多上下文信息
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", cfg.Addr, err)
	}

	return &RedisStorage{Client: client, Prefix: "conn:"}, nil
}

// SetConnection 注册或更新一个充电桩的连接信息
func (r *RedisStorage) SetConnection(ctx context.Context, chargePointID string, gatewayID string, ttl time.Duration) error {
	key := fmt.Sprintf("%s%s", r.Prefix, chargePointID)
	return r.Client.Set(ctx, key, gatewayID, ttl).Err()
}

// GetConnection 获取指定充电桩当前连接的 Gateway Pod ID
func (r *RedisStorage) GetConnection(ctx context.Context, chargePointID string) (string, error) {
	key := fmt.Sprintf("%s%s", r.Prefix, chargePointID)
	val, err := r.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", redis.Nil // 明确返回 redis.Nil 错误
	}
	return val, err
}

// DeleteConnection 删除一个充电桩的连接信息
func (r *RedisStorage) DeleteConnection(ctx context.Context, chargePointID string) error {
	key := fmt.Sprintf("%s%s", r.Prefix, chargePointID)
	return r.Client.Del(ctx, key).Err()
}

// Close 关闭与存储后端的连接
func (r *RedisStorage) Close() error {
	return r.Client.Close()
}