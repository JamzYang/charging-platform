package cache

import (
	"sync"
	"time"
)

// SimpleLRUCache 简单的LRU缓存实现（无分片）
type SimpleLRUCache struct {
	items    map[string]*LRUNode
	lruList  *LRUList
	mutex    sync.RWMutex
	config   *CacheConfig
	stats    *CacheStats
}

// NewSimpleLRUCache 创建简单的LRU缓存
func NewSimpleLRUCache(config *CacheConfig) *SimpleLRUCache {
	if config == nil {
		config = DefaultCacheConfig()
	}
	
	return &SimpleLRUCache{
		items:   make(map[string]*LRUNode),
		lruList: NewLRUList(),
		config:  config,
		stats: &CacheStats{
			MaxSize:       int64(config.MaxSize),
			MemoryLimitMB: int64(config.MemoryLimitMB),
			CreatedAt:     time.Now(),
		},
	}
}

// Get 获取缓存项
func (c *SimpleLRUCache) Get(key string) (interface{}, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	node, exists := c.items[key]
	if !exists {
		return nil, false
	}
	
	// 检查是否过期
	if node.Item.IsExpired() {
		delete(c.items, key)
		c.lruList.RemoveNode(node)
		return nil, false
	}
	
	// 更新访问信息
	node.Item.UpdateAccess()
	c.lruList.MoveToHead(node)
	
	return node.Item.Value, true
}

// Set 设置缓存项
func (c *SimpleLRUCache) Set(key string, value interface{}, ttl time.Duration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	if ttl == 0 {
		ttl = c.config.DefaultTTL
	}
	
	now := time.Now()
	item := &CacheItem{
		Key:         key,
		Value:       value,
		CreatedAt:   now,
		AccessAt:    now,
		AccessCount: 1,
		Size:        c.estimateSize(value),
	}
	
	if ttl > 0 {
		item.ExpiresAt = now.Add(ttl)
	}
	
	// 检查是否已存在
	if existingNode, exists := c.items[key]; exists {
		existingNode.Item = item
		c.lruList.MoveToHead(existingNode)
		return nil
	}
	
	// 检查容量限制
	if len(c.items) >= c.config.MaxSize {
		// 淘汰最少使用的项目
		if c.lruList.Size() > 0 {
			node := c.lruList.RemoveTail()
			if node != nil {
				delete(c.items, node.Key)
			}
		}
	}
	
	// 创建新节点
	node := &LRUNode{
		Key:  key,
		Item: item,
	}
	
	c.items[key] = node
	c.lruList.AddToHead(node)
	
	return nil
}

// Delete 删除缓存项
func (c *SimpleLRUCache) Delete(key string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	node, exists := c.items[key]
	if !exists {
		return false
	}
	
	delete(c.items, key)
	c.lruList.RemoveNode(node)
	
	return true
}

// Clear 清空所有缓存
func (c *SimpleLRUCache) Clear() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.items = make(map[string]*LRUNode)
	c.lruList = NewLRUList()
	
	return nil
}

// GetBatch 批量获取
func (c *SimpleLRUCache) GetBatch(keys []string) map[string]interface{} {
	result := make(map[string]interface{})
	
	for _, key := range keys {
		if value, exists := c.Get(key); exists {
			result[key] = value
		}
	}
	
	return result
}

// SetBatch 批量设置
func (c *SimpleLRUCache) SetBatch(items map[string]CacheItem) error {
	for key, item := range items {
		ttl := time.Until(item.ExpiresAt)
		if ttl < 0 {
			ttl = c.config.DefaultTTL
		}
		
		if err := c.Set(key, item.Value, ttl); err != nil {
			return err
		}
	}
	
	return nil
}

// DeleteBatch 批量删除
func (c *SimpleLRUCache) DeleteBatch(keys []string) int {
	deleted := 0
	for _, key := range keys {
		if c.Delete(key) {
			deleted++
		}
	}
	return deleted
}

// Exists 检查key是否存在
func (c *SimpleLRUCache) Exists(key string) bool {
	_, exists := c.Get(key)
	return exists
}

// Keys 获取所有key
func (c *SimpleLRUCache) Keys() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	keys := make([]string, 0, len(c.items))
	for key := range c.items {
		keys = append(keys, key)
	}
	
	return keys
}

// Size 获取缓存项数量
func (c *SimpleLRUCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.items)
}

// GetStats 获取统计信息
func (c *SimpleLRUCache) GetStats() *CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	return &CacheStats{
		TotalItems:    int64(len(c.items)),
		TotalSize:     c.getMemoryUsage(),
		MaxSize:       c.stats.MaxSize,
		MemoryLimitMB: c.stats.MemoryLimitMB,
		CreatedAt:     c.stats.CreatedAt,
	}
}

// GetMemoryUsage 获取内存使用量(字节)
func (c *SimpleLRUCache) GetMemoryUsage() int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.getMemoryUsage()
}

// getMemoryUsage 内部获取内存使用量(需要持有锁)
func (c *SimpleLRUCache) getMemoryUsage() int64 {
	var totalSize int64
	for _, node := range c.items {
		totalSize += node.Item.Size
	}
	return totalSize
}

// EvictLRU 淘汰最近最少使用的项
func (c *SimpleLRUCache) EvictLRU(count int) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	evicted := 0
	for evicted < count && c.lruList.Size() > 0 {
		node := c.lruList.RemoveTail()
		if node != nil {
			delete(c.items, node.Key)
			evicted++
		}
	}
	
	return evicted
}

// EvictExpired 清理过期项
func (c *SimpleLRUCache) EvictExpired() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	expired := 0
	var expiredKeys []string
	
	for key, node := range c.items {
		if node.Item.IsExpired() {
			expiredKeys = append(expiredKeys, key)
		}
	}
	
	for _, key := range expiredKeys {
		if node, exists := c.items[key]; exists {
			delete(c.items, key)
			c.lruList.RemoveNode(node)
			expired++
		}
	}
	
	return expired
}

// Start 启动缓存管理器
func (c *SimpleLRUCache) Start() error {
	return nil
}

// Stop 停止缓存管理器
func (c *SimpleLRUCache) Stop() error {
	return nil
}

// IsRunning 检查是否正在运行
func (c *SimpleLRUCache) IsRunning() bool {
	return true
}

// estimateSize 估算对象大小
func (c *SimpleLRUCache) estimateSize(value interface{}) int64 {
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
		return 256
	}
}
