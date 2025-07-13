package cache_test

import (
	"sync"
	"testing"

	"github.com/charging-platform/charge-point-gateway/internal/cache"
	"github.com/stretchr/testify/assert"
)

func TestSimpleCache_SetAndGet(t *testing.T) {
	c := cache.NewSimpleCache() // 假设存在 NewSimpleCache 函数
	key := "testKey"
	value := "testValue"

	c.Set(key, value)

	retrievedValue, ok := c.Get(key)
	assert.True(t, ok, "Expected key to be found")
	assert.Equal(t, value, retrievedValue, "Expected retrieved value to match original value")

	// Test non-existent key
	_, ok = c.Get("nonExistentKey")
	assert.False(t, ok, "Expected non-existent key not to be found")
}

func TestSimpleCache_Concurrency(t *testing.T) {
	c := cache.NewSimpleCache()
	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 1000

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "key" + string(rune(j))
				value := "value" + string(rune(j))
				c.Set(key, value)
				_, _ = c.Get(key)
			}
		}(i)
	}
	wg.Wait()

	// Verify some values after concurrent operations
	for j := 0; j < numOperations; j++ {
		key := "key" + string(rune(j))
		_, ok := c.Get(key)
		assert.True(t, ok, "Expected key %s to be found after concurrency test", key)
	}
}
