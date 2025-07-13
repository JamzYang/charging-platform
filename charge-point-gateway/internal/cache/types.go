package cache

import (
	"sync"
	"time"
)

// Cache 是一个通用的缓存接口，支持存储任意类型的值。
type Cache interface {
	// Get 根据键获取值。如果键不存在，返回 nil 和 false。
	Get(key string) (interface{}, bool)
	// Set 设置键值对。
	Set(key string, value interface{})
}

// CacheConfig 定义了缓存的配置，例如容量。
type CacheConfig struct {
	Capacity        int
	ShardCount      int
	MaxSize         int64
	MemoryLimitMB   int64
	DefaultTTL      time.Duration
	CleanupInterval time.Duration
	EvictionBatch   int
	EnableMetrics   bool
}

// DefaultCacheConfig 返回默认的缓存配置。
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Capacity:        10000,
		ShardCount:      32,
		MaxSize:         100 * 1024 * 1024, // 100MB
		MemoryLimitMB:   100,               // 100MB
		DefaultTTL:      10 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		EvictionBatch:   100,
		EnableMetrics:   true,
	}
}

// CacheStats 存储缓存的统计信息。
type CacheStats struct {
	TotalItems    int64
	TotalSize     int64
	MaxSize       int64
	MemoryLimitMB int64
	Hits          int64
	Misses        int64
	Sets          int64
	Gets          int64
	Deletes       int64
	Evictions     int64
	Expirations   int64
	CreatedAt     string
	LastCleanup   time.Time
	AvgGetTime    time.Duration
	AvgSetTime    time.Duration
	HitRate       float64
}

// CacheItem 表示缓存中的一个条目。
type CacheItem struct {
	Key         string
	Value       interface{}
	Size        int64
	CreatedAt   time.Time
	AccessAt    time.Time
	ExpiresAt   time.Time
	AccessCount int64
}

// IsExpired 检查缓存项是否过期。
func (item *CacheItem) IsExpired() bool {
	return !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt)
}

// UpdateAccess 更新访问信息。
func (item *CacheItem) UpdateAccess() {
	item.AccessAt = time.Now()
	item.AccessCount++
}

// LRUNode 是双向链表中的一个节点。
type LRUNode struct {
	Key  string
	Item *CacheItem
	Prev *LRUNode
	Next *LRUNode
}

// LRUList 是一个双向链表，用于维护LRU顺序。
type LRUList struct {
	head *LRUNode
	tail *LRUNode
	size int
}

// NewLRUList 创建并返回一个新的LRUList。
func NewLRUList() *LRUList {
	return &LRUList{}
}

// AddToHead 将节点添加到链表头部。
func (l *LRUList) AddToHead(node *LRUNode) {
	node.Next = l.head
	node.Prev = nil
	if l.head != nil {
		l.head.Prev = node
	}
	l.head = node
	if l.tail == nil {
		l.tail = node
	}
	l.size++
}

// MoveToHead 将节点移动到链表头部。
func (l *LRUList) MoveToHead(node *LRUNode) {
	if node == l.head {
		return
	}
	l.RemoveNode(node)
	l.AddToHead(node)
}

// RemoveNode 从链表中移除节点。
func (l *LRUList) RemoveNode(node *LRUNode) {
	if node.Prev != nil {
		node.Prev.Next = node.Next
	} else {
		l.head = node.Next
	}
	if node.Next != nil {
		node.Next.Prev = node.Prev
	} else {
		l.tail = node.Prev
	}
	node.Next = nil
	node.Prev = nil
	l.size--
}

// RemoveTail 移除链表尾部节点。
func (l *LRUList) RemoveTail() *LRUNode {
	if l.tail == nil {
		return nil
	}
	node := l.tail
	l.RemoveNode(node)
	return node
}

// Size 返回链表大小。
func (l *LRUList) Size() int {
	return l.size
}

// CacheShard LRU缓存的分片
type CacheShard struct {
	items   map[string]*LRUNode
	lruList *LRUList
	mutex   sync.RWMutex
	config  *CacheConfig
}

// NewCacheShard 创建新的缓存分片
func NewCacheShard(config *CacheConfig) *CacheShard {
	return &CacheShard{
		items:   make(map[string]*LRUNode),
		lruList: NewLRUList(),
		config:  config,
	}
}

// Add 添加缓存项
func (s *CacheShard) Add(key string, value interface{}, ttl time.Duration) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	item := &CacheItem{
		Key:         key,
		Value:       value,
		CreatedAt:   now,
		AccessAt:    now,
		AccessCount: 1,
		Size:        s.estimateSize(value),
	}

	if ttl > 0 {
		item.ExpiresAt = now.Add(ttl)
	}

	if existingNode, exists := s.items[key]; exists {
		existingNode.Item = item
		s.lruList.MoveToHead(existingNode)
		return nil
	}

	node := &LRUNode{
		Key:  key,
		Item: item,
	}

	s.items[key] = node
	s.lruList.AddToHead(node)

	return nil
}

// Get 获取缓存项
func (s *CacheShard) Get(key string) (interface{}, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	node, exists := s.items[key]
	if !exists {
		return nil, false
	}

	if node.Item.IsExpired() {
		s.mutex.RUnlock() // 临时释放读锁以便获取写锁
		s.mutex.Lock()
		delete(s.items, key)
		s.lruList.RemoveNode(node)
		s.mutex.Unlock()
		s.mutex.RLock() // 重新获取读锁
		return nil, false
	}

	s.lruList.MoveToHead(node)
	node.Item.UpdateAccess()
	return node.Item.Value, true
}

// Remove 删除缓存项
func (s *CacheShard) Remove(key string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if node, exists := s.items[key]; exists {
		delete(s.items, key)
		s.lruList.RemoveNode(node)
		return true
	}
	return false
}

// Len 返回分片中的项目数量
func (s *CacheShard) Len() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.items)
}

// estimateSize 估算对象大小
func (s *CacheShard) estimateSize(value interface{}) int64 {
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
