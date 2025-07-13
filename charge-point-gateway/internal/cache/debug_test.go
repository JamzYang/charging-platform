package cache

/*
import (
	"testing"
	"time"
)

func TestSimpleSet(t *testing.T) {
	cache := NewLRUCache(DefaultCacheConfig())

	err := cache.Set("test", "value", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	t.Log("Set operation completed successfully")
}

func TestSimpleGet(t *testing.T) {
	cache := NewLRUCache(DefaultCacheConfig())

	// 先设置一个值
	err := cache.Set("test", "value", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// 然后获取
	value, exists := cache.Get("test")
	if !exists {
		t.Fatal("Get failed: key not found")
	}

	if value != "value" {
		t.Fatalf("Get failed: expected 'value', got %v", value)
	}

	t.Log("Get operation completed successfully")
}
*/
