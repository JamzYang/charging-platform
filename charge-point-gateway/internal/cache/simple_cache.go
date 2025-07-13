package cache

import "sync"

// SimpleCache 是一个简单的内存缓存实现。
type SimpleCache struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

// NewSimpleCache 创建并返回一个新的 SimpleCache 实例。
func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		data: make(map[string]interface{}),
	}
}

// Get 根据键获取值。如果键不存在，返回 nil 和 false。
func (c *SimpleCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.data[key]
	return value, ok
}

// Set 设置键值对。
func (c *SimpleCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}
