package cache

import (
	"fmt"
	"hash/fnv"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// LRUCache LRU缓存实现
type LRUCache struct {
	shards   []*CacheShard
	config   *CacheConfig
	stats    *CacheStats
	running  int32
	stopCh   chan struct{}
	wg       sync.WaitGroup
	
	// 全局统计
	globalStats struct {
		hits        int64
		misses      int64
		sets        int64
		gets        int64
		deletes     int64
		evictions   int64
		expirations int64
	}
}

// NewLRUCache 创建新的LRU缓存
func NewLRUCache(config *CacheConfig) *LRUCache {
	if config == nil {
		config = DefaultCacheConfig()
	}
	
	cache := &LRUCache{
		shards: make([]*CacheShard, config.ShardCount),
		config: config,
		stats: &CacheStats{
			MaxSize:       int64(config.MaxSize),
			MemoryLimitMB: int64(config.MemoryLimitMB),
			CreatedAt:     time.Now().Format(time.RFC3339), // 格式化时间
		},
		stopCh: make(chan struct{}),
	}
	
	// 初始化分片
	for i := 0; i < config.ShardCount; i++ {
		cache.shards[i] = NewCacheShard(config)
	}
	
	return cache
}

// getShard 根据key获取对应的分片
func (c *LRUCache) getShard(key string) *CacheShard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return c.shards[h.Sum32()%uint32(c.config.ShardCount)]
}

// Get 获取缓存项
func (c *LRUCache) Get(key string) (interface{}, bool) {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&c.globalStats.gets, 1)
		if c.config.EnableMetrics {
			// 更新平均获取时间
			c.updateAvgGetTime(time.Since(start))
		}
	}()
	
	shard := c.getShard(key)
	value, exists := shard.Get(key)
	if !exists {
		atomic.AddInt64(&c.globalStats.misses, 1)
		return nil, false
	}
	
	atomic.AddInt64(&c.globalStats.hits, 1)
	return value, true
}

// Set 设置缓存项
func (c *LRUCache) Set(key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&c.globalStats.sets, 1)
		if c.config.EnableMetrics {
			c.updateAvgSetTime(time.Since(start))
		}
	}()
	
	shard := c.getShard(key)
	err := shard.Add(key, value, ttl)
	if err != nil {
		return err
	}

	// 在添加新项后，检查并执行全局容量限制淘汰
	for int64(c.Size()) > c.config.MaxSize {
		evictedCount := c.EvictLRU(c.config.EvictionBatch)
		if evictedCount == 0 {
			// 如果无法淘汰更多项目，则退出循环以避免死循环
			break
		}
	}
	return nil
}

// Delete 删除缓存项
func (c *LRUCache) Delete(key string) bool {
	defer func() {
		atomic.AddInt64(&c.globalStats.deletes, 1)
	}()
	
	shard := c.getShard(key)
	return shard.Remove(key)
}

// Clear 清空所有缓存
func (c *LRUCache) Clear() error {
	for _, shard := range c.shards {
		shard.mutex.Lock()
		shard.items = make(map[string]*LRUNode)
		shard.lruList = NewLRUList()
		shard.mutex.Unlock()
	}
	
	// 重置统计
	atomic.StoreInt64(&c.globalStats.hits, 0)
	atomic.StoreInt64(&c.globalStats.misses, 0)
	atomic.StoreInt64(&c.globalStats.sets, 0)
	atomic.StoreInt64(&c.globalStats.gets, 0)
	atomic.StoreInt64(&c.globalStats.deletes, 0)
	atomic.StoreInt64(&c.globalStats.evictions, 0)
	atomic.StoreInt64(&c.globalStats.expirations, 0)
	
	return nil
}

// GetBatch 批量获取
func (c *LRUCache) GetBatch(keys []string) map[string]interface{} {
	result := make(map[string]interface{})
	
	for _, key := range keys {
		if value, exists := c.Get(key); exists {
			result[key] = value
		}
	}
	
	return result
}

// SetBatch 批量设置
func (c *LRUCache) SetBatch(items map[string]CacheItem) error {
	for key, item := range items {
		ttl := time.Until(item.ExpiresAt)
		if ttl < 0 {
			ttl = c.config.DefaultTTL
		}
		
		if err := c.Set(key, item.Value, ttl); err != nil {
			return fmt.Errorf("failed to set key %s: %w", key, err)
		}
	}
	
	return nil
}

// DeleteBatch 批量删除
func (c *LRUCache) DeleteBatch(keys []string) int {
	deleted := 0
	for _, key := range keys {
		if c.Delete(key) {
			deleted++
		}
	}
	return deleted
}

// Exists 检查key是否存在
func (c *LRUCache) Exists(key string) bool {
	_, exists := c.Get(key)
	return exists
}

// Keys 获取所有key
func (c *LRUCache) Keys() []string {
	var keys []string
	
	for _, shard := range c.shards {
		shard.mutex.RLock()
		for key := range shard.items {
			keys = append(keys, key)
		}
		shard.mutex.RUnlock()
	}
	
	return keys
}

// Size 获取缓存项数量
func (c *LRUCache) Size() int {
	total := 0
	for _, shard := range c.shards {
		shard.mutex.RLock()
		total += len(shard.items)
		shard.mutex.RUnlock()
	}
	return total
}

// GetStats 获取统计信息
func (c *LRUCache) GetStats() *CacheStats {
	stats := &CacheStats{
		TotalItems:    int64(c.Size()),
		TotalSize:     c.GetMemoryUsage(),
		MaxSize:       c.stats.MaxSize,
		MemoryLimitMB: c.stats.MemoryLimitMB,
		Hits:          atomic.LoadInt64(&c.globalStats.hits),
		Misses:        atomic.LoadInt64(&c.globalStats.misses),
		Sets:          atomic.LoadInt64(&c.globalStats.sets),
		Gets:          atomic.LoadInt64(&c.globalStats.gets),
		Deletes:       atomic.LoadInt64(&c.globalStats.deletes),
		Evictions:     atomic.LoadInt64(&c.globalStats.evictions),
		Expirations:   atomic.LoadInt64(&c.globalStats.expirations),
		CreatedAt:     c.stats.CreatedAt,
		LastCleanup:   c.stats.LastCleanup,
		AvgGetTime:    c.stats.AvgGetTime,
		AvgSetTime:    c.stats.AvgSetTime,
	}
	
	// 计算命中率
	totalRequests := stats.Hits + stats.Misses
	if totalRequests > 0 {
		stats.HitRate = float64(stats.Hits) / float64(totalRequests)
	}
	
	return stats
}

// GetMemoryUsage 获取内存使用量(字节)
func (c *LRUCache) GetMemoryUsage() int64 {
	var totalSize int64

	for _, shard := range c.shards {
		shard.mutex.RLock()
		for _, node := range shard.items {
			totalSize += node.Item.Size
		}
		shard.mutex.RUnlock()
	}

	return totalSize
}

// EvictLRU 淘汰最近最少使用的项
func (c *LRUCache) EvictLRU(count int) int {
	evicted := 0

	// 计算每个分片需要淘汰的数量
	shardEvictCount := count / len(c.shards)
	if shardEvictCount == 0 {
		shardEvictCount = 1 // 至少淘汰一个
	}

	for _, shard := range c.shards {
		shard.mutex.Lock()
		for i := 0; i < shardEvictCount && shard.lruList.Size() > 0; i++ {
			node := shard.lruList.RemoveTail()
			if node != nil {
				delete(shard.items, node.Key)
				evicted++
				atomic.AddInt64(&c.globalStats.evictions, 1)
			}
		}
		shard.mutex.Unlock()
	}

	return evicted
}

// EvictExpired 清理过期项
func (c *LRUCache) EvictExpired() int {
	expired := 0
	now := time.Now()

	for _, shard := range c.shards {
		shard.mutex.Lock()

		var expiredKeys []string
		for key, node := range shard.items {
			if node.Item.IsExpired() {
				expiredKeys = append(expiredKeys, key)
			}
		}

		for _, key := range expiredKeys {
			if node, exists := shard.items[key]; exists {
				delete(shard.items, key)
				shard.lruList.RemoveNode(node)
				expired++
				atomic.AddInt64(&c.globalStats.expirations, 1)
			}
		}

		shard.mutex.Unlock()
	}

	c.stats.LastCleanup = now
	return expired
}

// Start 启动缓存管理器
func (c *LRUCache) Start() error {
	if !atomic.CompareAndSwapInt32(&c.running, 0, 1) {
		return fmt.Errorf("cache is already running")
	}

	// 启动清理协程
	c.wg.Add(1)
	go c.cleanupWorker()

	return nil
}

// Stop 停止缓存管理器
func (c *LRUCache) Stop() error {
	if !atomic.CompareAndSwapInt32(&c.running, 1, 0) {
		return fmt.Errorf("cache is not running")
	}

	close(c.stopCh)
	c.wg.Wait()

	return nil
}

// IsRunning 检查是否正在运行
func (c *LRUCache) IsRunning() bool {
	return atomic.LoadInt32(&c.running) == 1
}

// checkCapacityLimits 检查容量限制
func (c *LRUCache) checkCapacityLimits(shard *CacheShard) error {
	// 检查项目数量限制
	if int64(c.Size()) >= c.config.MaxSize {
		// 淘汰一些项目
		evicted := c.EvictLRU(c.config.EvictionBatch)
		if evicted == 0 {
			return fmt.Errorf("cache is full and cannot evict items")
		}
	}

	// 检查内存限制
	memoryUsageMB := c.GetMemoryUsage() / (1024 * 1024)
	if memoryUsageMB >= int64(c.config.MemoryLimitMB) {
		// 淘汰一些项目释放内存
		evicted := c.EvictLRU(c.config.EvictionBatch)
		if evicted == 0 {
			return fmt.Errorf("cache memory limit exceeded and cannot evict items")
		}
	}

	return nil
}

// estimateSize 估算对象大小
func (c *LRUCache) estimateSize(value interface{}) int64 {
	// 简单的大小估算，实际项目中可以使用更精确的方法
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int, int32, int64, float32, float64:
		return 8
	case bool:
		return 1
	default:
		// 对于复杂对象，使用固定估算值
		return 256
	}
}

// updateAvgGetTime 更新平均获取时间
func (c *LRUCache) updateAvgGetTime(duration time.Duration) {
	// 简单的移动平均
	if c.stats.AvgGetTime == 0 {
		c.stats.AvgGetTime = duration
	} else {
		c.stats.AvgGetTime = (c.stats.AvgGetTime + duration) / 2
	}
}

// updateAvgSetTime 更新平均设置时间
func (c *LRUCache) updateAvgSetTime(duration time.Duration) {
	// 简单的移动平均
	if c.stats.AvgSetTime == 0 {
		c.stats.AvgSetTime = duration
	} else {
		c.stats.AvgSetTime = (c.stats.AvgSetTime + duration) / 2
	}
}

// cleanupWorker 清理工作协程
func (c *LRUCache) cleanupWorker() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 清理过期项
			expired := c.EvictExpired()
			if expired > 0 {
				// 可以添加日志记录
			}

			// 检查内存压力
			c.checkMemoryPressure()

		case <-c.stopCh:
			return
		}
	}
}

// checkMemoryPressure 检查内存压力
func (c *LRUCache) checkMemoryPressure() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 如果系统内存使用率过高，主动清理一些缓存
	memoryUsageMB := c.GetMemoryUsage() / (1024 * 1024)
	if memoryUsageMB > int64(c.config.MemoryLimitMB)*8/10 { // 80%阈值
		// 清理20%的缓存
		evictCount := c.Size() / 5
		if evictCount > 0 {
			c.EvictLRU(evictCount)
		}
	}
}
