package cache

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewLRUCache(t *testing.T) {
	config := DefaultCacheConfig()
	cache := NewLRUCache(config)

	assert.NotNil(t, cache)
	assert.Equal(t, config.ShardCount, len(cache.shards))
	assert.Equal(t, config, cache.config)
	assert.False(t, cache.IsRunning())
}

func TestLRUCache_BasicOperations(t *testing.T) {
	cache := NewLRUCache(DefaultCacheConfig())

	// 测试Set和Get
	err := cache.Set("key1", "value1", time.Hour)
	assert.NoError(t, err)

	value, exists := cache.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value)

	// 测试不存在的key
	value, exists = cache.Get("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, value)

	// 测试Delete
	deleted := cache.Delete("key1")
	assert.True(t, deleted)

	value, exists = cache.Get("key1")
	assert.False(t, exists)
	assert.Nil(t, value)

	// 测试删除不存在的key
	deleted = cache.Delete("nonexistent")
	assert.False(t, deleted)
}

func TestLRUCache_TTL(t *testing.T) {
	cache := NewLRUCache(DefaultCacheConfig())

	// 设置短TTL
	err := cache.Set("key1", "value1", 100*time.Millisecond)
	assert.NoError(t, err)

	// 立即获取应该成功
	value, exists := cache.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value)

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 过期后获取应该失败
	value, exists = cache.Get("key1")
	assert.False(t, exists)
	assert.Nil(t, value)
}

func TestLRUCache_LRUEviction(t *testing.T) {
	config := DefaultCacheConfig()
	config.MaxSize = 3
	config.EvictionBatch = 1
	cache := NewLRUCache(config)

	// 添加3个项目
	cache.Set("key1", "value1", time.Hour)
	cache.Set("key2", "value2", time.Hour)
	cache.Set("key3", "value3", time.Hour)

	assert.Equal(t, 3, cache.Size())

	// 访问key1，使其成为最近使用的
	cache.Get("key1")

	// 添加第4个项目，应该淘汰key2（最少使用的）
	cache.Set("key4", "value4", time.Hour)

	// key2应该被淘汰
	_, exists := cache.Get("key2")
	assert.False(t, exists)

	// 其他key应该还在
	_, exists = cache.Get("key1")
	assert.True(t, exists)
	_, exists = cache.Get("key3")
	assert.True(t, exists)
	_, exists = cache.Get("key4")
	assert.True(t, exists)
}

func TestLRUCache_BatchOperations(t *testing.T) {
	cache := NewLRUCache(DefaultCacheConfig())

	// 测试批量设置
	items := map[string]CacheItem{
		"key1": {Value: "value1", ExpiresAt: time.Now().Add(time.Hour)},
		"key2": {Value: "value2", ExpiresAt: time.Now().Add(time.Hour)},
		"key3": {Value: "value3", ExpiresAt: time.Now().Add(time.Hour)},
	}

	err := cache.SetBatch(items)
	assert.NoError(t, err)

	// 测试批量获取
	keys := []string{"key1", "key2", "key3", "nonexistent"}
	result := cache.GetBatch(keys)

	assert.Len(t, result, 3)
	assert.Equal(t, "value1", result["key1"])
	assert.Equal(t, "value2", result["key2"])
	assert.Equal(t, "value3", result["key3"])
	assert.NotContains(t, result, "nonexistent")

	// 测试批量删除
	deleteKeys := []string{"key1", "key3", "nonexistent"}
	deleted := cache.DeleteBatch(deleteKeys)
	assert.Equal(t, 2, deleted)

	// 验证删除结果
	_, exists := cache.Get("key1")
	assert.False(t, exists)
	_, exists = cache.Get("key2")
	assert.True(t, exists)
	_, exists = cache.Get("key3")
	assert.False(t, exists)
}

func TestLRUCache_Stats(t *testing.T) {
	cache := NewLRUCache(DefaultCacheConfig())

	// 执行一些操作
	cache.Set("key1", "value1", time.Hour)
	cache.Set("key2", "value2", time.Hour)

	cache.Get("key1")  // hit
	cache.Get("key3")  // miss

	cache.Delete("key2")

	stats := cache.GetStats()

	assert.Equal(t, int64(1), stats.TotalItems)
	assert.Equal(t, int64(2), stats.Sets)
	assert.Equal(t, int64(2), stats.Gets)
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(1), stats.Deletes)
	assert.Equal(t, 0.5, stats.HitRate)
}

func TestLRUCache_ConcurrentAccess(t *testing.T) {
	cache := NewLRUCache(DefaultCacheConfig())

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// 并发写入
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				cache.Set(key, value, time.Hour)
			}
		}(i)
	}

	// 并发读取
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// 验证没有数据竞争
	assert.True(t, cache.Size() > 0)
}

func TestLRUCache_MemoryLimit(t *testing.T) {
	config := DefaultCacheConfig()
	config.MemoryLimitMB = 1 // 1MB限制
	config.EvictionBatch = 10
	cache := NewLRUCache(config)

	// 添加大量数据直到触发内存限制
	largeValue := make([]byte, 1024) // 1KB
	for i := 0; i < 2000; i++ {
		key := "key_" + strconv.Itoa(i)
		cache.Set(key, largeValue, time.Hour)
	}

	// 验证内存使用没有超过限制太多
	memoryUsageMB := cache.GetMemoryUsage() / (1024 * 1024)
	assert.True(t, memoryUsageMB <= 2) // 允许一些误差
}

func TestLRUCache_Lifecycle(t *testing.T) {
	cache := NewLRUCache(DefaultCacheConfig())

	// 初始状态
	assert.False(t, cache.IsRunning())

	// 启动
	err := cache.Start()
	assert.NoError(t, err)
	assert.True(t, cache.IsRunning())

	// 重复启动应该失败
	err = cache.Start()
	assert.Error(t, err)

	// 停止
	err = cache.Stop()
	assert.NoError(t, err)
	assert.False(t, cache.IsRunning())

	// 重复停止应该失败
	err = cache.Stop()
	assert.Error(t, err)
}

func TestLRUCache_ExpiredCleanup(t *testing.T) {
	cache := NewLRUCache(DefaultCacheConfig())

	// 添加一些会过期的项目
	cache.Set("key1", "value1", 50*time.Millisecond)
	cache.Set("key2", "value2", 100*time.Millisecond)
	cache.Set("key3", "value3", time.Hour)

	assert.Equal(t, 3, cache.Size())

	// 等待部分过期
	time.Sleep(75 * time.Millisecond)

	// 手动清理过期项
	expired := cache.EvictExpired()
	assert.Equal(t, 1, expired) // key1应该过期
	assert.Equal(t, 2, cache.Size())

	// 等待更多过期
	time.Sleep(50 * time.Millisecond)

	expired = cache.EvictExpired()
	assert.Equal(t, 1, expired) // key2应该过期
	assert.Equal(t, 1, cache.Size())

	// key3应该还在
	_, exists := cache.Get("key3")
	assert.True(t, exists)
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(DefaultCacheConfig())

	// 添加一些数据
	cache.Set("key1", "value1", time.Hour)
	cache.Set("key2", "value2", time.Hour)
	cache.Set("key3", "value3", time.Hour)

	assert.Equal(t, 3, cache.Size())

	// 清空缓存
	err := cache.Clear()
	assert.NoError(t, err)
	assert.Equal(t, 0, cache.Size())

	// 验证所有数据都被清除
	_, exists := cache.Get("key1")
	assert.False(t, exists)
	_, exists = cache.Get("key2")
	assert.False(t, exists)
	_, exists = cache.Get("key3")
	assert.False(t, exists)
}
