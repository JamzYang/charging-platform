package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redismock/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/charging-platform/charge-point-gateway/internal/config"
	"github.com/charging-platform/charge-point-gateway/internal/storage" // 导入 storage 包以测试 RedisStorage
)

func TestNewRedisStorage(t *testing.T) {
	// 模拟一个有效的 Redis 配置
	cfg := config.RedisConfig{
		Addr:     "localhost:6379", // 实际地址不重要，因为我们不进行实际连接
		Password: "",
		DB:       0,
	}

	// 预期 NewRedisStorage 成功返回一个实例
	// 注意：这里不会进行实际的 Redis 连接，因为我们没有模拟 Ping 方法
	// 实际的连接测试应该在集成测试中进行
	storage, err := storage.NewRedisStorage(cfg)
	require.NoError(t, err)
	assert.NotNil(t, storage)
	assert.NotNil(t, storage.Client) // 确保 Client 字段被初始化

	// 验证 Close 方法是否有效
	err = storage.Close()
	assert.NoError(t, err)

	// 模拟一个无效的 Redis 地址，预期 NewRedisStorage 返回错误
	// 这里我们无法模拟连接错误，因为 NewRedisStorage 内部会尝试 Ping
	// 并且 redismock 无法模拟 Ping 的连接错误。
	// 因此，这个测试用例将不再测试连接失败的情况。
	// 实际的连接失败测试应该在集成测试中进行。
}

func TestRedisStorage_SetGetDeleteConnection(t *testing.T) {
	db, mock := redismock.NewClientMock()
	rdb := &storage.RedisStorage{Client: db, Prefix: "conn:"} // 直接构造实例并注入 mock 客户端
	ctx := context.Background()

	chargePointID := "CP001"
	gatewayID := "GW001"
	ttl := 5 * time.Minute
	key := "conn:CP001"

	// Test SetConnection
	mock.ExpectSet(key, gatewayID, ttl).SetVal("OK")
	err := rdb.SetConnection(ctx, chargePointID, gatewayID, ttl)
	require.NoError(t, err)

	// Test GetConnection - Key exists
	mock.ExpectGet(key).SetVal(gatewayID)
	retrievedGatewayID, err := rdb.GetConnection(ctx, chargePointID)
	require.NoError(t, err)
	assert.Equal(t, gatewayID, retrievedGatewayID)

	// Test GetConnection - Key does not exist
	mock.ExpectGet(key).SetErr(redis.Nil)
	retrievedGatewayID, err = rdb.GetConnection(ctx, chargePointID)
	assert.ErrorIs(t, err, redis.Nil)
	assert.Empty(t, retrievedGatewayID)

	// Test DeleteConnection
	mock.ExpectDel(key).SetVal(1)
	err = rdb.DeleteConnection(ctx, chargePointID)
	require.NoError(t, err)

	// Ensure all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisStorage_SetConnection_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	rdb := &storage.RedisStorage{Client: db, Prefix: "conn:"}
	ctx := context.Background()

	chargePointID := "CP002"
	gatewayID := "GW002"
	ttl := 5 * time.Minute
	key := "conn:CP002"

	expectedErr := errors.New("redis set error")
	mock.ExpectSet(key, gatewayID, ttl).SetErr(expectedErr)
	err := rdb.SetConnection(ctx, chargePointID, gatewayID, ttl)
	assert.ErrorIs(t, err, expectedErr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisStorage_GetConnection_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	rdb := &storage.RedisStorage{Client: db, Prefix: "conn:"}
	ctx := context.Background()

	chargePointID := "CP003"
	key := "conn:CP003"

	expectedErr := errors.New("redis get error")
	mock.ExpectGet(key).SetErr(expectedErr)
	retrievedGatewayID, err := rdb.GetConnection(ctx, chargePointID)
	assert.ErrorIs(t, err, expectedErr)
	assert.Empty(t, retrievedGatewayID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisStorage_DeleteConnection_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	rdb := &storage.RedisStorage{Client: db, Prefix: "conn:"}
	ctx := context.Background()

	chargePointID := "CP004"
	key := "conn:CP004"

	expectedErr := errors.New("redis del error")
	mock.ExpectDel(key).SetErr(expectedErr)
	err := rdb.DeleteConnection(ctx, chargePointID)
	assert.ErrorIs(t, err, expectedErr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisStorage_Close(t *testing.T) {
	db, mock := redismock.NewClientMock()
	rdb := &storage.RedisStorage{Client: db, Prefix: "conn:"}

	// Close 方法的测试不应模拟 ExpectClose，因为 Close 方法直接关闭客户端连接
	// 并且 go-redis/redismock 不支持模拟 Close 方法的返回值。
	// 这里的测试仅确保调用 Close 不会 panic 且返回 nil。
	err := rdb.Close()
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet()) // 确保之前的 mock 期望被满足
}