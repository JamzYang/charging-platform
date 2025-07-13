package cache

import (
	"sync"
	"time"
)

// CacheItem 缓存项
type CacheItem struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value"`
	ExpiresAt time.Time   `json:"expires_at"`
	CreatedAt time.Time   `json:"created_at"`
	AccessAt  time.Time   `json:"access_at"`
	AccessCount int64     `json:"access_count"`
	Size      int64       `json:"size"` // 估算的内存大小(字节)
}

// IsExpired 检查是否过期
func (item *CacheItem) IsExpired() bool {
	if item.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(item.ExpiresAt)
}

// UpdateAccess 更新访问信息
func (item *CacheItem) UpdateAccess() {
	item.AccessAt = time.Now()
	item.AccessCount++
}

// CacheStats 缓存统计信息
type CacheStats struct {
	// 基本统计
	TotalItems    int64 `json:"total_items"`
	TotalSize     int64 `json:"total_size"`     // 总内存使用(字节)
	MaxSize       int64 `json:"max_size"`       // 最大容量
	MemoryLimitMB int64 `json:"memory_limit_mb"` // 内存限制(MB)
	
	// 命中率统计
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	HitRate     float64 `json:"hit_rate"`
	
	// 操作统计
	Sets        int64 `json:"sets"`
	Gets        int64 `json:"gets"`
	Deletes     int64 `json:"deletes"`
	Evictions   int64 `json:"evictions"`   // 淘汰次数
	Expirations int64 `json:"expirations"` // 过期清理次数
	
	// 时间统计
	CreatedAt   time.Time `json:"created_at"`
	LastCleanup time.Time `json:"last_cleanup"`
	
	// 性能统计
	AvgGetTime time.Duration `json:"avg_get_time"`
	AvgSetTime time.Duration `json:"avg_set_time"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	// 容量配置
	MaxSize         int           `json:"max_size"`          // 最大条目数
	MemoryLimitMB   int           `json:"memory_limit_mb"`   // 内存限制(MB)
	
	// TTL配置
	DefaultTTL      time.Duration `json:"default_ttl"`       // 默认TTL
	CleanupInterval time.Duration `json:"cleanup_interval"`  // 清理间隔
	
	// 性能配置
	ShardCount      int           `json:"shard_count"`       // 分片数量(减少锁竞争)
	EnableMetrics   bool          `json:"enable_metrics"`    // 是否启用指标收集
	
	// 淘汰策略配置
	EvictionPolicy  string        `json:"eviction_policy"`   // 淘汰策略: lru, lfu, random
	EvictionBatch   int           `json:"eviction_batch"`    // 批量淘汰数量
}

// DefaultCacheConfig 默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		MaxSize:         10000,
		MemoryLimitMB:   512,
		DefaultTTL:      time.Hour,
		CleanupInterval: 10 * time.Minute,
		ShardCount:      16,
		EnableMetrics:   true,
		EvictionPolicy:  "lru",
		EvictionBatch:   100,
	}
}

// CacheManager 缓存管理器接口
type CacheManager interface {
	// 基本操作
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration) error
	Delete(key string) bool
	Clear() error
	
	// 批量操作
	GetBatch(keys []string) map[string]interface{}
	SetBatch(items map[string]CacheItem) error
	DeleteBatch(keys []string) int
	
	// 查询操作
	Exists(key string) bool
	Keys() []string
	Size() int
	
	// 统计和监控
	GetStats() *CacheStats
	GetMemoryUsage() int64
	
	// 容量控制
	EvictLRU(count int) int
	EvictExpired() int
	
	// 生命周期
	Start() error
	Stop() error
	IsRunning() bool
}

// LRUNode LRU链表节点
type LRUNode struct {
	Key   string
	Item  *CacheItem
	Prev  *LRUNode
	Next  *LRUNode
}

// LRUList LRU双向链表
type LRUList struct {
	head *LRUNode
	tail *LRUNode
	size int
}

// NewLRUList 创建新的LRU链表
func NewLRUList() *LRUList {
	head := &LRUNode{}
	tail := &LRUNode{}
	head.Next = tail
	tail.Prev = head
	
	return &LRUList{
		head: head,
		tail: tail,
		size: 0,
	}
}

// AddToHead 添加节点到头部
func (l *LRUList) AddToHead(node *LRUNode) {
	node.Prev = l.head
	node.Next = l.head.Next
	l.head.Next.Prev = node
	l.head.Next = node
	l.size++
}

// RemoveNode 移除节点
func (l *LRUList) RemoveNode(node *LRUNode) {
	node.Prev.Next = node.Next
	node.Next.Prev = node.Prev
	l.size--
}

// RemoveTail 移除尾部节点
func (l *LRUList) RemoveTail() *LRUNode {
	if l.size == 0 {
		return nil
	}

	lastNode := l.tail.Prev
	lastNode.Prev.Next = l.tail
	l.tail.Prev = lastNode.Prev
	l.size--

	return lastNode
}

// MoveToHead 移动节点到头部
func (l *LRUList) MoveToHead(node *LRUNode) {
	// 先从当前位置移除
	node.Prev.Next = node.Next
	node.Next.Prev = node.Prev

	// 然后添加到头部
	node.Prev = l.head
	node.Next = l.head.Next
	l.head.Next.Prev = node
	l.head.Next = node
}

// Size 获取链表大小
func (l *LRUList) Size() int {
	return l.size
}

// CacheShard 缓存分片
type CacheShard struct {
	items    map[string]*LRUNode
	lruList  *LRUList
	mutex    sync.RWMutex
	stats    *CacheStats
	config   *CacheConfig
}

// NewCacheShard 创建新的缓存分片
func NewCacheShard(config *CacheConfig) *CacheShard {
	return &CacheShard{
		items:   make(map[string]*LRUNode),
		lruList: NewLRUList(),
		stats: &CacheStats{
			CreatedAt: time.Now(),
		},
		config: config,
	}
}
